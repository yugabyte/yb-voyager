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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	reporterstats "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/stats"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/test/cdcbench"
)

func BenchmarkCDCIngest(b *testing.B) {
	// state shared between Bootstrap and StreamAll within one run
	var run struct {
		converter dbzm.StreamingPhaseValueConverter
		partMap   *utils.StructMap[sqlname.NameTuple, string]
		state     *ImportDataState
	}

	cdcbench.Run(b, cdcbench.Hooks{
		Bootstrap: func(artifactDir string, mock tgtdb.TargetDB) error {
			// mirrors the importData command's bootstrap, minus the target DB:
			// channel metadata and vsn state are fresh-migration values, and the
			// only target interaction left in the streaming path is the mocked
			// ExecuteBatch.
			exportDir = artifactDir
			metaDB = initMetaDB(exportDir)
			if err := retrieveMigrationUUID(); err != nil {
				return fmt.Errorf("retrieve migration uuid: %w", err)
			}
			sourceDBType = GetSourceDBTypeFromMSR()
			sqlname.SourceDBType = sourceDBType
			importerRole = TARGET_DB_IMPORTER_ROLE
			callhome.SendDiagnostics = false
			// production default resolution for tables without expression-based
			// unique indexes; avoids the target-DB query of the "auto" path
			cdcPartitioningStrategy = "pk"
			tconf = tgtdb.TargetConf{TargetDBType: YUGABYTEDB, SchemaConfig: "public"}
			tconf.Schemas = sqlname.ParseIdentifiersFromString(tconf.TargetDBType, tconf.SchemaConfig, ",")

			if err := InitNameRegistry(exportDir, importerRole, nil, nil, &tconf, nil, false); err != nil {
				return fmt.Errorf("init name registry: %w", err)
			}
			msr, err := metaDB.GetMigrationStatusRecord()
			if err != nil {
				return fmt.Errorf("get migration status record: %w", err)
			}
			tableList, err := getInitialImportTableListForLive(msr.TableListExportedFromSource)
			if err != nil {
				return fmt.Errorf("get import table list: %w", err)
			}
			run.converter, err = dbzm.NewStreamingPhaseDebeziumValueConverter(tableList, exportDir, tconf, importerRole, sourceDBType)
			if err != nil {
				return fmt.Errorf("create streaming value converter: %w", err)
			}
			run.partMap, err = getCdcPartitioningStrategyPerTable(tableList)
			if err != nil {
				return fmt.Errorf("get cdc partitioning strategy: %w", err)
			}
			run.state = NewImportDataState(exportDir)

			tdb = mock
			prevExporterRole = "" // force conflict-cache re-init on the first event
			return nil
		},

		StreamAll: func() error {
			// the segment loop from streamChanges (live_migration.go), with
			// fresh-migration channel metadata (what InitLiveMigrationState
			// inserts and GetEventChannelsMetaInfo returns on a first run)
			var evChans []chan *tgtdb.Event
			var doneChans []chan bool
			chanMeta := map[int]EventChannelMetaInfo{}
			for i := 0; i < NUM_EVENT_CHANNELS; i++ {
				evChans = append(evChans, make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE))
				doneChans = append(doneChans, make(chan bool, 1))
				chanMeta[i] = EventChannelMetaInfo{ChanNo: i, LastAppliedVsn: -1}
			}
			statsReporter := &reporterstats.StreamImportStatsReporter{}

			eventQueue = NewEventQueue(exportDir)
			for !eventQueue.EndOfQueue {
				segment, err := eventQueue.GetNextSegment()
				if err != nil {
					return fmt.Errorf("get next queue segment: %w", err)
				}
				err = streamChangesFromSegment(segment, evChans, doneChans, chanMeta, statsReporter, run.state, run.converter, run.partMap)
				if err != nil {
					return fmt.Errorf("stream changes from segment %s: %w", segment.FilePath, err)
				}
			}
			return nil
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
