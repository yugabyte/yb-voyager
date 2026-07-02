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
/*
POC: unique-key-value-indexed conflict detection.

Instead of scanning every in-flight event per incoming event (O(cache depth)),
maintain an inverted index keyed by (table, unique index, encoded UK value
tuple of the cached event's before-image). A conflict check is then two O(1)
lookups per unique index (incoming after-image tuple and before-image tuple),
independent of how many events are in flight.

Semantics preserved from the scan implementation:
  - a cached event contributes a key for an index only if its BeforeFields
    contain ALL index columns (existence requirement);
  - a lookup tuple is built only if the incoming image contains ALL columns;
  - per-column equality is nil==nil or string-equal (nil encoded as a
    sentinel distinct from every real value);
  - pairs with the same PK never conflict;
  - export-from-target (fall-back/fall-forward) keeps the op-based logic,
    evaluated over the table's in-flight entries only.
*/
type ukCacheEntry struct {
	vsn       int64
	chanNo    int // channel this event was dispatched to (for targeted batch flush)
	tableKey  string
	op        string
	pk        map[string]*string // shallow copy of event.Key (values are immutable strings)
	fieldCols []string           // column names present in Fields (target-exporter logic)
	ukKeys    []string           // ukIndex keys this event contributed
}

type ConflictDetectionCache struct {
	sync.Mutex
	// m holds one slim entry per in-flight u/d event, keyed by vsn.
	// (No full event copies: only the PK and the encoded UK tuples are needed.)
	m    map[int64]*ukCacheEntry
	cond *sync.Cond
	// ukIndex: (table, index ordinal, encoded before-image UK tuple) -> vsns holding it
	ukIndex map[string]map[int64]*ukCacheEntry
	// tableEntries: table -> in-flight entries, for the export-from-target path
	tableEntries         map[string]map[int64]*ukCacheEntry
	tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, [][]string]
	evChans              []chan *tgtdb.Event
	sourceDBType         string
}

func NewConflictDetectionCache(tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, [][]string], evChans []chan *tgtdb.Event, sourceDBType string) *ConflictDetectionCache {
	c := &ConflictDetectionCache{}
	c.m = make(map[int64]*ukCacheEntry)
	c.cond = sync.NewCond(&c.Mutex)
	c.ukIndex = make(map[string]map[int64]*ukCacheEntry)
	c.tableEntries = make(map[string]map[int64]*ukCacheEntry)
	c.tableToUniqueIndexes = tableToUniqueIndexes
	c.sourceDBType = sourceDBType
	c.evChans = evChans
	return c
}

// encodeUKTuple builds an unambiguous string encoding of the given columns'
// values from fields. Returns ok=false if any column is absent (matching the
// scan implementation's existence requirement). nil values get a sentinel so
// nil==nil compares equal and never collides with a real value; real values
// are length-prefixed so component boundaries are unambiguous.
func encodeUKTuple(fields map[string]*string, indexColumns []string) (string, bool) {
	var b strings.Builder
	for _, column := range indexColumns {
		v, ok := fields[column]
		if !ok {
			return "", false
		}
		if v == nil {
			b.WriteString("\x00N\x1f")
		} else {
			b.WriteString(strconv.Itoa(len(*v)))
			b.WriteByte(':')
			b.WriteString(*v)
			b.WriteByte('\x1f')
		}
	}
	return b.String(), true
}

func ukIndexKey(tableKey string, indexOrdinal int, tuple string) string {
	return tableKey + "\x1e" + strconv.Itoa(indexOrdinal) + "\x1e" + tuple
}

func pkEqual(pk1, pk2 map[string]*string) bool {
	if len(pk1) != len(pk2) {
		return false
	}
	for k, v := range pk1 {
		v2, ok := pk2[k]
		if !ok {
			return false
		}
		if !uniqueKeyColumnValuesEqual(v, v2) {
			return false
		}
	}
	return true
}

func (c *ConflictDetectionCache) Put(event *tgtdb.Event, chanNo int) {
	c.Lock()
	defer c.Unlock()

	tableKey := event.TableNameTup.Key()
	entry := &ukCacheEntry{
		vsn:       event.Vsn,
		chanNo:    chanNo,
		tableKey:  tableKey,
		op:        event.Op,
		pk:        make(map[string]*string, len(event.Key)),
		fieldCols: lo.Keys(event.Fields),
	}
	for k, v := range event.Key {
		entry.pk[k] = v
	}

	uniqueIndexes, _ := c.tableToUniqueIndexes.Get(event.TableNameTup)
	for i, indexColumns := range uniqueIndexes {
		tuple, ok := encodeUKTuple(event.BeforeFields, indexColumns)
		if !ok {
			continue
		}
		key := ukIndexKey(tableKey, i, tuple)
		set := c.ukIndex[key]
		if set == nil {
			set = make(map[int64]*ukCacheEntry)
			c.ukIndex[key] = set
		}
		set[event.Vsn] = entry
		entry.ukKeys = append(entry.ukKeys, key)
	}

	tset := c.tableEntries[tableKey]
	if tset == nil {
		tset = make(map[int64]*ukCacheEntry)
		c.tableEntries[tableKey] = tset
	}
	tset[event.Vsn] = entry

	c.m[event.Vsn] = entry
	log.Infof("adding event vsn(%d) to conflict cache", event.Vsn)
}

// findConflictingEntries returns ALL in-flight entries conflicting with the
// incoming event (deduped by vsn). Gathering every blocker in one pass lets
// the caller flush all their channels at once, so multiple blockers resolve
// in parallel (one batch round-trip) instead of serially (one per blocker).
func (c *ConflictDetectionCache) findConflictingEntries(incomingEvent *tgtdb.Event) map[int64]*ukCacheEntry {
	tableKey := incomingEvent.TableNameTup.Key()
	uniqueIndexes, _ := c.tableToUniqueIndexes.Get(incomingEvent.TableNameTup)
	blockers := map[int64]*ukCacheEntry{}

	if isTargetDBExporter(incomingEvent.ExporterRole) {
		/*
			Same semantics as the scan implementation: before values from yb-cdc
			are unreliable, so conflict on op-type per table (see original notes).
		*/
		uniqueKeyColumns := make([]string, 0)
		for _, indexColumns := range uniqueIndexes {
			uniqueKeyColumns = append(uniqueKeyColumns, indexColumns...)
		}
		incomingCols := lo.Keys(incomingEvent.Fields)
		for _, e := range c.tableEntries[tableKey] {
			if pkEqual(e.pk, incomingEvent.Key) {
				continue
			}
			if e.op == "d" {
				log.Infof("conflict detected for table %s, between event1(vsn=%d) and event2(vsn=%d)", incomingEvent.TableNameTup, e.vsn, incomingEvent.Vsn)
				blockers[e.vsn] = e
				continue
			}
			if e.op == "u" {
				ukList := lo.Intersect(e.fieldCols, uniqueKeyColumns)
				if lo.Some(incomingCols, ukList) {
					log.Infof("conflict detected for table %s, between event1(vsn=%d) and event2(vsn=%d)", incomingEvent.TableNameTup, e.vsn, incomingEvent.Vsn)
					blockers[e.vsn] = e
				}
			}
		}
		return blockers
	}

	for i, indexColumns := range uniqueIndexes {
		// before-after: cached.BeforeFields == incoming.Fields
		if tuple, ok := encodeUKTuple(incomingEvent.Fields, indexColumns); ok {
			for _, e := range c.lookupConflicts(tableKey, i, tuple, incomingEvent) {
				if _, seen := blockers[e.vsn]; !seen {
					log.Infof("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d) and after value of incoming-event2(vsn=%d)",
						incomingEvent.TableNameTup.ForKey(), indexColumns, e.vsn, incomingEvent.Vsn)
					blockers[e.vsn] = e
				}
			}
		}
		// before-before: cached.BeforeFields == incoming.BeforeFields
		if tuple, ok := encodeUKTuple(incomingEvent.BeforeFields, indexColumns); ok {
			for _, e := range c.lookupConflicts(tableKey, i, tuple, incomingEvent) {
				if _, seen := blockers[e.vsn]; !seen {
					log.Infof("conflict detected for table %s, index columns %v, between before value of cached-event1(vsn=%d) and before value of incoming-event2(vsn=%d)",
						incomingEvent.TableNameTup.ForKey(), indexColumns, e.vsn, incomingEvent.Vsn)
					blockers[e.vsn] = e
				}
			}
		}
	}
	return blockers
}

func (c *ConflictDetectionCache) lookupConflicts(tableKey string, indexOrdinal int, tuple string, incomingEvent *tgtdb.Event) []*ukCacheEntry {
	var hits []*ukCacheEntry
	set := c.ukIndex[ukIndexKey(tableKey, indexOrdinal, tuple)]
	for _, e := range set {
		if pkEqual(e.pk, incomingEvent.Key) {
			// same row: events for the same PK land on the same channel and are
			// already ordered; never a conflict (matches scan implementation)
			continue
		}
		hits = append(hits, e)
	}
	return hits
}

func (c *ConflictDetectionCache) WaitUntilNoConflict(incomingEvent *tgtdb.Event) {
	c.Lock()
	defer c.Unlock()

retry:
	if blockers := c.findConflictingEntries(incomingEvent); len(blockers) > 0 {
		// Targeted flush: only the channels holding blocking events need their
		// batches executed early; other channels keep filling their batches.
		// All blockers' channels are flushed at once so multiple blockers
		// resolve in parallel (one batch round-trip), like the old flush-all.
		// Non-blocking send because a blocking send can cause deadlock between
		// the main goroutine (holding the cache lock) and a processEvents
		// goroutine waiting for the lock in RemoveEvents.
		flushed := make(map[int]bool, len(blockers))
		for _, blocker := range blockers {
			// preserve the failpoint instrumentation (conflict count/validation
			// tests); cache entries are slim, so pass a minimal cached-event view
			// carrying the fields the failpoint uses (table, vsn)
			injectUniqueKeyConflictDetectedFailpoint(
				&tgtdb.Event{Vsn: blocker.vsn, TableNameTup: incomingEvent.TableNameTup, Key: blocker.pk},
				incomingEvent)
			if flushed[blocker.chanNo] {
				continue
			}
			flushed[blocker.chanNo] = true
			select {
			case c.evChans[blocker.chanNo] <- FLUSH_BATCH_EVENT:
			default:
				// channel is full, so it's okay not to send FLUSH_BATCH_EVENT
				// because MAX_EVENTS_PER_BATCH would likely be reached in the next batch.
				log.Infof("channel %d is full with size %d, not sending FLUSH_BATCH_EVENT", blocker.chanNo, len(c.evChans[blocker.chanNo]))
			}
			log.Infof("waiting for event(vsn=%d) to be complete before processing event(vsn=%d)", blocker.vsn, incomingEvent.Vsn)
		}
		// wait will release the lock and wait for a broadcast signal
		c.cond.Wait()

		// re-check after every wake-up: blockers may remain or new checks may apply
		goto retry
	}
}

func (c *ConflictDetectionCache) RemoveEvents(events ...*tgtdb.Event) {
	c.Lock()
	defer c.Unlock()
	eventsRemoved := false

	for _, event := range events {
		e, ok := c.m[event.Vsn]
		if !ok {
			continue
		}
		for _, key := range e.ukKeys {
			set := c.ukIndex[key]
			delete(set, e.vsn)
			if len(set) == 0 {
				delete(c.ukIndex, key)
			}
		}
		tset := c.tableEntries[e.tableKey]
		delete(tset, e.vsn)
		if len(tset) == 0 {
			delete(c.tableEntries, e.tableKey)
		}
		delete(c.m, event.Vsn)
		eventsRemoved = true
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
TODO: optimization if no partial unique index then no need to check before fields
TODO: prometheus metrics for unique conflict detection logic
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
		for _, indexColumns := range uniqueIndexes {
			uniqueKeyColumns = append(uniqueKeyColumns, indexColumns...)
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

	for _, indexColumns := range uniqueIndexes {
		if c.uniqueIndexConflicts(cachedEvent, incomingEvent, indexColumns) {
			return true
		}
	}
	return false
}

func (c *ConflictDetectionCache) uniqueIndexConflicts(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event, indexColumns []string) bool {
	if c.checkUniqueIndexBeforeAfterConflict(cachedEvent, incomingEvent, indexColumns) {
		return true
	}
	if c.checkUniqueIndexBeforeBeforeConflict(cachedEvent, incomingEvent, indexColumns) {
		return true
	}
	return false
}

func (c *ConflictDetectionCache) checkUniqueIndexBeforeAfterConflict(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event, indexColumns []string) bool {
	// Check conflict: cachedEvent.BeforeFields[index columns] == incomingEvent.Fields[index columns]
	if !uniqueIndexColumnsExistInBothFields(cachedEvent.BeforeFields, incomingEvent.Fields, indexColumns) {
		return false
	}
	if !uniqueIndexColumnValuesEqual(cachedEvent.BeforeFields, incomingEvent.Fields, indexColumns) {
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

func (c *ConflictDetectionCache) checkUniqueIndexBeforeBeforeConflict(cachedEvent *tgtdb.Event, incomingEvent *tgtdb.Event, indexColumns []string) bool {
	// Check conflict: cachedEvent.BeforeFields[index columns] == incomingEvent.BeforeFields[index columns]
	if !uniqueIndexColumnsExistInBothFields(cachedEvent.BeforeFields, incomingEvent.BeforeFields, indexColumns) {
		return false
	}
	if !uniqueIndexColumnValuesEqual(cachedEvent.BeforeFields, incomingEvent.BeforeFields, indexColumns) {
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

// uniqueKeyColumnValuesEqual applies the same nil/value equality check used for single-column unique keys.
func uniqueKeyColumnValuesEqual(left, right *string) bool {
	bothNil := left == nil && right == nil
	bothNotNil := left != nil && right != nil
	valuesEqual := bothNotNil && *left == *right
	return bothNil || valuesEqual
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

func uniqueIndexColumnValuesEqual(leftFields, rightFields map[string]*string, indexColumns []string) bool {
	for _, column := range indexColumns {
		if !uniqueKeyColumnValuesEqual(leftFields[column], rightFields[column]) {
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
	return event1.TableNameTup.Key() == event2.TableNameTup.Key()
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
