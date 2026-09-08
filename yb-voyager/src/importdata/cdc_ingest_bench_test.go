//go:build cdc_benchmark

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

/*
CDC ingest benchmark: replays real export-data queue segments through the
real import streaming path with only TargetDB.ExecuteBatch mocked.

All orchestration (workloads, artifact generation/caching, metrics,
assertions) lives in test/cdcbench; this file only injects the closures that
need importdata-package internals. Workloads are sub-benchmarks:

	go test -tags cdc_benchmark -bench CDCIngest -benchtime 1x -count 5 ./cmd/
	go test -tags cdc_benchmark -bench 'CDCIngest/updates-uk-no-conflict' -benchtime 1x ./cmd/

See test/cdcbench/README.md for workload authoring and knobs.
*/

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/test/cdcbench"
)

func BenchmarkCDCIngest(b *testing.B) {
	// state shared between Bootstrap and StreamAll within one run
	var run struct {
		imp                  *Importer
		state                *ImportDataState
		tableList            []sqlname.NameTuple
		tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]
		tableToPKColumns     *utils.StructMap[sqlname.NameTuple, []string]
	}

	cdcbench.Run(b, cdcbench.Hooks{
		Bootstrap: func(artifactDir string, mock tgtdb.TargetDB) error {
			// mirrors the importData command's bootstrap; the streaming-phase
			// setup itself (value converter, channels, conflict cache, channel
			// metadata, stats reporter) is done by the real streamChanges call
			// in StreamAll, with the mock answering the target-side metadata
			// queries with fresh-migration values.
			// mirrors the cmd-level setup the import data command does before it hands
			// over to the engine (metadb, migration uuid, name registry, table list)
			exportDir := artifactDir
			metaDB, err := initTestMetaDB(exportDir)
			if err != nil {
				return fmt.Errorf("init meta db: %w", err)
			}
			msr, err := metaDB.GetMigrationStatusRecord()
			if err != nil {
				return fmt.Errorf("get migration status record: %w", err)
			}
			sourceDBType := msr.SourceDBConf.DBType
			sqlname.SourceDBType = sourceDBType
			callhome.SendDiagnostics = false
			tconf := tgtdb.TargetConf{TargetDBType: constants.YUGABYTEDB, SchemaConfig: "public"}
			tconf.Schemas = sqlname.ParseIdentifiersFromString(tconf.TargetDBType, tconf.SchemaConfig, ",")

			if err := namereg.InitNameRegistry(namereg.NameRegistryParams{
				FilePath:       fmt.Sprintf("%s/metainfo/name_registry.json", exportDir),
				Role:           constants.TARGET_DB_IMPORTER_ROLE,
				TargetDBSchema: sqlname.ExtractIdentifiersUnquoted(tconf.Schemas),
			}); err != nil {
				return fmt.Errorf("init name registry: %w", err)
			}
			run.tableList = nil
			for _, qualifiedTableName := range msr.TableListExportedFromSource {
				table, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(qualifiedTableName)
				if err != nil {
					return fmt.Errorf("lookup table %s in name registry : %w", qualifiedTableName, err)
				}
				run.tableList = append(run.tableList, table)
			}
			run.imp = NewImporter(Config{
				ExportDir:     exportDir,
				MetaDB:        metaDB,
				MigrationUUID: uuid.MustParse(msr.MigrationUUID),
				ImporterRole:  constants.TARGET_DB_IMPORTER_ROLE,
				SourceDBType:  sourceDBType,
				Tconf:         tconf,
				Tdb:           mock,
				DisablePb:     true,
				// production default resolution for tables without expression-based
				// unique indexes; avoids the target-DB query of the "auto" path
				CdcPartitionKey: "pk",
				ImportTableList: run.tableList,
			})
			run.state = NewImportDataState(run.imp.importDataStateConfig())

			// make sure the mock has the same table list as the real target DB
			run.tableToUniqueIndexes, err = mock.GetTableToUniqueIndexesMap(run.tableList)
			if err != nil {
				utils.ErrExit("Failed to get table unique indexes map from target: %s", err)
			}

			run.tableToPKColumns, err = run.imp.getPrimaryKeyColumnsForImportTables(run.tableList)
			if err != nil {
				utils.ErrExit("Failed to get primary key columns for import tables: %s", err)
			}

			err = metaDB.UpdateImportDataStatusRecord(func(record *metadb.ImportDataStatusRecord) {
				record.CdcPartitioningStrategyConfig = "auto"
				record.TableToCDCPartitionKey = make(map[string]metadb.CDCPartitionKey)
				for _, table := range run.tableList {
					record.TableToCDCPartitionKey[table.ForKey()] = metadb.CDCPartitionKey{Strategy: "pk"}
				}
			})
			if err != nil {
				return fmt.Errorf("update import data status record: %w", err)
			}

			// reset streaming globals so this run initializes them afresh, as
			// production does on the first event of a stream. The framework's
			// depth sampler reads the conflictDetectionCache pointer while the
			// stream's first event assigns it — an unsynchronized read/write
			// pair, accepted for the benchmark: artifacts carry a single
			// exporter role, so the pointer is written exactly once and never
			// changes mid-run.
			run.imp.conflictDetectionCache = nil
			run.imp.prevExporterRole = ""
			return nil
		},

		// the real streaming entrypoint, end to end: value converter, channel
		// metadata (answered by the mock's metadata store), conflict cache,
		// stats reporter, and the segment loop
		StreamAll: func() error {
			return run.imp.streamChanges(run.state, run.tableList, run.tableToPKColumns, run.tableToUniqueIndexes)
		},

		CacheDepth: func() int {
			c := run.imp.conflictDetectionCache
			if c == nil {
				return 0
			}
			c.Lock()
			defer c.Unlock()
			return len(c.m) + cacheDepth(c.ukLookup)
		},
	})
}

func cacheDepth(ukLookup map[string]map[int64]*tgtdb.Event) int {
	var depth int
	for _, events := range ukLookup {
		depth += len(events)
	}
	return depth
}
