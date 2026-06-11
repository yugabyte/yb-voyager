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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func strPtr(v string) *string {
	return &v
}

func newConflictCacheForTest(indexes [][]string) *ConflictDetectionCache {
	tableToIndexes := utils.NewStructMap[sqlname.NameTuple, [][]string]()
	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	table := sqlname.NameTuple{CurrentName: oname, TargetName: oname}
	tableToIndexes.Put(table, indexes)
	return NewConflictDetectionCache(tableToIndexes, []chan *tgtdb.Event{make(chan *tgtdb.Event, 1)}, POSTGRESQL)
}

func testTableTuple() sqlname.NameTuple {
	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	return sqlname.NameTuple{CurrentName: oname, TargetName: oname}
}

func TestIndexTupleConflicts_CompositeTrueConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"a", "b"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	assert.True(t, cache.uniqueIndexConflicts(cached, incoming, []string{"a", "b"}))
}

func TestIndexTupleConflicts_CompositeFalsePositiveFix(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"a", "b"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("9")},
	}
	assert.False(t, cache.uniqueIndexConflicts(cached, incoming, []string{"a", "b"}))
}

func TestEventsConfict_TwoCompositeIndexes(t *testing.T) {
	cache := newConflictCacheForTest([][]string{
		{Columns: []string{"a", "b"}},
		{Columns: []string{"c", "d"}},
	})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2"), "c": strPtr("3"), "d": strPtr("4")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1"), "b": strPtr("9"), "c": strPtr("3"), "d": strPtr("4")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.True(t, cache.eventsConfict(cached, incoming))
}

func TestEventsConfict_MissingColumnInEvent(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"a", "b"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": strPtr("1")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.False(t, cache.eventsConfict(cached, incoming))
}

func TestEventsConfict_SamePKNoConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"email"}}})
	key := map[string]*string{"id": strPtr("1")}
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          key,
		BeforeFields: map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          key,
		Fields:       map[string]*string{"email": strPtr("a@example.com")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.False(t, cache.eventsConfict(cached, incoming))
}

func TestUniqueIndexConflicts_BothNilBeforeAfter(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"email"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": nil},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": nil},
	}
	assert.True(t, cache.checkUniqueIndexBeforeAfterConflict(cached, incoming, []string{"email"}))
	assert.True(t, cache.uniqueIndexConflicts(cached, incoming, []string{"email"}))
}

func TestUniqueIndexConflicts_OneNilOneValueBeforeAfter(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"email"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": nil},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": strPtr("a@example.com")},
	}
	assert.False(t, cache.checkUniqueIndexBeforeAfterConflict(cached, incoming, []string{"email"}))
	assert.False(t, cache.uniqueIndexConflicts(cached, incoming, []string{"email"}))
}

func TestUniqueIndexConflicts_CompositeBothNil(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"a", "b"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": nil, "b": nil},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": nil, "b": nil},
	}
	assert.True(t, cache.uniqueIndexConflicts(cached, incoming, []string{"a", "b"}))
}

func TestUniqueIndexConflicts_CompositeMixedNil(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"a", "b"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": nil, "b": nil},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"a": nil, "b": strPtr("2")},
	}
	assert.False(t, cache.uniqueIndexConflicts(cached, incoming, []string{"a", "b"}))
}

func TestUniqueIndexConflicts_BothNilBeforeBefore(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"check_id"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("10")},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": nil},
		Fields:       map[string]*string{"check_id": strPtr("20")},
	}
	assert.False(t, cache.checkUniqueIndexBeforeAfterConflict(cached, incoming, []string{"check_id"}))
	assert.True(t, cache.checkUniqueIndexBeforeBeforeConflict(cached, incoming, []string{"check_id"}))
	assert.True(t, cache.uniqueIndexConflicts(cached, incoming, []string{"check_id"}))
}

func TestUniqueIndexConflicts_BeforeBeforeConflictOnly(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"check_id"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		Fields:       map[string]*string{"check_id": strPtr("20")},
	}
	assert.False(t, cache.checkUniqueIndexBeforeAfterConflict(cached, incoming, []string{"check_id"}))
	assert.True(t, cache.checkUniqueIndexBeforeBeforeConflict(cached, incoming, []string{"check_id"}))
	assert.True(t, cache.uniqueIndexConflicts(cached, incoming, []string{"check_id"}))
}

func TestUniqueIndexConflicts_BeforeBeforeNoConflictWhenValuesDiffer(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"check_id"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		Fields:       map[string]*string{"check_id": strPtr("11")},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": strPtr("20")},
		Fields:       map[string]*string{"check_id": strPtr("21")},
	}
	assert.False(t, cache.checkUniqueIndexBeforeBeforeConflict(cached, incoming, []string{"check_id"}))
	assert.False(t, cache.uniqueIndexConflicts(cached, incoming, []string{"check_id"}))
}

func TestUniqueIndexConflicts_BeforeBeforeMissingColumn(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"a", "b"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"a": strPtr("1"), "b": strPtr("2")},
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"a": strPtr("1")},
		Fields:       map[string]*string{"a": strPtr("9"), "b": strPtr("2")},
	}
	assert.False(t, cache.checkUniqueIndexBeforeBeforeConflict(cached, incoming, []string{"a", "b"}))
}

func TestEventsConfict_BeforeBeforeConflict(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"check_id"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "u",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		BeforeFields: map[string]*string{"check_id": strPtr("10")},
		Fields:       map[string]*string{"check_id": strPtr("20")},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.True(t, cache.eventsConfict(cached, incoming))
}

func TestEventsConfict_BothNilBeforeAfter(t *testing.T) {
	cache := newConflictCacheForTest([][]string{{Columns: []string{"email"}}})
	cached := &tgtdb.Event{
		Vsn:          1,
		Op:           "d",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("1")},
		BeforeFields: map[string]*string{"email": nil},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	incoming := &tgtdb.Event{
		Vsn:          2,
		Op:           "c",
		TableNameTup: testTableTuple(),
		Key:          map[string]*string{"id": strPtr("2")},
		Fields:       map[string]*string{"email": nil},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	assert.True(t, cache.eventsConfict(cached, incoming))
}