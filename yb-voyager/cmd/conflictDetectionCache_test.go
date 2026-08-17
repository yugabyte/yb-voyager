//go:build unit

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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func strPtr(v string) *string {
	return &v
}

// uidx builds a default (NULLS DISTINCT) unique index for tests.
func uidx(columns ...string) tgtdb.UniqueIndex {
	return tgtdb.UniqueIndex{Columns: columns, IndexName: "idx_" + strings.Join(columns, "_")}
}

// uidxNND builds a NULLS NOT DISTINCT unique index for tests.
func uidxNND(columns ...string) tgtdb.UniqueIndex {
	return tgtdb.UniqueIndex{Columns: columns, NullsNotDistinct: true, IndexName: "idx_nnd_" + strings.Join(columns, "_")}
}

// newConflictCacheForTest builds a cache with default (NULLS DISTINCT) unique
// indexes from the given ordered column lists.
func newConflictCacheForTest(indexes [][]string) *ConflictDetectionCache {
	uniqueIndexes := make([]tgtdb.UniqueIndex, 0, len(indexes))
	for _, cols := range indexes {
		uniqueIndexes = append(uniqueIndexes, uidx(cols...))
	}
	return newConflictCacheForTestWithIndexes(uniqueIndexes...)
}

// newConflictCacheForTestWithIndexes builds a cache with the given unique indexes
// (allowing per-index NULLS NOT DISTINCT configuration).
func newConflictCacheForTestWithIndexes(indexes ...tgtdb.UniqueIndex) *ConflictDetectionCache {
	tableToIndexes := utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]()
	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	table := sqlname.NameTuple{CurrentName: oname, TargetName: oname}
	tableToIndexes.Put(table, indexes)
	// Default the test table to PARTITION_BY_PK so the partition-key exclusion behaves
	// like the previous same-PK exclusion (routing by primary key).
	tablePartitionKeyMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	tablePartitionKeyMap.Put(table, cdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
	return NewConflictDetectionCache(tableToIndexes, []chan *tgtdb.Event{make(chan *tgtdb.Event, 1)}, POSTGRESQL, tablePartitionKeyMap)
}

func testTableTuple() sqlname.NameTuple {
	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	return sqlname.NameTuple{CurrentName: oname, TargetName: oname}
}

func TestIndexTupleConflicts_CompositeTrueConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 1)
	assert.Equal(t, int64(1), conflicts[0].Vsn)
}

func TestIndexTupleConflicts_CompositeFalsePositiveFix(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("9")},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 0)
}

func TestEventsConfict_TwoCompositeIndexes(t *testing.T) {
	cache := newConflictCacheForTest([][]string{
		{"a", "b"},
		{"c", "d"},
	})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2"), "c": strPtr("3"), "d": strPtr("4")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("9"), "c": strPtr("3"), "d": strPtr("4")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 1)
	assert.Equal(t, int64(1), conflicts[0].Vsn)
}

func TestEventsConfict_MissingColumnInEvent(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 0)
}

func TestEventsConfict_SamePKNoConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	key := map[string]*string{"id": strPtr("1")}
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          key,
		BeforeFields: map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          key,
		Fields:       map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 0)
}

// With NULLS NOT DISTINCT, two NULL values are treated as equal and therefore conflict.
func TestEventsConfict_BothNilBeforeAfter_NullsNotDistinct(t *testing.T) {
	cache := newConflictCacheForTestWithIndexes(uidxNND("email"))
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": nil},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": nil},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 1)
	assert.Equal(t, int64(1), conflicts[0].Vsn)
}

// With the default NULLS DISTINCT, two NULL values are distinct and never conflict.
func TestEventsConfict_BothNilBeforeAfter_NullsDistinct(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": nil},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": nil},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 0)
}

func TestEventsConflict_OneNilOneValueBeforeAfter(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": nil},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": strPtr("a@example.com")},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 0)
}

// Composite NULLS NOT DISTINCT: all-NULL column values are treated as equal and conflict.
func TestEventsConflict_CompositeBothNil_NullsNotDistinct(t *testing.T) {
	cache := newConflictCacheForTestWithIndexes(uidxNND("a", "b"))
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": nil, "b": nil},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": nil, "b": nil},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 1)
	assert.Equal(t, int64(1), conflicts[0].Vsn)
}

// Composite default NULLS DISTINCT: all-NULL column values are distinct and never conflict.
func TestEventsConflict_CompositeBothNil_NullsDistinct(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": nil, "b": nil},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": nil, "b": nil},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 0)
}

func TestEventsConflict_CompositeMixedNil(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": nil, "b": nil},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": nil, "b": strPtr("2")},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 0)
}

// With NULLS NOT DISTINCT, two NULL before-values conflict (before-before check).
func TestEventsConflict_BothNilBeforeBefore_NullsNotDistinct(t *testing.T) {
	cache := newConflictCacheForTestWithIndexes(uidxNND("check_id"))
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("10")},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("20")},
	}
	foundConflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, foundConflicts, 1)
	assert.Equal(t, int64(1), foundConflicts[0].Vsn)
}

// With the default NULLS DISTINCT, two NULL before-values do not conflict (before-before check).
func TestEventsConflict_BothNilBeforeBefore_NullsDistinct(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"check_id"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("10")},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("20")},
	}
	foundConflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, foundConflicts, 0)
}

func TestEventsConflict_BeforeBeforeConflictOnly(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"check_id"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		Fields:       map[string]*string{"check_id": strPtr("20")},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 1)
	assert.Equal(t, int64(1), conflicts[0].Vsn)
}

func TestEventsConflict_BeforeBeforeNoConflictWhenValuesDiffer(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"check_id"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		Fields:       map[string]*string{"check_id": strPtr("11")},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": strPtr("20")},
		Fields:       map[string]*string{"check_id": strPtr("21")},
	}
	foundConflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, foundConflicts, 0)
}

func TestEventsConflict_BeforeBeforeMissingColumn(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	cache.Put(cached)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"a": strPtr("1")},
		Fields:       map[string]*string{"a": strPtr("9"), "b": strPtr("2")},
	}
	foundConflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, foundConflicts, 0)

}

func TestEventsConfict_BeforeBeforeConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"check_id"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		Fields:       map[string]*string{"check_id": strPtr("20")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	foundConflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, foundConflicts, 1)
	assert.Equal(t, int64(1), foundConflicts[0].Vsn)
}

func TestRecordUniqueKeyConflictCount_DedupesEventPair(t *testing.T) {
	exportDir = t.TempDir()
	ukConflictStats = UniqueKeyConflictStats{}
	ukConflictSeen = nil

	table := testTableTuple()
	cached := &tgtdb.Event{Vsn: 10, TableNameTup: table}
	incoming := &tgtdb.Event{Vsn: 20, TableNameTup: table}

	recordUniqueKeyConflictCount(cached, incoming)
	recordUniqueKeyConflictCount(cached, incoming)
	recordUniqueKeyConflictCount(incoming, cached)

	require.Equal(t, 1, ukConflictStats.Total)
	require.Equal(t, 1, ukConflictStats.ByTable[table.ForKey()])

	statsPath := filepath.Join(exportDir, "failpoints", uniqueKeyConflictStatsFileName)
	data, err := os.ReadFile(statsPath)
	require.NoError(t, err)
	require.Contains(t, string(data), `"total": 1`)
}

func TestUniqueKeyConflictPairKey_OrdersVsns(t *testing.T) {
	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	table := sqlname.NameTuple{CurrentName: oname, TargetName: oname}.ForKey()
	require.Equal(t, uniqueKeyConflictPairKey(table, 20, 10), uniqueKeyConflictPairKey(table, 10, 20))
}

// findConflictForTest exercises the lock-protected findConflictLocked without
// invoking the blocking wait loop in WaitUntilNoConflict.
func findConflictForTest(t *testing.T, c *ConflictDetectionCache, incoming *tgtdb.Event) []*tgtdb.Event {
	c.Lock()
	defer c.Unlock()
	conflicts, err := c.findConflictLocked(incoming)
	if err != nil {
		testutils.FatalIfError(t, err)
	}
	var events []*tgtdb.Event
	seen := make(map[int64]bool)
	for _, conflict := range conflicts {
		for _, e := range conflict.eventsConflicting {
			if seen[e.Vsn] {
				continue
			}
			seen[e.Vsn] = true
			events = append(events, e)
		}
	}
	return events
}

// The lookup index must find the same before-after conflict that a full scan would.
func TestConflictLookup_FindsBeforeAfterConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	cache.Put(cached)

	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	got := findConflictForTest(t, cache, incoming)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].Vsn)
}

// A composite-index before-after conflict must be found via the lookup index.
func TestConflictLookup_FindsCompositeConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	cache.Put(cached)

	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	got := findConflictForTest(t, cache, incoming)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].Vsn)
}

// A NULLS NOT DISTINCT before-before conflict on NULL values must be found.
func TestConflictLookup_FindsBeforeBeforeConflict_NullsNotDistinct(t *testing.T) {
	cache := newConflictCacheForTestWithIndexes(uidxNND("check_id"))
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("10")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	cache.Put(cached)

	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("20")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	got := findConflictForTest(t, cache, incoming)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].Vsn)
}

// Under default NULLS DISTINCT, a NULL index value is never indexed and never conflicts.
func TestConflictLookup_NullsDistinctNotIndexed(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": nil},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	cache.Put(cached)
	assert.Empty(t, cache.ukLookup, "NULL value under NULLS DISTINCT must not be indexed")

	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": nil},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.Empty(t, findConflictForTest(t, cache, incoming))
}

// Same-PK candidates are gathered by the lookup but rejected by eventsConfict.
func TestConflictLookup_SamePKNoConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	key := map[string]*string{"id": strPtr("1")}
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          key,
		BeforeFields: map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	cache.Put(cached)

	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          key,
		Fields:       map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.Empty(t, findConflictForTest(t, cache, incoming))
}

// A non-conflicting incoming event must not block or match.
func TestConflictLookup_NoConflictDoesNotBlock(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	cache.Put(cached)

	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": strPtr("b@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.Empty(t, findConflictForTest(t, cache, incoming))

	done := make(chan struct{})
	go func() {
		cache.WaitUntilNoConflict(incoming)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitUntilNoConflict blocked despite no conflict")
	}
}

// RemoveEvents must clear both the primary map and the lookup index.
func TestConflictLookup_RemoveDeindexes(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"email"}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	cache.Put(cached)
	require.NotEmpty(t, cache.ukLookup)
	require.NotEmpty(t, cache.vsnToBuckets)

	cache.RemoveEvents(cached)
	assert.Empty(t, cache.m)
	assert.Empty(t, cache.ukLookup)
	assert.Empty(t, cache.vsnToBuckets)

	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.Empty(t, findConflictForTest(t, cache, incoming))
}

// computeConflictBucketKey must not collide for tuples that differ only in the
// split of characters between adjacent columns.
func TestComputeConflictBucketKey_NoAmbiguity(t *testing.T) {
	idx := uidx("a", "b")
	k1, ok1, err := computeConflictBucketKey(testutils.CreateNameTupleWithTargetName("public.users", "", POSTGRESQL), map[string]*string{"a": strPtr("ab"), "b": strPtr("")}, idx)
	if err != nil {
		t.Fatalf("error computing conflict bucket key: %v", err)
	}
	k2, ok2, err := computeConflictBucketKey(testutils.CreateNameTupleWithTargetName("public.users", "", POSTGRESQL), map[string]*string{"a": strPtr("a"), "b": strPtr("b")}, idx)
	if err != nil {
		t.Fatalf("error computing conflict bucket key: %v", err)
	}
	require.True(t, ok1)
	require.True(t, ok2)
	assert.NotEqual(t, k1, k2)

	// missing column -> not indexable
	_, ok3, err := computeConflictBucketKey(testutils.CreateNameTupleWithTargetName("public.users", "", POSTGRESQL), map[string]*string{"a": strPtr("a")}, idx)
	if err != nil {
		t.Fatalf("error computing conflict bucket key: %v", err)
	}
	assert.False(t, ok3)
}

func TestConflictWithMultipleIndexes(t *testing.T) {
	cache := newConflictCacheForTestWithIndexes(uidx("a", "b"), uidxNND("c", "d"))
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2"), "c": strPtr("3"), "d": strPtr("4")},
	}
	err := cache.Put(cached)
	require.NoError(t, err)
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("3"), "c": strPtr("3"), "d": strPtr("4")},
	}
	conflicts := findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 1)
	assert.Equal(t, int64(1), conflicts[0].Vsn)

	incoming = &tgtdb.Event{
		Vsn:          3,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("3")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("2"), "c": strPtr("3"), "d": strPtr("4")},
	}
	conflicts = findConflictForTest(t, cache, incoming)
	require.Len(t, conflicts, 1)

	actualConflicts, err := cache.findConflictLocked(incoming)
	require.NoError(t, err)
	require.Len(t, actualConflicts, 2)
	assert.Equal(t, "idx_a_b", actualConflicts[0].indexName)
	assert.Equal(t, "idx_nnd_c_d", actualConflicts[1].indexName)
}

// An incoming UPDATE with nil BeforeFields (e.g. replica identity not FULL)
// makes the before-before probe fail; WaitUntilNoConflict must surface that
// error so handleEvent aborts the import instead of silently skipping
// conflict detection for the event.
func TestWaitUntilNoConflictPropagatesBeforeFieldsError(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{"a", "b"}})
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
		BeforeFields: nil,
	}
	err := cache.WaitUntilNoConflict(incoming)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fields are nil")
}