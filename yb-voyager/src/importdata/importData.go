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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	goerrors "github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/gosuri/uitable"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/adaptiveparallelism"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metrics"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/monitor"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	reporter "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/stats"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/types"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// Config holds every value the import engine reads that cmd used to keep in
// package-level variables. cmd fills it from its flags and command-level setup
// and passes it by value to NewImporter; nothing in here is a callback.
type Config struct {
	ExportDir                string
	MetaDB                   *metadb.MetaDB
	MigrationUUID            uuid.UUID
	ImporterRole             string
	ImportType               string
	IdentityColumnsMetaDBKey string
	SourceDBType             string
	Source                   srcdb.Source
	Tconf                    tgtdb.TargetConf
	Tdb                      tgtdb.TargetDB
	ControlPlane             cp.ControlPlane

	StartClean                     utils.BoolStr
	TruncateTables                 utils.BoolStr
	TruncateSplits                 utils.BoolStr
	DisablePb                      utils.BoolStr
	BatchSizeInNumRows             int64
	ErrorPolicySnapshot            ErrorPolicy
	EnableRandomBatchProduction    utils.BoolStr
	MaxConcurrentBatchProductions  int
	SkipReplicationChecks          utils.BoolStr
	SkipNodeHealthChecks           utils.BoolStr
	SkipDiskUsageHealthChecks      utils.BoolStr
	CdcPartitionKey                string
	CdcPartitionKeyOverrides       string                             // raw flag value: persisted to metadb and compared on resume
	CdcPartitionKeyOverridesParsed map[string]CdcPartitionKeyOverride // the same value parsed once by cmd's flag validation
	ImportUsePartitionRoot         utils.BoolStr
	EventBatchMaxRetryCount        int
	ReportProgressInBytes          bool

	DataFileDescriptor       *datafile.Descriptor
	DataStore                datastore.DataStore
	ImportTableList          []sqlname.NameTuple
	CallhomeMetricsCollector *callhome.ImportDataMetricsCollector
	// SnapshotImportStartedEvent is the control-plane event ImportData emits for the
	// target importer; cmd builds it from its own state (migration UUID, target conf).
	SnapshotImportStartedEvent cp.SnapshotImportStartedEvent
}

// Importer is the import-data engine: the snapshot import plus the CDC streaming
// phase for one importer role. Its fields are the former cmd package-level runtime
// variables; cmd reads the few it needs through the accessors below.
type Importer struct {
	cfg Config

	batchImportPool            *pool.Pool
	colocatedBatchImportPool   *pool.Pool
	colocatedBatchImportQueue  chan func()
	progressReporter           *ImportDataProgressReporter
	valueConverter             dbzm.SnapshotPhaseValueConverter
	tableToColumnNames         *utils.StructMap[sqlname.NameTuple, []string] // map of table name to columnNames
	tableToIdentityColumnNames *utils.StructMap[sqlname.NameTuple, []string] // map of table name to generated always as identity column's names
	tableNameToSchema          *utils.StructMap[sqlname.NameTuple, map[string]map[string]string]
	conflictDetectionCache     *ConflictDetectionCache
	eventQueue                 *EventQueue
	statsReporter              *reporter.StreamImportStatsReporter
	importPhase                string
	prevExporterRole           string // used to determine if cache reinitialization is needed

	state        *ImportDataState
	errorHandler ImportDataErrorHandler
}

func NewImporter(cfg Config) *Importer {
	return &Importer{
		cfg:                cfg,
		tableToColumnNames: utils.NewStructMap[sqlname.NameTuple, []string](),
		importPhase:        dbzm.MODE_SNAPSHOT,
	}
}

// ImportPhase is the phase the engine is in (snapshot, streaming, or a cutover
// phase name); cmd's callhome payloads report it.
func (imp *Importer) ImportPhase() string { return imp.importPhase }

// StatsReporter is the streaming-phase stats reporter, nil until streaming starts.
func (imp *Importer) StatsReporter() *reporter.StreamImportStatsReporter { return imp.statsReporter }

// State is the import state ImportData created; nil until ImportData runs.
func (imp *Importer) State() *ImportDataState { return imp.state }

// ErrorHandler is the error handler ImportData created; nil until ImportData runs.
func (imp *Importer) ErrorHandler() ImportDataErrorHandler { return imp.errorHandler }

// ShutdownProgressBars stops the mpb progress container so that its rendering
// goroutine no longer writes to stdout.
func (imp *Importer) ShutdownProgressBars() {
	if imp.progressReporter != nil {
		imp.progressReporter.Shutdown()
	}
}

// Component configs derived from Config plus the runtime state the components read.
func (imp *Importer) importDataStateConfig() ImportDataStateConfig {
	return ImportDataStateConfig{
		ExportDir:          imp.cfg.ExportDir,
		ImporterRole:       imp.cfg.ImporterRole,
		Tdb:                imp.cfg.Tdb,
		Tconf:              imp.cfg.Tconf,
		MigrationUUID:      imp.cfg.MigrationUUID,
		TruncateSplits:     imp.cfg.TruncateSplits,
		DataFileDescriptor: imp.cfg.DataFileDescriptor,
	}
}

func (imp *Importer) sequentialFileBatchProducerConfig() SequentialFileBatchProducerConfig {
	return SequentialFileBatchProducerConfig{
		ImporterRole:       imp.cfg.ImporterRole,
		Tdb:                imp.cfg.Tdb,
		Tconf:              imp.cfg.Tconf,
		DataFileDescriptor: imp.cfg.DataFileDescriptor,
		DataStore:          imp.cfg.DataStore,
		BatchSizeInNumRows: imp.cfg.BatchSizeInNumRows,
		ValueConverter:     imp.valueConverter,
		TableToColumnNames: imp.tableToColumnNames,
	}
}

func (imp *Importer) fileTaskImporterConfig() FileTaskImporterConfig {
	return FileTaskImporterConfig{
		ImporterRole:          imp.cfg.ImporterRole,
		Tdb:                   imp.cfg.Tdb,
		Tconf:                 imp.cfg.Tconf,
		ExportDir:             imp.cfg.ExportDir,
		MigrationUUID:         imp.cfg.MigrationUUID,
		DataFileDescriptor:    imp.cfg.DataFileDescriptor,
		ReportProgressInBytes: imp.cfg.ReportProgressInBytes,
		ControlPlane:          imp.cfg.ControlPlane,
		TableToColumnNames:    imp.tableToColumnNames,
		TableNameToSchema:     imp.tableNameToSchema,
	}
}

func (imp *Importer) initialiseErrorHandler() (ImportDataErrorHandler, error) {
	exportDirDataDir := filepath.Join(imp.cfg.ExportDir, "data")
	errorHandler, err := GetImportDataErrorHandler(imp.cfg.ErrorPolicySnapshot, exportDirDataDir, imp.cfg.ImporterRole)
	if err != nil {
		return nil, goerrors.Errorf("Failed to initialize error handler: %w", err)
	}
	err = imp.updateErrorPolicyInMetaDB(imp.cfg.ErrorPolicySnapshot)
	if err != nil {
		return nil, goerrors.Errorf("Failed to update error policy in meta DB: %w", err)
	}
	return errorHandler, nil
}

func (imp *Importer) updateTargetConfInMigrationStatus() error {
	err := imp.cfg.MetaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch imp.cfg.ImporterRole {
		case constants.TARGET_DB_IMPORTER_ROLE, constants.IMPORT_FILE_ROLE:
			record.TargetDBConf = imp.cfg.Tconf.Clone()
			record.TargetDBConf.Password = ""
			record.TargetDBConf.Uri = ""
		case constants.SOURCE_REPLICA_DB_IMPORTER_ROLE:
			record.SourceReplicaDBConf = imp.cfg.Tconf.Clone()
			record.SourceReplicaDBConf.Password = ""
			record.SourceReplicaDBConf.Uri = ""
		case constants.SOURCE_DB_IMPORTER_ROLE:
			record.SourceDBAsTargetConf = imp.cfg.Tconf.Clone()
			record.SourceDBAsTargetConf.Password = ""
			record.SourceDBAsTargetConf.Uri = ""
		default:
			panic(fmt.Sprintf("unsupported importer role: %s", imp.cfg.ImporterRole))
		}
	})
	if err != nil {
		return goerrors.Errorf("Failed to update target conf in migration status record: %w", err)
	}
	return nil
}

func (imp *Importer) prepareTargetDBForImport() error {
	//init target db connection pool
	err := imp.cfg.Tdb.InitConnPool()
	if err != nil {
		return goerrors.Errorf("Failed to initialize the target DB connection pool: %w", err)
	}

	//start adaptive parallelism
	var adaptiveParallelismStarted bool
	adaptiveParallelismStarted, err = imp.startAdaptiveParallelism(imp.cfg.Tconf.AdaptiveParallelismMode, imp.cfg.CallhomeMetricsCollector)
	if err != nil {
		return goerrors.Errorf("Failed to start adaptive parallelism: %w", err)
	}
	//start monitoring target YB health
	err = imp.startMonitoringTargetYBHealth()
	if err != nil {
		return goerrors.Errorf("Failed to start monitoring health: %w", err)
	}
	if adaptiveParallelismStarted {
		utils.PrintAndLogf("Using 1-%d parallel jobs (adaptive)", imp.cfg.Tconf.MaxParallelism)
	} else {
		utils.PrintAndLogf("Using %d parallel jobs.", imp.cfg.Tconf.Parallelism)
	}

	targetDBVersion := imp.cfg.Tdb.GetVersion()
	fmt.Printf("%s version: %s\n", imp.cfg.Tconf.TargetDBType, targetDBVersion)

	//create voyager metadata schema
	err = imp.cfg.Tdb.CreateVoyagerSchema()
	if err != nil {
		return goerrors.Errorf("Failed to create voyager metadata schema on target DB: %w", err)
	}
	return nil
}

func (imp *Importer) handleStartCleanForSnapshot(state *ImportDataState, importFileTasks []*ImportFileTask, errorHandler ImportDataErrorHandler) error {
	imp.cleanImportState(state, importFileTasks)
	err := cleanStoredErrors(errorHandler, importFileTasks)
	if err != nil {
		return goerrors.Errorf("Failed to clean stored errors: %w", err)
	}
	return nil
}

func (imp *Importer) initialiseValueConverter(importTableList []sqlname.NameTuple, msr *metadb.MigrationStatusRecord) error {
	var err error
	if msr.IsSnapshotExportedViaDebezium() {
		imp.valueConverter, err = dbzm.NewSnapshotPhaseValueConverter(imp.cfg.ExportDir, imp.cfg.Tdb, imp.cfg.Tconf, imp.cfg.ImporterRole, msr.SourceDBConf.DBType, importTableList)
	} else {
		imp.valueConverter, err = dbzm.NewSnapshotPhaseNoOpValueConverter()
	}
	if err != nil {
		return goerrors.Errorf("Failed to create value converter: %w", err)
	}

	imp.tableNameToSchema, err = imp.valueConverter.GetTableNameToSchema()
	if err != nil {
		return goerrors.Errorf("Failed to get table name to schema: %w", err)
	}
	return nil
}

func (imp *Importer) handleIdentityColumns(importTableList []sqlname.NameTuple) error {
	err := imp.fetchAndStoreGeneratedAlwaysIdentityColumnsInMetadb(importTableList)
	if err != nil {
		return goerrors.Errorf("Failed to fetch and store generated always identity columns: %w", err)
	}
	err = imp.disableGeneratedAlwaysAsIdentityColumns()
	if err != nil {
		return goerrors.Errorf("Failed to disable generated always identity columns: %w", err)
	}
	return nil
}

func (imp *Importer) importSnapshotData(msr *metadb.MigrationStatusRecord, errorHandler ImportDataErrorHandler,
	state *ImportDataState, importFileTasks []*ImportFileTask, importTableList []sqlname.NameTuple) error {
	var err error
	var pendingTasks, completedTasks []*ImportFileTask

	if imp.cfg.StartClean {
		pendingTasks = importFileTasks
	} else {
		pendingTasks, completedTasks, err = classifyTasksForImport(state, importFileTasks)
		if err != nil {
			utils.ErrExit("Failed to classify tasks: %w", err)
		}
	}
	log.Infof("pending tasks: %v", pendingTasks)
	log.Infof("completed tasks: %v", completedTasks)

	err = imp.runPKConflictModeGuardrails(state, importFileTasks)
	if err != nil {
		utils.ErrExit("Error checking PK conflict mode on fresh start: %w", err)
	}

	err = imp.initialiseValueConverter(importTableList, msr)
	if err != nil {
		utils.ErrExit("Failed to initialize value converter: %w", err)
	}

	utils.PrintAndLogf("Already imported tables: %v", ImportFileTasksToTableNames(completedTasks))
	if len(pendingTasks) == 0 {
		utils.PrintAndLogf("All the tables are already imported, nothing left to import\n")
		return nil
	}
	utils.PrintAndLogf("Tables to import: %v", ImportFileTasksToTableNames(pendingTasks))
	err = imp.prepareTableToColumns(pendingTasks) //prepare the tableToColumns map
	if err != nil {
		utils.ErrExit("failed to prepare table to columns: %w", err)
	}
	maxParallelConns, err := imp.getMaxParallelConnections()
	if err != nil {
		utils.ErrExit("Failed to get max parallel connections: %w", err)
	}
	if !imp.cfg.Tconf.AdaptiveParallelismMode.IsEnabled() {
		// Adaptive parallelism emits this gauge itself once it starts polling;
		// for a fixed --parallel-jobs run there's no such poller, so emit once here.
		metrics.Get().SetImportParallelism(imp.cfg.ImporterRole, maxParallelConns)
	}
	importDataAllTableMetrics := imp.createInitialImportDataTableMetrics(importFileTasks, pendingTasks)
	if imp.cfg.ImporterRole == constants.TARGET_DB_IMPORTER_ROLE {
		imp.cfg.ControlPlane.UpdateImportedRowCount(importDataAllTableMetrics)
	}

	useTaskPicker := utils.GetEnvAsBool("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT", true)
	if useTaskPicker {
		maxColocatedBatchesInProgress := utils.GetEnvAsInt("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS", 3)
		err := imp.importTasksViaTaskPicker(pendingTasks, state, imp.progressReporter,
			maxParallelConns, maxParallelConns, maxColocatedBatchesInProgress, msr.IsSnapshotExportedViaDebezium(),
			imp.cfg.MaxConcurrentBatchProductions, bool(imp.cfg.EnableRandomBatchProduction),
			errorHandler, imp.cfg.CallhomeMetricsCollector)
		if err != nil {
			utils.ErrExit("Failed to import tasks via task picker. %w", err)
		}
	} else {
		poolSize := maxParallelConns * 2
		for _, task := range pendingTasks {
			// The code can produce `poolSize` number of batches at a time. But, it can consume only
			// `parallelism` number of batches at a time.
			imp.batchImportPool = pool.New().WithMaxGoroutines(poolSize)
			log.Infof("created batch import pool of size: %d", poolSize)

			batchProducer, err := NewSequentialFileBatchProducer(imp.sequentialFileBatchProducerConfig(), task, state, msr.IsSnapshotExportedViaDebezium(), errorHandler, imp.progressReporter)
			if err != nil {
				utils.ErrExit("Failed to create batch producer: %w", err)
			}

			taskImporter, err := NewFileTaskImporter(imp.fileTaskImporterConfig(), task, state, batchProducer, imp.batchImportPool, imp.progressReporter, nil, false, errorHandler, imp.cfg.CallhomeMetricsCollector)
			if err != nil {
				utils.ErrExit("Failed to create file task importer: %w", err)
			}

			for !taskImporter.AllBatchesSubmitted() {
				err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
				if err != nil {
					utils.ErrExit("Failed to submit next batch: task:%v err: %w", task, err)
				}
			}

			imp.batchImportPool.Wait() // wait for file import to finish
			taskImporter.PostProcess()
		}
		time.Sleep(time.Second * 2)
	}

	return nil
}

func (imp *Importer) ImportData(importFileTasks []*ImportFileTask) {
	errorHandler, err := imp.initialiseErrorHandler()
	if err != nil {
		utils.ErrExit("Failed to initialize error policy and error handler: %w", err)
	}

	if imp.cfg.ImporterRole == constants.TARGET_DB_IMPORTER_ROLE {
		importDataStartEvent := imp.cfg.SnapshotImportStartedEvent
		imp.cfg.ControlPlane.SnapshotImportStarted(&importDataStartEvent)
	}
	err = imp.updateTargetConfInMigrationStatus()
	if err != nil {
		utils.ErrExit("Failed to update target conf in migration status record: %w", err)
	}
	msr, err := imp.cfg.MetaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %w", err)
	}
	//create progress reporter
	imp.progressReporter = NewImportDataProgressReporter(bool(imp.cfg.DisablePb))

	err = imp.prepareTargetDBForImport()
	if err != nil {
		utils.ErrExit("Failed to prepare target DB for import: %w", err)
	}

	state := NewImportDataState(imp.importDataStateConfig())
	// kept on the importer so the caller's post-processing (sequence restore,
	// cutover) can read import state and stashed errors
	imp.state = state
	imp.errorHandler = errorHandler

	err = imp.clearMigrationStateForImportDataStartClean(state, importFileTasks, errorHandler)
	if err != nil {
		utils.ErrExit("Failed to clean MigrationStatusRecord for import data start clean: %w", err)
	}

	var tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]
	if changeStreamingIsEnabled(imp.cfg.ImportType) {
		tableToUniqueIndexes, err = imp.cfg.Tdb.GetTableToUniqueIndexesMap(imp.cfg.ImportTableList)
		if err != nil {
			utils.ErrExit("Failed to get table unique indexes map from target: %s", err)
		}
	}
	// Validate/resolve cdc-partition-key (+ overrides) and persist the per-table map
	// before snapshot so bad configs fail fast (not at streamChanges).
	// Runs after start-clean so a cleared map is recomputed for the new run.
	// Must run before updateImportDataStartedInMetaDB so a failed prepare does not
	// lock change-guard / ImportDataStarted for a config that never took effect.
	err = imp.prepareCdcPartitionKey(imp.cfg.ImportTableList, tableToUniqueIndexes)
	if err != nil {
		utils.ErrExit("Failed to prepare cdc-partition-key: %w", err)
	}

	// Fetch the primary-key columns of the import tables from the target (before snapshot) so
	// they can be passed to streamChanges and the conflict-detection cache without re-querying
	// during streaming. Also fails fast if a custom-partition-key table has no primary key.
	importTableToPKColumns, err := imp.getPrimaryKeyColumnsForImportTables(imp.cfg.ImportTableList)
	if err != nil {
		utils.ErrExit("Failed to get primary key columns for import tables: %w", err)
	}
	//updating the metadb after the startclean clears any required metadb state
	err = imp.updateImportDataStartedAndSomeConfigsInMetaDB()
	if err != nil {
		utils.ErrExit("Failed to update import data started in meta DB: %w", err)
	}

	if state.HasExistingState() {
		utils.PrintAndLogf("\nResuming import of data in %q database", imp.cfg.Tconf.DBName)
	} else {
		utils.PrintAndLogf("\nimport of data in %q database started", imp.cfg.Tconf.DBName)
	}

	// Handle identity columns before snapshot import for Oracle targets (source-replica/source)
	// Oracle requires GENERATED ALWAYS columns to be converted to GENERATED BY DEFAULT for COPY to work
	if imp.identityColumnsNeedHandlingForSnapshotImport() {
		err = imp.handleIdentityColumns(imp.cfg.ImportTableList)
		if err != nil {
			utils.ErrExit("Failed to handle identity columns: %w", err)
		}
	}

	// Import snapshots
	if ImportSnapshotRequired(imp.cfg.ImporterRole, imp.cfg.ImportType) {
		err = imp.importSnapshotData(msr, errorHandler, state, importFileTasks, imp.cfg.ImportTableList)
		if err != nil {
			utils.ErrExit("failed to import snapshot data: %w", err)
		}
		utils.PrintAndLogf("snapshot data import complete\n\n")
	}

	if changeStreamingIsEnabled(imp.cfg.ImportType) {
		// For non-Oracle targets (YugabyteDB/PostgreSQL), handle identity columns before streaming phase
		// For Oracle targets, this was already done before snapshot import
		if !imp.identityColumnsNeedHandlingForSnapshotImport() {
			err = imp.handleIdentityColumns(imp.cfg.ImportTableList)
			if err != nil {
				utils.ErrExit("Failed to handle identity columns: %w", err)
			}
		}
		if ImportSnapshotRequired(imp.cfg.ImporterRole, imp.cfg.ImportType) {
			DisplayImportedRowCountSnapshot(imp.importDataStateConfig(), state, importFileTasks, errorHandler)
		}
		err = imp.streamChanges(state, imp.cfg.ImportTableList, importTableToPKColumns, tableToUniqueIndexes)
		if err != nil {
			utils.ErrExit("Failed to stream changes to %s: %w", imp.cfg.Tconf.TargetDBType, err)
		}
	}
	// The post-processing that followed here (sequence restore, cutover
	// processing, the snapshot import report) is run by the caller; see
	// cmd's importDataPostProcessing.
}

/*
getPrimaryKeyColumnsForImportTables fetches the primary-key columns of every import table
from the target DB (in one batched query) so they can be threaded into the streaming
conflict-detection cache.

It is called once near the start of importData (for the target PG→YB live path only; other
paths return an empty map since conflict detection does not run for them). Because importData
runs in the same process before streamChanges on both the first run and resume, the map does
not need to be persisted in metaDB.

It also fails fast, before the snapshot import, if a table routed by a custom partition key
has no primary key on the target: custom routing adds the PK as a synthetic unique index for
conflict detection (a recycled PK across different custom keys must be serialized), so a
custom-key table without a PK cannot be made correct. This is scoped to custom-key tables so
that legitimately PK-less tables under pk/table routing (e.g. partitioned roots imported via
--use-partition-root) are not blocked.
*/
func (imp *Importer) getPrimaryKeyColumnsForImportTables(tableNames []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, []string], error) {
	tableToPKColumns := utils.NewStructMap[sqlname.NameTuple, []string]()

	// Only the target PG→YB live streaming path runs conflict detection and needs primary keys.
	if imp.cfg.ImporterRole != constants.TARGET_DB_IMPORTER_ROLE || !changeStreamingIsEnabled(imp.cfg.ImportType) || imp.cfg.SourceDBType != constants.POSTGRESQL {
		return tableToPKColumns, nil
	}

	tableToPKColumns, err := imp.cfg.Tdb.GetPrimaryKeyColumnsForTables(tableNames)
	if err != nil {
		return nil, fmt.Errorf("error getting primary key columns for import tables: %w", err)
	}

	var tablesWithoutPK []sqlname.NameTuple
	for _, t := range tableNames {
		if pkColumns, _ := tableToPKColumns.Get(t); len(pkColumns) == 0 {
			tablesWithoutPK = append(tablesWithoutPK, t)
		}
	}
	if len(tablesWithoutPK) > 0 {
		return nil, goerrors.Errorf("table(s) %v have no primary key on the target; live migration is not allowed for these tables", tablesWithoutPK)
	}

	return tableToPKColumns, nil
}

// For a fresh start but non empty tables in tableList && OnPrimaryKeyConflict is set to IGNORE -> notify user
func (imp *Importer) runPKConflictModeGuardrails(state *ImportDataState, allTasks []*ImportFileTask) error {
	// in case of ERROR mode, no need to check for non-empty tables
	// but for IGNORE or UPDATE(in future), we need to prompt user
	if imp.cfg.Tconf.OnPrimaryKeyConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_ERROR_POLICY {
		return nil
	}

	if !isTargetDBImporter(imp.cfg.ImporterRole) {
		return nil
	}

	if !isFreshStart(state, allTasks) {
		log.Info("Not a fresh start, skipping primary key conflict mode check.")
		return nil
	}

	pendingTasks := getPendingTasks(state, allTasks)
	pendingTablesList := ImportFileTasksToTableNameTuples(pendingTasks)
	nonEmptyTables := imp.cfg.Tdb.GetNonEmptyTables(pendingTablesList)
	if len(nonEmptyTables) == 0 {
		log.Info("No non-empty tables found in the target DB, skipping primary key conflict mode check.")
		return nil
	}

	tableToPKColumns, err := imp.cfg.Tdb.GetPrimaryKeyColumnsForTables(nonEmptyTables)
	if err != nil {
		return fmt.Errorf("failed to get primary key columns for tables: %w", err)
	}
	var nonEmptyTablesWithPK []sqlname.NameTuple
	for _, table := range nonEmptyTables {
		colList, _ := tableToPKColumns.Get(table)
		if len(colList) > 0 { // table has PK
			nonEmptyTablesWithPK = append(nonEmptyTablesWithPK, table)
		}
	}

	// all nonEmptyTables have no primary key columns
	if len(nonEmptyTablesWithPK) == 0 {
		log.Infof("No non-empty tables with primary key found in %v, skipping primary key conflict mode check.",
			sqlname.NameTupleListToStrings(nonEmptyTables))
		return nil
	}

	utils.PrintAndLogf(
		"\nTarget tables with pre-existing data: %v\n"+
			"Note that because of the config on-primary-key-conflict as 'IGNORE', rows that have a primary key conflict "+
			"with an existing row in the above set of tables will be silently ignored.\n",
		sqlname.NameTupleListToStrings(nonEmptyTablesWithPK),
	)
	if !utils.AskPrompt("Please confirm whether to proceed") {
		utils.ErrExit("Aborting import.")
	}

	return nil
}

// A fresh start is when all tasks are pending(non-zero), no started or completed tasks.
func isFreshStart(state *ImportDataState, allTasks []*ImportFileTask) bool {
	inProgressTasks := getInProgressTasks(state, allTasks)
	notStartedTasks := getNotStartedTasks(state, allTasks)
	completedTasks := getCompletedTasks(state, allTasks)

	return len(inProgressTasks) == 0 && len(completedTasks) == 0 && len(notStartedTasks) == len(allTasks)
}

// restoreGeneratedIdentityColumns enables & restores generated identity columns
// 1. GENERATED ALWAYS: Re-enables(also restores the values) 'GENERATED ALWAYS' identity columns previously disabled before import data
// 2. GENERATED BY DEFAULT: Remaining columns were always 'generated by default'; just restore their values.
func (imp *Importer) RestoreGeneratedIdentityColumns(importTableList []sqlname.NameTuple) error {
	if importTableList == nil {
		return nil
	}

	err := imp.enableGeneratedAlwaysAsIdentityColumns()
	if err != nil {
		return err
	}

	// TODO: these sequences of all identity columns are also covered as part of the RestoreSequences as well so we don't need this ALTER
	// see if we should remove it.
	err = imp.restoreGeneratedByDefaultAsIdentityColumns(importTableList)
	if err != nil {
		return err
	}

	return nil
}

func (imp *Importer) getMaxParallelConnections() (int, error) {
	maxParallelConns := imp.cfg.Tconf.Parallelism
	if imp.cfg.Tconf.AdaptiveParallelismMode.IsEnabled() {
		// in case of adaptive parallelism, we need to use maxParalllelism * 2
		yb, ok := imp.cfg.Tdb.(*tgtdb.TargetYugabyteDB)
		if !ok {
			return 0, goerrors.Errorf("adaptive parallelism is only supported if target DB is YugabyteDB")
		}
		maxParallelConns = yb.GetNumMaxConnectionsInPool()
	}
	return maxParallelConns, nil
}

func waitIfNoBatchAvailableForAllTasks(taskPicker FileTaskPicker, taskImporters map[int]*FileTaskImporter) {
	inProgressTasks := taskPicker.InProgressTasks()
	if len(inProgressTasks) == 0 {
		return
	}

	allTasksBatchNotAvailable := true

	for _, task := range inProgressTasks {
		importer, exists := taskImporters[task.ID]
		if !exists {
			// Importer not yet created for this task - the picker has picked a new task
			// that the main loop hasn't processed yet. Don't wait, let the loop create it.
			return
		}

		if importer.IsNextBatchAvailable() {
			allTasksBatchNotAvailable = false
			break
		}
	}

	if allTasksBatchNotAvailable {
		log.Infof("No batches available for all in-progress tasks. Waiting for batch production.")
		time.Sleep(100 * time.Millisecond)
	}
}

func waitIfAllBatchesSubmittedForAllTasks(taskPicker FileTaskPicker, taskImporters map[int]*FileTaskImporter) {
	inProgressTasks := taskPicker.InProgressTasks()
	if len(inProgressTasks) == 0 {
		return
	}

	allTasksAllBatchesSubmitted := true

	for _, task := range inProgressTasks {
		importer, exists := taskImporters[task.ID]
		if !exists {
			// Importer not yet created for this task - the picker has picked a new task
			// that the main loop hasn't processed yet. Don't wait, let the loop create it.
			return
		}
		if !importer.AllBatchesSubmitted() {
			allTasksAllBatchesSubmitted = false
			break
		}
	}

	if allTasksAllBatchesSubmitted {
		log.Infof("All batches submitted for all in-progress tasks. Waiting for import completion.")
		time.Sleep(100 * time.Millisecond)
	}
}

/*
1. Initialize a worker pool. In case of TARGET_DB_IMPORTER_ROLE  or IMPORT_FILE_ROLE, also create a colocated batch import pool and a corresponding queue.
2. Create a task picker which helps the importer choose which task to process in each iteration.
3. Loop until all tasks are done:
  - Pick a task from the task picker.
  - If the task is not already being processed, create a new FileTaskImporter for the task.
  - For the task that is picked, produce the next batch and submit it to the worker pool. Worker will asynchronously import the batch.
  - If task is done, mark it as done in the task picker.
*/
func (imp *Importer) importTasksViaTaskPicker(pendingTasks []*ImportFileTask, state *ImportDataState, progressReporter *ImportDataProgressReporter,
	maxParallelConns int, maxShardedTasksInProgress int, maxColocatedBatchesInProgress int,
	isRowTransformationRequired bool, maxConcurrentBatchProductions int, enableRandomBatchProduction bool,
	errorHandler ImportDataErrorHandler, callhomeMetricsCollector *callhome.ImportDataMetricsCollector) error {

	var err error
	imp.setupWorkerPoolAndQueue(maxParallelConns, maxColocatedBatchesInProgress)
	taskImporters := map[int]*FileTaskImporter{}
	tableTypes, err := GetTableTypes(imp.cfg.ImporterRole, imp.cfg.Tconf.TargetDBType, imp.cfg.Tdb, pendingTasks)
	if err != nil {
		return fmt.Errorf("get table types: %w", err)
	}

	// Initialize semaphore to limit concurrent batch productions
	concurrentBatchProductionSem := semaphore.NewWeighted(int64(maxConcurrentBatchProductions))

	var taskPicker FileTaskPicker
	// The colocation-aware task picker only applies to a real YugabyteDB
	// target. yb-amp (PostgreSQL-compatible, no colocation/tablets) and the
	// PG fall-forward/back roles use the sequential picker instead.
	if (imp.cfg.ImporterRole == constants.TARGET_DB_IMPORTER_ROLE || imp.cfg.ImporterRole == constants.IMPORT_FILE_ROLE) && imp.cfg.Tconf.TargetDBType == constants.YUGABYTEDB {
		yb, ok := imp.cfg.Tdb.(*tgtdb.TargetYugabyteDB)
		if !ok {
			return goerrors.Errorf("expected tdb to be of type TargetYugabyteDB, got: %T", imp.cfg.Tdb)
		}
		taskPicker, err = NewColocatedCappedRandomTaskPicker(maxShardedTasksInProgress, maxColocatedBatchesInProgress, pendingTasks, state, yb, imp.colocatedBatchImportQueue, tableTypes)
		if err != nil {
			return fmt.Errorf("create colocated aware randmo task picker: %w", err)
		}
	} else {
		taskPicker, err = NewSequentialTaskPicker(pendingTasks, state)
		if err != nil {
			return fmt.Errorf("create sequential task picker: %w", err)
		}
	}

	for taskPicker.HasMoreTasks() {
		task, err := taskPicker.Pick()
		if err != nil {
			return fmt.Errorf("get next task: %w", err)
		}
		log.Debugf("Picked task for import: %s", task)
		var taskImporter *FileTaskImporter
		var ok bool
		taskImporter, ok = taskImporters[task.ID]
		if !ok {
			taskImporter, err = imp.createFileTaskImporter(task, state, imp.batchImportPool, progressReporter, imp.colocatedBatchImportQueue, tableTypes, isRowTransformationRequired, enableRandomBatchProduction, concurrentBatchProductionSem, errorHandler, callhomeMetricsCollector)
			if err != nil {
				return fmt.Errorf("create file task importer: %w", err)
			}
			log.Infof("created file task importer for table: %s, task: %v", task.TableNameTup.ForOutput(), task)
			taskImporters[task.ID] = taskImporter
		}

		if taskImporter.AllBatchesSubmitted() {
			// All batches for this task have been submitted.
			// task could have been completed (all batches imported) OR still in progress
			// in case task is done, we should inform task picker so that we stop picking that task.
			log.Debugf("All batches submitted for task: %s", task)
			taskDone, err := state.AllBatchesImported(task.FilePath, task.TableNameTup)
			if err != nil {
				return fmt.Errorf("check if all batches are imported: task: %v err :%w", task, err)
			}
			if taskDone {
				taskImporter.PostProcess()
				err = taskPicker.MarkTaskAsDone(task)
				if err != nil {
					return fmt.Errorf("mark task as done: task: %v, err: %w", task, err)
				}
				state.UnregisterFileTaskImporter(taskImporter)
				log.Infof("Import of task done: %s", task)
				continue
			}
			// Batches still being imported by workers; continue with some other task.
			waitIfAllBatchesSubmittedForAllTasks(taskPicker, taskImporters)
			continue
		}

		if !taskImporter.IsNextBatchAvailable() {
			// Picked task has no batch ready. Small sleep to prevent tight spinning
			// in case picker keeps returning tasks without batches.
			log.Debugf("No next batch available for table: %s", task.TableNameTup.ForOutput())
			waitIfNoBatchAvailableForAllTasks(taskPicker, taskImporters)
			continue
		}

		log.Infof("Producing and submitting next batch for task: %s", task)

		err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		if err != nil {
			return goerrors.Errorf("submit next batch: task:%v err: %w", task, err)
		}
	}
	return nil
}

func (imp *Importer) setupWorkerPoolAndQueue(maxParallelConns int, maxColocatedBatchesInProgress int) {
	shardedPoolSize := maxParallelConns * 2
	imp.batchImportPool = pool.New().WithMaxGoroutines(shardedPoolSize)
	log.Infof("created batch import pool of size: %d", shardedPoolSize)

	if imp.cfg.ImporterRole == constants.TARGET_DB_IMPORTER_ROLE || imp.cfg.ImporterRole == constants.IMPORT_FILE_ROLE {
		imp.colocatedBatchImportPool = pool.New().WithMaxGoroutines(maxColocatedBatchesInProgress)
		log.Infof("created colocated batch import pool of size: %d", maxColocatedBatchesInProgress)

		imp.colocatedBatchImportQueue = make(chan func(), maxColocatedBatchesInProgress*2)

		colocatedBatchImportQueueConsumer := func() {
			// just read from channel and submit to the worker pool.
			// worker pool has a max size of maxColocatedBatchesInProgress, so it will block if all workers are busy.
			for f := range imp.colocatedBatchImportQueue {
				imp.colocatedBatchImportPool.Go(f)
			}
		}
		go colocatedBatchImportQueueConsumer()
	}
}

/*
when TARGET_DB_IMPORTER_ROLE or IMPORT_FILE_ROLE, we pass on
the batchImportPool and the colocatedBatchImportQueue to the FileTaskImporter
so that it can submit sharded table batches to the batchImportPool,
and colocated table batches to the colocatedBatchImportQueue.

Otherwise, we simply pass the batchImportPool to the FileTaskImporter.
*/
func (imp *Importer) createFileTaskImporter(task *ImportFileTask, state *ImportDataState, batchImportPool *pool.Pool, progressReporter *ImportDataProgressReporter, colocatedBatchImportQueue chan func(),
	tableTypes *utils.StructMap[sqlname.NameTuple, string], isRowTransformationRequired bool, enableRandomBatchProduction bool, concurrentBatchProductionSem *semaphore.Weighted, errorHandler ImportDataErrorHandler, callhomeMetricsCollector *callhome.ImportDataMetricsCollector) (*FileTaskImporter, error) {
	var taskImporter *FileTaskImporter
	var err error
	var batchProducer FileBatchProducer

	// tableTypes (the colocation map) is only populated for a real YugabyteDB
	// target. For yb-amp (PostgreSQL-compatible, no colocation) and the PG
	// fall-forward/back roles it is nil, so they take the plain sequential
	// importer path below.
	// The colocation-aware importer applies only to a real YugabyteDB target;
	// tableTypes is populated (non-nil) exactly for that case (see getTableTypes).
	// Non-YB targets (yb-amp, etc.) take the plain sequential importer below.
	if (imp.cfg.ImporterRole == constants.TARGET_DB_IMPORTER_ROLE || imp.cfg.ImporterRole == constants.IMPORT_FILE_ROLE) && imp.cfg.Tconf.TargetDBType == constants.YUGABYTEDB {
		tableType, ok := tableTypes.Get(task.TableNameTup)
		if !ok {
			return nil, goerrors.Errorf("table type not found for table: %s", task.TableNameTup.ForOutput())
		}

		if enableRandomBatchProduction {
			batchProducer, err = NewRandomFileBatchProducer(imp.sequentialFileBatchProducerConfig(), task, state, isRowTransformationRequired, errorHandler, progressReporter, concurrentBatchProductionSem)
			if err != nil {
				return nil, fmt.Errorf("creating random batch producer: %w", err)
			}
		} else {
			batchProducer, err = NewSequentialFileBatchProducer(imp.sequentialFileBatchProducerConfig(), task, state, isRowTransformationRequired, errorHandler, progressReporter)
			if err != nil {
				return nil, fmt.Errorf("creating sequential batch producer: %w", err)
			}
		}
		taskImporter, err = NewFileTaskImporter(imp.fileTaskImporterConfig(), task, state, batchProducer, batchImportPool, progressReporter, colocatedBatchImportQueue, tableType == COLOCATED, errorHandler, callhomeMetricsCollector)

		if err != nil {
			return nil, fmt.Errorf("create file task importer: %w", err)
		}
	} else {
		batchProducer, err = NewSequentialFileBatchProducer(imp.sequentialFileBatchProducerConfig(), task, state, isRowTransformationRequired, errorHandler, progressReporter)
		if err != nil {
			return nil, fmt.Errorf("creating sequential batch producer: %w", err)
		}

		taskImporter, err = NewFileTaskImporter(imp.fileTaskImporterConfig(), task, state, batchProducer, batchImportPool, progressReporter, nil, false, errorHandler, callhomeMetricsCollector)
		if err != nil {
			return nil, fmt.Errorf("create file task importer: %w", err)
		}
	}
	return taskImporter, nil
}

func (imp *Importer) startMonitoringTargetYBHealth() error {
	if !slices.Contains([]string{constants.TARGET_DB_IMPORTER_ROLE, constants.IMPORT_FILE_ROLE}, imp.cfg.ImporterRole) {
		return nil
	}
	// Target health monitoring (node / disk-usage / replication metrics) is a
	// YugabyteDB-cluster concept. Non-YB targets (yb-amp's stateless PG17 compute,
	// etc.) expose no such API, so it does not apply.
	if imp.cfg.Tconf.TargetDBType != constants.YUGABYTEDB {
		return nil
	}
	if imp.cfg.SkipNodeHealthChecks && imp.cfg.SkipDiskUsageHealthChecks && imp.cfg.SkipReplicationChecks {
		return nil
	}
	yb, ok := imp.cfg.Tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		return goerrors.Errorf("monitoring health is only supported if target DB is YugabyteDB")
	}

	go func() {
		//for now not sending any other parameters as not required for monitor usage
		ybClient := dbzm.NewYugabyteDBCDCClient(imp.cfg.ExportDir, "", imp.cfg.Tconf.SSLRootCert, imp.cfg.Tconf.DBName, "", nil)
		err := ybClient.Init()
		if err != nil {
			log.Errorf("error intialising the yb client : %v", err)
		}
		monitorTDBHealth := monitor.NewMonitorTargetYBHealth(yb, bool(imp.cfg.SkipDiskUsageHealthChecks), bool(imp.cfg.SkipReplicationChecks), bool(imp.cfg.SkipNodeHealthChecks), ybClient, func(info string) {
			imp.displayMonitoringInformationOnTheConsole(info)
		})

		err = monitorTDBHealth.StartMonitoring()
		//lint:ignore SA4023 StartMonitoring currently only returns on error; keep the conventional check
		if err != nil {
			log.Errorf("error monitoring the target health: %v", err)
		}
	}()
	return nil
}

func (imp *Importer) displayMonitoringInformationOnTheConsole(info string) {
	if info == "" {
		return
	}
	if imp.cfg.DisablePb {
		utils.PrintAndLog(info)
	} else {
		log.Warnf("monitoring: %v", info)
		if imp.importPhase == dbzm.MODE_SNAPSHOT || imp.cfg.ImporterRole == constants.IMPORT_FILE_ROLE {
			imp.progressReporter.DisplayInformation(info)
		} else {
			imp.statsReporter.DisplayInformation(info)

		}
	}
}

func (imp *Importer) startAdaptiveParallelism(mode types.AdaptiveParallelismMode, callhomeMetricsCollector *callhome.ImportDataMetricsCollector) (bool, error) {
	if !mode.IsEnabled() {
		return false, nil
	}
	yb, ok := imp.cfg.Tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		return false, goerrors.Errorf("adaptive parallelism is only supported if target DB is YugabyteDB")
	}

	if !yb.IsAdaptiveParallelismSupported() {
		utils.PrintAndLog(color.YellowString("Note: Continuing without adaptive parallelism as it is not supported in this version of YugabyteDB."))
		return false, nil
	}

	go func() {
		err := adaptiveparallelism.AdaptParallelism(yb, mode, callhomeMetricsCollector)
		if err != nil {
			log.Errorf("adaptive parallelism error: %v", err)
		}
	}()
	return true, nil
}

func (imp *Importer) waitForDebeziumStartIfRequired() error {
	msr, err := imp.cfg.MetaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("failed to get migration status record: %w", err)
	}
	if msr.SnapshotMechanism == "debezium" {
		// we already wait for snapshot to have completed by debezium
		// so no need to wait again here.
		return nil
	}

	// in case pg_dump was used to export snapshot data,
	// we need to wait for export-data to have started debezium in cdc phase
	// in order to avoid any race conditions.
	fmt.Println("Initializing streaming phase...")
	log.Infof("waiting for export-data to have started debezium in cdc phase")
	for {
		msr, err = imp.cfg.MetaDB.GetMigrationStatusRecord()
		if err != nil {
			return fmt.Errorf("failed to get migration status record: %w", err)
		}
		if lo.Contains([]string{constants.TARGET_DB_IMPORTER_ROLE, constants.SOURCE_REPLICA_DB_IMPORTER_ROLE}, imp.cfg.ImporterRole) &&
			msr.ExportDataSourceDebeziumStarted {
			break
		}
		if imp.cfg.ImporterRole == constants.SOURCE_DB_IMPORTER_ROLE && msr.ExportDataTargetDebeziumStarted {
			break
		}
		time.Sleep(2 * time.Second)
	}

	return nil
}

// Handling identity columns for resumable import data
// Background: A previous incomplete import run may have disabled "GENERATED ALWAYS AS IDENTITY" columns.
// Two scenarios:
//  1. Columns are disabled: Restore TableToIdentityColumnNames map from metaDB
//  2. Columns are enabled: Fetch identity columns from database and persist to metaDB for import data resumability
func (imp *Importer) fetchAndStoreGeneratedAlwaysIdentityColumnsInMetadb(tables []sqlname.NameTuple) error {
	tableKeyToIdentityColumnNames := make(map[string][]string)

	// Fetch the table to identity columns information from metadb if present
	found, err := imp.cfg.MetaDB.GetJsonObject(nil, imp.cfg.IdentityColumnsMetaDBKey, &tableKeyToIdentityColumnNames)
	if err != nil {
		return goerrors.Errorf("failed to get identity columns from meta db: %w", err)
	}
	if found {
		// Using retrieved identity columns from metaDB to populate TableToIdentityColumns
		imp.tableToIdentityColumnNames = utils.NewStructMap[sqlname.NameTuple, []string]()
		for key, columns := range tableKeyToIdentityColumnNames {
			nameTuple, err := namereg.NameReg.LookupTableName(key)
			if err != nil {
				return goerrors.Errorf("lookup for table name in name reg: %v with: %w", key, err)
			}
			imp.tableToIdentityColumnNames.Put(nameTuple, columns)
		}
		return nil
	}

	// If not found, fetch it from target db and populate TableToIdentityColumnNames and persist it to metaDB
	imp.tableToIdentityColumnNames, err = imp.cfg.Tdb.GetIdentityColumnNamesForTables(tables, constants.IDENTITY_GENERATION_ALWAYS)
	if err != nil {
		return fmt.Errorf("failed to get identity(%s) columns for tables: %w", constants.IDENTITY_GENERATION_ALWAYS, err)
	}

	err = imp.tableToIdentityColumnNames.IterKV(func(key sqlname.NameTuple, value []string) (bool, error) {
		tableKeyToIdentityColumnNames[key.ForKey()] = value
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to iterate identity column names: %w", err)
	}
	err = imp.cfg.MetaDB.InsertJsonObject(nil, imp.cfg.IdentityColumnsMetaDBKey, tableKeyToIdentityColumnNames)
	if err != nil {
		return goerrors.Errorf("failed to insert into the key '%s': %w", imp.cfg.IdentityColumnsMetaDBKey, err)
	}
	return nil
}
func (imp *Importer) disableGeneratedAlwaysAsIdentityColumns() error {
	err := imp.cfg.Tdb.DisableGeneratedAlwaysAsIdentityColumns(imp.tableToIdentityColumnNames)
	if err != nil {
		return goerrors.Errorf("failed to disable generated always as identity columns: %w", err)
	}
	return nil
}

// identityColumnsNeedHandlingForSnapshotImport checks if identity columns need to be handled
// (disabled before snapshot import) for Oracle targets in live migration.
// This is required because Oracle's SQL*Loader/COPY fails for GENERATED ALWAYS columns,
// so we need to temporarily convert them to GENERATED BY DEFAULT for snapshot import.
// The restoration is handled in postCutoverProcessing() for live migrations.
// For PostgreSQL/YugabyteDB targets, COPY works with GENERATED ALWAYS columns, so no handling is needed.
// Note: SOURCE_DB_IMPORTER_ROLE (fallback) doesn't import snapshots, so not included here.
func (imp *Importer) identityColumnsNeedHandlingForSnapshotImport() bool {
	// Only source-replica import has Oracle as target and requires snapshot import
	return imp.cfg.Tconf.TargetDBType == constants.ORACLE && imp.cfg.ImporterRole == constants.SOURCE_REPLICA_DB_IMPORTER_ROLE
}

func (imp *Importer) enableGeneratedAlwaysAsIdentityColumns() error {
	err := imp.cfg.Tdb.EnableGeneratedAlwaysAsIdentityColumns(imp.tableToIdentityColumnNames)
	if err != nil {
		return goerrors.Errorf("failed to enable generated always as identity columns: %w", err)
	}
	return nil
}

func (imp *Importer) restoreGeneratedByDefaultAsIdentityColumns(tables []sqlname.NameTuple) error {
	log.Infof("restoring generated by default as identity columns for tables: %v", tables)
	tablesToIdentityColumnNames, err := imp.cfg.Tdb.GetIdentityColumnNamesForTables(tables, constants.IDENTITY_GENERATION_BY_DEFAULT)
	if err != nil {
		return fmt.Errorf("failed to get identity(%s) columns for tables: %w", constants.IDENTITY_GENERATION_BY_DEFAULT, err)
	}
	err = imp.cfg.Tdb.EnableGeneratedByDefaultAsIdentityColumns(tablesToIdentityColumnNames)
	if err != nil {
		return fmt.Errorf("failed to enable generated by default as identity columns: %w", err)
	}
	return nil
}

func ImportFileTasksToTableNames(tasks []*ImportFileTask) []string {
	tableNames := []string{}
	for _, t := range tasks {
		tableNames = append(tableNames, t.TableNameTup.ForKey())
	}
	return lo.Uniq(tableNames)
}

func ImportFileTasksToTableNameTuples(tasks []*ImportFileTask) []sqlname.NameTuple {
	tableNames := []sqlname.NameTuple{}
	for _, t := range tasks {
		tableNames = append(tableNames, t.TableNameTup)
	}
	return lo.UniqBy(tableNames, func(t sqlname.NameTuple) string {
		return t.ForKey()
	})
}

func classifyTasksForImport(state *ImportDataState, tasks []*ImportFileTask) (pendingTasks, completedTasks []*ImportFileTask, err error) {
	inProgressTasks := []*ImportFileTask{}
	notStartedTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			return nil, nil, fmt.Errorf("get table import state: %w", err)
		}
		switch fileImportState {
		case FILE_IMPORT_COMPLETED, FILE_IMPORT_COMPLETED_WITH_ERRORS:
			completedTasks = append(completedTasks, task)
		case FILE_IMPORT_IN_PROGRESS:
			inProgressTasks = append(inProgressTasks, task)
		case FILE_IMPORT_NOT_STARTED:
			notStartedTasks = append(notStartedTasks, task)
		default:
			return nil, nil, goerrors.Errorf("invalid table import state: %s", fileImportState)
		}
	}
	// Start with in-progress tasks, followed by not-started tasks.
	return append(inProgressTasks, notStartedTasks...), completedTasks, nil
}

func getNotStartedTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	notStartedTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("get table import state: %s: %w", task.TableNameTup, err)
		}
		if fileImportState == FILE_IMPORT_NOT_STARTED {
			notStartedTasks = append(notStartedTasks, task)
		}
	}
	return notStartedTasks
}

func getInProgressTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	inProgressTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("get table import state: %s: %w", task.TableNameTup, err)
		}
		if fileImportState == FILE_IMPORT_IN_PROGRESS {
			inProgressTasks = append(inProgressTasks, task)
		}
	}
	return inProgressTasks
}

func getPendingTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	return append(getInProgressTasks(state, tasks), getNotStartedTasks(state, tasks)...)
}

func getCompletedTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	completedTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("get table import state: %s: %w", task.TableNameTup, err)
		}
		if fileImportState == FILE_IMPORT_COMPLETED || fileImportState == FILE_IMPORT_COMPLETED_WITH_ERRORS {
			completedTasks = append(completedTasks, task)
		}
	}
	return completedTasks
}

func (imp *Importer) cleanImportState(state *ImportDataState, tasks []*ImportFileTask) {
	tableNames := ImportFileTasksToTableNameTuples(tasks)
	nonEmptyNts := imp.cfg.Tdb.GetNonEmptyTables(tableNames)
	if len(nonEmptyNts) > 0 {
		nonEmptyTableNames := lo.Map(nonEmptyNts, func(nt sqlname.NameTuple, _ int) string {
			return nt.ForOutput()
		})
		if imp.cfg.TruncateTables {
			// truncate tables only supported for import-data-to-target.
			utils.PrintAndLogf("Non-empty tables on DB: %v", nonEmptyTableNames)
			utils.PrintAndLogf("Truncating all tables in import scope on DB to keep FK-dependents consistent")
			err := imp.cfg.Tdb.TruncateTables(tableNames)
			if err != nil {
				utils.ErrExit("failed to truncate tables: %w", err)
			}
		} else {
			utils.PrintAndLogf("Non-Empty tables: [%s]", strings.Join(nonEmptyTableNames, ", "))
			utils.PrintAndLogf("The above list of tables on DB are not empty.")
			utils.PrintAndLogf("If you wish to truncate them, re-run the import command with --truncate-tables true")
			yes := utils.AskPrompt("Do you want to start afresh without truncating tables")
			if !yes {
				utils.ErrExit("Aborting import.")
			}
		}

	}

	for _, task := range tasks {
		err := state.Clean(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("failed to clean import data state for table: %q: %w", task.TableNameTup, err)
		}
	}

	sqlldrDir := filepath.Join(imp.cfg.ExportDir, "sqlldr")
	if utils.FileOrFolderExists(sqlldrDir) {
		err := os.RemoveAll(sqlldrDir)
		if err != nil {
			utils.ErrExit("failed to remove sqlldr directory: %q: %w", sqlldrDir, err)
		}
	}
}

func (imp *Importer) prepareTableToColumns(tasks []*ImportFileTask) error {
	for _, task := range tasks {
		var columns []string
		dfdTableToExportedColumns, err := getDfdTableNameToExportedColumns(tasks, imp.cfg.DataFileDescriptor)
		if err != nil {
			return goerrors.Errorf("failed to get dfd table to exported columns: %w", err)
		}
		if dfdTableToExportedColumns != nil {
			columns, _ = dfdTableToExportedColumns.Get(task.TableNameTup)
		} else if imp.cfg.DataFileDescriptor.HasHeader {
			// File is either exported from debezium OR this is `import data file` case.
			reader, err := imp.cfg.DataStore.Open(task.FilePath)
			if err != nil {
				return goerrors.Errorf("datastore.Open: %q: %w", task.FilePath, err)
			}
			df, err := datafile.NewDataFile(task.FilePath, reader, imp.cfg.DataFileDescriptor, 0)
			if err != nil {
				return goerrors.Errorf("opening datafile: %q: %w", task.FilePath, err)
			}
			header := df.GetHeader()
			columns = strings.Split(header, imp.cfg.DataFileDescriptor.Delimiter)
			log.Infof("read header from file %q: %s", task.FilePath, header)
			log.Infof("header row split using delimiter %q: %v\n", imp.cfg.DataFileDescriptor.Delimiter, columns)
			df.Close()
		}
		imp.tableToColumnNames.Put(task.TableNameTup, columns)
	}
	return nil
}

func getDfdTableNameToExportedColumns(tasks []*ImportFileTask, dataFileDescriptor *datafile.Descriptor) (*utils.StructMap[sqlname.NameTuple, []string], error) {
	if dataFileDescriptor.TableNameToExportedColumns == nil {
		return nil, nil
	}
	tableTupleToexportedColumns := utils.NewStructMap[sqlname.NameTuple, []string]()
	for tableName, columnList := range dataFileDescriptor.TableNameToExportedColumns {
		//Using lookup with ignoring if target not found as we are creating tuple for tables in datafile descriptor which are tables exported
		tuple, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(tableName)
		if err != nil {
			return nil, goerrors.Errorf("failed to lookup table name: %w", err)
		}
		tableTupleToexportedColumns.Put(tuple, columnList)
	}

	result := utils.NewStructMap[sqlname.NameTuple, []string]()
	// checking columns for all tables in the datafile descriptor by using the file tasks
	//as this is used only for import batch which is snapshot
	for _, task := range tasks {
		columnList, ok := tableTupleToexportedColumns.Get(task.TableNameTup)
		if ok {
			result.Put(task.TableNameTup, columnList)
		} else {
			return nil, goerrors.Errorf("table %q not found in data file descriptor", task.TableNameTup.ForKey())
		}
	}
	return result, nil
}

// createInitialImportDataTableMetrics sets table-scope metrics (totals, expected
// rows, pre-registered per-table series) from allTasks so they stay accurate on
// resume, when pendingTasks alone would under-report tables already completed in
// a prior run. The control-plane event list is still built from pendingTasks only
// (unchanged behaviour: the control plane only expects updates for tables being
// worked on in this run).
func (imp *Importer) createInitialImportDataTableMetrics(allTasks, pendingTasks []*ImportFileTask) []*cp.UpdateImportedRowCountEvent {
	metrics.Get().SetImportSnapshotTablesTotal(imp.cfg.ImporterRole, len(allTasks))
	for _, task := range allTasks {
		metrics.Get().SetImportSnapshotTableExpectedRows(imp.cfg.ImporterRole, task.TableNameTup, task.RowCount)
		metrics.Get().InitImportSnapshotTable(imp.cfg.ImporterRole, task.TableNameTup)
	}

	result := []*cp.UpdateImportedRowCountEvent{}
	for _, task := range pendingTasks {
		schemaName, tableName := task.TableNameTup.ForKeyTableSchema()
		tableMetrics := cp.UpdateImportedRowCountEvent{
			BaseUpdateRowCountEvent: cp.BaseUpdateRowCountEvent{
				BaseEvent: cp.BaseEvent{
					EventType:     "IMPORT DATA",
					MigrationUUID: imp.cfg.MigrationUUID,
					SchemaNames:   []string{schemaName},
				},
				TableName:         tableName,
				Status:            cp.EXPORT_OR_IMPORT_DATA_STATUS_INT_TO_STR[constants.ROW_UPDATE_STATUS_NOT_STARTED],
				TotalRowCount:     GetTotalProgressAmount(task, imp.cfg.ReportProgressInBytes),
				CompletedRowCount: 0,
			},
		}
		result = append(result, &tableMetrics)
	}

	return result
}

func (imp *Importer) clearMigrationStateForImportDataStartClean(state *ImportDataState, importFileTasks []*ImportFileTask, errorHandler ImportDataErrorHandler) error {
	if !imp.cfg.StartClean {
		log.Infof("skipping cleaning migration status record for import data command start clean")
		return nil
	}

	if imp.cfg.ImportType == utils.CHANGES_ONLY {
		utils.ErrExit("start-clean flag is not supported for changes-only import type")
	}

	msr, err := imp.cfg.MetaDB.GetMigrationStatusRecord()
	if err != nil {
		return goerrors.Errorf("failed to get migration status record: %w", err)
	}

	if msr == nil {
		return goerrors.Errorf("migration status record not found.")
	}

	// Guardrail: for change-streaming imports, start-clean resets the per-importer
	// imported_by flags and re-streams the queue from the earliest segment. If that
	// segment has already been archived/deleted by `archive changes`, the queue can
	// no longer be re-streamed from the beginning. Detect this and fail before
	// mutating any metaDB state.
	if changeStreamingIsEnabled(imp.cfg.ImportType) {
		resumeSegmentDeleted, err := imp.cfg.MetaDB.AnySegmentsDeletedOrArchived()
		if err != nil {
			return goerrors.Errorf("failed to check for archived/deleted queue segments: %w", err)
		}
		if resumeSegmentDeleted {
			return goerrors.Errorf("cannot perform import data with --start-clean: some queue segments have already been archived/deleted by 'archive changes'. The change-event queue can no longer be re-streamed from the beginning, so a clean restart is not possible.")
		}
	}

	err = imp.cfg.MetaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.OnPrimaryKeyConflictAction = ""
	})
	if err != nil {
		return goerrors.Errorf("failed to update migration status record: %w", err)
	}
	err = imp.cfg.MetaDB.UpdateImportDataStatusRecord(func(record *metadb.ImportDataStatusRecord) {
		record.TableToCDCPartitionKey = nil
	})
	if err != nil {
		return goerrors.Errorf("failed to update import data status record: %w", err)
	}

	err = imp.handleStartCleanForSnapshot(state, importFileTasks, errorHandler)
	if err != nil {
		utils.ErrExit("Failed to handle fresh start: %w", err)
	}

	if changeStreamingIsEnabled(imp.cfg.ImportType) {
		// clearing state from metaDB based on importerRole
		err := imp.cfg.MetaDB.ResetQueueSegmentMeta(imp.cfg.ImporterRole)
		if err != nil {
			utils.ErrExit("failed to reset queue segment meta: %w", err)
		}
		err = imp.cfg.MetaDB.DeleteJsonObject(imp.cfg.IdentityColumnsMetaDBKey)
		if err != nil {
			utils.ErrExit("failed to reset identity columns meta: %w", err)
		}
	}
	return nil
}

func cleanStoredErrors(errorHandler ImportDataErrorHandler, tasks []*ImportFileTask) error {
	// clean stored errors for all tasks
	for _, task := range tasks {
		err := errorHandler.CleanUpStoredErrors(task.TableNameTup, task.FilePath)
		if err != nil {
			return fmt.Errorf("failed to clean up stored errors for task %s: %w", task.TableNameTup.ForOutput(), err)
		}
	}
	return nil
}

func isTargetDBImporter(importerRole string) bool {
	return importerRole == constants.TARGET_DB_IMPORTER_ROLE || importerRole == constants.IMPORT_FILE_ROLE
}

func (imp *Importer) updateErrorPolicyInMetaDB(errorPolicy ErrorPolicy) error {

	switch imp.cfg.ImporterRole {
	case constants.IMPORT_FILE_ROLE:
		err := imp.cfg.MetaDB.UpdateImportDataFileStatusRecord(func(record *metadb.ImportDataFileStatusRecord) {
			record.ErrorPolicy = errorPolicy.String()
		})
		if err != nil {
			return fmt.Errorf("failed to update error policy in import data file status record: %w", err)
		}
	case constants.TARGET_DB_IMPORTER_ROLE:
		err := imp.cfg.MetaDB.UpdateImportDataStatusRecord(func(record *metadb.ImportDataStatusRecord) {
			record.ErrorPolicySnapshot = errorPolicy.String()
		})
		if err != nil {
			return fmt.Errorf("failed to update error policy in import data status record: %w", err)
		}
		// Not applicable for other roles
	}
	return nil
}

func (imp *Importer) updateImportDataStartedAndSomeConfigsInMetaDB() error {
	switch imp.cfg.ImporterRole {
	case constants.TARGET_DB_IMPORTER_ROLE:
		log.Infof("updating import data started in meta db with cdc-partition-key: %s, overrides: %q", imp.cfg.CdcPartitionKey, imp.cfg.CdcPartitionKeyOverrides)
		err := imp.cfg.MetaDB.UpdateImportDataStatusRecord(func(record *metadb.ImportDataStatusRecord) {
			record.ImportDataStarted = true
			record.CdcPartitioningStrategyConfig = imp.cfg.CdcPartitionKey
			record.CdcPartitionKeyOverridesConfig = imp.cfg.CdcPartitionKeyOverrides
		})
		if err != nil {
			return goerrors.Errorf("Failed to update import data status record: %w", err)
		}
		err = imp.cfg.MetaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			record.ImportDataToTargetStarted = true
		})
		if err != nil {
			return goerrors.Errorf("failed to update migration status record: %w", err)
		}

	case constants.IMPORT_FILE_ROLE:
		err := imp.cfg.MetaDB.UpdateImportDataFileStatusRecord(func(record *metadb.ImportDataFileStatusRecord) {
			record.ImportDataStarted = true
		})
		if err != nil {
			return goerrors.Errorf("Failed to update import data file status record: %w", err)
		}
	case constants.SOURCE_DB_IMPORTER_ROLE:
		err := imp.cfg.MetaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			record.ImportDataToSourceStarted = true
		})
		if err != nil {
			return goerrors.Errorf("failed to update migration status record: %w", err)
		}
	}
	return nil
}

func ImportSnapshotRequired(importerRole string, importType string) bool {
	switch importerRole {
	case constants.SOURCE_REPLICA_DB_IMPORTER_ROLE:
		return true
	case constants.IMPORT_FILE_ROLE:
		return true
	case constants.SOURCE_DB_IMPORTER_ROLE:
		return false
	case constants.TARGET_DB_IMPORTER_ROLE:
		return importType == utils.SNAPSHOT_ONLY || importType == utils.SNAPSHOT_AND_CHANGES
	default:
		panic(fmt.Sprintf("invalid importer role: %s", importerRole))
	}
}

// RowCountPair holds imported and errored row counts for a table.
type RowCountPair struct {
	Imported int64
	Errored  int64
}

// DisplayImportedRowCountSnapshot prints the snapshot import report. stateCfg carries the
// role, export dir, target conf and data-file descriptor it used to read from cmd globals.
func DisplayImportedRowCountSnapshot(stateCfg ImportDataStateConfig, state *ImportDataState, tasks []*ImportFileTask, errorHandler ImportDataErrorHandler) {
	importerRole := stateCfg.ImporterRole
	if importerRole == constants.IMPORT_FILE_ROLE {
		fmt.Printf("import report\n")
	} else {
		fmt.Printf("snapshot import report\n")
	}
	tableList := ImportFileTasksToTableNameTuples(tasks)
	uitable := uitable.New()

	// TODO: refactor this; we don't need to pass the dbType as a parameter,
	// we can just pass the importerRole directly.
	var dbType string
	switch importerRole {
	case constants.IMPORT_FILE_ROLE:
		dbType = "target-file"
	case constants.SOURCE_REPLICA_DB_IMPORTER_ROLE:
		dbType = "source-replica"
	case constants.TARGET_DB_IMPORTER_ROLE:
		dbType = "target"
	}

	snapshotRowCount, err := GetImportedSnapshotRowsMap(dbType, tableList, errorHandler, stateCfg)
	if err != nil {
		utils.ErrExit("failed to get imported snapshot rows map: %w", err)
	}

	keys := make([]sqlname.NameTuple, 0, len(snapshotRowCount.Keys()))
	// the callback never returns an error
	_ = snapshotRowCount.IterKV(func(k sqlname.NameTuple, v RowCountPair) (bool, error) {
		keys = append(keys, k)
		return true, nil
	})

	sort.Slice(keys, func(i, j int) bool {
		val1, _ := snapshotRowCount.Get(keys[i])
		val2, _ := snapshotRowCount.Get(keys[j])
		return val1.Imported > val2.Imported
	})

	hasErrors := false
	for _, tableName := range keys {
		rowCountPair, _ := snapshotRowCount.Get(tableName)
		if rowCountPair.Errored > 0 {
			hasErrors = true
			break
		}
	}

	for i, tableName := range keys {
		if i == 0 {
			if hasErrors {
				addHeader(uitable, "SCHEMA", "TABLE", "IMPORTED ROW COUNT", "ERRORED ROW COUNT")
			} else {
				addHeader(uitable, "SCHEMA", "TABLE", "IMPORTED ROW COUNT")
			}
		}
		s, t := tableName.ForCatalogQuery()
		rowCountPair, _ := snapshotRowCount.Get(tableName)
		if hasErrors {
			uitable.AddRow(s, t, rowCountPair.Imported, rowCountPair.Errored)
		} else {
			uitable.AddRow(s, t, rowCountPair.Imported)
		}
	}
	if len(tableList) > 0 {
		fmt.Printf("\n")
		fmt.Println(uitable)
		fmt.Printf("\n")
	}
	if hasErrors {
		// in case there are errored rows, and we are in on-pk-conflict ignore mode,
		// it is possible that the batches which errored out were partially ingested.
		if stateCfg.Tconf.OnPrimaryKeyConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE {
			utils.PrintAndLog(color.YellowString("Note: It is possible that the table row count on the target DB may not match the IMPORTED ROW COUNT as some batches may have been partially ingested."))
		}
		utils.PrintAndLog(color.RedString("Errored snapshot rows are stashed in %q", errorHandler.GetErrorsLocation()))
	}
}

// GetImportedSnapshotRowsMap returns imported/errored snapshot row counts per table for the
// importer identified by dbType ("target", "target-file", "source-replica"). stateCfg supplies
// the export dir, target conf and data-file descriptor; its ImporterRole is overridden by
// dbType exactly as the cmd package-level importerRole used to be.
func GetImportedSnapshotRowsMap(dbType string, tableList []sqlname.NameTuple, errorHandler ImportDataErrorHandler, stateCfg ImportDataStateConfig) (*utils.StructMap[sqlname.NameTuple, RowCountPair], error) {

	var err error
	cfg := stateCfg
	switch dbType {
	case "target":
		cfg.ImporterRole = constants.TARGET_DB_IMPORTER_ROLE
	case "target-file":
		cfg.ImporterRole = constants.IMPORT_FILE_ROLE
	case "source-replica":
		cfg.ImporterRole = constants.SOURCE_REPLICA_DB_IMPORTER_ROLE
	}
	importerRole := cfg.ImporterRole
	exportDir := cfg.ExportDir
	dataFileDescriptor := cfg.DataFileDescriptor
	state := NewImportDataState(cfg)
	var snapshotDataFileDescriptor *datafile.Descriptor

	if dataFileDescriptor != nil {
		// in case of import-data and import-data-file, import-data-to-source-replica,
		// the data file descriptor is already loaded in memory
		snapshotDataFileDescriptor = dataFileDescriptor
	} else {
		// get data-migration-report use-case where
		// we need to read the data file descriptor from export-dir
		dataFileDescriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
		if utils.FileOrFolderExists(dataFileDescriptorPath) {
			snapshotDataFileDescriptor = datafile.OpenDescriptor(exportDir)
		}
	}
	snapshotRowsMap := utils.NewStructMap[sqlname.NameTuple, RowCountPair]()
	nameTupleTodataFileEntry := utils.NewStructMap[sqlname.NameTuple, []*datafile.FileEntry]()
	nameTupleTodataFilesMap := utils.NewStructMap[sqlname.NameTuple, []string]()
	if snapshotDataFileDescriptor != nil {
		for _, fileEntry := range snapshotDataFileDescriptor.DataFileList {
			//ignoring target as the dataFileDescriptor can contain tables that exported but not present in target
			nt, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(fileEntry.TableName)
			if err != nil {
				return nil, goerrors.Errorf("lookup table name from data file descriptor %s : %w", fileEntry.TableName, err)
			}
			fileEntries, ok := nameTupleTodataFileEntry.Get(nt)
			if !ok {
				fileEntries = []*datafile.FileEntry{}
			}
			fileEntries = append(fileEntries, fileEntry)
			nameTupleTodataFileEntry.Put(nt, fileEntries)
		}
		for _, table := range tableList {
			fileEntries, ok := nameTupleTodataFileEntry.Get(table)
			if !ok {
				//We can't error out here as this is possible in case there are empty tables in live migraton scenario and
				//In get data-migration-report, table list can consist that table name but it won't be present in dataFileDescriptor
				log.Warnf("table %s not found in data file descriptor", table.ForKey())
				continue
			}
			list := lo.Map(fileEntries, func(fileEntry *datafile.FileEntry, _ int) string {
				return fileEntry.FilePath
			})
			nameTupleTodataFilesMap.Put(table, list)
		}
	}

	err = nameTupleTodataFilesMap.IterKV(func(nt sqlname.NameTuple, dataFilePaths []string) (bool, error) {
		for _, dataFilePath := range dataFilePaths {
			importedRowCount, err := state.GetImportedRowCount(dataFilePath, nt)
			if err != nil {
				return false, fmt.Errorf("could not fetch imported row count for table %q: %w", nt, err)
			}
			erroredRowCount, err := state.GetErroredRowCount(dataFilePath, nt)
			if err != nil {
				return false, fmt.Errorf("could not fetch errored row count for table %q: %w", nt, err)
			}
			existingRowCountPair, _ := snapshotRowsMap.Get(nt)
			existingRowCountPair.Imported += importedRowCount
			existingRowCountPair.Errored += erroredRowCount

			if isTargetDBImporter(importerRole) && errorHandler != nil {
				// error handler will be nil if import-data/import-data-file was not run yet
				processingErrorRowCount, _, err := errorHandler.GetProcessingErrorCountSize(nt, dataFilePath)
				if err != nil {
					return false, fmt.Errorf("get processing error count size: %w", err)
				}
				existingRowCountPair.Errored += processingErrorRowCount
			}
			snapshotRowsMap.Put(nt, existingRowCountPair)

		}
		return true, nil
	})
	if err != nil {
		return nil, goerrors.Errorf("error getting row count of tables: %w", err)
	}
	return snapshotRowsMap, nil
}

// addHeader mirrors cmd's uitable header helper of the same name.
func addHeader(table *uitable.Table, cols ...string) {
	headerfmt := color.New(color.FgGreen, color.Underline).SprintFunc()
	columns := lo.Map(cols, func(col string, _ int) interface{} {
		return headerfmt(col)
	})
	table.AddRow(columns...)
}
