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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	PROCESSING_ERRORS_LOG_FILE = "processing-errors.log"
	INGESTION_ERROR_PREFIX     = "ingestion-error"
)

var defaultProcessingErrorFileSize int64 = 5 * 1024 * 1024 // 5MB

type ImportDataErrorHandler interface {
	ShouldAbort() bool
	HandleRowProcessingError(row string, rowErr error, tableName sqlname.NameTuple, taskFilePath string) error
	HandleBatchIngestionError(batch ErroredBatch, taskFilePath string, batchErr error, isPartialBatchIngestionPossible bool) error
	CleanUpStoredErrors(tableName sqlname.NameTuple, taskFilePath string) error
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

func (handler *ImportDataAbortHandler) HandleRowProcessingError(row string, rowErr error, tableName sqlname.NameTuple, taskFilePath string) error {
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

// -----------------------------------------------------------------------------------------------------//

/*
Stash the error to some file(s) with the relevant error information
*/
type ImportDataStashAndContinueHandler struct {
	dataDir                 string
	rowProcessingErrorFiles map[string]*utils.RotatableFile // one per table/task file
}

func NewImportDataStashAndContinueHandler(dataDir string) *ImportDataStashAndContinueHandler {
	return &ImportDataStashAndContinueHandler{
		dataDir:                 dataDir,
		rowProcessingErrorFiles: make(map[string]*utils.RotatableFile),
	}
}

func (handler *ImportDataStashAndContinueHandler) ShouldAbort() bool {
	return false
}

// HandleRowProcessingError writes the row and error to a processing-errors.log roratingFile.
// <export-dir>/data/errors/table::<table-name>/file::<base-path>:<hash>/processing-errors.log
// On rotation, new files of the format processing-errors-<timestamp>.log will be created.
func (handler *ImportDataStashAndContinueHandler) HandleRowProcessingError(row string, rowErr error, tableName sqlname.NameTuple, taskFilePath string) error {
	var err error
	if row == "" && rowErr == nil {
		return nil
	}
	errorsDir := handler.getErrorsFolderPathForTableTask(tableName, taskFilePath)
	if err := os.MkdirAll(errorsDir, os.ModePerm); err != nil {
		return fmt.Errorf("creating errors dir: %w", err)
	}

	tableFilePathKey := fmt.Sprintf("%s::%s", tableName.ForMinOutput(), ComputePathHash(taskFilePath))
	errorFile, ok := handler.rowProcessingErrorFiles[tableFilePathKey]
	if !ok {
		errorFilePath := filepath.Join(errorsDir, PROCESSING_ERRORS_LOG_FILE)
		errorFile, err = utils.NewRotatableFile(errorFilePath, defaultProcessingErrorFileSize)
		if err != nil {
			return fmt.Errorf("creating file rotator: %w", err)
		}
		handler.rowProcessingErrorFiles[tableFilePathKey] = errorFile
	}

	/*
		ERROR: <error message>
		ROW: <row data>
	*/
	msg := fmt.Sprintf("ERROR: %s\nROW: %s\n\n", rowErr, row)
	_, err = errorFile.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("writing to %s: %w", PROCESSING_ERRORS_LOG_FILE, err)
	}
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
