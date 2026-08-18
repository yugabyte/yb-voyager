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
	"strconv"
	"strings"
	"sync"

	goerrors "github.com/go-errors/errors"
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
	m                    map[int64]*tgtdb.Event
	cond                 *sync.Cond
	tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]
	evChans              []chan *tgtdb.Event
	sourceDBType         string

	// Per-table CDC partition key (strategy + custom key columns), used to compute an
	// event's partition key (see GetEventPartitionKey). Two events with the same partition
	// key are routed to the same channel and applied in commit order, so they are excluded
	// from conflict detection (they can never race).
	tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]

	/*
		ukLookup is a value-keyed secondary index over the cached events used to avoid
		scanning the entire cache on every incoming event (see WaitUntilNoConflict).

		The key encodes: table + unique-index position + the index-column value tuple of
		the cached event's BeforeFields (see computeConflictBucketKey). The value is the
		set of cached events (by VSN) whose BeforeFields produce that tuple.

		Only used for the normal source path; target-DB-exporter events have nil
		BeforeFields and are therefore never indexed here.
	*/
	ukLookup map[string]map[int64]*tgtdb.Event
	// vsnToBuckets records, per cached event VSN, the bucket keys it was added to,
	// so removal from ukLookup is O(#buckets-for-event) instead of a full scan.
	vsnToBuckets map[int64][]string
}

func NewConflictDetectionCache(tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex], evChans []chan *tgtdb.Event, sourceDBType string, tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) *ConflictDetectionCache {
	c := &ConflictDetectionCache{}
	c.m = make(map[int64]*tgtdb.Event)
	c.cond = sync.NewCond(&c.Mutex)
	c.tableToUniqueIndexes = tableToUniqueIndexes
	c.sourceDBType = sourceDBType
	c.evChans = evChans
	c.ukLookup = make(map[string]map[int64]*tgtdb.Event)
	c.vsnToBuckets = make(map[int64][]string)
	c.tablePartitionKeyMap = tablePartitionKeyMap
	return c
}

/*
Put()

	if ! U/D
		return
	if U:
		if changedColumns <intersection> (UK columns <union> predictate columns) == EMPTY:
		Can't add this check until we have predicate columns as we still need the event in index even if its unique columns are unchanged for predicate cases 
			return
		if uniqueINdex.NullsDistinct AND ANY unique key column value is NULL:
			return
		add-to-cache (uklookup) <---- computeKey (beforeFields) (all columns should be present)
*/
func (c *ConflictDetectionCache) Put(event *tgtdb.Event) error {
	if event.BeforeFields == nil || event.Op == "c" {
		//put can only insert an event in the cache if its delete/update event
		return goerrors.Errorf("before fields are nil for event vsn(%d)", event.Vsn)
	}
	c.Lock()
	defer c.Unlock()
	// caches a separate copy of the event, not a pointer, otherwise it will be modified
	// by ConvertEvent() causing issues in events comparison for conflict detection.
	cachedEvent := event.Copy()
	if isTargetDBExporter(event.ExporterRole) {
		//TODO: see if we should remove this now as per PARTITION_BY_TABLE
		c.m[cachedEvent.Vsn] = cachedEvent
	} else {
		err := c.indexEventLocked(cachedEvent)
		if err != nil {
			return err
		}
	}
	log.Debugf("adding event vsn(%d) to conflict cache for table %s", event.Vsn, event.TableNameTup.ForKey())
	return nil
}

// indexEventLocked adds the cached event to the value-keyed lookup index for every
// unique index of its table whose columns are all present (and indexable) in the
// event's BeforeFields. It is a no-op for events with nil/empty BeforeFields (e.g.
// target-DB-exporter events). Caller must hold the lock.
func (c *ConflictDetectionCache) indexEventLocked(event *tgtdb.Event) error {
	uniqueIndexes, _ := c.tableToUniqueIndexes.Get(event.TableNameTup)
	if len(uniqueIndexes) == 0 {
		return nil
	}
	for _, index := range uniqueIndexes {
		if !index.NullsNotDistinct && anyUniqueIndexColumnValueIsNull(event.BeforeFields, index.Columns) {
			//		if uniqueINdex.NullsDistinct AND ANY unique key column value is NULL:
			continue
		}
		key, ok, err := computeConflictBucketKey(event.TableNameTup, event.BeforeFields, index)
		if err != nil {
			return goerrors.Errorf("error computing conflict bucket key for table %s, index %s: %v", event.TableNameTup.ForKey(), index.IndexName, err)
		}
		if !ok {
			continue
		}
		bucket := c.ukLookup[key]
		if bucket == nil {
			bucket = make(map[int64]*tgtdb.Event)
			c.ukLookup[key] = bucket
		}
		bucket[event.Vsn] = event
		c.vsnToBuckets[event.Vsn] = append(c.vsnToBuckets[event.Vsn], key)
	}
	return nil
}

// deindexEventLocked removes the given VSN from every bucket it was added to and
// drops empty buckets. Caller must hold the lock.
func (c *ConflictDetectionCache) deindexEventLocked(vsn int64) {
	keys, ok := c.vsnToBuckets[vsn]
	if !ok {
		return
	}
	for _, key := range keys {
		bucket := c.ukLookup[key]
		if bucket == nil {
			continue
		}
		delete(bucket, vsn)
		if len(bucket) == 0 {
			delete(c.ukLookup, key)
		}
	}
	delete(c.vsnToBuckets, vsn)
}

func (c *ConflictDetectionCache) WaitUntilNoConflict(incomingEvent *tgtdb.Event) error {
	c.Lock()
	defer c.Unlock()

	conflictLogged := false
	for {
		conflictInfo, err := c.findConflictLocked(incomingEvent)
		if err != nil {
			return err
		}
		if len(conflictInfo) == 0 {
			return nil
		}

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
				log.Debugf("channel %d is full with size %d, not sending FLUSH_BATCH_EVENT", i, len(c.evChans[i]))
			}
		}

		for _, info := range conflictInfo {
			for _, cachedEvent := range info.eventsConflicting {
				injectUniqueKeyConflictDetectedFailpoint(cachedEvent, incomingEvent)
			}
			// Concise INFO once per blocked incoming event; the per-index/column-value
			// detail is logged at DEBUG in findValueConflictLocked. The target-DB-exporter
			// path already emits its own "conflict detected" INFO in eventsConfict, so here
			// we only log at DEBUG for that path to avoid double counting.
			cachedVsns := lo.Map(info.eventsConflicting, func(e *tgtdb.Event, _ int) int64 { return e.Vsn })
			if isTargetDBExporter(incomingEvent.ExporterRole) {
				log.Debugf("event(vsn=%d) waiting for in-flight event(s) %v to be applied", incomingEvent.Vsn, cachedVsns)
			} else if !conflictLogged {
				log.Infof("conflict detected: event(vsn=%d) on table %s, unique index columns %v with matched value %s (%s conflict) waiting for in-flight event(s) %v to be applied",
					incomingEvent.Vsn, incomingEvent.TableNameTup.ForKey(), info.indexColumns, info.matchedValue, info.matchType, cachedVsns)
			} else {
				log.Debugf("still waiting: event(vsn=%d) blocked by in-flight event(s) %v", incomingEvent.Vsn, cachedVsns)
			}
		}
		conflictLogged = true
		// cond.Wait releases the lock and blocks until RemoveEvents broadcasts (some
		// cached event was applied/removed). We then loop and re-check, because one
		// incoming event can conflict with multiple cached events that clear at
		// different times (e.g. a table with several unique indexes).
		c.cond.Wait()
	}
}

// Conflict describes the unique-index match that caused a value-path conflict.
type Conflict struct {
	indexName         string         // name of the unique index that caused the conflict
	eventsConflicting []*tgtdb.Event // events that caused the conflict
	indexColumns      []string       // columns of the unique index that caused the conflict
	matchType         string         // "before-after" or "before-before"
	matchedValue      string         // formatted incoming-side index-column values (shared by all cached events)
}

// findConflictLocked returns the cached events that conflict with the incoming event,
// or an empty slice if there is none, along with detail describing the value-path match
// (empty conflictInfo on the target-DB-exporter path). Caller must hold the lock.
//
// For the target-DB-exporter path (fall-back/fall-forward) conflicts are table-level
// and value-agnostic (see eventsConfict), so we retain the full scan over the cache.
// For the normal source path we use the value-keyed lookup index (findValueConflictLocked).
func (c *ConflictDetectionCache) findConflictLocked(incomingEvent *tgtdb.Event) ([]Conflict, error) {
	if isTargetDBExporter(incomingEvent.ExporterRole) {
		for _, cachedEvent := range c.m {
			log.Debugf("checking conflict for event(vsn=%d) and event(vsn=%d)", cachedEvent.Vsn, incomingEvent.Vsn)
			if c.eventsConfict(cachedEvent, incomingEvent) {
				return []Conflict{{
					indexName:         "",
					eventsConflicting: []*tgtdb.Event{cachedEvent},
					indexColumns:      []string{},
					matchType:         "",
					matchedValue:      "",
				}}, nil
			}
		}
		return []Conflict{}, nil
	}

	return c.findValueConflictLocked(incomingEvent)
}

// findValueConflictLocked returns every cached event that conflicts with the incoming
// event on the normal source path, using the value-keyed lookup index. Returns an
// empty slice when there is no conflict. Caller must hold the lock.
//
// A cached event conflicts when, for some unique index, its BeforeFields equal the
// incoming event's Fields (before-after) or BeforeFields (before-before) AND the two
// events have different partition keys. The exact value equality (including column
// existence and NULLS [NOT] DISTINCT semantics) is already guaranteed by the bucket-key
// match built in computeConflictBucketKey, so - unlike the full-scan target path - this
// does not re-run eventsConfict. The only residual filter is the partition-key exclusion
// (getEventsWithDifferentPartitionKey): events that share the incoming event's partition
// key are routed to the same channel and applied in order, so they are never
// cross-channel conflicts. The partition key is the PK under PARTITION_BY_PK and the
// custom key columns under PARTITION_BY_CUSTOM.
//
// All unique indexes and both checks (before-after, before-before) are evaluated so the
// incoming event can wait on the complete set of conflicting cached events at once. The
// result is de-duplicated by VSN, since a single cached event can match several indexes.

/*
TODO: to arrange it in this manner

FindConflicts()

	for each unique index:
		if D:
			skip-check
		if I:
			if uniqueINdex.NullsDistinct AND ANY unique key column value is NULL:
				skip-check
			check before-after conflicts; <---- computeKey (afterFields) (all columns should be present)
			 no before-before
		if U:

			if changedColumns <intersection> (UK columns <union> predictate columns) == EMPTY:
				skip-check
			before-after:
				if uniqueINdex.NullsDistinct AND ANY unique key column value (after fields) is NULL:
					skip-check
				check before-after conflicts <---- computeKey (afterFields) (possible that only a subset of columns are present)

			if uniqueIndex has partial predicate: (before-before)
				if uniqueINdex.NullsDistinct AND ANY unique key column value (before fields) is NULL:
					skip-check
				check before-before conflicts.  <---- computeKey (beforeFields) (all columns should be present)
*/
func (c *ConflictDetectionCache) findValueConflictLocked(incomingEvent *tgtdb.Event) ([]Conflict, error) {
	if incomingEvent.Op == "d" {
		return []Conflict{}, nil
	}
	uniqueIndexes, _ := c.tableToUniqueIndexes.Get(incomingEvent.TableNameTup)
	if len(uniqueIndexes) == 0 {
		return []Conflict{}, nil
	}
	totalConflictInfo := make([]Conflict, 0)
	for _, index := range uniqueIndexes {
		// before-after conflict: cachedEvent.BeforeFields == incomingEvent.Fields
		switch incomingEvent.Op {
		case "c":
			conflict, err := c.checkBeforeAfterConflict(incomingEvent, index)
			if err != nil {
				return []Conflict{}, err
			}
			if len(conflict.eventsConflicting) > 0 {
				totalConflictInfo = append(totalConflictInfo, conflict)
			}
		case "u":
			if anyUniqueIndexColumnChanged(incomingEvent.Fields, index.Columns) {
				conflict, err := c.checkBeforeAfterConflict(incomingEvent, index)
				if err != nil {
					return []Conflict{}, err
				}
				if len(conflict.eventsConflicting) > 0 {
					totalConflictInfo = append(totalConflictInfo, conflict)
				}
			}
			conflict, err := c.checkBeforeBeforeConflict(incomingEvent, index)
			if err != nil {
				return []Conflict{}, err
			}
			if len(conflict.eventsConflicting) > 0 {
				totalConflictInfo = append(totalConflictInfo, conflict)
			}
		}
		// if (incomingEvent.Op == "u" && anyUniqueIndexColumnChanged(incomingEvent.AfterFields, index.Columns)) || incomingEvent.Op == "c" {
		// 	key, ok, err := computeConflictBucketKey(incomingEvent.TableNameTup, incomingEvent.AfterFields, index)
		// 	if err != nil {
		// 		return []Conflict{}, err
		// 	}
		// 	if ok {
		// 		cachedEvents, err := c.getConflictingEventsWithDifferentPartitionKey(key, incomingEvent)
		// 		if err != nil {
		// 			return []Conflict{}, err
		// 		}
		// 		if len(cachedEvents) > 0 {
		// 			for _, cachedEvent := range cachedEvents {
		// 				log.Debugf("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d, colVal=%s) and after value of incoming-event2(vsn=%d, colVal=%s)",
		// 					incomingEvent.TableNameTup.ForKey(), index.Columns,
		// 					cachedEvent.Vsn, formatUniqueIndexColumnValuesForLog(cachedEvent.BeforeFields, index.Columns),
		// 					incomingEvent.Vsn, formatUniqueIndexColumnValuesForLog(incomingEvent.AfterFields, index.Columns))
		// 			}
		// 			totalConflictInfo = append(totalConflictInfo, Conflict{
		// 				indexName:         index.IndexName,
		// 				eventsConflicting: cachedEvents,
		// 				indexColumns:      index.Columns,
		// 				matchType:         "before-after",
		// 				matchedValue:      formatUniqueIndexColumnValuesForLog(incomingEvent.AfterFields, index.Columns),
		// 			})
		// 		}
		// 	}
		// }
		// if incomingEvent.Op == "c" {
		// 	//as for the insert event we don't have before fields so we don't need to check for before-before conflict
		// 	continue
		// }
		// // before-before conflict: cachedEvent.BeforeFields == incomingEvent.BeforeFields
		// key, ok, err := computeConflictBucketKey(incomingEvent.TableNameTup, incomingEvent.BeforeFields, index)
		// if err != nil {
		// 	return []Conflict{}, err
		// }
		// if ok {
		// 	cachedEvents, err := c.getConflictingEventsWithDifferentPartitionKey(key, incomingEvent)
		// 	if err != nil {
		// 		return []Conflict{}, err
		// 	}
		// 	if len(cachedEvents) > 0 {
		// 		for _, cachedEvent := range cachedEvents {
		// 			log.Debugf("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d, colVal=%s) and before value of incoming-event2(vsn=%d, colVal=%s)",
		// 				incomingEvent.TableNameTup.ForKey(), index.Columns,
		// 				cachedEvent.Vsn, formatUniqueIndexColumnValuesForLog(cachedEvent.BeforeFields, index.Columns),
		// 				incomingEvent.Vsn, formatUniqueIndexColumnValuesForLog(incomingEvent.BeforeFields, index.Columns))
		// 		}
		// 		totalConflictInfo = append(totalConflictInfo, Conflict{
		// 			indexName:         index.IndexName,
		// 			eventsConflicting: cachedEvents,
		// 			indexColumns:      index.Columns,
		// 			matchType:         "before-before",
		// 			matchedValue:      formatUniqueIndexColumnValuesForLog(incomingEvent.BeforeFields, index.Columns),
		// 		})
		// 	}
		// }
	}
	return totalConflictInfo, nil
}

func (c *ConflictDetectionCache) checkBeforeAfterConflict(incomingEvent *tgtdb.Event, index tgtdb.UniqueIndex) (Conflict, error) {
	if !index.NullsNotDistinct && anyUniqueIndexColumnValueIsNull(incomingEvent.AfterFields, index.Columns) {
		//		if uniqueINdex.NullsDistinct AND ANY unique key column value is NULL:
		return Conflict{}, nil
	}
	key, ok, err := computeConflictBucketKey(incomingEvent.TableNameTup, incomingEvent.AfterFields, index)
	if err != nil {
		return Conflict{}, err
	}
	if !ok {
		return Conflict{}, nil
	}
	cachedEvents, err := c.getConflictingEventsWithDifferentPartitionKey(key, incomingEvent)
	if err != nil {
		return Conflict{}, err
	}
	if len(cachedEvents) > 0 {
		for _, cachedEvent := range cachedEvents {
			log.Debugf("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d, colVal=%s) and after value of incoming-event2(vsn=%d, colVal=%s)",
				incomingEvent.TableNameTup.ForKey(), index.Columns,
				cachedEvent.Vsn, formatUniqueIndexColumnValuesForLog(cachedEvent.BeforeFields, index.Columns),
				incomingEvent.Vsn, formatUniqueIndexColumnValuesForLog(incomingEvent.AfterFields, index.Columns))
		}
	}
	return Conflict{
		indexName:         index.IndexName,
		eventsConflicting: cachedEvents,
		indexColumns:      index.Columns,
		matchType:         "before-after",
		matchedValue:      formatUniqueIndexColumnValuesForLog(incomingEvent.AfterFields, index.Columns),
	}, nil
}

func (c *ConflictDetectionCache) checkBeforeBeforeConflict(incomingEvent *tgtdb.Event, index tgtdb.UniqueIndex) (Conflict, error) {

	if !index.NullsNotDistinct && anyUniqueIndexColumnValueIsNull(incomingEvent.BeforeFields, index.Columns) {
		//		if uniqueINdex.NullsDistinct AND ANY unique key column value is NULL:
		return Conflict{}, nil
	}
	key, ok, err := computeConflictBucketKey(incomingEvent.TableNameTup, incomingEvent.BeforeFields, index)
	if err != nil {
		return Conflict{}, err
	}
	if !ok {
		return Conflict{}, nil
	}
	cachedEvents, err := c.getConflictingEventsWithDifferentPartitionKey(key, incomingEvent)
	if err != nil {
		return Conflict{}, err
	}
	if len(cachedEvents) > 0 {
		for _, cachedEvent := range cachedEvents {
			log.Debugf("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d, colVal=%s) and before value of incoming-event2(vsn=%d, colVal=%s)",
				incomingEvent.TableNameTup.ForKey(), index.Columns,
				cachedEvent.Vsn, formatUniqueIndexColumnValuesForLog(cachedEvent.BeforeFields, index.Columns),
				incomingEvent.Vsn, formatUniqueIndexColumnValuesForLog(incomingEvent.BeforeFields, index.Columns))
		}
	}
	return Conflict{
		indexName:         index.IndexName,
		eventsConflicting: cachedEvents,
		indexColumns:      index.Columns,
		matchType:         "before-before",
		matchedValue:      formatUniqueIndexColumnValuesForLog(incomingEvent.BeforeFields, index.Columns),
	}, nil

}

func anyUniqueIndexColumnChanged(fields map[string]*string, indexColumns []string) bool {
	for _, column := range indexColumns {
		_, ok := fields[column]
		if ok {
			return true
		}
	}
	return false
}

func anyUniqueIndexColumnValueIsNull(fields map[string]*string, indexColumns []string) bool {
	for _, column := range indexColumns {
		val, exists := fields[column]
		if exists && val == nil {
			return true
		}
	}
	return false
}

// getConflictingEventsWithDifferentPartitionKey returns all cached events in the given lookup bucket
// whose partition key differs from the incoming event's (nil if none). Events sharing the
// incoming event's partition key are excluded because they route to the same channel (see
// GetEventPartitionKey / hashEvent) and are therefore applied in commit order - they can
// never be a cross-channel conflict. This generalizes the same-PK exclusion: the partition
// key is the primary key under PARTITION_BY_PK and the custom key columns under
// PARTITION_BY_CUSTOM. Caller must hold the lock.
func (c *ConflictDetectionCache) getConflictingEventsWithDifferentPartitionKey(key string, incomingEvent *tgtdb.Event) ([]*tgtdb.Event, error) {
	incomingPartitionKey, err := GetEventPartitionKey(incomingEvent, c.tablePartitionKeyMap)
	if err != nil {
		return nil, goerrors.Errorf("error computing partition key for incoming event(vsn=%d): %v", incomingEvent.Vsn, err)
	}
	var cachedEventsMatchingKey []*tgtdb.Event
	for _, cachedEvent := range c.ukLookup[key] {
		cachedPartitionKey, err := GetEventPartitionKey(cachedEvent, c.tablePartitionKeyMap)
		if err != nil {
			return nil, goerrors.Errorf("error computing partition key for cached event(vsn=%d): %v", cachedEvent.Vsn, err)
		}
		if cachedPartitionKey == incomingPartitionKey {
			continue
		}
		cachedEventsMatchingKey = append(cachedEventsMatchingKey, cachedEvent)
	}
	return cachedEventsMatchingKey, nil
}

const (
	// conflictBucketNull marks a NULL value in a bucket key (only used for
	// NULLS NOT DISTINCT indexes, where two NULLs are considered equal).
	conflictBucketNull = "N"
	// conflictBucketValue prefixes a non-NULL, length-prefixed value in a bucket key.
	conflictBucketValue = "V"
)

// computeConflictBucketKey builds an unambiguous lookup key for the given table,
// unique-index name and the index-column values found in fields. It returns
// ok=false when the tuple is not indexable, mirroring the conflict semantics in
// uniqueIndexColumnsExistInBothFields and uniqueKeyColumnValuesEqual:
//   - a column is missing from fields, OR
//   - a column value is NULL under the default NULLS DISTINCT (NULLs never conflict).
//
// Values are length-prefixed so distinct tuples never collide (e.g. {"ab",""} vs
// {"a","b"}); NULLs under NULLS NOT DISTINCT use a dedicated sentinel.
func computeConflictBucketKey(table sqlname.NameTuple, fields map[string]*string, index tgtdb.UniqueIndex) (string, bool, error) {
	if fields == nil {
		return "", false, goerrors.Errorf("fields are nil")
	}
	var b strings.Builder
	b.WriteString(table.ForKey())
	b.WriteByte(0) //separator for the table name and index name
	b.WriteString(index.IndexName)
	for _, column := range index.Columns {
		val, exists := fields[column]
		if !exists {
			//In case the incoming event like update is not having this field in after fields then it is not indexable as the unique key is not changed so it won't conflict with anything
			return "", false, goerrors.Errorf("column %s is missing from fields", column)
		}
		b.WriteByte(0) //separator for the field name and value
		if val == nil {
			if !index.NullsNotDistinct {
				// default NULLS DISTINCT: NULLs never conflict,so it should never come into this compute
				return "", false, goerrors.Errorf("column %s has null value but index is not NULLS NOT DISTINCT", column)
			}
			b.WriteString(conflictBucketNull) //placeholder for the null value
			continue
		}
		b.WriteString(conflictBucketValue)     //placeholder for the non-null value
		b.WriteString(strconv.Itoa(len(*val))) //length of the value
		b.WriteByte(':')                       //separator for the length and value
		b.WriteString(*val)                    //value
	}
	return b.String(), true, nil
}

func (c *ConflictDetectionCache) RemoveEvents(events ...*tgtdb.Event) {
	c.Lock()
	defer c.Unlock()
	eventsRemoved := false

	for _, event := range events {
		if _, ok := c.m[event.Vsn]; ok {
			//event is in main cache
			delete(c.m, event.Vsn)
			eventsRemoved = true
		} else if _, ok := c.vsnToBuckets[event.Vsn]; ok {
			//event is in lookup index
			c.deindexEventLocked(event.Vsn)
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


NOTE: tableToUniqueIndexes is fetched live from the import target DB (see initializeConflictDetectionCache),
which is the DB that actually enforces the unique constraints. Oracle sources always use PARTITION_BY_TABLE
during live migration, so conflict detection never runs for them.
TODO: optimization if no partial unique index then no need to check before=before
TODO: DO not add-to-cache OR check-for-conflicts if the UPDATE does not change UK columns or partial predicate columns
*/

func (c *ConflictDetectionCache) eventsConfict(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event) bool {
	if !c.eventsAreOfSameTable(cachedEvent, incomingEvent) {
		return false
	}

	if c.sameTableEventsHaveSamePK(cachedEvent, incomingEvent) {
		return false
	}

	uniqueIndexes, _ := c.tableToUniqueIndexes.Get(cachedEvent.TableNameTup)
	if isTargetDBExporter(incomingEvent.ExporterRole) {
		uniqueKeyColumns := make([]string, 0) // flattening the unique indexes to get the unique key columns
		for _, index := range uniqueIndexes {
			uniqueKeyColumns = append(uniqueKeyColumns, index.Columns...)
		}
		/*
			Not checking for value of unique key values conflict in case of export from yb because of inconsistency issues in before values of events provided by yb-cdc
			TODO(future): Fix this in our debezium voyager plugin

			For now, we just check if the event is from same table then we consider it as a conflict

			For the export data from target - we don't check for conflicts because we have default partition by table cdc strategy for import data to source/source-replica
			so there won't be any conflicts detected as all the events for a table will be in the same channel.

			In case user changes the cdc strategy to partition by pk/auto then we need to check for conflicts as we will execute the events for a table in different channels based on PK.
		*/
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

	for _, index := range uniqueIndexes {
		if c.uniqueIndexConflicts(cachedEvent, incomingEvent, index) {
			return true
		}
	}
	return false
}

func (c *ConflictDetectionCache) uniqueIndexConflicts(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event, index tgtdb.UniqueIndex) bool {
	if c.checkUniqueIndexBeforeAfterConflict(cachedEvent, incomingEvent, index) {
		return true
	}
	if c.checkUniqueIndexBeforeBeforeConflict(cachedEvent, incomingEvent, index) {
		return true
	}
	return false
}

func (c *ConflictDetectionCache) checkUniqueIndexBeforeAfterConflict(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event, index tgtdb.UniqueIndex) bool {
	indexColumns := index.Columns
	// Check conflict: cachedEvent.BeforeFields[index columns] == incomingEvent.Fields[index columns]
	if !uniqueIndexColumnsExistInBothFields(cachedEvent.BeforeFields, incomingEvent.Fields, indexColumns) {
		return false
	}
	if !uniqueIndexColumnValuesEqual(cachedEvent.BeforeFields, incomingEvent.Fields, indexColumns, index.NullsNotDistinct) {
		return false
	}

	//for logging purposes
	cachedEventBeforeVal := formatUniqueIndexColumnValuesForLog(cachedEvent.BeforeFields, indexColumns)
	incomingEventAfterVal := formatUniqueIndexColumnValuesForLog(incomingEvent.Fields, indexColumns)
	/*
		If uk column is changes then it is a pure conflict
		Handles all cases of UPDATE-UPDATE, UPDATE-INSERT, DELETE-INSERT, DELETE-UPDATE

		If uk is not changed but the partial predicate is updated in cached and the same uk with before predicate is inserted in the incoming event then it is a conflict due to partial predicate
		handles UPDATE-INSERT, DELETE-INSERT

		False positives can be detected in case of conflict detected due to partial predicate but the unique key column is actually changed
		1. UPDATE-INSERT:
			UK - check_id where most_recent
			1 10 t
			UPDATE 1 10 true->false
			INSERT 2 10 false
			In this case, the conflict is detected due to partial predicate because the unique key column is same for both the events and the Update is removing the index key
			but the insert is not adding the index key so they can be processed in parallel

			UK - check_id where most_recent
			1 10 t xyz
			UPDATE 1 10 xyz->abc
			INSERT 2 10 false
			In this case, the conflict is detected due to partial predicate because the unique key column is same for both the events but the Update is neither removing nor adding the index key,
			the insert is also not adding the index key so they can be processed in parallel

		2. DELETE-INSERT:
			UK - check_id where most_recent
			1 10 t
			DELETE 1
			INSERT 2 10 false
			In this case, the conflict is detected due to partial predicate but the unique key column is same for both the events and the Delete is removing the index key
			but the insert is not adding the index key so they can be processed in parallel
	*/
	log.Infof("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d, colVal=%s) and after value of incoming-event2(vsn=%d, colVal=%s)",
		cachedEvent.TableNameTup.ForKey(), indexColumns, cachedEvent.Vsn, cachedEventBeforeVal, incomingEvent.Vsn, incomingEventAfterVal)
	return true
}

func (c *ConflictDetectionCache) checkUniqueIndexBeforeBeforeConflict(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event, index tgtdb.UniqueIndex) bool {
	indexColumns := index.Columns
	// Check conflict: cachedEvent.BeforeFields[index columns] == incomingEvent.BeforeFields[index columns]
	if !uniqueIndexColumnsExistInBothFields(cachedEvent.BeforeFields, incomingEvent.BeforeFields, indexColumns) {
		return false
	}
	if !uniqueIndexColumnValuesEqual(cachedEvent.BeforeFields, incomingEvent.BeforeFields, indexColumns, index.NullsNotDistinct) {
		return false
	}

	//for logging purposes
	cachedEventBeforeVal := formatUniqueIndexColumnValuesForLog(cachedEvent.BeforeFields, indexColumns)
	incomingEventBeforeVal := formatUniqueIndexColumnValuesForLog(incomingEvent.BeforeFields, indexColumns)
	/*
		If two events are operating on same uk then it is a conflict due to partial predicate
		handles UPDATE-UPDATE, DELETE-UPDATE

		False positives can be detected in case of conflict detected due to partial predicate but the unique key column is actually changed
		1. UPDATE-UPDATE:
			UK - check_id where most_recent
			1 10 f xyz
			2 10 f
			UPDATE 1 10 xyz -> abc
			UPDATE 2 10 false->true
			In this case, the conflict is detected due to partial predicate because the unique key column is same for both the events and the Update is neither removing nor adding the index key,
			but the second update is adding the index key so they can be processed in parallel

			UK - check_id where most_recent
			1 10 f xyz
			2 10 f def
			UPDATE 1 10 xyz -> abc
			UPDATE 2 10 def->ghi
			In this case, the conflict is detected due to partial predicate because the unique key column is same for both the events and the Update is neither removing nor adding the index key,
			but the second update is also neither removing nor adding the index key so they can be processed in parallel

		2. DELETE-UPDATE:
			UK - check_id where most_recent
			1 10 f
			2 10 f
			DELETE 1
			UPDATE 2 10 false->true
			In this case, the conflict is detected due to partial predicate because the unique key column is same for both the events and the Delete is not removing the index key
			but the update is adding the index key so they can be processed in parallel

			UK - check_id where most_recent
			1 10 f
			2 10 t
			DELETE 1
			UPDATE 2 10 t->f
			In this case, the conflict is detected due to partial predicate because the unique key column is same for both the events and the Delete is not removing the index key
			but the update is removing the index key so they can be processed in parallel

			UK - check_id where most_recent
			1 10 f
			2 10 t abx
			DELETE 1
			UPDATE 2 10 abx->ghy
			In this case, the conflict is detected due to partial predicate because the unique key column is same for both the events and the Delete is not removing the index key
			but the update is neither removing nor adding the index key so they can be processed in parallel


	*/
	log.Infof("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d, colVal=%s) and before value of incoming-event2(vsn=%d, colVal=%s)",
		cachedEvent.TableNameTup.ForKey(), indexColumns, cachedEvent.Vsn, cachedEventBeforeVal, incomingEvent.Vsn, incomingEventBeforeVal)
	return true
}

// uniqueKeyColumnValuesEqual reports whether two values of a unique-index column
// should be treated as equal (i.e. conflicting) for conflict detection.
//
// Two non-NULL values conflict when they are equal. NULL handling depends on the
// index's NULLS [NOT] DISTINCT property:
//   - nullsNotDistinct == true (UNIQUE ... NULLS NOT DISTINCT): two NULLs are treated
//     as equal and therefore conflict.
//   - nullsNotDistinct == false (default NULLS DISTINCT): NULLs are all distinct and
//     never conflict with each other.
func uniqueKeyColumnValuesEqual(left, right *string, nullsNotDistinct bool) bool {
	bothNil := left == nil && right == nil
	bothNotNil := left != nil && right != nil
	valuesEqual := bothNotNil && *left == *right
	if nullsNotDistinct {
		//NULLS NOT DISTINCT: two NULLs are equal and conflict.
		return bothNil || valuesEqual
	}
	//Default NULLS DISTINCT: NULLs never conflict with each other.
	return valuesEqual
}

func uniqueIndexColumnsExistInBothFields(leftFields, rightFields map[string]*string, indexColumns []string) bool {
	for _, column := range indexColumns {
		_, leftExists := leftFields[column]
		_, rightExists := rightFields[column]
		if !leftExists || !rightExists {
			return false
		}
	}
	return true
}

func uniqueIndexColumnValuesEqual(leftFields, rightFields map[string]*string, indexColumns []string, nullsNotDistinct bool) bool {
	for _, column := range indexColumns {
		if !uniqueKeyColumnValuesEqual(leftFields[column], rightFields[column], nullsNotDistinct) {
			return false
		}
	}
	return true
}

func formatUniqueIndexColumnValuesForLog(fields map[string]*string, indexColumns []string) string {
	var logStr strings.Builder
	for _, column := range indexColumns {
		val, ok := fields[column]
		if !ok {
			logStr.WriteString(column + "=<missing>")
			continue
		}
		if val == nil {
			logStr.WriteString(column + "=nil")
		} else {
			logStr.WriteString(column + "=" + *val)
		}
	}
	return logStr.String()
}

func (c *ConflictDetectionCache) eventsAreOfSameTable(event1 *tgtdb.Event, event2 *tgtdb.Event) bool {
	return event1.TableNameTup.Equals(event2.TableNameTup)
}

func (c *ConflictDetectionCache) sameTableEventsHaveSamePK(event1 *tgtdb.Event, event2 *tgtdb.Event) bool {
	event1KeyColumns := lo.Keys(event1.Key)
	event2KeyColumns := lo.Keys(event2.Key)
	if len(event1KeyColumns) != len(event2KeyColumns) {
		return false
	}
	for key, value := range event1.Key {
		value2, ok := event2.Key[key]
		if !ok {
			return false
		}
		if *value != *value2 {
			return false
		}
	}
	return true
}
