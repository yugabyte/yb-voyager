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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/errorpolicy"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type ErroredBatch interface {
	GetFilePath() string
	GetFileDirectory() string
	GetTableName() sqlname.NameTuple
	IsInterrupted() bool
	MarkError() error
}

type ImportDataAbortHandler struct {
}

func NewImportDataAbortHandler() *ImportDataAbortHandler {
	return &ImportDataAbortHandler{}
}

func (handler *ImportDataAbortHandler) ShouldAbort() bool {
	return true
}

func (handler *ImportDataAbortHandler) HandleRowProcessingError() error {
	// nothing to do.
	return nil
}

func (handler *ImportDataAbortHandler) HandleBatchIngestionError(batch ErroredBatch, err error) error {
	// nothing to do.
	return nil
}

/*
Stash the error to some file(s) with the relevant error information
*/
type ImportDataStashAndContinueHandler struct {
}

func NewImportDataStashAndContinueHandler() *ImportDataStashAndContinueHandler {
	return &ImportDataStashAndContinueHandler{}
}

func (handler *ImportDataStashAndContinueHandler) ShouldAbort() bool {
	return false
}

func (handler *ImportDataStashAndContinueHandler) HandleRowProcessingError() error {
	return nil
}

func (handler *ImportDataStashAndContinueHandler) HandleBatchIngestionError(batch ErroredBatch, batchErr error) error {
	err := batch.MarkError()
	if err != nil {
		return fmt.Errorf("marking batch as errored: %s", err)
	}
	err = handler.createBatchSymlinkInErrorsFolder(batch)
	if err != nil {
		return fmt.Errorf("creating symlink in errors folder: %s", err)
	}
	return nil
}

func (handler *ImportDataStashAndContinueHandler) getTaskErrorsFolderPath(batch ErroredBatch) string {
	return filepath.Join(batch.GetFileDirectory(), "errors")
}

func (handler *ImportDataStashAndContinueHandler) mkdirPErrorsFolder(batch ErroredBatch) error {
	err := os.MkdirAll(handler.getTaskErrorsFolderPath(batch), os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating errors folder: %s", err)
	}
	return nil
}

func (handler *ImportDataStashAndContinueHandler) createBatchSymlinkInErrorsFolder(batch ErroredBatch) error {
	err := handler.mkdirPErrorsFolder(batch)
	if err != nil {
		return fmt.Errorf("creating errors folder: %s", err)
	}
	// create a symlink to the batch file in the errors folder so that all errors are in one place.
	// errors/ingestion-error.<batch_file_name> -> <batch_file_name>
	symlinkFileName := fmt.Sprintf("%s.%s", "ingestion-error", filepath.Base(batch.GetFilePath()))
	err = os.Symlink(batch.GetFilePath(), filepath.Join(handler.getTaskErrorsFolderPath(batch), symlinkFileName))
	if err != nil {
		return fmt.Errorf("creating symlink: %s", err)
	}
	return nil
}

type ImportDataErrorHandler interface {
	ShouldAbort() bool
	HandleRowProcessingError() error
	HandleBatchIngestionError(batch ErroredBatch, err error) error
}

func GetImportDataErrorHandler(errorPolicy errorpolicy.ErrorPolicy) (ImportDataErrorHandler, error) {
	switch errorPolicy {
	case errorpolicy.AbortErrorPolicy:
		return NewImportDataAbortHandler(), nil
	case errorpolicy.StashAndContinueErrorPolicy:
		return NewImportDataStashAndContinueHandler(), nil
	default:
		return nil, fmt.Errorf("unknown error policy: %s", errorPolicy)
	}
}
