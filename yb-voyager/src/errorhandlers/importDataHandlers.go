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

package errorhandlers

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

type ErroredBatch interface {
	GetFilePath() string
	GetTableName() sqlname.NameTuple
	IsInterrupted() bool
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

func (handler *ImportDataStashAndContinueHandler) HandleBatchIngestionError(batch ErroredBatch, err error) error {
	return nil
}
