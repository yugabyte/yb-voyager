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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/errs"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var (
	COPY_MAX_RETRY_COUNT = 10
	MAX_SLEEP_SECOND     = 60
)

/*
FileTaskImporter is responsible for importing an ImportFileTask.
It uses a FileBatchProducer to produce batches. It submits each batch to a provided
worker pool for processing. It also maintains and updates the progress of the task.
*/
type FileTaskImporter struct {
	state *ImportDataState

	task                 *ImportFileTask
	batchProducer        *FileBatchProducer
	importBatchArgsProto *tgtdb.ImportBatchArgs
	workerPool           *pool.Pool // worker pool to submit batches for import. Shared across all tasks.

	isTableColocated          bool
	colocatedImportBatchQueue chan func() // Queue for colocated import batches. shared across all tasks.

	totalProgressAmount   int64
	currentProgressAmount int64
	progressReporter      *ImportDataProgressReporter

	errorHandler             importdata.ImportDataErrorHandler
	callhomeMetricsCollector *callhome.ImportDataMetricsCollector
}

func NewFileTaskImporter(task *ImportFileTask, state *ImportDataState, workerPool *pool.Pool,
	progressReporter *ImportDataProgressReporter, colocatedImportBatchQueue chan func(), isTableColocated bool,
	errorHandler importdata.ImportDataErrorHandler, callhomeMetricsCollector *callhome.ImportDataMetricsCollector) (*FileTaskImporter, error) {
	batchProducer, err := NewFileBatchProducer(task, state, errorHandler)
	if err != nil {
		return nil, fmt.Errorf("creating file batch producer: %s", err)
	}
	totalProgressAmount := getTotalProgressAmount(task)
	progressReporter.ImportFileStarted(task, totalProgressAmount)
	currentProgressAmount := getImportedProgressAmount(task, state)
	progressReporter.AddProgressAmount(task, currentProgressAmount)

	fti := &FileTaskImporter{
		state:                     state,
		task:                      task,
		batchProducer:             batchProducer,
		workerPool:                workerPool,
		colocatedImportBatchQueue: colocatedImportBatchQueue,
		isTableColocated:          isTableColocated,
		importBatchArgsProto:      getImportBatchArgsProto(task.TableNameTup, task.FilePath),
		progressReporter:          progressReporter,
		totalProgressAmount:       totalProgressAmount,
		currentProgressAmount:     currentProgressAmount,
		errorHandler:              errorHandler,
		callhomeMetricsCollector:  callhomeMetricsCollector,
	}
	state.RegisterFileTaskImporter(fti)
	return fti, nil
}

func (fti *FileTaskImporter) GetTaskID() int {
	return fti.task.ID
}

// as of now, batch production and batch submission
// is done together in `SubmitNextBatch` method.
// In other words, as soon as a batch is produced, it is submitted.
// Therefore, to check whether all batches are submitted, we can check
// if the batch producer is done.
func (fti *FileTaskImporter) AllBatchesSubmitted() bool {
	return fti.batchProducer.Done()
}

func (fti *FileTaskImporter) TableHasPrimaryKey() bool {
	return len(fti.importBatchArgsProto.PrimaryKeyColumns) > 0
}

func (fti *FileTaskImporter) ProduceAndSubmitNextBatchToWorkerPool() error {
	if fti.AllBatchesSubmitted() {
		return fmt.Errorf("no more batches to submit")
	}
	batch, err := fti.batchProducer.NextBatch()
	if err != nil {
		return fmt.Errorf("getting next batch: %w", err)
	}
	return fti.submitBatch(batch)
}

func (fti *FileTaskImporter) submitBatch(batch *Batch) error {
	importBatchFunc := func() {
		// There are `poolSize` number of competing go-routines trying to invoke COPY.
		// But the `connPool` will allow only `parallelism` number of connections to be
		// used at a time. Thus limiting the number of concurrent COPYs to `parallelism`.
		fti.importBatch(batch)
		fti.updateProgressForCompletedBatch(batch)
	}
	if fti.colocatedImportBatchQueue != nil && fti.isTableColocated {
		fti.colocatedImportBatchQueue <- importBatchFunc
	} else {
		fti.workerPool.Go(importBatchFunc)
	}

	log.Infof("Queued batch: %s", spew.Sdump(batch))
	return nil
}

func (fti *FileTaskImporter) importBatch(batch *Batch) {
	var rowsAffected int64
	var err error // Note: make sure to not define any other err variable in this scope
	var isPartialBatchIngestionPossibleOnError bool

	if batch.RecordCount == 0 {
		// an empty batch is possible in case there are errors while reading and procesing rows in the file
		// and the errors are handled by the error handler.
		log.Infof("Skipping empty batch: %s", spew.Sdump(batch))
		err = batch.MarkDone()
		if err != nil {
			utils.ErrExit("marking empty batch as done: %q: %s", batch.FilePath, err)
		}
		return
	}

	/*
		recoveryBatch means the batch is already in progress state due to partial ingestion or transient errors in last run
		Need this info for decision making in tgtdb.ImportBatch() to whether follow which of three paths
		- Normal mode
		- Fast path mode
		- Fast path recovery mode
	*/
	recoveryBatch := batch.IsInterrupted()
	if !recoveryBatch {
		err = batch.MarkInProgress()
		if err != nil {
			utils.ErrExit("marking batch as pending: %d: %s", batch.Number, err)
		}
	}

	log.Infof("Importing %q", batch.FilePath)

	importBatchArgs := *fti.importBatchArgsProto
	importBatchArgs.FilePath = batch.FilePath
	importBatchArgs.RowsPerTransaction = batch.OffsetEnd - batch.OffsetStart

	sleepIntervalSec := 0
	/*
		Cases:
		1. First time batch import with onPrimaryKeyConflictAction == "ERROR" 		-> Normal Path
		2. First time batch import with onPrimaryKeyConflictAction == "IGNORE" 		-> Fast Path
		3. Batch in *.P state with onPrimaryKeyConflictAction == "IGNORE" 			-> Fast Path Recovery

		If 2 fails due to already existing rows in table, then first ImportBatch() internally switch to Fast Path Recovery
		If that also fails then batch gets executed as fast path recovery due to attempt > 0 from here.
	*/
	for attempt := 0; attempt < COPY_MAX_RETRY_COUNT; attempt++ {
		tableSchema, _ := TableNameToSchema.Get(batch.TableNameTup)
		isRecoveryCandidate := (recoveryBatch || attempt > 0)
		rowsAffected, err, isPartialBatchIngestionPossibleOnError = tdb.ImportBatch(batch, &importBatchArgs, exportDir, tableSchema, isRecoveryCandidate)
		if err == nil || tdb.IsNonRetryableCopyError(err) {
			break
		}
		log.Warnf("COPY FROM file %q: %s", batch.FilePath, err)
		sleepIntervalSec += 10
		if sleepIntervalSec > MAX_SLEEP_SECOND {
			sleepIntervalSec = MAX_SLEEP_SECOND
		}
		log.Infof("sleep for %d seconds before retrying the file %s (attempt %d)",
			sleepIntervalSec, batch.FilePath, attempt)
		time.Sleep(time.Duration(sleepIntervalSec) * time.Second)
	}
	log.Infof("%q => %d rows affected", batch.FilePath, rowsAffected)
	if err != nil {
		if fti.errorHandler.ShouldAbort() {
			var ibe errs.ImportBatchError
			if errors.As(err, &ibe) {
				// If the error is an ImportBatchError, we abort directly because the string
				// representation of the error is already formatted with all the details.
				utils.ErrExit("%w", err)
			}
			utils.ErrExit("import batch: %q into %s: %w", batch.FilePath, batch.TableNameTup.ForOutput(), err)
		}

		// Handle the error
		log.Errorf("Handling error for batch: %q into %s: %s", batch.FilePath, batch.TableNameTup, err)
		var err2 error
		err2 = fti.errorHandler.HandleBatchIngestionError(batch, fti.task.FilePath, err, isPartialBatchIngestionPossibleOnError)
		if err2 != nil {
			utils.ErrExit("handling error for batch: %q into %s: %s", batch.FilePath, batch.TableNameTup, err2)
		}
	} else {
		err = batch.MarkDone()
		if err != nil {
			utils.ErrExit("marking batch as done: %q: %s", batch.FilePath, err)
		}
	}
}

func (fti *FileTaskImporter) updateProgressForCompletedBatch(batch *Batch) {
	// Update basic progress update for progress bar and control plane.
	var progressAmount int64
	if reportProgressInBytes {
		progressAmount = batch.ByteCount
	} else {
		progressAmount = batch.RecordCount
	}

	fti.currentProgressAmount += progressAmount
	fti.progressReporter.AddProgressAmount(fti.task, progressAmount) // TODO: remove this

	// The metrics are sent after evry 5 secs in implementation of UpdateImportedRowCount
	if fti.totalProgressAmount > fti.currentProgressAmount {
		fti.updateProgressInControlPlane(ROW_UPDATE_STATUS_IN_PROGRESS)
	}

	// update callhome metrics collector
	fti.callhomeMetricsCollector.IncrementSnapshotProgress(batch.RecordCount, batch.ByteCount)
}

func (fti *FileTaskImporter) PostProcess() {
	if fti.batchProducer != nil {
		fti.batchProducer.Close()
	}

	fti.updateProgressInControlPlane(ROW_UPDATE_STATUS_COMPLETED)

	fti.progressReporter.FileImportDone(fti.task) // Remove the progress-bar for the file.\
}

func (fti *FileTaskImporter) updateProgressInControlPlane(status int) {
	if importerRole == TARGET_DB_IMPORTER_ROLE {
		importDataTableMetrics := createImportDataTableMetrics(fti.task.TableNameTup.ForKey(),
			fti.currentProgressAmount, fti.totalProgressAmount, status)
		controlPlane.UpdateImportedRowCount(
			[]*cp.UpdateImportedRowCountEvent{&importDataTableMetrics})
	}
}

// ============================================================================= //

func getTotalProgressAmount(task *ImportFileTask) int64 {
	if reportProgressInBytes {
		return task.FileSize
	} else {
		return task.RowCount
	}
}

func getImportedProgressAmount(task *ImportFileTask, state *ImportDataState) int64 {
	if reportProgressInBytes {
		byteCount, err := state.GetImportedByteCount(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("Failed to get imported byte count for table: %s: %s", task.TableNameTup, err)
		}
		return byteCount
	} else {
		rowCount, err := state.GetImportedRowCount(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("Failed to get imported row count for table: %s: %s", task.TableNameTup, err)
		}
		return rowCount
	}
}

func createImportDataTableMetrics(tableName string, countLiveRows int64, countTotalRows int64,
	status int) cp.UpdateImportedRowCountEvent {

	var schemaName, tableName2 string
	if strings.Count(tableName, ".") == 1 {
		schemaName, tableName2 = cp.SplitTableNameForPG(tableName)
	} else {
		schemaName, tableName2 = tconf.Schema, tableName
	}
	result := cp.UpdateImportedRowCountEvent{
		BaseUpdateRowCountEvent: cp.BaseUpdateRowCountEvent{
			BaseEvent: cp.BaseEvent{
				EventType:     "IMPORT DATA",
				MigrationUUID: migrationUUID,
				SchemaNames:   []string{schemaName},
			},
			TableName:         tableName2,
			Status:            cp.EXPORT_OR_IMPORT_DATA_STATUS_INT_TO_STR[status],
			TotalRowCount:     countTotalRows,
			CompletedRowCount: countLiveRows,
		},
	}

	return result
}

func getImportBatchArgsProto(tableNameTup sqlname.NameTuple, filePath string) *tgtdb.ImportBatchArgs {
	columns, _ := TableToColumnNames.Get(tableNameTup)
	columns, err := tdb.QuoteAttributeNames(tableNameTup, columns)
	if err != nil {
		utils.ErrExit("if required quote column names: %s", err)
	}

	/*
		How is table partitioning handled here?
		- For partitioned tables, we import the datafiles of leafs into root
		  Hence query is made on root tables which will fetch all the constraints names(parent and all children)
	*/
	// TODO: Optimize this by fetching the primary key columns and constraint names in one go for all tables
	pkColumns, err := tdb.GetPrimaryKeyColumns(tableNameTup)
	if err != nil {
		utils.ErrExit("getting primary key columns for table %s: %s", tableNameTup.ForMinOutput(), err)
	}
	pkColumns, err = tdb.QuoteAttributeNames(tableNameTup, pkColumns)
	if err != nil {
		utils.ErrExit("if required quote primary key column names: %s", err)
	}

	pkConstraintNames, err := tdb.GetPrimaryKeyConstraintNames(tableNameTup)
	if err != nil {
		utils.ErrExit("getting primary key constraint name for table %s: %s", tableNameTup.ForMinOutput(), err)
	}

	// If `columns` is unset at this point, no attribute list is passed in the COPY command.
	fileFormat := dataFileDescriptor.FileFormat

	// from export data with ora2pg, it comes as an SQL file, with COPY command having data.
	// Import-data also reads it appropriately with the help of sqlDataFile.
	// But while running COPY for a batch, we need to set the format as TEXT (SQL does not make sense)
	if fileFormat == datafile.SQL {
		fileFormat = datafile.TEXT
	}
	importBatchArgsProto := &tgtdb.ImportBatchArgs{
		TableNameTup:      tableNameTup,
		Columns:           columns,
		PrimaryKeyColumns: pkColumns,
		PKConstraintNames: pkConstraintNames,
		PKConflictAction:  tconf.OnPrimaryKeyConflictAction,
		FileFormat:        fileFormat,
		Delimiter:         dataFileDescriptor.Delimiter,
		HasHeader:         dataFileDescriptor.HasHeader && fileFormat == datafile.CSV,
		QuoteChar:         dataFileDescriptor.QuoteChar,
		EscapeChar:        dataFileDescriptor.EscapeChar,
		NullString:        dataFileDescriptor.NullString,
	}
	log.Infof("ImportBatchArgs: %v", spew.Sdump(importBatchArgsProto))
	return importBatchArgsProto
}
