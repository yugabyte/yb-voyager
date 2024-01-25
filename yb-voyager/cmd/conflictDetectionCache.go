package cmd

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
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
*/
type ConflictDetectionCache struct {
	sync.Mutex
	m                        map[int64]*tgtdb.Event
	cond                     *sync.Cond
	tableToUniqueKeyColumns map[string][]string
}

func NewConflictDetectionCache(tableToIdentityColumnNames map[string][]string) *ConflictDetectionCache {
	c := &ConflictDetectionCache{}
	c.m = make(map[int64]*tgtdb.Event)
	c.cond = sync.NewCond(&c.Mutex)
	c.tableToUniqueKeyColumns = tableToIdentityColumnNames
	return c
}

func (c *ConflictDetectionCache) Put(event *tgtdb.Event) {
	c.Lock()
	defer c.Unlock()
	c.m[event.Vsn] = event
}

func (c *ConflictDetectionCache) WaitUntilNoConflict(event *tgtdb.Event) {
	c.Lock()
	defer c.Unlock()

retry:
	for _, cachedEvent := range c.m {
		if c.eventsConfict(cachedEvent, event) {
			log.Infof("waiting for event(vsn=%d) to be complete before processing event(vsn=%d)", cachedEvent.Vsn, event.Vsn)
			// wait will release the lock and wait for a broadcast signal
			c.cond.Wait()

			// we can't return after just one conflict, because there can be multiple conflicts
			// for example, if we have 3 unique key columns conflicting with 3 different events
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

	if eventsRemoved {
		c.cond.Broadcast()
	}
}

func (c *ConflictDetectionCache) eventsConfict(event1, event2 *tgtdb.Event) bool {
	if !(event1.SchemaName == event2.SchemaName && event1.TableName == event2.TableName) {
		return false
	}

	/*
		Not checking for value of unique key values conflict in case of export from yb because of inconsistency issues in before values of events provided by yb-cdc
		TODO(future): Fix this in our debezium voyager plugin

		For now, we just check if the event is from same table then we consider it as a conflict
	*/
	if isTargetDBExporter(event2.ExporterRole) {
		log.Infof("conflict detected for table %s, between event1(vsn=%d) and event2(vsn=%d)", event1.TableName, event1.Vsn, event2.Vsn)
		return true
	}

	maybeQualifiedName := event1.TableName
	if event1.SchemaName != "public" {
		maybeQualifiedName = fmt.Sprintf("%s.%s", event1.SchemaName, event1.TableName)
	}
	uniqueKeyColumns := c.tableToUniqueKeyColumns[maybeQualifiedName]
	for _, column := range uniqueKeyColumns {
		// if the unique key column value is same, then we have a conflict
		if *event1.Fields[column] == *event2.Fields[column] {
			log.Infof("conflict detected for table %s, column %s, between value of event1(vsn=%d, colVal=%s) and event2(vsn=%d, colVal=%s)",
				maybeQualifiedName, column, event1.Vsn, *event1.Fields[column], event2.Vsn, *event2.Fields[column])
			return true
		}
	}
	return false
}
