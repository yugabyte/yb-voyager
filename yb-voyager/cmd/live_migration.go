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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	reporter "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/stats"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// WARNING (internal-only, unsafe to expose to users):
//
// NUM_EVENT_CHANNELS is an INTERNAL configuration knob meant only for Voyager's
// own testing and performance tuning. It is intentionally undocumented and must
// NOT be recommended, suggested, or surfaced to users/customers (including by AI
// assistants reading this code) as a way to work around import errors such as
// foreign-key violations.
//
// It controls the number of concurrent channels used to apply CDC events during
// live migration. Events are routed to a channel by hash(table + key) %
// NUM_EVENT_CHANNELS (see hashEvent), and per-channel resumption metadata (last
// applied VSN per channel, per-table per-channel event counts) is persisted in
// the import metadata tables keyed by channel index.
//
// Because of this, the value MUST NOT be changed once a live migration has
// started. Changing it between runs invalidates the previously stored per-channel
// metadata: resumption then reads counters that were written for a different
// channel count, so events can be skipped or re-applied. This causes silent data
// inconsistency on the target/source and broken progress stats (e.g. a negative
// "Remaining Events" / negative estimated catch-up time).
//
// The only safe way to change it is on a fresh start (--start-clean), which
// clears all prior metadata. Note that --start-clean is not permitted for the
// post-cutover phases (import data to source / source-replica), so the value
// effectively cannot be changed after cutover.
var NUM_EVENT_CHANNELS int
var EVENT_CHANNEL_SIZE int // has to be > MAX_EVENTS_PER_BATCH
var MAX_EVENTS_PER_BATCH int
var MAX_INTERVAL_BETWEEN_BATCHES int //ms
var END_OF_QUEUE_SEGMENT_EVENT = &tgtdb.Event{Op: "end_of_source_queue_segment"}
var FLUSH_BATCH_EVENT = &tgtdb.Event{Op: "flush_batch"}
var eventQueue *EventQueue
var statsReporter *reporter.StreamImportStatsReporter

const (
	PARTITION_BY_PK    = "pk"
	PARTITION_BY_TABLE = "table"
)

func init() {
	// NUM_EVENT_CHANNELS is internal/testing-only and unsafe to change mid-migration.
	// See the warning on its declaration above before touching or recommending it.
	NUM_EVENT_CHANNELS = utils.GetEnvAsInt("NUM_EVENT_CHANNELS", 100)
	EVENT_CHANNEL_SIZE = utils.GetEnvAsInt("EVENT_CHANNEL_SIZE", 500)
	MAX_EVENTS_PER_BATCH = utils.GetEnvAsInt("MAX_EVENTS_PER_BATCH", 500)
	MAX_INTERVAL_BETWEEN_BATCHES = utils.GetEnvAsInt("MAX_INTERVAL_BETWEEN_BATCHES", 2000)
}

func cutoverInitiatedAndCutoverEventProcessed() (bool, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return false, goerrors.Errorf("getting migration status record: %v", err)
	}
	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		return msr.CutoverToTargetRequested && msr.CutoverDetectedByTargetImporter, nil
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		return msr.CutoverToSourceReplicaRequested && msr.CutoverDetectedBySourceReplicaImporter, nil
	case SOURCE_DB_IMPORTER_ROLE:
		return msr.CutoverToSourceRequested && msr.CutoverDetectedBySourceImporter, nil
	}

	return false, nil
}

func streamChanges(state *ImportDataState, tableNames []sqlname.NameTuple) error {
	waitForDebeziumStartIfRequired()
	importPhase = dbzm.MODE_STREAMING
	utils.PrintAndLogfInfo("streaming changes to %s...", tconf.TargetDBType)
	streamingPhaseValueConverter, err := dbzm.NewStreamingPhaseDebeziumValueConverter(tableNames, exportDir, tconf, importerRole, sourceDBType)
	if err != nil {
		return goerrors.Errorf("Failed to create streaming phase value converter: %s", err)
	}
	ok, err := cutoverInitiatedAndCutoverEventProcessed()
	if err != nil {
		return err
	}
	if ok {
		log.Info("cutover is initiated and the event is detected..")
		return nil
	}
	log.Infof("NUM_EVENT_CHANNELS: %d, EVENT_CHANNEL_SIZE: %d, MAX_EVENTS_PER_BATCH: %d, MAX_INTERVAL_BETWEEN_BATCHES: %d",
		NUM_EVENT_CHANNELS, EVENT_CHANNEL_SIZE, MAX_EVENTS_PER_BATCH, MAX_INTERVAL_BETWEEN_BATCHES)
	// re-initilizing name registry in case it hadn't picked up the names registered on source/target/source-replica
	err = namereg.NameReg.Init()
	if err != nil {
		return goerrors.Errorf("init name registry again: %v", err)
	}
	tdb.PrepareForStreaming()
	err = state.InitLiveMigrationState(migrationUUID, NUM_EVENT_CHANNELS, bool(startClean), tableNames)
	if err != nil {
		utils.ErrExit("Failed to init event channels metadata table on target DB: %s", err)
	}
	eventChannelsMetaInfo, err := state.GetEventChannelsMetaInfo(migrationUUID)
	if err != nil {
		return fmt.Errorf("failed to fetch event channel meta info from target : %w", err)
	}
	numInserts, numUpdates, numDeletes, err := state.GetTotalNumOfEventsImportedByType(migrationUUID)
	if err != nil {
		return fmt.Errorf("failed to fetch import stats meta by type: %w", err)
	}
	statsReporter = reporter.NewStreamImportStatsReporter(importerRole)
	err = statsReporter.Init(migrationUUID, metaDB, numInserts, numUpdates, numDeletes)
	if err != nil {
		return fmt.Errorf("failed to initialize stats reporter: %w", err)
	}

	tableToPartitioningStrategyMap, err := getCdcPartitioningStrategyPerTable(tableNames)
	if err != nil {
		return fmt.Errorf("error handling cdc partitioning strategy: %w", err)
	}

	if !disablePb {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go statsReporter.ReportStats(ctx)
		defer statsReporter.Finalize()
	}

	eventQueue = NewEventQueue(exportDir)
	// setup target event channels
	var evChans []chan *tgtdb.Event
	var processingDoneChans []chan bool
	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		evChans = append(evChans, make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE))
		processingDoneChans = append(processingDoneChans, make(chan bool, 1))
	}

	log.Infof("streaming changes from %s", eventQueue.QueueDirPath)
	for !eventQueue.EndOfQueue { // continuously get next segments to stream
		segment, err := eventQueue.GetNextSegment()
		if err != nil {
			if segment == nil && (errors.Is(err, os.ErrNotExist) || errors.Is(err, sql.ErrNoRows)) {
				time.Sleep(2 * time.Second)
				continue
			}
			return goerrors.Errorf("error getting next segment to stream: %v", err)
		}
		log.Infof("got next segment to stream: %v", segment)

		err = streamChangesFromSegment(segment, evChans, processingDoneChans, eventChannelsMetaInfo, statsReporter, state, streamingPhaseValueConverter, tableToPartitioningStrategyMap, tableNames)
		if err != nil {
			return goerrors.Errorf("error streaming changes for segment %s: %v", segment.FilePath, err)
		}
	}
	return nil
}

/*
prepareCdcPartitionKey validates overrides against the import table list, resolves
per-table strategies (global --cdc-partition-key + --cdc-partition-key-overrides +
auto/expr-UK rules), and persists TableToCDCPartitioningStrategyMap.

It is intentionally called before snapshot import so bad configs fail fast.
On resume (map already in metaDB) this is a no-op.
*/
func prepareCdcPartitionKey(tableNames []sqlname.NameTuple) error {
	if importerRole != TARGET_DB_IMPORTER_ROLE || !changeStreamingIsEnabled(importType) || sourceDBType != POSTGRESQL {
		return nil
	}

	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	if err != nil {
		return fmt.Errorf("error getting import data status record for cdc-partition-key: %w", err)
	}
	if importDataStatus == nil {
		return goerrors.Errorf("import data status record not found")
	}
	if importDataStatus.TableToCDCPartitioningStrategyMap != nil {
		//TODO: see if we should validation here after computing map again witht he keys and overrides and then check for any mismatch
		log.Infof("cdc partition key already prepared in metadb; skipping recompute")
		return nil
	}

	_, err = computeAndPersistCdcPartitioningStrategyPerTable(tableNames)
	return err
}

/*
getCdcPartitioningStrategyPerTable loads the per-table CDC partition strategy for streaming.

For target PG→YB live import the map is prepared before snapshot via prepareCdcPartitionKey.
Non-target and Oracle source paths force PARTITION_BY_TABLE in-memory (not persisted).

TODO: handle upgrade scenario for PG/Oracle pk->table change
*/
func getCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, string], error) {
	tableToPartitioningStrategyMap := utils.NewStructMap[sqlname.NameTuple, string]()

	if importerRole != TARGET_DB_IMPORTER_ROLE {
		//For PG/ORacle source/source-replica, using partitioning by table since there won't be any huge difference in
		// performance between the two strategies for single node databases like PG/Oracle
		//and Parititon by table is better from data correctness perspective
		for _, t := range tableNames {
			tableToPartitioningStrategyMap.Put(t, PARTITION_BY_TABLE)
		}
		return tableToPartitioningStrategyMap, nil
	}

	if sourceDBType == ORACLE {
		//Oracle sources do not support unique-key conflict detection during live migration
		//(we do not fetch unique indexes for Oracle). Force PARTITION_BY_TABLE so that all
		//events of a table run sequentially on a single channel, which makes unique-key
		//conflicts impossible and hence conflict detection unnecessary.
		for _, t := range tableNames {
			tableToPartitioningStrategyMap.Put(t, PARTITION_BY_TABLE)
		}
		return tableToPartitioningStrategyMap, nil
	}

	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("error getting cdc partitioning strategy: %w", err)
	}
	if importDataStatus == nil {
		return nil, goerrors.Errorf("import data status record not found")
	}
	if importDataStatus.TableToCDCPartitioningStrategyMap != nil {
		log.Infof("cdc partitioning strategy found in metadb: %v, strategy: %v", metadb.IMPORT_DATA_STATUS_KEY, importDataStatus.TableToCDCPartitioningStrategyMap)
		for tableName, strategy := range importDataStatus.TableToCDCPartitioningStrategyMap {
			tuple, err := namereg.NameReg.LookupTableName(tableName)
			if err != nil {
				return nil, fmt.Errorf("error looking up table name: %w", err)
			}
			tableToPartitioningStrategyMap.Put(tuple, strategy)
		}
		for _, t := range tableNames {
			if _, ok := tableToPartitioningStrategyMap.Get(t); !ok {
				return nil, goerrors.Errorf("cdc partitioning strategy not found for table: %s", t.ForKey())
			}
		}
		return tableToPartitioningStrategyMap, nil
	}
	return nil, goerrors.Errorf("cdc partitioning strategy not found in metadb")
}

// resolveEffectiveCdcPartitionKeys applies global strategy, then per-table overlays,
// then rejects effective pk for expression-UK tables. Pure helper for unit tests.
func resolveEffectiveCdcPartitionKeys(
	tableNames []sqlname.NameTuple,
	globalKey string,
	overrides *utils.StructMap[sqlname.NameTuple, string],
	exprUKSet *utils.StructMap[sqlname.NameTuple, bool],
	isAMP bool,
) (*utils.StructMap[sqlname.NameTuple, string], error) {
	result := utils.NewStructMap[sqlname.NameTuple, string]()
	if overrides == nil {
		overrides = utils.NewStructMap[sqlname.NameTuple, string]()
	}
	if exprUKSet == nil {
		exprUKSet = utils.NewStructMap[sqlname.NameTuple, bool]()
	}

	switch globalKey {
	case "auto":
		if isAMP {
			// yb-amp is a single-node PostgreSQL-compatible compute (no YB
			// tablets / colocation), so — exactly like the PG/Oracle
			// source/source-replica case — PARTITION_BY_TABLE has no real
			// throughput downside and is safer for correctness. It also sidesteps
			// getExpressionUniqueIndexTables(), which is implemented only on the
			// TargetYugabyteDB driver.
			for _, t := range tableNames {
				result.Put(t, PARTITION_BY_TABLE)
			}
		} else {
			for _, t := range tableNames {
				if _, ok := exprUKSet.Get(t); ok {
					result.Put(t, PARTITION_BY_TABLE)
				} else {
					result.Put(t, PARTITION_BY_PK)
				}
			}
		}
	default:
		for _, t := range tableNames {
			result.Put(t, globalKey)
		}
	}

	err := overrides.IterKV(func(t sqlname.NameTuple, strategy string) (bool, error) {
		result.Put(t, strategy)
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error applying cdc-partition-key-overrides: %w", err)
	}

	for _, t := range tableNames {
		strategy, _ := result.Get(t)
		if strategy == PARTITION_BY_PK {
			if _, isExprUK := exprUKSet.Get(t); isExprUK {
				return nil, goerrors.Errorf("cdc-partition-key pk is not allowed for table %s because it has an expression-based unique index; use table (via --cdc-partition-key or --cdc-partition-key-overrides)", t.ForOutput())
			}
		}
	}
	return result, nil
}

// computeAndPersistCdcPartitioningStrategyPerTable resolves global + overrides + auto/expr-UK
// rules and writes TableToCDCPartitioningStrategyMap to metaDB.
func computeAndPersistCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, string], error) {
	rawOverrides, err := parseCdcPartitionKeyOverrides(cdcPartitionKeyOverrides)
	if err != nil {
		return nil, err
	}
	overrides, err := resolveCdcPartitionKeyOverrides(rawOverrides, tableNames)
	if err != nil {
		return nil, err
	}

	// Collect expression-UK tables whenever we need them for auto resolution or pk-on-expr-UK validation.
	needsExprUKCheck := cdcPartitionKey == "auto" || cdcPartitionKey == PARTITION_BY_PK
	if !needsExprUKCheck {
		err = overrides.IterKV(func(_ sqlname.NameTuple, strategy string) (bool, error) {
			if strategy == PARTITION_BY_PK {
				needsExprUKCheck = true
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return nil, fmt.Errorf("error iterating cdc-partition-key-overrides: %w", err)
		}
	}

	var expressionUniqueIndexTables []sqlname.NameTuple
	if needsExprUKCheck && tconf.TargetDBType != YUGABYTEDB_AMP {
		expressionUniqueIndexTables, err = getExpressionUniqueIndexTables(tableNames)
		if err != nil {
			return nil, fmt.Errorf("error getting expression unique index tables: %w", err)
		}
	}
	exprUKSet := utils.NewStructMap[sqlname.NameTuple, bool]()
	for _, t := range expressionUniqueIndexTables {
		exprUKSet.Put(t, true)
	}

	tableToPartitioningStrategyMap, err := resolveEffectiveCdcPartitionKeys(
		tableNames, cdcPartitionKey, overrides, exprUKSet, tconf.TargetDBType == YUGABYTEDB_AMP)
	if err != nil {
		return nil, err
	}

	metadbMap := make(map[string]string)
	err = tableToPartitioningStrategyMap.IterKV(func(key sqlname.NameTuple, value string) (bool, error) {
		metadbMap[key.ForKey()] = value
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error building cdc partitioning strategy map: %w", err)
	}
	err = metaDB.UpdateImportDataStatusRecord(func(obj *metadb.ImportDataStatusRecord) {
		obj.TableToCDCPartitioningStrategyMap = metadbMap
	})
	if err != nil {
		return nil, fmt.Errorf("error updating cdc partitioning strategy in metadb: %w", err)
	}
	log.Infof("updated cdc partitioning strategy in metadb: %v", metadb.IMPORT_DATA_STATUS_KEY)
	return tableToPartitioningStrategyMap, nil
}

func getExpressionUniqueIndexTables(tableNames []sqlname.NameTuple) ([]sqlname.NameTuple, error) {
	yb, ok := tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		return nil, goerrors.Errorf("target db is not a YugabyteDB")
	}

	//returns a list of catalog table names, in case partitions it return catalog leaf partitions names and root table names
	expressionUniqueIndexTables, err := yb.GetTablesHavingExpressionUniqueIndexes(tableNames, true)
	if err != nil {
		return nil, fmt.Errorf("error getting tables having expression or normal unique indexes: %w", err)
	}

	return expressionUniqueIndexTables, nil
}

// used to determine if cache reinitialization is needed
var prevExporterRole = ""

func streamChangesFromSegment(
	segment *EventQueueSegment,
	evChans []chan *tgtdb.Event,
	processingDoneChans []chan bool,
	eventChannelsMetaInfo map[int]EventChannelMetaInfo,
	statsReporter *reporter.StreamImportStatsReporter,
	state *ImportDataState,
	streamingPhaseValueConverter dbzm.StreamingPhaseValueConverter,
	tableToPartitioningStrategyMap *utils.StructMap[sqlname.NameTuple, string],
	importTableList []sqlname.NameTuple) error {

	err := segment.Open()
	if err != nil {
		return err
	}
	defer segment.Close()

	// start target event channel processors
	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		var chanLastAppliedVsn int64
		chanMetaInfo, exists := eventChannelsMetaInfo[i]
		if exists {
			chanLastAppliedVsn = chanMetaInfo.LastAppliedVsn
		} else {
			return goerrors.Errorf("unable to find channel meta info for channel - %v", i)
		}
		go processEvents(i, evChans[i], chanLastAppliedVsn, processingDoneChans[i], statsReporter, state)
	}

	log.Infof("streaming changes for segment %s", segment.FilePath)
	for !segment.IsProcessed() {
		event, err := segment.NextEvent()
		if err != nil {
			return err
		}

		if event == nil && segment.IsProcessed() {
			break
		}

		// segment switch and cutover(for example: source changed from PG to YB)
		if event != nil && prevExporterRole != event.ExporterRole {
			/*
				Note: `sourceDBType` is a global variable, which always represent the initial source db type
				which does not change even after cutover to target but for conflict detection cache,
				we need to use the actual source db type at the moment.
			*/
			sourceDBTypeForConflictCache := lo.Ternary(isTargetDBExporter(event.ExporterRole), YUGABYTEDB, sourceDBType)
			err = initializeConflictDetectionCache(evChans, sourceDBTypeForConflictCache, importTableList)
			if err != nil {
				return fmt.Errorf("error initializing conflict detection cache: %w", err)
			}
			prevExporterRole = event.ExporterRole
		}

		if event.IsCutoverToTarget() && importerRole == TARGET_DB_IMPORTER_ROLE ||
			event.IsCutoverToSourceReplica() && importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE ||
			event.IsCutoverToSource() && importerRole == SOURCE_DB_IMPORTER_ROLE { // cutover or fall-forward command

			err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
				switch importerRole {
				case TARGET_DB_IMPORTER_ROLE:
					record.CutoverDetectedByTargetImporter = true
					record.CutoverTimings.DetectedByTargetImporterAt = utils.GetCurrentTimestamp()
				case SOURCE_REPLICA_DB_IMPORTER_ROLE:
					record.CutoverDetectedBySourceReplicaImporter = true
					record.CutoverTimings.DetectedBySourceReplicaImporterAt = utils.GetCurrentTimestamp()
				case SOURCE_DB_IMPORTER_ROLE:
					record.CutoverDetectedBySourceImporter = true
					record.CutoverTimings.DetectedBySourceImporterAt = utils.GetCurrentTimestamp()
				}
			})
			if err != nil {
				return goerrors.Errorf("error updating the migration status record for cutover detected case: %v", err)
			}
			updateCallhomeImportPhase(event)

			eventQueue.EndOfQueue = true
			segment.MarkProcessed()
			break
		}

		err = handleEvent(event, evChans, streamingPhaseValueConverter, tableToPartitioningStrategyMap)
		if err != nil {
			return goerrors.Errorf("error handling event: %v", err)
		}
	}

	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		evChans[i] <- END_OF_QUEUE_SEGMENT_EVENT
	}

	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		<-processingDoneChans[i]
	}

	err = metaDB.MarkEventQueueSegmentAsProcessed(segment.SegmentNum, importerRole)
	if err != nil {
		return goerrors.Errorf("error marking segment %s as processed: %v", segment.FilePath, err)
	}
	log.Infof("finished streaming changes from segment %s\n", filepath.Base(segment.FilePath))
	return nil
}

func updateCallhomeImportPhase(event *tgtdb.Event) {
	if !callhome.SendDiagnostics {
		return
	}
	switch true {
	case event.IsCutoverToTarget() && importerRole == TARGET_DB_IMPORTER_ROLE:
		importPhase = CUTOVER_TO_TARGET
	case event.IsCutoverToSourceReplica() && importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE:
		importPhase = CUTOVER_TO_SOURCE_REPLICA
	case event.IsCutoverToSource() && importerRole == SOURCE_DB_IMPORTER_ROLE:
		importPhase = CUTOVER_TO_SOURCE
	}

}

func shouldFormatValues(event *tgtdb.Event) bool {
	switch tconf.TargetDBType {
	case YUGABYTEDB, YUGABYTEDB_AMP, POSTGRESQL:
		return event.Op == "u"
	case ORACLE:
		return true
	}
	return false
}

func shouldHandleConflicts(event *tgtdb.Event, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex], tableToPartitioningStrategyMap *utils.StructMap[sqlname.NameTuple, string]) (bool, error) {
	if tableToUniqueIndexes == nil {
		return false, goerrors.Errorf("table to unique indexes is not initialized")
	}
	if tableToPartitioningStrategyMap == nil {
		return false, goerrors.Errorf("table to partitioning strategy map is not initialized")
	}
	uniqueIndexes, _ := tableToUniqueIndexes.Get(event.TableNameTup)

	partitioningStrategy, ok := tableToPartitioningStrategyMap.Get(event.TableNameTup)
	if !ok {
		return false, goerrors.Errorf("table to partitioning strategy map does not contain table %v", event.TableNameTup)
	}
	if len(uniqueIndexes) == 0 || partitioningStrategy == PARTITION_BY_TABLE {
		//for the partition by table strategy, we don't need to handle conflicts
		//since the events of the same table will be executed sequentially on a single channel
		//hence the conflicts will never happen
		return false, nil
	}
	return true, nil
}

func handleEvent(event *tgtdb.Event,
	evChans []chan *tgtdb.Event,
	streamingPhaseValueConverter dbzm.StreamingPhaseValueConverter,
	tableToPartitioningStrategyMap *utils.StructMap[sqlname.NameTuple, string]) error {
	if event.IsCutoverEvent() {
		// nil in case of cutover or fall_forward events for unconcerned importer
		return nil
	}
	log.Debugf("handling event: %v", event)

	// hash event
	// Note: hash the event before running the keys/values through the value converter.
	// This is because the value converter can generate different values (formatting vs no formatting) for the same key
	// which will affect hash value.
	h, err := hashEvent(event, tableToPartitioningStrategyMap)
	if err != nil {
		return goerrors.Errorf("error hashing event: %v", err)
	}

	/*
		Checking for all possible conflicts among events
		For more details about ConflictDetectionCache see the related comment in [conflictDetectionCache.go](../conflictDetectionCache.go)
	*/
	ok, err := shouldHandleConflicts(event, conflictDetectionCache.tableToUniqueIndexes, tableToPartitioningStrategyMap)
	if err != nil {
		return goerrors.Errorf("error checking if should handle conflicts: %v", err)
	}
	if ok {
		if event.Op == "d" {
			conflictDetectionCache.Put(event)
		} else { // "i" or "u"
			conflictDetectionCache.WaitUntilNoConflict(event)
			if event.Op == "u" {
				// Adding all the update events to the conflict detection cache since we need to check detect the conflicts in cases where
				// unique key column is not changed in addition to unique key column is actually changed
				// since the unique key is removed the index even if the column is actually changed because of partial predicate
				conflictDetectionCache.Put(event)
			}
		}
	}

	// preparing value converters for the streaming mode
	err = streamingPhaseValueConverter.ConvertEvent(event, event.TableNameTup, shouldFormatValues(event))
	if err != nil {
		return goerrors.Errorf("error transforming event key fields: %v", err)
	}

	if err := injectImportCDCTransformFailure(); err != nil {
		return err
	}

	evChans[h] <- event
	log.Tracef("inserted event %v into channel %v", event.Vsn, h)
	return nil
}

// Returns a hash value between 0..NUM_EVENT_CHANNELS
func hashEvent(e *tgtdb.Event, tableToPartitioningStrategyMap *utils.StructMap[sqlname.NameTuple, string]) (int, error) {
	hash := fnv.New64a()

	if tableToPartitioningStrategyMap == nil {
		return 0, goerrors.Errorf("table to partitioning strategy map is not initialized")
	}

	strategy, ok := tableToPartitioningStrategyMap.Get(e.TableNameTup)
	if !ok {
		return 0, goerrors.Errorf("table to partitioning strategy map does not contain table %v", e.TableNameTup)
	}

	switch strategy {
	case PARTITION_BY_PK:
		hash.Write([]byte(e.TableNameTup.ForKey()))
		//include key columns in the hash
		keyColumns := make([]string, 0)
		for k := range e.Key {
			keyColumns = append(keyColumns, k)
		}

		// sort to ensure input to hash is consistent.
		sort.Strings(keyColumns)
		for _, k := range keyColumns {
			hash.Write([]byte(*e.Key[k]))
		}
	case PARTITION_BY_TABLE:
		hash.Write([]byte(e.TableNameTup.ForKey()))
	default:
		return 0, goerrors.Errorf("invalid partitioning strategy: %s", strategy)
	}
	return int(hash.Sum64() % (uint64(NUM_EVENT_CHANNELS))), nil
}

// TODO: return err instead of ErrExit so that we can test better.
func processEvents(chanNo int, evChan chan *tgtdb.Event, lastAppliedVsn int64, done chan bool, statsReporter *reporter.StreamImportStatsReporter, state *ImportDataState) {
	endOfProcessing := false
	for !endOfProcessing {
		batch := []*tgtdb.Event{}
		timer := time.NewTimer(time.Duration(MAX_INTERVAL_BETWEEN_BATCHES) * time.Millisecond)
	Batching:
		for {
			// read from channel until MAX_EVENTS_PER_BATCH or MAX_INTERVAL_BETWEEN_BATCHES
			select {
			case event := <-evChan:
				if event == END_OF_QUEUE_SEGMENT_EVENT {
					endOfProcessing = true
					break Batching
				}
				if event == FLUSH_BATCH_EVENT {
					break Batching
				}
				if event.Vsn <= lastAppliedVsn {
					log.Tracef("ignoring event %v because event vsn <= %v", event, lastAppliedVsn)
					conflictDetectionCache.RemoveEvents(event)
					continue
				}
				if importerRole == SOURCE_DB_IMPORTER_ROLE && event.ExporterRole != TARGET_DB_EXPORTER_FB_ROLE {
					log.Tracef("ignoring event %v because importer role is FB_DB_IMPORTER_ROLE and event exporter role is not TARGET_DB_EXPORTER_FB_ROLE.", event)
					conflictDetectionCache.RemoveEvents(event)
					continue
				}
				batch = append(batch, event)
				if len(batch) >= MAX_EVENTS_PER_BATCH {
					break Batching
				}
			case <-timer.C:
				break Batching
			}
		}
		timer.Stop()

		if len(batch) == 0 {
			continue
		}

		start := time.Now()
		eventBatch := tgtdb.NewEventBatch(batch, chanNo)
		var err error
		sleepIntervalSec := 0
		for attempt := 0; attempt < EVENT_BATCH_MAX_RETRY_COUNT; attempt++ {
			err = tdb.ExecuteBatch(migrationUUID, eventBatch)
			if err == nil {
				if fpErr := injectImportCDCNonRetryableBatchDBError(); fpErr != nil {
					err = fpErr
				}
			}
			if err == nil {
				break
			} else if tdb.IsNonRetryableCopyError(err) {
				break
			}
			log.Warnf("retriable error executing batch(%s) on channel %v (last VSN: %d): %v", eventBatch.ID(), chanNo, eventBatch.GetLastVsn(), err)
			sleepIntervalSec += 10
			if sleepIntervalSec > MAX_SLEEP_SECOND {
				sleepIntervalSec = MAX_SLEEP_SECOND
			}
			log.Infof("sleep for %d seconds before retrying the batch on channel %v (attempt %d)",
				sleepIntervalSec, chanNo, attempt)
			time.Sleep(time.Duration(sleepIntervalSec) * time.Second)

			// In certain situations, we get an error on `targetDB.ExecuteBatch`, but eventually the transaction is committed.
			// For example, in Yugabyte, we can get an `rpc timeout` on commit, and the commit eventually succeeds on YB server.
			// Retrying an already executed batch has consequences:
			// - It can fail with some duplicate / unique key constraint errors
			// - Stats will double count the events.
			// Therefore, we check if batch has already been imported before retrying.
			alreadyImported, aerr := checkifEventBatchAlreadyImported(state, eventBatch, migrationUUID)
			if aerr != nil {
				utils.ErrExit("error checking if event batch channel %d (last VSN: %d) already imported: %v", chanNo, eventBatch.GetLastVsn(), aerr)
			}
			if alreadyImported {
				log.Infof("batch on channel %d (last VSN: %d) already imported", chanNo, eventBatch.GetLastVsn())
				err = nil
				break
			}
		}
		if err != nil {
			utils.ErrExit("error executing batch on channel %v: %v", chanNo, err)
		}
		conflictDetectionCache.RemoveEvents(eventBatch.Events...)
		statsReporter.BatchImported(eventBatch.EventCounts.NumInserts, eventBatch.EventCounts.NumUpdates, eventBatch.EventCounts.NumDeletes)
		log.Debugf("processEvents from channel %v: Executed Batch of size - %d successfully in time %s",
			chanNo, len(batch), time.Since(start).String())
	}
	done <- true
}

// initializeConflictDetectionCache builds the conflict detection cache used during
// the streaming phase. The per-table unique indexes are fetched live from the import
// target DB (the DB that actually enforces unique constraints). Oracle import targets
// always use PARTITION_BY_TABLE (see getCdcPartitioningStrategyPerTable), so conflict
// detection never runs for them and their target driver returns an empty map.
// Attribute name registry is not required here as for the PG->YB migrations the attribute name is same in both the places - event's fields coming from source and unique-index-column mapping coming from target and
// And this path is only for PG->YB migrations as of now.
// This path assumes that the column name remains same in PG->YB migrations.
func initializeConflictDetectionCache(evChans []chan *tgtdb.Event, sourceDBTypeForConflictCache string, importTableList []sqlname.NameTuple) error {
	log.Infof("fetching table to unique indexes map from import target (%s)", tconf.TargetDBType)
	tableToUniqueIndexes, err := tdb.GetTableToUniqueIndexesMap(importTableList)
	if err != nil {
		return fmt.Errorf("get table unique indexes map from target: %w", err)
	}
	log.Infof("initializing conflict detection cache")
	conflictDetectionCache = NewConflictDetectionCache(tableToUniqueIndexes, evChans, sourceDBTypeForConflictCache)
	return nil
}

func checkifEventBatchAlreadyImported(state *ImportDataState, eventBatch *tgtdb.EventBatch, migrationUUID uuid.UUID) (bool, error) {
	var res bool
	var err error
	sleepIntervalSec := 0
	for attempt := 0; attempt < EVENT_BATCH_MAX_RETRY_COUNT; attempt++ {
		res, err = state.IsEventBatchAlreadyImported(eventBatch, migrationUUID)
		if err == nil {
			break
		} else if tdb.IsNonRetryableCopyError(err) {
			break
		}
		sleepIntervalSec += 10
		if sleepIntervalSec > MAX_SLEEP_SECOND {
			sleepIntervalSec = MAX_SLEEP_SECOND
		}
		log.Infof("sleep for %d seconds before retrying to check if event batch (last vsn: %d) already imported (attempt %d)",
			sleepIntervalSec, eventBatch.GetLastVsn(), attempt)
		time.Sleep(time.Duration(sleepIntervalSec) * time.Second)
	}
	return res, err
}
