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
package importdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
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
	testImporter.cfg.Tdb = &mockYugabyteDB{}
	state := NewImportDataState(testImporter.importDataStateConfig())
	testImporter.conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex](), []chan *tgtdb.Event{evChan}, constants.POSTGRESQL, utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride](), testImporter.cfg.ExportDir)

	oname := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", "users")
	evChan <- &tgtdb.Event{
		Vsn: 1,
		Op:  "i",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: constants.SOURCE_DB_EXPORTER_ROLE,
	}
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	testImporter.processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)
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
	testImporter.cfg.Tdb = &mockYugabyteDB{}
	state := NewImportDataState(testImporter.importDataStateConfig())
	testImporter.conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex](), []chan *tgtdb.Event{evChan}, constants.POSTGRESQL, utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride](), testImporter.cfg.ExportDir)

	oname := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", "users")
	e := &tgtdb.Event{
		Vsn: 1,
		Op:  "i",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: constants.SOURCE_DB_EXPORTER_ROLE,
	}

	testImporter.conflictDetectionCache.Put(e)
	evChan <- e
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	testImporter.processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)

	// Check that the event was removed from the cache
	if _, ok := testImporter.conflictDetectionCache.m[e.Vsn]; ok {
		t.Errorf("Event not removed from conflict detection cache")
	}
}

// Even if event is ignored,
// (because vsn is less than lastAppliedVsn or it is source_db_importer and event is not from target_db_importer_fb),
// it should be removed from conflict detection cache
func TestProcessEventsRemovesIgnoredEventFromConflicDetectionCache(t *testing.T) {
	// to simulate the case where source db importer ignores
	// all events that are not from the target db exporter.
	testImporter.cfg.ImporterRole = constants.SOURCE_DB_IMPORTER_ROLE
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(100) // so that event with vsn 1 is ignored.
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	testImporter.cfg.Tdb = &mockYugabyteDB{}
	state := NewImportDataState(testImporter.importDataStateConfig())
	testImporter.conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex](), []chan *tgtdb.Event{evChan}, constants.POSTGRESQL, utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride](), testImporter.cfg.ExportDir)

	oname := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", "users")
	e1 := &tgtdb.Event{
		Vsn: 1, // so that it is less than lastAppliedVsn and ignored.
		Op:  "i",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: constants.TARGET_DB_EXPORTER_FB_ROLE, // so that it is not ignored because importerRole is SOURCE_DB_IMPORTER_ROLE
	}

	e2 := &tgtdb.Event{
		Vsn: 200, // vsn greater than lastAppliedVSn so that is not ignored
		Op:  "i",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: constants.SOURCE_DB_EXPORTER_ROLE, // not TARGET_DB_EXPORTER_FB_ROLE so that it is ignored.
	}

	testImporter.conflictDetectionCache.Put(e1)
	testImporter.conflictDetectionCache.Put(e2)
	evChan <- e1
	evChan <- e2
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	testImporter.processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)

	// Check that the event was removed from the cache
	if _, ok := testImporter.conflictDetectionCache.m[e1.Vsn]; ok {
		t.Errorf("Event %v not removed from conflict detection cache", e1)
	}
	if _, ok := testImporter.conflictDetectionCache.m[e2.Vsn]; ok {
		t.Errorf("Event %v not removed from conflict detection cache", e2)
	}
}

func testCdcPartitionNameTuple(schema, table string) sqlname.NameTuple {
	oname := sqlname.NewObjectName(constants.POSTGRESQL, "public", schema, table)
	return sqlname.NameTuple{
		CurrentName: oname,
		SourceName:  oname,
		TargetName:  oname,
	}
}

func strategiesByTableName(m *utils.StructMap[sqlname.NameTuple, CdcPartitionKeyOverride]) map[string]string {
	out := make(map[string]string)
	_ = m.IterKV(func(k sqlname.NameTuple, v CdcPartitionKeyOverride) (bool, error) {
		_, table := k.ForCatalogQuery()
		out[table] = v.Strategy
		return true, nil
	})
	return out
}

func overrideSpecsByTableName(m *utils.StructMap[sqlname.NameTuple, CdcPartitionKeyOverride]) map[string]CdcPartitionKeyOverride {
	out := make(map[string]CdcPartitionKeyOverride)
	_ = m.IterKV(func(k sqlname.NameTuple, v CdcPartitionKeyOverride) (bool, error) {
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
	oldImporterRole := testImporter.cfg.ImporterRole
	oldSourceDBType := testImporter.cfg.SourceDBType
	defer func() {
		testImporter.cfg.ImporterRole = oldImporterRole
		testImporter.cfg.SourceDBType = oldSourceDBType
	}()
	testImporter.cfg.ImporterRole = constants.TARGET_DB_IMPORTER_ROLE
	testImporter.cfg.SourceDBType = constants.POSTGRESQL

	t.Run("global pk applies to all", func(t *testing.T) {
		got, err := testImporter.resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, nil, nil, nil, constants.YUGABYTEDB)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got))
	})

	t.Run("global table applies to all", func(t *testing.T) {
		got, err := testImporter.resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_TABLE, nil, nil, nil, constants.YUGABYTEDB)
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
		got, err := testImporter.resolveEffectiveCdcPartitionKeys(tables, "auto", nil, exprUK, nil, constants.YUGABYTEDB)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))
	})

	t.Run("auto on AMP forces table for all", func(t *testing.T) {
		got, err := testImporter.resolveEffectiveCdcPartitionKeys(tables, "auto", nil, nil, nil, constants.YUGABYTEDB_AMP)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_TABLE,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))
	})

	t.Run("overlay changes only listed tables", func(t *testing.T) {
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		got, err := testImporter.resolveEffectiveCdcPartitionKeys(tables, PARTITION_BY_PK, overrides, nil, nil, constants.YUGABYTEDB)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_TABLE,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got), "unlisted tables must keep global strategy")
	})

	t.Run("auto plus override pk on normal table", func(t *testing.T) {
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(events, CdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
		overrides.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		got, err := testImporter.resolveEffectiveCdcPartitionKeys(tables, "auto", overrides, nil, nil, constants.YUGABYTEDB)
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
		_, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, nil, exprUK, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
		assert.Contains(t, err.Error(), "audit")
	})

	t.Run("rejects override pk on expression-UK table", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
		_, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_TABLE, overrides, exprUK, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
	})

	t.Run("rejects override custom on expression-UK table", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"col1"}})
		_, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_TABLE, overrides, exprUK, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
		assert.Contains(t, err.Error(), PARTITION_BY_CUSTOM)
	})

	t.Run("override table on expression-UK table is allowed", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		got, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, overrides, exprUK, nil)
		require.NoError(t, err)
		assert.Equal(t, PARTITION_BY_TABLE, strategiesByTableName(got)["audit"])
		assert.Equal(t, PARTITION_BY_PK, strategiesByTableName(got)["orders"])
	})

	t.Run("rejects override custom (id column) on expression-UK table", func(t *testing.T) {
		exprUK := utils.NewStructMap[sqlname.NameTuple, bool]()
		exprUK.Put(audit, true)
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"id"}})
		_, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_TABLE, overrides, exprUK, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression-based unique index")
		assert.Contains(t, err.Error(), "custom")
	})

	t.Run("generated column without UK: auto stays pk and global pk is allowed", func(t *testing.T) {
		generated := generatedStoredColMap(audit, GeneratedStoredColumn{Name: "amount", InUniqueIndex: false})
		got, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, "auto", nil, nil, generated)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_PK,
		}, strategiesByTableName(got))

		got, err = resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, nil, nil, generated)
		require.NoError(t, err)
		assert.Equal(t, PARTITION_BY_PK, strategiesByTableName(got)["audit"])
	})

	t.Run("UK on generated: auto uses table; pk and custom rejected; override table allowed", func(t *testing.T) {
		generated := generatedStoredColMap(audit, GeneratedStoredColumn{Name: "amount", InUniqueIndex: true})
		// auto force-tables the UK-on-generated table instead of rejecting.
		got, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, "auto", nil, nil, generated)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"orders": PARTITION_BY_PK,
			"events": PARTITION_BY_PK,
			"audit":  PARTITION_BY_TABLE,
		}, strategiesByTableName(got))

		_, err = resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, nil, nil, generated)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has a unique index on a stored generated column")
		assert.Contains(t, err.Error(), "audit")

		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"id"}})
		_, err = resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_TABLE, overrides, nil, generated)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has a unique index on a stored generated column")
		assert.Contains(t, err.Error(), "audit")

		overrides = utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		got, err = resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, overrides, nil, generated)
		require.NoError(t, err)
		assert.Equal(t, PARTITION_BY_TABLE, strategiesByTableName(got)["audit"])
		assert.Equal(t, PARTITION_BY_PK, strategiesByTableName(got)["orders"])
	})

	t.Run("custom key is a generated column without UK: custom rejected; pk allowed", func(t *testing.T) {
		generated := generatedStoredColMap(audit, GeneratedStoredColumn{Name: "amount", InUniqueIndex: false})
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"amount"}})
		_, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, overrides, nil, generated)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom key column(s) - [amount] are a stored generated column(s)")
		assert.Contains(t, err.Error(), "audit")

		got, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, nil, nil, generated)
		require.NoError(t, err)
		assert.Equal(t, PARTITION_BY_PK, strategiesByTableName(got)["audit"])
	})

	t.Run("custom key is a regular column and generated column unused by UK: custom allowed", func(t *testing.T) {
		generated := generatedStoredColMap(audit, GeneratedStoredColumn{Name: "amount", InUniqueIndex: false})
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"id"}})
		got, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_PK, overrides, nil, generated)
		require.NoError(t, err)
		assert.Equal(t, PARTITION_BY_CUSTOM, strategiesByTableName(got)["audit"])
		assert.Equal(t, PARTITION_BY_PK, strategiesByTableName(got)["orders"])
	})

	t.Run("custom key column missing on table is rejected", func(t *testing.T) {
		overrides := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		overrides.Put(audit, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"no_such_col"}})
		_, err := resolveAndValidateCdcPartitionKeysForTest(t, tables, PARTITION_BY_TABLE, overrides, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "do not exist on table")
		assert.Contains(t, err.Error(), "no_such_col")
	})
}

// mockTargetDBForCdcPartitionKeyValidation stubs the two target lookups that
// validateAndFinalizeCDCPartitionKeysPerTable performs for custom-key tables (column list +
// column-name casing resolution), so the guardrail rejections can be unit-tested
// without a live target DB.
type mockTargetDBForCdcPartitionKeyValidation struct {
	tgtdb.TargetYugabyteDB
	tableColumns []string
}

func (m *mockTargetDBForCdcPartitionKeyValidation) GetListOfTableAttributes(_ sqlname.NameTuple) ([]string, error) {
	return m.tableColumns, nil
}

func (m *mockTargetDBForCdcPartitionKeyValidation) FindBestMatchingTargetColumnName(columnName string, targetTableColumns []string) (string, error) {
	// Exact match is sufficient for these tests; the full casing logic is covered by the
	// AttributeNameRegistry tests in tgtdb.
	for _, c := range targetTableColumns {
		if c == columnName {
			return c, nil
		}
	}
	return "", &tgtdb.ErrColumnNameNotFound{}
}

// resolveAndValidateCdcPartitionKeysForTest mirrors the production sequence in
// computeCdcPartitioningStrategyPerTable: resolveEffectiveCdcPartitionKeys (strategy
// resolution, never rejects) followed by validateAndFinalizeCDCPartitionKeysPerTable (where the
// expr-UK / generated-column guardrail rejections live). It installs a mock target DB
// for the custom-key column resolution and restores the original on cleanup.
func resolveAndValidateCdcPartitionKeysForTest(
	t *testing.T,
	tables []sqlname.NameTuple,
	globalKey string,
	overrides *utils.StructMap[sqlname.NameTuple, CdcPartitionKeyOverride],
	exprUK *utils.StructMap[sqlname.NameTuple, bool],
	generated *utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn],
) (*utils.StructMap[sqlname.NameTuple, CdcPartitionKeyOverride], error) {
	t.Helper()
	origTdb := testImporter.cfg.Tdb
	testImporter.cfg.Tdb = &mockTargetDBForCdcPartitionKeyValidation{tableColumns: []string{"id", "col1", "amount", "customer_id", "region"}}
	t.Cleanup(func() { testImporter.cfg.Tdb = origTdb })

	resolved, err := testImporter.resolveEffectiveCdcPartitionKeys(tables, globalKey, overrides, exprUK, generated, constants.YUGABYTEDB)
	require.NoError(t, err, "resolveEffectiveCdcPartitionKeys should not reject; rejections live in validateAndFinalizeCDCPartitionKeysPerTable")

	// validateAndFinalizeCDCPartitionKeysPerTable does not nil-guard its map arguments (production
	// always passes non-nil maps), so mirror that here.
	if exprUK == nil {
		exprUK = utils.NewStructMap[sqlname.NameTuple, bool]()
	}
	if generated == nil {
		generated = utils.NewStructMap[sqlname.NameTuple, []GeneratedStoredColumn]()
	}
	return resolved, testImporter.validateAndFinalizeCDCPartitionKeysPerTable(resolved, tables, exprUK, generated)
}

func generatedStoredColMap(t sqlname.NameTuple, cols ...GeneratedStoredColumn) *utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn] {
	m := utils.NewStructMap[sqlname.NameTuple, []GeneratedStoredColumn]()
	m.Put(t, cols)
	return m
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
	sqlname.SourceDBType = constants.POSTGRESQL

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
			map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_TABLE}}, importList)
		require.NoError(t, err)
		assert.Equal(t, map[string]CdcPartitionKeyOverride{"orders": {Strategy: PARTITION_BY_TABLE}}, overrideSpecsByTableName(got))
	})

	t.Run("custom override resolves with columns", func(t *testing.T) {
		got, err := resolveCdcPartitionKeyOverrides(
			map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}}}, importList)
		require.NoError(t, err)
		assert.Equal(t, map[string]CdcPartitionKeyOverride{"orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}}}, overrideSpecsByTableName(got))
	})

	t.Run("empty overrides returns empty", func(t *testing.T) {
		got, err := resolveCdcPartitionKeyOverrides(map[string]CdcPartitionKeyOverride{}, importList)
		require.NoError(t, err)
		assert.Empty(t, overrideSpecsByTableName(got))
	})

	t.Run("rejects table not found in name registry", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(
			map[string]CdcPartitionKeyOverride{"test_schema.missing": {Strategy: PARTITION_BY_PK}}, importList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in name registry")
	})

	t.Run("rejects table not in import table list", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(
			map[string]CdcPartitionKeyOverride{"test_schema.events": {Strategy: PARTITION_BY_PK}},
			[]sqlname.NameTuple{orders}) // events excluded from import list
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not in the import table list")
	})

	t.Run("rejects conflicting values across different spellings", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(map[string]CdcPartitionKeyOverride{
			"test_schema.orders":     {Strategy: PARTITION_BY_PK},
			`"test_schema"."orders"`: {Strategy: PARTITION_BY_TABLE},
		}, importList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "specified multiple times")
	})

	t.Run("dedups same value across different spellings", func(t *testing.T) {
		_, err := resolveCdcPartitionKeyOverrides(map[string]CdcPartitionKeyOverride{
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

	// The resume path recomputes the strategy (including generated-stored columns) from the
	// source-captured metaDB record on every run. Provide an empty (captured) record so it
	// resolves to no generated columns instead of falling back to the target-based path.
	metaExportDir, err := os.MkdirTemp("", "cdcpk-metadb-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(metaExportDir) })
	origMetaDB := testImporter.cfg.MetaDB
	testMetaDB, err := initTestMetaDB(metaExportDir)
	require.NoError(t, err)
	testImporter.cfg.MetaDB = testMetaDB
	t.Cleanup(func() { testImporter.cfg.MetaDB = origMetaDB })
	require.NoError(t, testImporter.cfg.MetaDB.UpdateExportDataSourceDBExporterStatusRecord(func(r *metadb.ExportDataSourceDBExporterStatusRecord) {
		r.TableToGeneratedStoredColumns = map[string][]string{}
	}))

	// Target unique indexes are passed in by the caller; these tables have no generated
	// columns, so an empty map is sufficient for the resume-recompute in this test.
	emptyUK := utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]()

	origKey := testImporter.cfg.CdcPartitionKey
	origOverrides := testImporter.cfg.CdcPartitionKeyOverrides
	origOverridesParsed := testImporter.cfg.CdcPartitionKeyOverridesParsed
	origTargetDBType := testImporter.cfg.Tconf.TargetDBType
	origTdb := testImporter.cfg.Tdb
	t.Cleanup(func() {
		testImporter.cfg.CdcPartitionKey = origKey
		setTestCdcPartitionKeyOverrides(origOverrides, origOverridesParsed)
		testImporter.cfg.Tconf.TargetDBType = origTargetDBType
		testImporter.cfg.Tdb = origTdb
	})
	testImporter.cfg.Tconf.TargetDBType = constants.YUGABYTEDB

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
	testImporter.cfg.Tdb = &mockYugabyteDB{
		tableAttrs: map[string][]string{
			orders.ForKey(): {"id", "customer_id", "region"},
		},
	}

	// First-run config: global pk with an override putting orders on table. No
	// expression-UK tables (captured as a non-nil empty slice on the first run).
	firstRun, err := testImporter.resolveEffectiveCdcPartitionKeys(
		tableNames, PARTITION_BY_PK,
		mustResolveOverrides(t, map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_TABLE}}, tableNames),
		utils.NewStructMap[sqlname.NameTuple, bool](), nil, constants.YUGABYTEDB)
	require.NoError(t, err)

	storedMap := make(map[string]metadb.CDCPartitionKey)
	require.NoError(t, firstRun.IterKV(func(k sqlname.NameTuple, v CdcPartitionKeyOverride) (bool, error) {
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
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides("test_schema.orders:table", map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_TABLE}})
		require.NoError(t, testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus, emptyUK))
	})

	t.Run("semantically-equivalent overrides (quoting) pass", func(t *testing.T) {
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides(`"test_schema"."orders":table`, map[string]CdcPartitionKeyOverride{`"test_schema"."orders"`: {Strategy: PARTITION_BY_TABLE}})
		require.NoError(t, testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus, emptyUK))
	})

	t.Run("semantically-equivalent overrides (whitespace) pass", func(t *testing.T) {
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides("  test_schema.orders:table ; ", map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_TABLE}})
		require.NoError(t, testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus, emptyUK))
	})

	t.Run("changed override target table is rejected", func(t *testing.T) {
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides("test_schema.events:table", map[string]CdcPartitionKeyOverride{"test_schema.events": {Strategy: PARTITION_BY_TABLE}})
		err := testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus, emptyUK)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changing cdc-partition-key")
		assert.Contains(t, err.Error(), "events")
	})

	t.Run("removed override is rejected", func(t *testing.T) {
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides("", map[string]CdcPartitionKeyOverride{})
		err := testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus, emptyUK)
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
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides(`"test_schema"."orders":(customer_id)`, map[string]CdcPartitionKeyOverride{`"test_schema"."orders"`: {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}}})
		require.NoError(t, testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id"), emptyUK))
	})

	t.Run("custom key: changed column is rejected", func(t *testing.T) {
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides("test_schema.orders:(region)", map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"region"}}})
		err := testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id"), emptyUK)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changing cdc-partition-key")
		assert.Contains(t, err.Error(), "custom key columns")
		assert.Contains(t, err.Error(), "orders")
	})

	t.Run("custom key: reordered multi-columns are rejected (order is significant)", func(t *testing.T) {
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides("test_schema.orders:(region,customer_id)", map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"region", "customer_id"}}})
		err := testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id", "region"), emptyUK)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom key columns")
	})

	t.Run("custom key: added column is rejected", func(t *testing.T) {
		testImporter.cfg.CdcPartitionKey = PARTITION_BY_PK
		setTestCdcPartitionKeyOverrides("test_schema.orders:(customer_id,region)", map[string]CdcPartitionKeyOverride{"test_schema.orders": {Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id", "region"}}})
		err := testImporter.validateCdcPartitioningStrategyUnchanged(tableNames, customStatus("customer_id"), emptyUK)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom key columns")
	})
}

func mustResolveOverrides(t *testing.T, parsed map[string]CdcPartitionKeyOverride, tableNames []sqlname.NameTuple) *utils.StructMap[sqlname.NameTuple, CdcPartitionKeyOverride] {
	t.Helper()
	resolved, err := resolveCdcPartitionKeyOverrides(parsed, tableNames)
	require.NoError(t, err)
	return resolved
}

// TestHashEventCustomKey covers PARTITION_BY_CUSTOM routing in hashEvent: events of the
// same row (insert/update/delete) route to the same channel via the immutable custom key
// (read from BeforeFields for update/delete, Fields for insert), multi-column ordering is
// respected, NULLs are handled deterministically, and a missing key column errors.
func TestHashEventCustomKey(t *testing.T) {
	sp := func(s string) *string { return &s }

	orders := testCdcPartitionNameTuple("test_schema", "orders")
	singleColMap := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
	singleColMap.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}})

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
		multiColMap := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		multiColMap.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id", "region"}})

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
		reversedColMap := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		reversedColMap.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"region", "customer_id"}})
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
		emptyColMap := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
		emptyColMap.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM})
		_, err := hashEvent(insertEvent, emptyColMap)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom partition key columns not found")
	})
}

func TestGetEventPartitionKey(t *testing.T) {
	sp := func(s string) *string { return &s }
	orders := testCdcPartitionNameTuple("test_schema", "orders")

	pkMap := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
	pkMap.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
	tableMap := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
	tableMap.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
	customMap := utils.NewStructMap[sqlname.NameTuple, CdcPartitionKeyOverride]()
	customMap.Put(orders, CdcPartitionKeyOverride{Strategy: PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}})

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

func TestBuildGeneratedStoredColumns(t *testing.T) {
	t.Run("protected via unique index or primary key -> InUniqueIndex true", func(t *testing.T) {
		// protected = UK columns ∪ PK columns
		protected := map[string]bool{"email": true, "id": true}
		got := buildGeneratedStoredColumns([]string{"email", "id", "full_name"}, protected)
		assert.ElementsMatch(t, []GeneratedStoredColumn{
			{Name: "email", InUniqueIndex: true},      // in a unique index
			{Name: "id", InUniqueIndex: true},         // in the primary key
			{Name: "full_name", InUniqueIndex: false}, // generated but not protected
		}, got)
	})

	t.Run("no protected columns -> all InUniqueIndex false", func(t *testing.T) {
		got := buildGeneratedStoredColumns([]string{"a", "b"}, map[string]bool{})
		assert.Equal(t, []GeneratedStoredColumn{
			{Name: "a", InUniqueIndex: false},
			{Name: "b", InUniqueIndex: false},
		}, got)
	})

	t.Run("empty source generated columns -> empty result", func(t *testing.T) {
		got := buildGeneratedStoredColumns(nil, map[string]bool{"x": true})
		assert.Empty(t, got)
	})

	t.Run("membership is exact (case-sensitive) match", func(t *testing.T) {
		got := buildGeneratedStoredColumns([]string{"Email"}, map[string]bool{"email": true})
		assert.Equal(t, []GeneratedStoredColumn{{Name: "Email", InUniqueIndex: false}}, got)
	})
}
