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

During live migration, we create N different parallel channels via which events are batched and applied
on the target database. Hash(event.PK) % N decides which channel to use for the event.
Given that event1 and event2 will have different PKs, they can be part of different channels and can be processed in parallel.
This can be problematic because event2 can be applied before event1 and can cause a unique constraint error.
ConflictDetectionCache aims to solve this problem by making sure that conflicting events are processed in order.
i.e event2 will be processed only after event1 is processed because they share the same unique key column value.

It might seem like simply retrying can solve the problem.
I.e, if we retry the event2 enough times, after event1 is processed, it will be applied eventually.
However consider this case:
event1: DELETE FROM users WHERE id = 1;
event2: INSERT INTO users (id, email) VALUES (2, 'abc@example.com');
event3: DELETE FROM users WHERE id = 2;
event4: INSERT INTO users (id, email) VALUES (3, 'abc@example.com');

1. event1 is being processed in channel 1
2. event2 is being processed in channel 2
3. event2 is applied before event1, failing with unique constraint error, and is retried after a sleep of 10s.
4. event4 is being processed in channel 3
5. event1 is applied successfully.
6. event4 is applied successfully.
7. event2 is retried but still fails (because now event4 is already applied).

Here, event2 will continue to fail even after multiple retries because event4 is already applied.
--------------------------------------

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

		Worst event size can be 7kb for 30-50 columns in the table 
		so for the 500000 events (100 channels * 500 events per channel) at worst in the cache it will be 500000 * 7kb = 3.5GB
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
				// non-blocking send because blocking send can cause deadlock
				// between main goroutine acquiring lock and blocking on sending to channel
				// and processEvents goroutine waiting to acquire lock in RemoveEvents.
				select {
				case c.evChans[i] <- FLUSH_BATCH_EVENT:
				default:
					// channel is full, so it's okay not to send FLUSH_BATCH_EVENT
					// because MAX_EVENTS_PER_BATCH would likely be reached in the next batch.
					log.Infof("channel %d is full with size %d, not sending FLUSH_BATCH_EVENT", i, len(c.evChans[i]))
				}
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

func (c *ConflictDetectionCache) RemoveEvents(events ...*tgtdb.Event) {
	c.Lock()
	defer c.Unlock()
	eventsRemoved := false

	for _, event := range events {
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

/*
CASES

c->cached
i->incoming

if UK changed
PK - id
UK - email
UPDATE-UPDATE
	1 abc
	2 xyz
	UPDATE 1 abc to def
	UPDATE 2 xyz to abc
	c.before-i.after

	1 nil
	2 xyz
	UPDATE 1 nil to abc
	UPDATE 2 xyz to nil
	c.before-i.after

UPDATE-INSERT
	1 abc
	UPDATE 1 abc to def
	INSERT 2 abc
	c.before-i.after

	1 nil
	UPDATE 1 nil to def
	INSERT 2 abc
	c.before-i.after
DELETE-INSERT
	1 abc
	DELETE 1
	INSERT 2 abc
	c.before-i.after
DELETE-UPDATE
	1 abc
	2 def
	DELETE 1
	UPDATE 2 def to abc
	c.before-i.after

if uk change case: not change in both c and i but two events operating on same uk, change in one of the events

PK - id
UK - check_id WHERE most_recent
UPDATE-UPDATE
	uk not changed in both c and i
	1 10 t
	2 10 f

	UPDATE 1 to false
	UPDATE 2 to true
	c.before-i.before

	uk changed in c
	1 10 t
	2 10 f
	UPDATE 1 10->11 uk is changed
	UPDATE 2 to true
	c.before-i.before

	uk changed i
	1 10 t
	2 11 t
	UPDATE 1 to false
	UPDATE 2 11 -> 10
	c.before-i.after
UPDATE-INSERT
	uk not changed in both c and i
	1 10 t
	UPDATE 1 to false
	INSERT 2 10 t
	c.before-i.after

	uk in i is changed
	1 10 t
	UPDATE 1 10 -> 11
	INSERT 2 10 t
	c.before-i.after
DELETE-INSERT
	uk not changed in both c and i
	1 10 t
	DELETE 1
	INSERT 2 10 t
	c.before-i.after

	no other cases possible for delete-insert
DELETE-UPDATE
	uk not changed in both c and i
	1 10 t
	2 10 f
	DELETE 1
	UPDATE 2 to true
	c.before-i.before

	uk is changed in i
	1 10 t
	2 11 t
	DELETE 1
	UPDATE 2 11 -> 10
	c.before-i.after


TODO: partition by table - no need to do conflict detection
*/

func (c *ConflictDetectionCache) eventsConfict(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event) bool {
	if !c.eventsAreOfSameTable(cachedEvent, incomingEvent) {
		return false
	}

	if c.HaveSamePK(cachedEvent, incomingEvent) {
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
			// future: https://yugabyte.atlassian.net/browse/DB-9681
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
		//todo: handle nil values properly in before fields / fields

		if cachedEvent.BeforeFields[column] == nil && incomingEvent.Fields[column] == nil {
			//if c.before and i.after are nil then it can be a conflict
			log.Infof("conflict detected for table %s, column %s, between value of event1(vsn=%d, colVal=%s) and event2(vsn=%d, colVal=%s)",
				cachedEvent.TableNameTup.ForKey(), column, cachedEvent.Vsn, "nil", incomingEvent.Vsn, "nil")
			return true
		}

		if cachedEvent.BeforeFields[column] == nil && incomingEvent.BeforeFields[column] != nil {
			//if c.before and i.before are nil then it can be a conflict
			log.Infof("conflict detected for table %s, column %s, between value of event1(vsn=%d, colVal=%s) and event2(vsn=%d, colVal=%s)",
				cachedEvent.TableNameTup.ForKey(), column, cachedEvent.Vsn, "nil", incomingEvent.Vsn, "nil")
			return true
		}

		cachedEventBefore := ""
		if cachedEvent.BeforeFields[column] != nil {
			cachedEventBefore = *cachedEvent.BeforeFields[column]
		}
		incomingEventBefore := ""
		if incomingEvent.BeforeFields[column] != nil {
			incomingEventBefore = *incomingEvent.BeforeFields[column]
		}
		incomingEventAfter := ""
		if incomingEvent.Fields[column] != nil {
			incomingEventAfter = *incomingEvent.Fields[column]
		}

		switch true {
		case cachedEventBefore == incomingEventAfter:
			//If uk column is changes then it is a pure conflict
			//Handles all cases of UPDATE-UPDATE, UPDATE-INSERT, DELETE-INSERT, DELETE-UPDATE

			//If uk is not changed but the partial predicate is updated in cached and the same uk with before predicate is inserted in the incoming event then it is a partial conflict
			//handles UPDATE-INSERT, DELETE-INSERT
			log.Infof("conflict detected for table %s, column %s, between value of event1(vsn=%d, colVal=%s) and event2(vsn=%d, colVal=%s)",
				cachedEvent.TableNameTup.ForKey(), column, cachedEvent.Vsn, cachedEventBefore, incomingEvent.Vsn, incomingEventAfter)
			return true
		case cachedEventBefore == incomingEventBefore:
			//If two events are operating on same uk then it is a partial conflict
			//handles UPDATE-UPDATE, DELETE-UPDATE
			log.Infof("conflict detected for table %s, column %s, between value of event1(vsn=%d, colVal=%s) and event2(vsn=%d, colVal=%s)",
				cachedEvent.TableNameTup.ForKey(), column, cachedEvent.Vsn, cachedEventBefore, incomingEvent.Vsn, incomingEventBefore)
			return true
		}

	}
	return false
}

func (c *ConflictDetectionCache) eventsAreOfSameTable(event1 *tgtdb.Event, event2 *tgtdb.Event) bool {
	return event1.TableNameTup.Equals(event2.TableNameTup)
}

func (c *ConflictDetectionCache) HaveSamePK(event1 *tgtdb.Event, event2 *tgtdb.Event) bool {
	for key, value := range event1.Key {
		if event2.Key[key] != value {
			return false
		}
	}
	return true
}
