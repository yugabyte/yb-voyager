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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	PROCESSING_ERRORS_BASE_NAME               = "processing-errors"
	PROCESSING_ERRORS_LOG_FILE                = "processing-errors.log"
	INGESTION_ERROR_PREFIX                    = "ingestion-error"
	STASH_AND_CONTINUE_RECOMMENDATION_MESSAGE = "To continue with the import without aborting, set the configuration parameter `error-policy`/`error-policy-snapshot` to `stash-and-continue`"
)

type ImportDataErrorHandler interface {
	ShouldAbort() bool
	HandleRowProcessingError(row string, rowByteCount int64, rowErr error, tableName sqlname.NameTuple, taskFilePath string, batchNumber int64) error
	HandleBatchIngestionError(batch ErroredBatch, taskFilePath string, batchErr error, isPartialBatchIngestionPossible bool) error
	CleanUpStoredErrors(tableName sqlname.NameTuple, taskFilePath string) error
	GetErrorsLocation() string
	FinalizeRowProcessingErrorsForBatch(batchNumber int64, tableName sqlname.NameTuple, taskFilePath string) error
	GetProcessingErrorCountSize(tableName sqlname.NameTuple, taskFilePath string) (int64, int64, error)
}

type ErroredBatch interface {
	GetFilePath() string
	GetTableName() sqlname.NameTuple
	IsInterrupted() bool
	MarkError(batchErr error, isPartialBatchIngestionPossible bool) error
}

// -----------------------------------------------------------------------------------------------------//
type ImportDataAbortHandler struct {
}

func NewImportDataAbortHandler() *ImportDataAbortHandler {
	return &ImportDataAbortHandler{}
}

func (handler *ImportDataAbortHandler) ShouldAbort() bool {
	return true
}

func (handler *ImportDataAbortHandler) HandleRowProcessingError(row string, rowByteCount int64, rowErr error, tableName sqlname.NameTuple, taskFilePath string, batchNumber int64) error {
	// nothing to do.
	return nil
}

func (handler *ImportDataAbortHandler) HandleBatchIngestionError(batch ErroredBatch, taskFilePath string, batchErr error, isPartialBatchIngestionPossible bool) error {
	// nothing to do.
	return nil
}

func (handler *ImportDataAbortHandler) CleanUpStoredErrors(tableName sqlname.NameTuple, taskFilePath string) error {
	// nothing to do.
	return nil
}

func (handler *ImportDataAbortHandler) GetErrorsLocation() string {
	return ""
}

func (handler *ImportDataAbortHandler) FinalizeRowProcessingErrorsForBatch(batchNumber int64, tableName sqlname.NameTuple, taskFilePath string) error {
	// nothing to do for abort handler
	return nil
}

func (handler *ImportDataAbortHandler) GetProcessingErrorCountSize(tableName sqlname.NameTuple, taskFilePath string) (int64, int64, error) {
	// nothing to do for abort handler
	return 0, 0, nil
}

// -----------------------------------------------------------------------------------------------------//

/*
Stash the error to some file(s) with the relevant error information
*/
type ImportDataStashAndContinueHandler struct {
	dataDir                     string
	rowProcessingErrorFiles     map[string]*os.File // key is table-task-batch
	rowProcessingErrorRowCount  map[string]int64    // key is table-task-batch
	rowProcessingErrorByteCount map[string]int64    // key is table-task-batch
}

func NewImportDataStashAndContinueHandler(dataDir string) *ImportDataStashAndContinueHandler {
	return &ImportDataStashAndContinueHandler{
		dataDir:                     dataDir,
		rowProcessingErrorFiles:     make(map[string]*os.File),
		rowProcessingErrorRowCount:  make(map[string]int64),
		rowProcessingErrorByteCount: make(map[string]int64),
	}
}

func (handler *ImportDataStashAndContinueHandler) ShouldAbort() bool {
	return false
}

// HandleRowProcessingError writes the row and error to a processing-errors.<batchNumber>.log.tmp file.
// <export-dir>/data/errors/table::<table-name>/file::<base-path>:<hash>/processing-errors.<batchNumber>.log.tmp
func (handler *ImportDataStashAndContinueHandler) HandleRowProcessingError(row string, rowByteCount int64, rowErr error, tableName sqlname.NameTuple, taskFilePath string, batchNumber int64) error {
	var err error
	if row == "" && rowErr == nil {
		return nil
	}
	errorsDir := handler.getErrorsFolderPathForTableTask(tableName, taskFilePath)
	if err := os.MkdirAll(errorsDir, os.ModePerm); err != nil {
		return fmt.Errorf("creating errors dir: %w", err)
	}

	tableTaskBatchKey := handler.generateTableTaskBatchKey(tableName, taskFilePath, batchNumber)
	errorFile, ok := handler.rowProcessingErrorFiles[tableTaskBatchKey]
	if !ok {
		errorFileName := fmt.Sprintf("%s.%d.log.tmp", PROCESSING_ERRORS_BASE_NAME, batchNumber)
		errorFilePath := filepath.Join(errorsDir, errorFileName)
		errorFile, err = os.OpenFile(errorFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("creating error file %s: %w", errorFilePath, err)
		}
		handler.rowProcessingErrorFiles[tableTaskBatchKey] = errorFile
	}
	handler.rowProcessingErrorRowCount[tableTaskBatchKey]++
	handler.rowProcessingErrorByteCount[tableTaskBatchKey] += rowByteCount

	/*
		ERROR: <error message>
		ROW: <row data>
	*/
	msg := fmt.Sprintf("ERROR: %s\nROW: %s\n\n", rowErr, row)
	_, err = errorFile.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("writing to error file %s: %w", errorFile.Name(), err)
	}
	return nil
}

// generateTableTaskBatchKey creates a unique key for identifying data by table, task, and batch
func (handler *ImportDataStashAndContinueHandler) generateTableTaskBatchKey(tableName sqlname.NameTuple, taskFilePath string, batchNumber int64) string {
	return fmt.Sprintf("%s-%s-%d", tableName.ForMinOutput(), ComputePathHash(taskFilePath), batchNumber)
}

// GetProcessingErrorCountSize extracts row count and byte count from existing processing error file names
// Returns the total row count and byte count across all error files for the given table and task
func (handler *ImportDataStashAndContinueHandler) GetProcessingErrorCountSize(tableName sqlname.NameTuple, taskFilePath string) (int64, int64, error) {
	errorsDir := handler.getErrorsFolderPathForTableTask(tableName, taskFilePath)

	// Pattern to match processing error files: processing-errors.<batchNumber>.<rowCount>.<byteCount>.log
	globPattern := fmt.Sprintf("%s.*.*.*.log", PROCESSING_ERRORS_BASE_NAME)
	searchPath := filepath.Join(errorsDir, globPattern)

	files, err := filepath.Glob(searchPath)
	if err != nil {
		return 0, 0, fmt.Errorf("error globbing for processing error files with pattern %s: %w", searchPath, err)
	}

	var totalRowCount, totalByteCount int64

	for _, file := range files {
		fileName := filepath.Base(file)
		// Extract row count and byte count from filename: processing-errors.<batchNumber>.<rowCount>.<byteCount>.log
		parts := strings.Split(fileName, ".")
		if len(parts) < 4 {
			return 0, 0, fmt.Errorf("filename %s does not have enough parts to parse (expected 4, got %d)", fileName, len(parts))
		}

		// parts[0] = "processing-errors", parts[1] = batchNumber, parts[2] = rowCount, parts[3] = byteCount
		rowCount, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse row count from filename %s: %w", fileName, err)
		}
		totalRowCount += rowCount

		byteCount, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse byte count from filename %s: %w", fileName, err)
		}
		totalByteCount += byteCount
	}

	return totalRowCount, totalByteCount, nil
}

// FinalizeRowProcessingErrorsForBatch renames the temporary error file to include statistics
// from processing-errors.<batchNumber>.log.tmp to processing-errors.<batchNumber>.<rowCount>.<batchByteCount>.log
func (handler *ImportDataStashAndContinueHandler) FinalizeRowProcessingErrorsForBatch(batchNumber int64, tableName sqlname.NameTuple, taskFilePath string) error {
	tableTaskBatchKey := handler.generateTableTaskBatchKey(tableName, taskFilePath, batchNumber)

	// Get the current error file
	errorFile, ok := handler.rowProcessingErrorFiles[tableTaskBatchKey]
	if !ok {
		return nil
	}
	if errorFile != nil {
		errorFile.Close()
	}

	// Delete old error files potentially left over from previous run.
	errorsDir := handler.getErrorsFolderPathForTableTask(tableName, taskFilePath)
	globPattern := fmt.Sprintf("%s.%d.*.log", PROCESSING_ERRORS_BASE_NAME, batchNumber)
	searchPath := filepath.Join(errorsDir, globPattern)

	filesToDelete, err := filepath.Glob(searchPath)
	if err != nil {
		// Log the error but don't return, as the main task (renaming the current file) can still proceed.
		log.Errorf("Error globbing for old error files with pattern %s: %v", searchPath, err)
	} else {
		for _, file := range filesToDelete {
			err := os.Remove(file)
			if err != nil {
				log.Errorf("Error deleting old error file %s: %v", file, err)
			} else {
				log.Debugf("Deleted old error file: %s", file)
			}
		}
	}

	// 	rename to the new filename with row count and byte count
	rowCount := handler.rowProcessingErrorRowCount[tableTaskBatchKey]
	byteCount := handler.rowProcessingErrorByteCount[tableTaskBatchKey]
	newFileName := fmt.Sprintf("%s.%d.%d.%d.log", PROCESSING_ERRORS_BASE_NAME, batchNumber, rowCount, byteCount)
	newFilePath := filepath.Join(errorsDir, newFileName)

	oldFilePath := errorFile.Name()
	err = os.Rename(oldFilePath, newFilePath)
	if err != nil {
		return fmt.Errorf("renaming error file from %s to %s: %w", filepath.Base(oldFilePath), newFileName, err)
	}

	// Clean up the maps
	delete(handler.rowProcessingErrorFiles, tableTaskBatchKey)
	delete(handler.rowProcessingErrorRowCount, tableTaskBatchKey)
	delete(handler.rowProcessingErrorByteCount, tableTaskBatchKey)

	return nil
}

/*
- Mark the batch as errored out. x.x.x.x.P -> x.x.x.x.E
- Create a symlink in the errors folder, pointing to metainfo/import_data_state/.../<batch_file_name>
*/
func (handler *ImportDataStashAndContinueHandler) HandleBatchIngestionError(batch ErroredBatch, taskFilePath string, batchErr error, isPartialBatchIngestionPossible bool) error {
	if batch == nil {
		return fmt.Errorf("batch cannot be nil")
	}
	if taskFilePath == "" {
		return fmt.Errorf("task file path cannot be empty")
	}

	err := batch.MarkError(batchErr, isPartialBatchIngestionPossible)
	if err != nil {
		return fmt.Errorf("marking batch as errored: %s", err)
	}
	err = handler.createBatchSymlinkInErrorsFolder(batch, taskFilePath)
	if err != nil {
		return fmt.Errorf("creating symlink in errors folder: %s", err)
	}
	return nil
}

// create a symlink to the batch file in the errors folder so that all errors are in one place.
// ingestion-error.<batch_file_name> -> metainfo/.../<batch_file_name>
func (handler *ImportDataStashAndContinueHandler) createBatchSymlinkInErrorsFolder(batch ErroredBatch, taskFilePath string) error {
	errorsFolderPathForTableTask := handler.getErrorsFolderPathForTableTask(batch.GetTableName(), taskFilePath)
	err := os.MkdirAll(errorsFolderPathForTableTask, os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating errors folder: %s", err)
	}

	symlinkFileName := fmt.Sprintf("%s.%s", INGESTION_ERROR_PREFIX, filepath.Base(batch.GetFilePath()))
	err = os.Symlink(batch.GetFilePath(), filepath.Join(errorsFolderPathForTableTask, symlinkFileName))
	if err != nil {
		return fmt.Errorf("creating symlink: %s", err)
	}
	return nil
}

// <export-dir>/data/errors/table::<table-name>/file::<base-path>:<hash>/
func (handler *ImportDataStashAndContinueHandler) getErrorsFolderPathForTableTask(tableName sqlname.NameTuple, taskFilePath string) string {
	tableFolder := fmt.Sprintf("table::%s", tableName.ForMinOutput())
	// the entire path of the file can be long, so to make it shorter,
	// we compute a hash of the file path and also include the base file name of the file.
	taskFolder := fmt.Sprintf("file::%s:%s", filepath.Base(taskFilePath), ComputePathHash(taskFilePath))
	return filepath.Join(handler.dataDir, "errors", tableFolder, taskFolder)
}

func (handler *ImportDataStashAndContinueHandler) CleanUpStoredErrors(tableName sqlname.NameTuple, taskFilePath string) error {
	if taskFilePath == "" {
		return fmt.Errorf("task file path cannot be empty")
	}

	err := os.RemoveAll(handler.getErrorsFolderPathForTableTask(tableName, taskFilePath))
	if err != nil {
		return fmt.Errorf("removing errors folder for table : %s", err)
	}
	return nil
}

func (handler *ImportDataStashAndContinueHandler) GetErrorsLocation() string {
	return filepath.Join(handler.dataDir, "errors")
}

// exporting this only to enable unit testing from cmd package.
// ideally, this should be private to this package.
func ComputePathHash(filePath string) string {
	hash := sha1.New()
	hash.Write([]byte(filePath))
	return hex.EncodeToString(hash.Sum(nil))[0:8]
}

// -----------------------------------------------------------------------------------------------------//

func GetImportDataErrorHandler(errorPolicy ErrorPolicy, dataDir string) (ImportDataErrorHandler, error) {
	switch errorPolicy {
	case AbortErrorPolicy:
		return NewImportDataAbortHandler(), nil
	case StashAndContinueErrorPolicy:
		return NewImportDataStashAndContinueHandler(dataDir), nil
	default:
		return nil, fmt.Errorf("unknown error policy: %s", errorPolicy)
	}
}
