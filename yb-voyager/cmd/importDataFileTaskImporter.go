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
	"fmt"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

/*
FileTaskImporter is responsible for importing an ImportFileTask.
It uses a FileBatchProducer to produce batches. It submits each batch to a provided
worker pool for processing. It also maintains and updates the progress of the task.
*/
type FileTaskImporter struct {
	task                 *ImportFileTask
	batchProducer        *FileBatchProducer
	importBatchArgsProto *tgtdb.ImportBatchArgs
	workerPool           *pool.Pool // worker pool to submit batches for import. Shared across all tasks.

	isTableColocated          bool
	colocatedImportBatchQueue chan func() // Queue for colocated import batches. shared across all tasks.

	totalProgressAmount   int64
	currentProgressAmount int64
	progressReporter      *ImportDataProgressReporter
}

func NewFileTaskImporter(task *ImportFileTask, state *ImportDataState, workerPool *pool.Pool,
	progressReporter *ImportDataProgressReporter, colocatedImportBatchQueue chan func(), isTableColocated bool) (*FileTaskImporter, error) {
	batchProducer, err := NewFileBatchProducer(task, state)
	if err != nil {
		return nil, fmt.Errorf("creating file batch producer: %s", err)
	}
	totalProgressAmount := getTotalProgressAmount(task)
	progressReporter.ImportFileStarted(task, totalProgressAmount)
	currentProgressAmount := getImportedProgressAmount(task, state)
	progressReporter.AddProgressAmount(task, currentProgressAmount)

	fti := &FileTaskImporter{
		task:                      task,
		batchProducer:             batchProducer,
		workerPool:                workerPool,
		colocatedImportBatchQueue: colocatedImportBatchQueue,
		isTableColocated:          isTableColocated,
		importBatchArgsProto:      getImportBatchArgsProto(task.TableNameTup, task.FilePath),
		progressReporter:          progressReporter,
		totalProgressAmount:       totalProgressAmount,
		currentProgressAmount:     currentProgressAmount,
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

func (fti *FileTaskImporter) shouldUseNonTransactionalPath() bool {
	return bool(enableFastPath) &&
		fti.TableHasPrimaryKey() &&
		(importerRole == TARGET_DB_IMPORTER_ROLE || importerRole == IMPORT_FILE_ROLE)
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
		if reportProgressInBytes {
			fti.updateProgress(batch.ByteCount)
		} else {
			fti.updateProgress(batch.RecordCount)
		}
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
	/*
		Summary:

		Check if the batch is already in progress then we need to recover with failure handling for unique constraint violation
		Now we can have two paths:
		1. If --enable-fast-path is true and allowed for the batch(check PK/non-PK table)
			then use non-txn path for COPY
		2. Else
			- Use normal path with txn for COPY

		Fast Path - Does everything like normal path but without transaction around COPY command
		And in case of recovery; the batch will import via INSERT ON CONFLICT DO NOTHING/UPDATE path

		Case: What if user change the flag/behaviour to use txn/non-txn path at the time of resumption?
		- Complication comes as we will have to recover the files as per the previous fast path approach, rest of the batches(not created or just created) can continue as asked by the user
		- For the first phase: Can we restrict the user to not change the flag/behaviour among fast/normal path.

		Assumption: user won't change the PK once import data is started(for eg: after interruption / before retry)
	*/

	if fti.shouldUseNonTransactionalPath() {
		fti.importBatchViaNonTxnPath(batch)
	} else {
		fti.importBatchViaTxnPath(batch)
	}
}

func (fti *FileTaskImporter) importBatchViaNonTxnPath(batch *Batch) {
	/*
		Fast path do non-txn import of batches
		In case of recovery(last run imported partial batch):
		in progress batch(es) will import via INSERT ON CONFLICT DO NOTHING approach
		to recover with failure handling for unique constraint violation
	*/
	log.Infof("importing batch %q via non-transactional path", batch.FilePath)

	// TODO: Need to implement/enable this Recovery logic not just for in-progress batches on command rerun,
	// but also when we internally retry the batches in case of any database retryable-errors
	recoverBatch := batch.IsInterrupted()
	if recoverBatch {
		// TODO: implement recovery logic
		// TODO2: Ensure the task picker logic to first pick and import the interrupted task(in progress ones)
		fti.importBatchViaRecoverMode(batch, onPrimaryKeyConflictAction)
	} else {
		fti.importBatchCore(batch, true)
	}
}

func (fti *FileTaskImporter) importBatchViaTxnPath(batch *Batch) {
	/*
		Normal path uses txn for importing batches
		In case of recovery(last run imported full batch but not marked as done):
		we check the metadata table on target if batch is already imported in last run
		which avoids unique constraint violation and we can skip the batch
	*/
	log.Infof("importing batch %q via normal transactional path", batch.FilePath)
	fti.importBatchCore(batch, false)
}

// importBatchCore func is used for both txn/non-txn path(depending on the caller)
// It will do the actual COPY command for the batch in the target DB with retries
// called from importBatchViaTxnPath and importBatchViaNonTxnPath(non recovery mode)
func (fti *FileTaskImporter) importBatchCore(batch *Batch, nonTxnPath bool) {
	err := batch.MarkInProgress()
	if err != nil {
		utils.ErrExit("marking batch as pending: %d: %s", batch.Number, err)
	}

	importBatchArgs := *fti.importBatchArgsProto
	importBatchArgs.FilePath = batch.FilePath
	importBatchArgs.RowsPerTransaction = batch.OffsetEnd - batch.OffsetStart

	var rowsAffected int64
	sleepIntervalSec := 0
	log.Infof("start importing batch %q with nonTxnPath=%v", batch.FilePath, nonTxnPath)
	for attempt := 0; attempt < COPY_MAX_RETRY_COUNT; attempt++ {
		tableSchema, _ := TableNameToSchema.Get(batch.TableNameTup)
		if attempt > 0 && nonTxnPath {
			log.Infof("(recovery) retrying batch %q with nonTxnPath=%v", batch.FilePath, nonTxnPath)
			// Caller ensures only valid target DB type(YugabyteDB) reach here
			ybdb, ok := tdb.(*tgtdb.TargetYugabyteDB)
			if !ok {
				utils.ErrExit("(recovery) target db is not of type TargetYugabyteDB to use for importing batch %q", batch.FilePath)
			}
			rowsAffected, err = ybdb.ImportBatchViaRecoveryMode(batch, onPrimaryKeyConflictAction, &importBatchArgs, exportDir, tableSchema)
		} else {
			rowsAffected, err = tdb.ImportBatch(batch, &importBatchArgs, exportDir, tableSchema, nonTxnPath)
		}
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
		utils.ErrExit("import batch: %q into %s: %s", batch.FilePath, batch.TableNameTup, err)
	}
	err = batch.MarkDone()
	if err != nil {
		utils.ErrExit("marking batch as done: %q: %s", batch.FilePath, err)
	}
}

// importBatchViaRecoverMode func is used for recovery in non transactional path
func (fti *FileTaskImporter) importBatchViaRecoverMode(batch *Batch, action string) {
	/*
		Recovery mode for batch import (in-progress batches only)
		In case of recovery(last run imported partial batch):
		in progress batch(es) will import via INSERT ON CONFLICT DO (ACTION) approach
		to recover with failure handling for unique constraint violation, if occurs.

		This logic also need to retried for retryable errors for transient database errors
		If the batch keeps fails after N retries(via recovery mode) then Error out
	*/

	// Caller ensures only valid target DB type(YugabyteDB) reach here
	ybdb, ok := tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		utils.ErrExit("(recovery) target db is not of type TargetYugabyteDB to use for importing batch %q", batch.FilePath)
	}

	var err error
	var rowsAffected int64
	for attempt := 0; attempt < BATCH_RECOVERY_MAX_RETRY_COUNT; attempt++ {
		tableSchema, _ := TableNameToSchema.Get(batch.TableNameTup)
		rowsAffected, err = ybdb.ImportBatchViaRecoveryMode(batch, action, fti.importBatchArgsProto, exportDir, tableSchema)
		if err == nil || ybdb.IsNonRetryableInsertError(err, onPrimaryKeyConflictAction) {
			break
		}

		log.Warnf("(recovery) import batch %q: %s", batch.FilePath, err)
		log.Infof("(recovery) sleep for %d seconds before retrying the file %s (attempt %d)",
			RECOVERY_MODE_SLEEP_SECOND, batch.FilePath, attempt)
		time.Sleep(time.Duration(RECOVERY_MODE_SLEEP_SECOND) * time.Second)
	}

	log.Infof("(recovery) %q => %d rows affected", batch.FilePath, rowsAffected)
	if err != nil {
		utils.ErrExit("(recovery) import batch: %q into %s: %s", batch.FilePath, batch.TableNameTup, err)
	}

	err = batch.MarkDone()
	if err != nil {
		utils.ErrExit("(recovery) marking batch as done: %q: %s", batch.FilePath, err)
	}
}

func (fti *FileTaskImporter) updateProgress(progressAmount int64) {
	fti.currentProgressAmount += progressAmount
	fti.progressReporter.AddProgressAmount(fti.task, progressAmount)

	// The metrics are sent after evry 5 secs in implementation of UpdateImportedRowCount
	if fti.totalProgressAmount > fti.currentProgressAmount {
		fti.updateProgressInControlPlane(ROW_UPDATE_STATUS_IN_PROGRESS)
	}
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

	// TODO: Implement caching since this is called for every batch
	pkColumns, err := tdb.GetPrimaryKeyColumns(tableNameTup)
	if err != nil {
		utils.ErrExit("getting primary key columns for table %s: %s", tableNameTup.ForMinOutput(), err)
	}
	pkColumns, err = tdb.QuoteAttributeNames(tableNameTup, pkColumns)
	if err != nil {
		utils.ErrExit("if required quote primary key column names: %s", err)
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
