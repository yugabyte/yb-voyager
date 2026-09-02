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

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	reporter "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/stats"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type mockYugabyteDB struct {
	tgtdb.TargetYugabyteDB // to satisfy interface
	// tableAttrs lets unit tests stub GetListOfTableAttributes (keyed by NameTuple.ForKey())
	// so custom cdc-partition-key column validation/normalization can run without a live
	// target DB.
	tableAttrs map[string][]string
}

func (myb *mockYugabyteDB) ExecuteBatch(migrationUUID uuid.UUID, batch *tgtdb.EventBatch) error {
	return nil
}

func (myb *mockYugabyteDB) GetListOfTableAttributes(tableNameTup sqlname.NameTuple) ([]string, error) {
	if myb.tableAttrs == nil {
		return nil, nil
	}
	return myb.tableAttrs[tableNameTup.ForKey()], nil
}

func (myb *mockYugabyteDB) FindBestMatchingTargetColumnName(columnName string, targetTableColumns []string) (string, error) {
	return myb.FindBestMatchingColumnName(columnName, targetTableColumns)
}

func TestProcessEventsBasic(t *testing.T) {
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(0)
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	state := NewImportDataState(exportDir)
	tdb = &mockYugabyteDB{}
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex](), []chan *tgtdb.Event{evChan}, POSTGRESQL, utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]())

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	evChan <- &tgtdb.Event{
		Vsn: 1,
		Op:  "i",
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
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex](), []chan *tgtdb.Event{evChan}, POSTGRESQL, utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]())

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	e := &tgtdb.Event{
		Vsn: 1,
		Op:  "i",
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
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex](), []chan *tgtdb.Event{evChan}, POSTGRESQL, utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]())

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	e1 := &tgtdb.Event{
		Vsn: 1, // so that it is less than lastAppliedVsn and ignored.
		Op:  "i",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: TARGET_DB_EXPORTER_FB_ROLE, // so that it is not ignored because importerRole is SOURCE_DB_IMPORTER_ROLE
	}

	e2 := &tgtdb.Event{
		Vsn: 200, // vsn greater than lastAppliedVSn so that is not ignored
		Op:  "i",
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
		want    map[string]cdcPartitionKeyOverride
		wantErr string
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]cdcPartitionKeyOverride{},
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  map[string]cdcPartitionKeyOverride{},
		},
		{
			name:  "single pk override",
			input: "public.orders:pk",
			want:  map[string]cdcPartitionKeyOverride{"public.orders": {Strategy: PARTITION_BY_PK}},
		},
		{
			name:  "multiple overrides with semicolon",
			input: "public.orders:table;sales.events:pk",
			want: map[string]cdcPartitionKeyOverride{
				"public.orders": {Strategy: PARTITION_BY_TABLE},
				"sales.events":  {Strategy: PARTITION_BY_PK},
			},
		},
		{
			name:  "trims whitespace around entries",
			input: " public.orders : table ; sales.events : pk ",
			want: map[string]cdcPartitionKeyOverride{
				"public.orders": {Strategy: PARTITION_BY_TABLE},
				"sales.events":  {Strategy: PARTITION_BY_PK},
			},
		},
		{
			name:  "trailing semicolon ignored",
			input: "public.orders:pk;",
			want:  map[string]cdcPartitionKeyOverride{"public.orders": {Strategy: PARTITION_BY_PK}},
		},
		{
			name:  "single custom column",
			input: "public.orders:(customer_id)",
			want: map[string]cdcPartitionKeyOverride{
				"public.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}},
			},
		},
		{
			name:  "multi custom columns",
			input: "public.orders:(customer_id,region)",
			want: map[string]cdcPartitionKeyOverride{
				"public.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id", "region"}},
			},
		},
		{
			name:  "custom columns trim inner whitespace and preserve order",
			input: "public.orders:( region , customer_id )",
			want: map[string]cdcPartitionKeyOverride{
				"public.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"region", "customer_id"}},
			},
		},
		{
			name:  "custom mixed with pk override",
			input: "public.orders:(customer_id);sales.events:pk",
			want: map[string]cdcPartitionKeyOverride{
				"public.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}},
				"sales.events":  {Strategy: PARTITION_BY_PK},
			},
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
			name:    "rejects custom column list without parentheses",
			input:   "public.orders:customer_id",
			wantErr: "parenthesized custom key column list",
		},
		{
			name:    "rejects empty parenthesized custom key",
			input:   "public.orders:()",
			wantErr: "custom key column list is empty",
		},
		{
			name:    "rejects empty column in custom key",
			input:   "public.orders:(customer_id,,region)",
			wantErr: "empty column name",
		},
		{
			name:    "rejects duplicate column in custom key",
			input:   "public.orders:(customer_id,customer_id)",
			wantErr: "duplicate column",
		},
		{
			name:    "rejects duplicate table with conflicting values",
			input:   "public.orders:pk;public.orders:table",
			wantErr: "duplicate table",
		},
		{
			name:    "rejects duplicate table even with same value",
			input:   "public.orders:(customer_id);public.orders:(customer_id)",
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

func strategiesByTableName(m *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) map[string]string {
	out := make(map[string]string)
	_ = m.IterKV(func(k sqlname.NameTuple, v cdcPartitionKeyOverride) (bool, error) {
		_, table := k.ForCatalogQuery()
		out[table] = v.Strategy
		return true, nil
	})
	return out
}

func overrideSpecsByTableName(m *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) map[string]cdcPartitionKeyOverride {
	out := make(map[string]cdcPartitionKeyOverride)
	_ = m.IterKV(func(k sqlname.NameTuple, v cdcPartitionKeyOverride) (bool, error) {
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
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, nil, nil, YUGABYTEDB)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got))
	})

	t.Run("global table applies to all", func(t *testing.T) {
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_TABLE, nil, nil, YUGABYTEDB)
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
		got, err := resolveEffectiveCdcPartitionKeys(tables, "auto", nil, exprUK, YUGABYTEDB)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))
	})

	t.Run("auto on AMP forces table for all", func(t *testing.T) {
		got, err := resolveEffectiveCdcPartitionKeys(tables, "auto", nil, nil, YUGABYTEDB_AMP)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_TABLE,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))
	})

	t.Run("overlay changes only listed tables", func(t *testing.T) {
		overrides := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		overrides.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, overrides, nil, YUGABYTEDB)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got), "unlisted tables must keep global strategy")
	})

	t.Run("auto plus override pk on normal table", func(t *testing.T) {
		overrides := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		overrides.Put(events, cdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
		overrides.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		got, err := resolveEffectiveCdcPartitionKeys(tables, "auto", overrides, nil, YUGABYTEDB)
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
		_, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, nil, exprUK, YUGABYTEDB)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
		assert.Contains(t, err.Error(), "audit")
	})

	t.Run("rejects override pk on expression-UK table", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		overrides.Put(audit, cdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
		_, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_TABLE, overrides, exprUK, YUGABYTEDB)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
	})

	t.Run("rejects override custom on expression-UK table", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		overrides.Put(audit, cdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"col1"}})
		_, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_TABLE, overrides, exprUK, YUGABYTEDB)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
		assert.Contains(t, err.Error(), PARTITION_BY_CUSTOM)
	})

	t.Run("override table on expression-UK table is allowed", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		overrides.Put(audit, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		got, err := resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, overrides, exprUK, YUGABYTEDB)
		require.NoError(t, err)
		assert.Equal(t, PARTITION_BY_TABLE, strategiesByTableName(got)["audit"])
		assert.Equal(t, PARTITION_BY_PK, strategiesByTableName(got)["orders"])
	})
}

// setupCdcOverridesNameRegistry installs an in-memory PG->YB name registry (via a
// JSON file so no DB is needed) with tables test_schema.{orders,events,audit}, and
// restores the previous global registry on cleanup.
func setupCdcOverridesNameRegistry(t *testing.T) {
	t.Helper()
	origNameReg := namereg.NameReg
	origSourceDBType := sqlname.SourceDBType
	t.Cleanup(func() {
		namereg.NameReg = origNameReg
		sqlname.SourceDBType = origSourceDBType
	})
	sqlname.SourceDBType = POSTGRESQL

	dir, err := os.MkdirTemp("", "cdcpk-namereg-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	regFile := filepath.Join(dir, "name_registry.json")
	content := `{
  "SourceDBType": "postgresql",
  "SourceDBSchemaNames": ["test_schema"],
  "DefaultSourceDBSchemaName": "test_schema",
  "SourceDBTableNames": {"test_schema": ["orders", "events", "audit"]},
  "YBSchemaNames": ["test_schema"],
  "DefaultYBSchemaName": "test_schema",
  "YBTableNames": {"test_schema": ["orders", "events", "audit"]}
}`
	require.NoError(t, os.WriteFile(regFile, []byte(content), 0644))
	require.NoError(t, namereg.InitNameRegistry(namereg.NameRegistryParams{
		FilePath: regFile,
		Role:     namereg.TARGET_DB_IMPORTER_ROLE,
	}))
}

func TestResolveCdcPartitionKeyOverrides(t *testing.T) {
	setupCdcOverridesNameRegistry(t)

	lookup := func(name string) sqlname.NameTuple {
		nt, err := namereg.NameReg.LookupTableName(name)
		require.NoError(t, err)
		return nt
	}
	orders := lookup("test_schema.orders")
	events := lookup("test_schema.events")
	importList := []sqlname.NameTuple{orders, events}

	t.Run("valid override resolves", func(t *testing.T) {
		got, err := resolveCdcPartitionKeyOverrides(
			map[string]cdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_TABLE}}, importList)
		require.NoError(t, err)
		assert.Equal(t, map[string]cdcPartitionKeyOverride{"orders": {Strategy: PARTITION_BY_TABLE}}, overrideSpecsByTableName(got))
	})

	t.Run("custom override resolves with columns", func(t *testing.T) {
		got, err := resolveCdcPartitionKeyOverrides(
			map[string]cdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}}}, importList)
		require.NoError(t, err)
		assert.Equal(t, map[string]cdcPartitionKeyOverride{"orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}}}, overrideSpecsByTableName(got))
	})

	t.Run("empty overrides returns empty", func(t *testing.T) {
		got, err := resolveCdcPartitionKeyOverrides(map[string]cdcPartitionKeyOverride{}, importList)
		require.NoError(t, err)
		assert.Empty(t, overrideSpecsByTableName(got))
	})

	t.Run("rejects table not found in name registry", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(
			map[string]cdcPartitionKeyOverride{"test_schema.missing": {Strategy: PARTITION_BY_PK}}, importList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in name registry")
	})

	t.Run("rejects table not in import table list", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(
			map[string]cdcPartitionKeyOverride{"test_schema.events": {Strategy: PARTITION_BY_PK}},
			[]sqlname.NameTuple{orders}) // events excluded from import list
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not in the import table list")
	})

	t.Run("rejects conflicting values across different spellings", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(map[string]cdcPartitionKeyOverride{
			"test_schema.orders":     {Strategy: PARTITION_BY_PK},
			`"test_schema"."orders"`: {Strategy: PARTITION_BY_TABLE},
		}, importList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "specified multiple times")
	})

	t.Run("dedups same value across different spellings", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(map[string]cdcPartitionKeyOverride{
			"test_schema.orders": {Strategy: PARTITION_BY_PK},
			"orders":             {Strategy: PARTITION_BY_PK}, // unqualified resolves to default schema
		}, importList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "specified multiple times")
	})
}

// TestValidateCdcPartitioningStrategyUnchanged covers the semantic resume guard: the
// current flags are re-resolved into an effective per-table strategy and compared against
// the map persisted on the first run. Semantically-equivalent overrides (different
// spelling/quoting/ordering/whitespace) must pass, while any change to the effective
// per-table strategy must be rejected.
func TestValidateCdcPartitioningStrategyUnchanged(t *testing.T) {
	setupCdcOverridesNameRegistry(t)

	origKey := cdcPartitionKey
	origOverrides := cdcPartitionKeyOverrides
	origTargetDBType := tconf.TargetDBType
	origTdb := tdb
	t.Cleanup(func() {
		cdcPartitionKey = origKey
		cdcPartitionKeyOverrides = origOverrides
		tconf.TargetDBType = origTargetDBType
		tdb = origTdb
	})
	tconf.TargetDBType = YUGABYTEDB

	lookup := func(name string) sqlname.NameTuple {
		nt, err := namereg.NameReg.LookupTableName(name)
		require.NoError(t, err)
		return nt
	}
	orders := lookup("test_schema.orders")
	events := lookup("test_schema.events")
	tableNames := []sqlname.NameTuple{orders, events}

	// validateCustomPartitionKeyTables (invoked on both first-run and resume) resolves custom
	// key columns against the target table's attributes, so stub them for the custom-key
	// resume-guard subtests below. Only "orders" is ever routed by a custom key here.
	tdb = &mockYugabyteDB{
		tableAttrs: map[string][]string{
			orders.ForKey(): {"id", "customer_id", "region"},
		},
	}

	// First-run config: global pk with an override putting orders on table. No
	// expression-UK tables (captured as a non-nil empty slice on the first run).
	firstRun, err := resolveEffectiveCdcPartitionKeys(
		tableNames, PARTITION_BY_PK,
		mustResolveOverrides(t, "test_schema.orders:table", tableNames),
		utils.NewStructMap[sqlname.NameTuple, bool](), YUGABYTEDB)
	require.NoError(t, err)

	storedMap := make(map[string]metadb.CDCPartitionKey)
	require.NoError(t, firstRun.IterKV(func(k sqlname.NameTuple, v cdcPartitionKeyOverride) (bool, error) {
		storedMap[k.ForKey()] = metadb.CDCPartitionKey{Strategy: v.Strategy, Columns: v.Columns}
		return true, nil
	}))
	importDataStatus := &metadb.ImportDataStatusRecord{
		ImportDataStarted:              true,
		CdcPartitioningStrategyConfig:  PARTITION_BY_PK,
		CdcPartitionKeyOverridesConfig: "test_schema.orders:table",
		TableToCDCPartitionKey:         storedMap,
		CdcExpressionUniqueIndexTables: []string{}, // captured, none
	}

	t.Run("same config passes", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = "test_schema.orders:table"
		require.NoError(t, validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus))
	})

	t.Run("semantically-equivalent overrides (quoting) pass", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = `"test_schema"."orders":table`
		require.NoError(t, validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus))
	})

	t.Run("semantically-equivalent overrides (whitespace) pass", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = "  test_schema.orders:table ; "
		require.NoError(t, validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus))
	})

	t.Run("changed override target table is rejected", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = "test_schema.events:table"
		err := validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changing cdc-partition-key")
		assert.Contains(t, err.Error(), "events")
	})

	t.Run("removed override is rejected", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = ""
		err := validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changing cdc-partition-key")
		assert.Contains(t, err.Error(), "orders")
	})

	// Custom-key resume guard: the strategy string ("custom") is unchanged across these
	// cases, so the guard must compare the persisted vs new custom key column lists.
	customStatus := func(columns ...string) *metadb.ImportDataStatusRecord {
		return &metadb.ImportDataStatusRecord{
			ImportDataStarted:              true,
			CdcPartitioningStrategyConfig:  PARTITION_BY_PK,
			CdcPartitionKeyOverridesConfig: "test_schema.orders:(" + strings.Join(columns, ",") + ")",
			TableToCDCPartitionKey: map[string]metadb.CDCPartitionKey{
				orders.ForKey(): {Strategy: PARTITION_BY_CUSTOM, Columns: columns},
				events.ForKey(): {Strategy: PARTITION_BY_PK},
			},
			CdcExpressionUniqueIndexTables: []string{},
		}
	}

	t.Run("custom key: same columns (equivalent spelling) pass", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = `"test_schema"."orders":(customer_id)`
		require.NoError(t, validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id")))
	})

	t.Run("custom key: changed column is rejected", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = "test_schema.orders:(region)"
		err := validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changing cdc-partition-key")
		assert.Contains(t, err.Error(), "custom key columns")
		assert.Contains(t, err.Error(), "orders")
	})

	t.Run("custom key: reordered multi-columns are rejected (order is significant)", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = "test_schema.orders:(region,customer_id)"
		err := validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id", "region"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom key columns")
	})

	t.Run("custom key: added column is rejected", func(t *testing.T) {
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = "test_schema.orders:(customer_id,region)"
		err := validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom key columns")
	})
}

func mustResolveOverrides(t *testing.T, raw string, tableNames []sqlname.NameTuple) *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride] {
	t.Helper()
	parsed, err := parseCdcPartitionKeyOverrides(raw)
	require.NoError(t, err)
	resolved, err := resolveCdcPartitionKeyOverrides(parsed, tableNames)
	require.NoError(t, err)
	return resolved
}

// TestValidateCdcPartitionKeyFlags covers the flag-level guardrails in
// validateCdcPartitionKeyFlags: context restrictions (target/YB/PG/streaming),
// value validation, and the resume change-guards (including the positive
// same-config and start-clean bypass paths).
func TestValidateCdcPartitionKeyFlags(t *testing.T) {
	origImporterRole := importerRole
	origTargetDBType := tconf.TargetDBType
	origSourceDBType := sourceDBType
	origImportType := importType
	origKey := cdcPartitionKey
	origOverrides := cdcPartitionKeyOverrides
	origStartClean := startClean
	origMetaDB := metaDB
	t.Cleanup(func() {
		importerRole = origImporterRole
		tconf.TargetDBType = origTargetDBType
		sourceDBType = origSourceDBType
		importType = origImportType
		cdcPartitionKey = origKey
		cdcPartitionKeyOverrides = origOverrides
		startClean = origStartClean
		metaDB = origMetaDB
	})

	// newCmd registers the two flags bound to the package globals. StringVar resets
	// the globals to their defaults, so callers must set globals AFTER calling this.
	newCmd := func() *cobra.Command {
		c := &cobra.Command{Use: "import-data-test"}
		c.Flags().StringVar(&cdcPartitionKey, "cdc-partition-key", "auto", "")
		c.Flags().StringVar(&cdcPartitionKeyOverrides, "cdc-partition-key-overrides", "", "")
		return c
	}
	setValidContext := func() {
		importerRole = TARGET_DB_IMPORTER_ROLE
		tconf.TargetDBType = YUGABYTEDB
		sourceDBType = POSTGRESQL
		importType = SNAPSHOT_AND_CHANGES
		startClean = false
	}
	seedMetaDB := func(t *testing.T, mutate func(*metadb.ImportDataStatusRecord)) {
		dir, err := os.MkdirTemp("", "cdcpk-metadb-*")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		metaDB = CreateMigrationProjectIfNotExists(POSTGRESQL, dir)
		if mutate != nil {
			require.NoError(t, metaDB.UpdateImportDataStatusRecord(mutate))
		}
	}

	t.Run("rejected for non-target importer role", func(t *testing.T) {
		setValidContext()
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only supported for import data to target")
	})

	t.Run("rejected for non-yugabytedb target", func(t *testing.T) {
		setValidContext()
		tconf.TargetDBType = POSTGRESQL
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only supported for import data to target")
	})

	t.Run("allowed (no-op) for non-target when no flags passed", func(t *testing.T) {
		setValidContext()
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
		cmd := newCmd() // no flags Set -> not passed
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})

	t.Run("rejected for offline migration", func(t *testing.T) {
		setValidContext()
		importType = SNAPSHOT_ONLY
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported for offline migration")
	})

	t.Run("rejected for non-postgres source", func(t *testing.T) {
		setValidContext()
		sourceDBType = ORACLE
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only supported for PostgreSQL source")
	})

	t.Run("rejects empty cdc-partition-key", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, nil)
		cmd := newCmd()
		cdcPartitionKey = ""
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cdc-partition-key is required")
	})

	t.Run("rejects invalid cdc-partition-key value", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, nil)
		cmd := newCmd()
		cdcPartitionKey = "foo"
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cdc-partition-key")
	})

	t.Run("valid fresh start passes", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, nil) // no import started yet
		cmd := newCmd()
		cdcPartitionKey = "pk"
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})

	t.Run("resume with same config passes", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = PARTITION_BY_PK
			r.CdcPartitionKeyOverridesConfig = "test_schema.orders:table"
		})
		cmd := newCmd()
		cdcPartitionKey = PARTITION_BY_PK
		cdcPartitionKeyOverrides = "test_schema.orders:table"
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})

	t.Run("resume rejects changed global key", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = "auto"
		})
		cmd := newCmd()
		cdcPartitionKey = PARTITION_BY_PK
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changing cdc-partition-key is not allowed")
	})

	// NOTE: changed cdc-partition-key-overrides is no longer rejected here by a raw string
	// compare; the semantic per-table comparison is covered by
	// TestValidateCdcPartitioningStrategyUnchanged.

	t.Run("resume from older version without stored strategy is rejected", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = ""
		})
		cmd := newCmd()
		cdcPartitionKey = PARTITION_BY_PK
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Resuming from an earlier version")
	})

	t.Run("start-clean bypasses resume change-guards", func(t *testing.T) {
		setValidContext()
		startClean = true
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = "auto"
		})
		cmd := newCmd()
		cdcPartitionKey = PARTITION_BY_PK
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})
}

// TestHashEventCustomKey covers PARTITION_BY_CUSTOM routing in hashEvent: events of the
// same row (insert/update/delete) route to the same channel via the immutable custom key
// (read from BeforeFields for update/delete, Fields for insert), multi-column ordering is
// respected, NULLs are handled deterministically, and a missing key column errors.
func TestHashEventCustomKey(t *testing.T) {
	sp := func(s string) *string { return &s }

	orders := testCdcPartitionNameTuple("test_schema", "orders")
	singleColMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	singleColMap.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}})

	// insert: key value in Fields (BeforeFields nil)
	insertEvent := &tgtdb.Event{
		Vsn: 1, Op: "c", TableNameTup: orders,
		Key:    map[string]*string{"id": sp("1")},
		Fields: map[string]*string{"id": sp("1"), "customer_id": sp("C1"), "amount": sp("10")},
	}
	// update on the same row: key value only in BeforeFields (immutable key not in Fields)
	updateEvent := &tgtdb.Event{
		Vsn: 2, Op: "u", TableNameTup: orders,
		Key:          map[string]*string{"id": sp("1")},
		Fields:       map[string]*string{"id": sp("1"), "amount": sp("20")},
		BeforeFields: map[string]*string{"id": sp("1"), "customer_id": sp("C1"), "amount": sp("10")},
	}
	// delete on the same row: Fields carries PK only, BeforeFields has the full row
	deleteEvent := &tgtdb.Event{
		Vsn: 3, Op: "d", TableNameTup: orders,
		Key:          map[string]*string{"id": sp("1")},
		Fields:       map[string]*string{"id": sp("1")},
		BeforeFields: map[string]*string{"id": sp("1"), "customer_id": sp("C1"), "amount": sp("10")},
	}

	insertHash, err := hashEvent(insertEvent, singleColMap)
	require.NoError(t, err)
	updateHash, err := hashEvent(updateEvent, singleColMap)
	require.NoError(t, err)
	deleteHash, err := hashEvent(deleteEvent, singleColMap)
	require.NoError(t, err)

	assert.Equal(t, insertHash, updateHash, "same custom key value must route to same channel (insert vs update)")
	assert.Equal(t, insertHash, deleteHash, "same custom key value must route to same channel (insert vs delete)")
	assert.GreaterOrEqual(t, insertHash, 0)
	assert.Less(t, insertHash, NUM_EVENT_CHANNELS)

	t.Run("multi-column order is respected", func(t *testing.T) {
		multiColMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		multiColMap.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id", "region"}})

		ev := &tgtdb.Event{
			Vsn: 10, Op: "c", TableNameTup: orders,
			Fields: map[string]*string{"customer_id": sp("C1"), "region": sp("US")},
		}
		h1, err := hashEvent(ev, multiColMap)
		require.NoError(t, err)
		// deterministic for the same input
		h2, err := hashEvent(ev, multiColMap)
		require.NoError(t, err)
		assert.Equal(t, h1, h2)

		// reversed column order is a different key ordering; usually a different channel.
		reversedColMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		reversedColMap.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"region", "customer_id"}})
		hRev, err := hashEvent(ev, reversedColMap)
		require.NoError(t, err)
		_ = hRev // no strict inequality assertion to avoid rare hash collisions
	})

	t.Run("null key value is handled deterministically", func(t *testing.T) {
		ev := &tgtdb.Event{
			Vsn: 20, Op: "c", TableNameTup: orders,
			Fields: map[string]*string{"customer_id": nil, "amount": sp("5")},
		}
		h1, err := hashEvent(ev, singleColMap)
		require.NoError(t, err)
		h2, err := hashEvent(ev, singleColMap)
		require.NoError(t, err)
		assert.Equal(t, h1, h2)
	})

	t.Run("missing custom key column errors", func(t *testing.T) {
		ev := &tgtdb.Event{
			Vsn: 30, Op: "c", TableNameTup: orders,
			Fields: map[string]*string{"id": sp("1"), "amount": sp("5")}, // no customer_id anywhere
		}
		_, err := hashEvent(ev, singleColMap)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "customer_id")
	})

	t.Run("missing custom columns map entry errors", func(t *testing.T) {
		emptyColMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
		emptyColMap.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM})
		_, err := hashEvent(insertEvent, emptyColMap)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom partition key columns not found")
	})
}

func TestGetEventPartitionKey(t *testing.T) {
	sp := func(s string) *string { return &s }
	orders := testCdcPartitionNameTuple("test_schema", "orders")

	pkMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	pkMap.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
	tableMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	tableMap.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
	customMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	customMap.Put(orders, cdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}})

	t.Run("pk: same PK -> same key, different PK -> different key", func(t *testing.T) {
		e1 := &tgtdb.Event{Vsn: 1, Op: "u", TableNameTup: orders, Key: map[string]*string{"id": sp("1")}}
		e2 := &tgtdb.Event{Vsn: 2, Op: "u", TableNameTup: orders, Key: map[string]*string{"id": sp("1")}}
		e3 := &tgtdb.Event{Vsn: 3, Op: "u", TableNameTup: orders, Key: map[string]*string{"id": sp("2")}}
		k1, err := GetEventPartitionKey(e1, pkMap)
		require.NoError(t, err)
		k2, err := GetEventPartitionKey(e2, pkMap)
		require.NoError(t, err)
		k3, err := GetEventPartitionKey(e3, pkMap)
		require.NoError(t, err)
		assert.Equal(t, k1, k2)
		assert.NotEqual(t, k1, k3)
	})

	t.Run("table: all events share one key", func(t *testing.T) {
		e1 := &tgtdb.Event{Vsn: 1, Op: "u", TableNameTup: orders, Key: map[string]*string{"id": sp("1")}}
		e2 := &tgtdb.Event{Vsn: 2, Op: "u", TableNameTup: orders, Key: map[string]*string{"id": sp("999")}}
		k1, err := GetEventPartitionKey(e1, tableMap)
		require.NoError(t, err)
		k2, err := GetEventPartitionKey(e2, tableMap)
		require.NoError(t, err)
		assert.Equal(t, k1, k2)
	})

	t.Run("custom: same custom value -> same key regardless of PK/op", func(t *testing.T) {
		insert := &tgtdb.Event{Vsn: 1, Op: "c", TableNameTup: orders,
			Key:    map[string]*string{"id": sp("1")},
			Fields: map[string]*string{"id": sp("1"), "customer_id": sp("C1")}}
		update := &tgtdb.Event{Vsn: 2, Op: "u", TableNameTup: orders,
			Key:          map[string]*string{"id": sp("2")},
			Fields:       map[string]*string{"id": sp("2"), "amount": sp("5")},
			BeforeFields: map[string]*string{"id": sp("2"), "customer_id": sp("C1")}}
		different := &tgtdb.Event{Vsn: 3, Op: "c", TableNameTup: orders,
			Key:    map[string]*string{"id": sp("3")},
			Fields: map[string]*string{"id": sp("3"), "customer_id": sp("C2")}}
		kInsert, err := GetEventPartitionKey(insert, customMap)
		require.NoError(t, err)
		kUpdate, err := GetEventPartitionKey(update, customMap)
		require.NoError(t, err)
		kDifferent, err := GetEventPartitionKey(different, customMap)
		require.NoError(t, err)
		assert.Equal(t, kInsert, kUpdate, "same custom key value must yield the same partition key")
		assert.NotEqual(t, kInsert, kDifferent)
	})

	t.Run("custom: missing column errors", func(t *testing.T) {
		ev := &tgtdb.Event{Vsn: 1, Op: "c", TableNameTup: orders,
			Fields: map[string]*string{"id": sp("1")}}
		_, err := GetEventPartitionKey(ev, customMap)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "customer_id")
	})

	t.Run("hashEvent routes identically to partition key equality", func(t *testing.T) {
		e1 := &tgtdb.Event{Vsn: 1, Op: "c", TableNameTup: orders,
			Key: map[string]*string{"id": sp("1")}, Fields: map[string]*string{"customer_id": sp("C1")}}
		e2 := &tgtdb.Event{Vsn: 2, Op: "u", TableNameTup: orders,
			Key: map[string]*string{"id": sp("2")}, BeforeFields: map[string]*string{"customer_id": sp("C1")}}
		h1, err := hashEvent(e1, customMap)
		require.NoError(t, err)
		h2, err := hashEvent(e2, customMap)
		require.NoError(t, err)
		assert.Equal(t, h1, h2, "same custom partition key must route to the same channel")
	})
}
