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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	reporter "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/stats"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type mockYugabyteDB struct {
	tgtdb.TargetYugabyteDB // to satisfy interface
}

func (myb *mockYugabyteDB) ExecuteBatch(migrationUUID uuid.UUID, batch *tgtdb.EventBatch) error {
	return nil
}

func TestProcessEventsBasic(t *testing.T) {
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(0)
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	state := NewImportDataState(exportDir)
	tdb = &mockYugabyteDB{}
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, [][]string](), []chan *tgtdb.Event{evChan}, POSTGRESQL)

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	evChan <- &tgtdb.Event{
		Vsn: 1,
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)
}

// Test that the event is removed from the conflict detection cache after it is processed
// GIVEN: an event is added to to the conflict detection cache, and is added to the event channel
// WHEN: the event is processed, and successfully applied on the target
// THEN: the event should be removed from the conflict detection cache
func TestProcessEventsRemovesEventFromConflicDetectionCache(t *testing.T) {
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(0)
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	state := NewImportDataState(exportDir)
	tdb = &mockYugabyteDB{}
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, [][]string](), []chan *tgtdb.Event{evChan}, POSTGRESQL)

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	e := &tgtdb.Event{
		Vsn: 1,
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}

	conflictDetectionCache.Put(e)
	evChan <- e
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)

	// Check that the event was removed from the cache
	if _, ok := conflictDetectionCache.m[e.Vsn]; ok {
		t.Errorf("Event not removed from conflict detection cache")
	}
}

// Even if event is ignored,
// (because vsn is less than lastAppliedVsn or it is source_db_importer and event is not from target_db_importer_fb),
// it should be removed from conflict detection cache
func TestProcessEventsRemovesIgnoredEventFromConflicDetectionCache(t *testing.T) {
	// to simulate the case where source db importer ignores
	// all events that are not from the target db exporter.
	importerRole = SOURCE_DB_IMPORTER_ROLE
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(100) // so that event with vsn 1 is ignored.
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	state := NewImportDataState(exportDir)
	tdb = &mockYugabyteDB{}
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, [][]string](), []chan *tgtdb.Event{evChan}, POSTGRESQL)

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	e1 := &tgtdb.Event{
		Vsn: 1, // so that it is less than lastAppliedVsn and ignored.
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: TARGET_DB_EXPORTER_FB_ROLE, // so that it is not ignored because importerRole is SOURCE_DB_IMPORTER_ROLE
	}

	e2 := &tgtdb.Event{
		Vsn: 200, // vsn greater than lastAppliedVSn so that is not ignored
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE, // not TARGET_DB_EXPORTER_FB_ROLE so that it is ignored.
	}

	conflictDetectionCache.Put(e1)
	conflictDetectionCache.Put(e2)
	evChan <- e1
	evChan <- e2
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)

	// Check that the event was removed from the cache
	if _, ok := conflictDetectionCache.m[e1.Vsn]; ok {
		t.Errorf("Event %v not removed from conflict detection cache", e1)
	}
	if _, ok := conflictDetectionCache.m[e2.Vsn]; ok {
		t.Errorf("Event %v not removed from conflict detection cache", e2)
	}
}

func TestIsCDCSavepointFixedInTargetDBVersion(t *testing.T) {
	tests := []struct {
		name               string
		dbVersionStr       string
		expectedFixed      bool
		expectedFixVersion string
	}{
		{name: "empty version string", dbVersionStr: "", expectedFixed: false, expectedFixVersion: ""},
		{name: "malformed version string", dbVersionStr: "not-a-version", expectedFixed: false, expectedFixVersion: ""},

		{name: "2024.2 series below fix", dbVersionStr: "11.2-YB-2024.2.7.0-b1", expectedFixed: false, expectedFixVersion: "2024.2.8.0"},
		{name: "2024.2 series exactly at fix", dbVersionStr: "11.2-YB-2024.2.8.0-b85", expectedFixed: true, expectedFixVersion: "2024.2.8.0"},
		{name: "2024.2 series above fix", dbVersionStr: "11.2-YB-2024.2.9.0-b1", expectedFixed: true, expectedFixVersion: "2024.2.8.0"},

		{name: "2025.1 series below fix", dbVersionStr: "11.2-YB-2025.1.3.0-b1", expectedFixed: false, expectedFixVersion: "2025.1.4.0"},
		{name: "2025.1 series exactly at fix", dbVersionStr: "11.2-YB-2025.1.4.0-b42", expectedFixed: true, expectedFixVersion: "2025.1.4.0"},
		{name: "2025.1 series above fix", dbVersionStr: "11.2-YB-2025.1.5.0-b1", expectedFixed: true, expectedFixVersion: "2025.1.4.0"},

		{name: "2025.2 series below fix", dbVersionStr: "11.2-YB-2025.2.1.0-b1", expectedFixed: false, expectedFixVersion: "2025.2.2.0"},
		{name: "2025.2 series exactly at fix", dbVersionStr: "11.2-YB-2025.2.2.0-b10", expectedFixed: true, expectedFixVersion: "2025.2.2.0"},
		{name: "2025.2 series above fix", dbVersionStr: "11.2-YB-2025.2.3.0-b1", expectedFixed: true, expectedFixVersion: "2025.2.2.0"},

		{name: "2024.1 series has no fix entry", dbVersionStr: "11.2-YB-2024.1.5.0-b1", expectedFixed: false, expectedFixVersion: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed, fixVersion := isCDCSavepointFixedInTargetDBVersion(tt.dbVersionStr)
			assert.Equal(t, tt.expectedFixed, fixed)
			assert.Equal(t, tt.expectedFixVersion, fixVersion)
		})
	}
}

func TestShouldWarnServerSeriesNewerThanConnector(t *testing.T) {
	tests := []struct {
		name             string
		connectorVersion string
		serverYBVersion  string
		expectedWarn     bool
		wantErr          bool
	}{
		{"same release, server higher maintenance is NOT newer", "2025.2.3", "2025.2.9.0", false, false},
		{"same release, server higher connector-counter-equivalent is NOT newer", "2025.2.3", "2025.2.4.0", false, false},
		{"same release exact", "2025.2.3", "2025.2.0.0", false, false},
		{"server on newer release warns", "2024.2.5", "2025.1.0.0", true, false},
		{"server on older release does not warn", "2025.2.3", "2024.2.8.0", false, false},
		{"server on unrecognized (newer) release warns", "2025.2.3", "2026.1.0.0", true, false},
		{"connector 2-segment release tag, same release", "2025.2", "2025.2.9.0", false, false},
		{"preview server warns (connector not built for preview)", "2025.2.3", "2.25.1.0", true, false},
		{"stable-old server does not warn (connector is newer and backward-compatible)", "2025.2.3", "2.20.1.0", false, false},
		{"unparseable server version returns error", "2025.2.3", "not-a-version", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warn, _, err := shouldWarnServerSeriesNewerThanConnector(tt.connectorVersion, tt.serverYBVersion)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedWarn, warn)
		})
	}
}

func TestParseCdcPartitionKeyOverrides(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr string
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  map[string]string{},
		},
		{
			name:  "single pk override",
			input: "public.orders:pk",
			want:  map[string]string{"public.orders": "pk"},
		},
		{
			name:  "multiple overrides with semicolon",
			input: "public.orders:table;sales.events:pk",
			want: map[string]string{
				"public.orders": "table",
				"sales.events":  "pk",
			},
		},
		{
			name:  "trims whitespace around entries",
			input: " public.orders : table ; sales.events : pk ",
			want: map[string]string{
				"public.orders": "table",
				"sales.events":  "pk",
			},
		},
		{
			name:  "trailing semicolon ignored",
			input: "public.orders:pk;",
			want:  map[string]string{"public.orders": "pk"},
		},
		{
			name:    "rejects auto strategy",
			input:   "public.orders:auto",
			wantErr: "supported values are pk, table",
		},
		{
			name:    "rejects custom column list",
			input:   "public.orders:customer_id,region",
			wantErr: "supported values are pk, table",
		},
		{
			name:    "rejects missing colon",
			input:   "public.orders",
			wantErr: "expected format",
		},
		{
			name:    "rejects empty strategy",
			input:   "public.orders:",
			wantErr: "non-empty",
		},
		{
			name:    "rejects duplicate table",
			input:   "public.orders:pk;public.orders:table",
			wantErr: "duplicate table",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCdcPartitionKeyOverrides(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func testCdcPartitionNameTuple(schema, table string) sqlname.NameTuple {
	oname := sqlname.NewObjectName(POSTGRESQL, "public", schema, table)
	return sqlname.NameTuple{
		CurrentName: oname,
		SourceName:  oname,
		TargetName:  oname,
	}
}

func strategiesByTableName(m *utils.StructMap[sqlname.NameTuple, string]) map[string]string {
	out := make(map[string]string)
	_ = m.IterKV(func(k sqlname.NameTuple, v string) (bool, error) {
		_, table := k.ForCatalogQuery()
		out[table] = v
		return true, nil
	})
	return out
}

func TestResolveEffectiveCdcPartitionKeys(t *testing.T) {
	orders := testCdcPartitionNameTuple("test_schema", "orders")
	events := testCdcPartitionNameTuple("test_schema", "events")
	audit := testCdcPartitionNameTuple("test_schema", "audit")
	tables := []sqlname.NameTuple{orders, events, audit}

	t.Run("global pk applies to all", func(t *testing.T) {
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, nil, nil, false)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got))
	})

	t.Run("global table applies to all", func(t *testing.T) {
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_TABLE, nil, nil, false)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_TABLE,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))
	})

	t.Run("auto uses pk except expression-UK tables", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		got, err := resolveEffectiveCdcPartitionKeys(tables, "auto", nil, exprUK, false)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))
	})

	t.Run("auto on AMP forces table for all", func(t *testing.T) {
		got, err := resolveEffectiveCdcPartitionKeys(tables, "auto", nil, nil, true)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_TABLE,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))
	})

	t.Run("overlay changes only listed tables", func(t *testing.T) {
		overrides := utils.NewStructMap[sqlname.NameTuple, string]()
		overrides.Put(orders, PARTITION_BY_TABLE)
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, overrides, nil, false)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got), "unlisted tables must keep global strategy")
	})

	t.Run("auto plus override pk on normal table", func(t *testing.T) {
		overrides := utils.NewStructMap[sqlname.NameTuple, string]()
		overrides.Put(events, PARTITION_BY_PK)
		overrides.Put(orders, PARTITION_BY_TABLE)
		got, err := resolveEffectiveCdcPartitionKeys(tables, "auto", overrides, nil, false)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got))
	})

	t.Run("rejects global pk on expression-UK table", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		_, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, nil, exprUK, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
		assert.Contains(t, err.Error(), "audit")
	})

	t.Run("rejects override pk on expression-UK table", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, string]()
		overrides.Put(audit, PARTITION_BY_PK)
		_, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_TABLE, overrides, exprUK, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
	})

	t.Run("override table on expression-UK table is allowed", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, string]()
		overrides.Put(audit, PARTITION_BY_TABLE)
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, overrides, exprUK, false)
		require.NoError(t, err)
		assert.Equal(t, PARTITION_BY_TABLE, strategiesByTableName(got)["audit"])
		assert.Equal(t, PARTITION_BY_PK, strategiesByTableName(got)["orders"])
	})
}
