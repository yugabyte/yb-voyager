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
	workerPool           *pool.Pool

	totalProgressAmount   int64
	currentProgressAmount int64
	progressReporter      *ImportDataProgressReporter
}

func NewFileTaskImporter(task *ImportFileTask, state *ImportDataState, workerPool *pool.Pool,
	progressReporter *ImportDataProgressReporter) (*FileTaskImporter, error) {
	batchProducer, err := NewFileBatchProducer(task, state)
	if err != nil {
		return nil, fmt.Errorf("creating file batch producer: %s", err)
	}
	totalProgressAmount := getTotalProgressAmount(task)
	progressReporter.ImportFileStarted(task, totalProgressAmount)
	currentProgressAmount := getImportedProgressAmount(task, state)
	progressReporter.AddProgressAmount(task, currentProgressAmount)

	fti := &FileTaskImporter{
		task:                  task,
		batchProducer:         batchProducer,
		workerPool:            workerPool,
		importBatchArgsProto:  getImportBatchArgsProto(task.TableNameTup, task.FilePath),
		progressReporter:      progressReporter,
		totalProgressAmount:   totalProgressAmount,
		currentProgressAmount: currentProgressAmount,
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

func (fti *FileTaskImporter) importBatch(batch *Batch) {
	err := batch.MarkInProgress()
	if err != nil {
		utils.ErrExit("marking batch as pending: %d: %s", batch.Number, err)
	}
	log.Infof("Importing %q", batch.FilePath)

	importBatchArgs := *fti.importBatchArgsProto
	importBatchArgs.FilePath = batch.FilePath
	importBatchArgs.RowsPerTransaction = batch.OffsetEnd - batch.OffsetStart

	var rowsAffected int64
	sleepIntervalSec := 0
	for attempt := 0; attempt < COPY_MAX_RETRY_COUNT; attempt++ {
		tableSchema, _ := TableNameToSchema.Get(batch.TableNameTup)
		rowsAffected, err = tdb.ImportBatch(batch, &importBatchArgs, exportDir, tableSchema)
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

func (fti *FileTaskImporter) submitBatch(batch *Batch) error {
	fti.workerPool.Go(func() {
		// There are `poolSize` number of competing go-routines trying to invoke COPY.
		// But the `connPool` will allow only `parallelism` number of connections to be
		// used at a time. Thus limiting the number of concurrent COPYs to `parallelism`.
		fti.importBatch(batch)
		if reportProgressInBytes {
			fti.updateProgress(batch.ByteCount)
		} else {
			fti.updateProgress(batch.RecordCount)
		}
	})
	log.Infof("Queued batch: %s", spew.Sdump(batch))
	return nil
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
	// If `columns` is unset at this point, no attribute list is passed in the COPY command.
	fileFormat := dataFileDescriptor.FileFormat

	// from export data with ora2pg, it comes as an SQL file, with COPY command having data.
	// Import-data also reads it appropriately with the help of sqlDataFile.
	// But while running COPY for a batch, we need to set the format as TEXT (SQL does not make sense)
	if fileFormat == datafile.SQL {
		fileFormat = datafile.TEXT
	}
	importBatchArgsProto := &tgtdb.ImportBatchArgs{
		TableNameTup: tableNameTup,
		Columns:      columns,
		FileFormat:   fileFormat,
		Delimiter:    dataFileDescriptor.Delimiter,
		HasHeader:    dataFileDescriptor.HasHeader && fileFormat == datafile.CSV,
		QuoteChar:    dataFileDescriptor.QuoteChar,
		EscapeChar:   dataFileDescriptor.EscapeChar,
		NullString:   dataFileDescriptor.NullString,
	}
	log.Infof("ImportBatchArgs: %v", spew.Sdump(importBatchArgsProto))
	return importBatchArgsProto
}
