/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"sync"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

/*
ConflictDetectionCache is a thread-safe class used to store and manage conflicting events during migration's streaming phase.
Conflict occurs when two events have the same unique key column value.
For example, if we have a table with a unique key column "email" with a existing row: {id: 1, email: 'abc@example.com'},
and two new events comes in:
event1: DELETE FROM users WHERE id = 1;
event2: INSERT INTO users (id, email) VALUES (2, 'abc@example.com');

In this case, event1 and event2 are considered as a conflicting events, because they have the same unique key column value.

In a concurrent environment we can't just apply the second event because both the events can be part of different parallel batches
and we can't guarantee the order of the events in the batches.

So, this cache stores events like event1 and wait for them to be processed before processing event2.

There can be total 4 types of conflicts:
1. DELETE-INSERT
2. DELETE-UPDATE
3. UPDATE-INSERT
4. UPDATE-UPDATE

Case: UPDATE-INSERT conflict:

	example_table (id PK, email UNIQUE)

// Insert initial rows
INSERT INTO example_table VALUES (1, 'user21@example.com');
INSERT INTO example_table VALUES (2, 'user22@example.com');
INSERT INTO example_table VALUES (3, 'user23@example.com');
INSERT INTO example_table VALUES (4, 'user24@example.com');

UPDATE example_table SET email = 'user224@example.com' WHERE id = 4;

-- Insert a new row with the conflicting email
INSERT INTO example_table VALUES (5, 'user24@example.com');

Case: UPDATE-UPDATE conflict:

	example_table (id PK, email UNIQUE)

// Insert initial rows
INSERT INTO example_table VALUES (1, 'user31@example.com');
INSERT INTO example_table VALUES (2, 'user32@example.com');
INSERT INTO example_table VALUES (3, 'user33@example.com');
INSERT INTO example_table VALUES (4, 'user34@example.com');

UPDATE example_table SET email = 'updated_user2@example.com' WHERE id = 2;

-- Another conflicting update for id = 3, setting it to previous value of id = 2
UPDATE example_table SET email = 'user32@example.com' WHERE id = 3;

Case: DELETE-UPDATE conflict:

	example_table (id PK, email UNIQUE)

// Insert initial rows
INSERT INTO example_table VALUES (1, 'user41@example.com');
INSERT INTO example_table VALUES (2, 'user42@example.com');
INSERT INTO example_table VALUES (3, 'user43@example.com');
INSERT INTO example_table VALUES (4, 'user44@example.com');

DELETE FROM example_table WHERE id = 2;

-- Another conflicting update for id = 3, setting it to previous value of id = 2
UPDATE example_table SET email = 'user42@example.com' WHERE id = 3;
*/
type ConflictDetectionCache struct {
	sync.Mutex
	/*
		m caches separate copy of events not pointer, otherwise it will be modified by ConvertEvent() causing issue in events comparison for conflict detection
		ConvertEvent() in some case modifies schemaName, tableName and before after values
	*/
	m                       map[int64]*tgtdb.Event
	cond                    *sync.Cond
	tableToUniqueKeyColumns *utils.StructMap[sqlname.NameTuple, []string]
	evChans                 []chan *tgtdb.Event
	sourceDBType            string
}

func NewConflictDetectionCache(tableToUniqueKeyColumns *utils.StructMap[sqlname.NameTuple, []string], evChans []chan *tgtdb.Event, sourceDBType string) *ConflictDetectionCache {
	c := &ConflictDetectionCache{}
	c.m = make(map[int64]*tgtdb.Event)
	c.cond = sync.NewCond(&c.Mutex)
	c.tableToUniqueKeyColumns = tableToUniqueKeyColumns
	c.sourceDBType = sourceDBType
	c.evChans = evChans
	return c
}

func (c *ConflictDetectionCache) Put(event *tgtdb.Event) {
	c.Lock()
	defer c.Unlock()
	c.m[event.Vsn] = event.Copy()
	log.Infof("adding event vsn(%d) to conflict cache", event.Vsn)
}

func (c *ConflictDetectionCache) WaitUntilNoConflict(incomingEvent *tgtdb.Event) {
	c.Lock()
	defer c.Unlock()

retry:
	for _, cachedEvent := range c.m {
		if c.eventsConfict(cachedEvent, incomingEvent) {
			// flushing all the batches in channels instead of waiting for MAX_INTERVAL_BETWEEN_BATCHES
			for i := 0; i < NUM_EVENT_CHANNELS; i++ {
				c.evChans[i] <- FLUSH_BATCH_EVENT
			}
			log.Infof("waiting for event(vsn=%d) to be complete before processing event(vsn=%d)", cachedEvent.Vsn, incomingEvent.Vsn)
			// wait will release the lock and wait for a broadcast signal
			c.cond.Wait()

			// can't return after just one conflict, incoming event can have multiple conflicts
			// for example: table with 3 unique key columns conflicting with 3 different events
			goto retry
		}
	}
}

func (c *ConflictDetectionCache) RemoveEvents(batch *tgtdb.EventBatch) {
	c.Lock()
	defer c.Unlock()
	eventsRemoved := false

	for _, event := range batch.Events {
		if _, ok := c.m[event.Vsn]; ok {
			delete(c.m, event.Vsn)
			eventsRemoved = true
		}
	}

	// if we removed any event then broadcast to all waiting threads to check for conflicts again
	if eventsRemoved {
		c.cond.Broadcast()
	}
}

func (c *ConflictDetectionCache) eventsConfict(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event) bool {
	if !c.eventsAreOfSameTable(cachedEvent, incomingEvent) {
		return false
	}

	uniqueKeyColumns, _ := c.tableToUniqueKeyColumns.Get(cachedEvent.TableNameTup)
	/*
		Not checking for value of unique key values conflict in case of export from yb because of inconsistency issues in before values of events provided by yb-cdc
		TODO(future): Fix this in our debezium voyager plugin

		For now, we just check if the event is from same table then we consider it as a conflict
	*/
	if isTargetDBExporter(incomingEvent.ExporterRole) {
		conflict := false
		if cachedEvent.Op == "d" {
			conflict = true
		} else if cachedEvent.Op == "u" {
			// if both events are dealing with the same unique key columns then we consider it as a conflict
			cachedEventCols := lo.Keys(cachedEvent.Fields)
			incomingEventCols := lo.Keys(incomingEvent.Fields)
			ukList := lo.Intersect(cachedEventCols, uniqueKeyColumns)
			if lo.Some(incomingEventCols, ukList) {
				conflict = true
			}
		}

		if conflict {
			log.Infof("conflict detected for table %s, between event1(vsn=%d) and event2(vsn=%d)", cachedEvent.TableNameTup, cachedEvent.Vsn, incomingEvent.Vsn)
		}
		return conflict
	}

	for _, column := range uniqueKeyColumns {
		if cachedEvent.BeforeFields[column] == nil || incomingEvent.Fields[column] == nil {
			continue // check for the other columns(case: multiple unique keys)
		}

		if *cachedEvent.BeforeFields[column] == *incomingEvent.Fields[column] {
			log.Infof("conflict detected for table %s, column %s, between value of event1(vsn=%d, colVal=%s) and event2(vsn=%d, colVal=%s)",
				cachedEvent.TableNameTup.ForKey(), column, cachedEvent.Vsn, *cachedEvent.BeforeFields[column], incomingEvent.Vsn, *incomingEvent.Fields[column])
			return true
		}
	}
	return false
}

func (c *ConflictDetectionCache) eventsAreOfSameTable(event1 *tgtdb.Event, event2 *tgtdb.Event) bool {
	return event1.TableNameTup.Equals(event2.TableNameTup)
}
