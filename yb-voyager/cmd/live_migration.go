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
	"strings"
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
	PARTITION_BY_PK     = "pk"
	PARTITION_BY_TABLE  = "table"
	PARTITION_BY_CUSTOM = "custom"
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

	tablePartitionKeyMap, err := getCdcPartitioningStrategyPerTable(tableNames)
	if err != nil {
		return fmt.Errorf("error handling cdc partitioning strategy: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go statsReporter.ReportStats(ctx, !bool(disablePb))
	defer statsReporter.Finalize()

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

		err = streamChangesFromSegment(segment, evChans, processingDoneChans, eventChannelsMetaInfo, statsReporter, state, streamingPhaseValueConverter, tablePartitionKeyMap, tableNames)
		if err != nil {
			return goerrors.Errorf("error streaming changes for segment %s: %v", segment.FilePath, err)
		}
	}
	return nil
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
	tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride],
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
			err = initializeConflictDetectionCache(evChans, sourceDBTypeForConflictCache, importTableList, tablePartitionKeyMap)
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

		err = handleEvent(event, evChans, streamingPhaseValueConverter, tablePartitionKeyMap)
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

func shouldHandleConflicts(event *tgtdb.Event, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex], tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) (bool, error) {
	if tableToUniqueIndexes == nil {
		return false, goerrors.Errorf("table to unique indexes is not initialized")
	}
	if tablePartitionKeyMap == nil {
		return false, goerrors.Errorf("table partition key map is not initialized")
	}
	uniqueIndexes, _ := tableToUniqueIndexes.Get(event.TableNameTup)

	partitionKey, ok := tablePartitionKeyMap.Get(event.TableNameTup)
	if !ok {
		return false, goerrors.Errorf("table partition key map does not contain table %v", event.TableNameTup)
	}
	if len(uniqueIndexes) == 0 || partitionKey.Strategy == PARTITION_BY_TABLE {
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
	tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) error {
	if event.IsCutoverEvent() {
		// nil in case of cutover or fall_forward events for unconcerned importer
		return nil
	}
	log.Debugf("handling event: %v", event)

	// hash event
	// Note: hash the event before running the keys/values through the value converter.
	// This is because the value converter can generate different values (formatting vs no formatting) for the same key
	// which will affect hash value.
	h, err := hashEvent(event, tablePartitionKeyMap)
	if err != nil {
		return goerrors.Errorf("error hashing event: %v", err)
	}

	/*
		Checking for all possible conflicts among events
		For more details about ConflictDetectionCache see the related comment in [conflictDetectionCache.go](../conflictDetectionCache.go)
	*/
	ok, err := shouldHandleConflicts(event, conflictDetectionCache.tableToUniqueIndexes, tablePartitionKeyMap)
	if err != nil {
		return goerrors.Errorf("error checking if should handle conflicts: %v", err)
	}
	if ok {
		if event.Op == "d" {
			err = conflictDetectionCache.Put(event)
			if err != nil {
				return goerrors.Errorf("error putting event into conflict detection cache: %v", err)
			}
		} else { // "i" or "u"
			conflictDetectionCache.WaitUntilNoConflict(event)
			if event.Op == "u" {
				// Adding all the update events to the conflict detection cache since we need to check detect the conflicts in cases where
				// unique key column is not changed in addition to unique key column is actually changed
				// since the unique key is removed the index even if the column is actually changed because of partial predicate
				err = conflictDetectionCache.Put(event)
				if err != nil {
					return goerrors.Errorf("error putting event into conflict detection cache: %v", err)
				}
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

// customKeyNullSentinel is written to the partition key for a NULL-valued custom key
// column so NULLs route deterministically (and cannot collide with any real value).
const customKeyNullSentinel = "\x00NULL\x00"

// GetEventPartitionKey returns a deterministic string that identifies the routing
// partition of an event under its table's CDC partitioning strategy:
//   - PARTITION_BY_PK:     table + primary-key column values
//   - PARTITION_BY_TABLE:  table only (all events of the table share one partition)
//   - PARTITION_BY_CUSTOM: table + custom key column values (immutable; NULL -> sentinel)
//
// This single function is the source of truth for both routing and conflict exclusion:
//   - hashEvent hashes the partition key to pick the event's channel.
//   - the conflict cache excludes cached events that share the incoming event's partition
//     key, because equal partition keys hash to the same channel and are therefore applied
//     in commit order and can never race.
//
// Because the same string drives both, "same partition key" always implies "same channel"
// by construction, so the exclusion is always safe regardless of value encoding.
//
// Note: the byte layout (table prefix followed by the raw values, no separators) is kept
// identical to the previous hashEvent implementation so channel assignment - which is
// baked into per-channel resumption state - does not change across upgrades for existing
// pk/table migrations.
func GetEventPartitionKey(e *tgtdb.Event, tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) (string, error) {
	if tablePartitionKeyMap == nil {
		return "", goerrors.Errorf("table partition key map is not initialized")
	}
	partitionKey, ok := tablePartitionKeyMap.Get(e.TableNameTup)
	if !ok {
		return "", goerrors.Errorf("table partition key map does not contain table %v", e.TableNameTup)
	}

	var b strings.Builder
	// Prefix with the table so events of different tables never share a partition key.
	b.WriteString(e.TableNameTup.ForKey())

	switch partitionKey.Strategy {
	case PARTITION_BY_PK:
		// include primary-key column values, sorted by column name for a stable order.
		keyColumns := make([]string, 0, len(e.Key))
		for k := range e.Key {
			keyColumns = append(keyColumns, k)
		}
		sort.Strings(keyColumns)
		for _, k := range keyColumns {
			b.WriteString(*e.Key[k])
		}
	case PARTITION_BY_TABLE:
		// table prefix alone is the partition key.
	case PARTITION_BY_CUSTOM:
		columns := partitionKey.Columns
		if len(columns) == 0 {
			return "", goerrors.Errorf("custom partition key columns not found for table %v", e.TableNameTup)
		}
		sort.Strings(columns)
		// Custom key columns are immutable (guardrail enforced separately), so a row's
		// key value is the same in BeforeFields (update/delete) and Fields (insert).
		// Prefer BeforeFields (present for update/delete under REPLICA IDENTITY FULL);
		// fall back to Fields for inserts.
		for _, col := range columns {
			val, ok, err := customPartitionKeyColumnValue(e, col)
			if err != nil {
				return "", err
			}
			if !ok {
				return "", goerrors.Errorf("custom partition key column %q not found in event(vsn=%d) for table %v; ensure REPLICA IDENTITY FULL is set", col, e.Vsn, e.TableNameTup)
			}
			if val == nil {
				b.WriteString(customKeyNullSentinel)
			} else {
				b.WriteString(*val)
			}
		}
	default:
		return "", goerrors.Errorf("invalid partitioning strategy: %s", partitionKey.Strategy)
	}
	return b.String(), nil
}

// Returns a hash value between 0..NUM_EVENT_CHANNELS
func hashEvent(e *tgtdb.Event, tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) (int, error) {
	partitionKey, err := GetEventPartitionKey(e, tablePartitionKeyMap)
	if err != nil {
		return 0, err
	}
	hash := fnv.New64a()
	hash.Write([]byte(partitionKey))
	return int(hash.Sum64() % (uint64(NUM_EVENT_CHANNELS))), nil
}

// customPartitionKeyColumnValue returns the value as per the event operation for the custom partition key column
// if the column is not found, the second return is false

func customPartitionKeyColumnValue(e *tgtdb.Event, col string) (*string, bool, error) {
	switch e.Op {
	case "i":
		val, ok := e.Fields[col]
		if ok {
			return val, true, nil
		} else {
			return nil, false, goerrors.Errorf("custom partition key column %q not found in event(vsn=%d) for table %v", col, e.Vsn, e.TableNameTup)
		}
	case "u":
		_, ok := e.Fields[col]
		if ok {
			return nil, false, goerrors.Errorf("custom partition key column %q is required to be immutable, it is changing in update event(vsn=%d) for table %v", col, e.Vsn, e.TableNameTup)
		}
		val, ok := e.BeforeFields[col]
		if ok {
			return val, true, nil
		} else {
			return nil, false, goerrors.Errorf("custom partition key column %q not found in event(vsn=%d) for table %v; ensure REPLICA IDENTITY FULL is set", col, e.Vsn, e.TableNameTup)
		}
	case "d":
		val, ok := e.BeforeFields[col]
		if ok {
			return val, true, nil
		} else {
			return nil, false, goerrors.Errorf("custom partition key column %q not found in event(vsn=%d) for table %v;", col, e.Vsn, e.TableNameTup)
		}
	}
	return nil, false, nil
}
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
func initializeConflictDetectionCache(evChans []chan *tgtdb.Event, sourceDBTypeForConflictCache string, importTableList []sqlname.NameTuple, tablePartitionKeyMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) error {
	log.Infof("fetching table to unique indexes map from import target (%s)", tconf.TargetDBType)
	tableToUniqueIndexes, err := tdb.GetTableToUniqueIndexesMap(importTableList)
	if err != nil {
		return fmt.Errorf("get table unique indexes map from target: %w", err)
	}

	log.Infof("initializing conflict detection cache")
	conflictDetectionCache = NewConflictDetectionCache(tableToUniqueIndexes, evChans, sourceDBTypeForConflictCache, tablePartitionKeyMap)
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
