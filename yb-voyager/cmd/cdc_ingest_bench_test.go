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

package cmd

/*
CDC ingest benchmark: replays real export-data queue segments through the
real import streaming path with only TargetDB.ExecuteBatch mocked.

All orchestration (workloads, artifact generation/caching, metrics,
assertions) lives in test/cdcbench; this file only injects the closures that
need cmd-package internals. Workloads are sub-benchmarks:

	go test -tags cdc_benchmark -bench CDCIngest -benchtime 1x -count 5 ./cmd/
	go test -tags cdc_benchmark -bench 'CDCIngest/updates-uk-no-conflict' -benchtime 1x ./cmd/

See test/cdcbench/README.md for workload authoring and knobs.
*/

import (
	"fmt"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/test/cdcbench"
)

func BenchmarkCDCIngest(b *testing.B) {
	// state shared between Bootstrap and StreamAll within one run
	var run struct {
		state     *ImportDataState
		tableList []sqlname.NameTuple
	}

	cdcbench.Run(b, cdcbench.Hooks{
		Bootstrap: func(artifactDir string, mock tgtdb.TargetDB) error {
			// mirrors the importData command's bootstrap; the streaming-phase
			// setup itself (value converter, channels, conflict cache, channel
			// metadata, stats reporter) is done by the real streamChanges call
			// in StreamAll, with the mock answering the target-side metadata
			// queries with fresh-migration values.
			exportDir = artifactDir
			metaDB = initMetaDB(exportDir)
			if err := retrieveMigrationUUID(); err != nil {
				return fmt.Errorf("retrieve migration uuid: %w", err)
			}
			sourceDBType = GetSourceDBTypeFromMSR()
			sqlname.SourceDBType = sourceDBType
			importerRole = TARGET_DB_IMPORTER_ROLE
			callhome.SendDiagnostics = false
			disablePb = true
			// production default resolution for tables without expression-based
			// unique indexes; avoids the target-DB query of the "auto" path
			cdcPartitionKey = "pk"
			tconf = tgtdb.TargetConf{TargetDBType: YUGABYTEDB, SchemaConfig: "public"}
			tconf.Schemas = sqlname.ParseIdentifiersFromString(tconf.TargetDBType, tconf.SchemaConfig, ",")

			if err := InitNameRegistry(exportDir, importerRole, nil, nil, &tconf, nil, false); err != nil {
				return fmt.Errorf("init name registry: %w", err)
			}
			msr, err := metaDB.GetMigrationStatusRecord()
			if err != nil {
				return fmt.Errorf("get migration status record: %w", err)
			}
			run.tableList, err = getInitialImportTableListForLive(msr.TableListExportedFromSource)
			if err != nil {
				return fmt.Errorf("get import table list: %w", err)
			}
			run.state = NewImportDataState(exportDir)
			tdb = mock

			// reset streaming globals so this run initializes them afresh, as
			// production does on the first event of a stream. The framework's
			// depth sampler reads the conflictDetectionCache pointer while the
			// stream's first event assigns it — an unsynchronized read/write
			// pair, accepted for the benchmark: artifacts carry a single
			// exporter role, so the pointer is written exactly once and never
			// changes mid-run.
			conflictDetectionCache = nil
			prevExporterRole = ""
			return nil
		},

		// the real streaming entrypoint, end to end: value converter, channel
		// metadata (answered by the mock's metadata store), conflict cache,
		// stats reporter, and the segment loop
		StreamAll: func() error {
			return streamChanges(run.state, run.tableList)
		},

		CacheDepth: func() int {
			c := conflictDetectionCache
			if c == nil {
				return 0
			}
			c.Lock()
			defer c.Unlock()
			return len(c.m)
		},
	})
}
