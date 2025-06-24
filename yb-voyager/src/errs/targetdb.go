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

package errs

import (
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	// steps
	IMPORT_BATCH_ERROR_STEP_COPY                         = "copy"
	IMPORT_BATCH_ERROR_STEP_METADATA_ENTRY               = "metadata_entry"
	IMPORT_BATCH_ERROR_STEP_CHECK_BATCH_ALREADY_IMPORTED = "check_batch_already_imported"
	IMPORT_BATCH_ERROR_STEP_OPEN_BATCH                   = "open_batch"
	IMPORT_BATCH_ERROR_STEP_READ_LINE_BATCH              = "read_line_batch"
	IMPORT_BATCH_ERROR_STEP_COMMIT_TXN                   = "commit_txn"
	IMPORT_BATCH_ERROR_STEP_ROLLBACK_TXN                 = "rollback_txn"
)

type ImportBatchError struct {
	tableName         sqlname.NameTuple
	batchFilePath     string
	err               error
	flow              string
	step              string
	dbSpecificContext map[string]string
}

func (e ImportBatchError) Error() string {
	return fmt.Sprintf("import batch: %q into %s: flow=%s: step=%s: %s (%s)", e.batchFilePath, e.tableName.ForOutput(), e.flow, e.step, e.err.Error(), mapToString(e.dbSpecificContext))
}

func (e ImportBatchError) Unwrap() error {
	return e.err
}

func NewImportBatchError(tableName sqlname.NameTuple, batchFilePath string, err error, flow, step string, dbSpecificContext map[string]string) ImportBatchError {
	if dbSpecificContext == nil {
		dbSpecificContext = make(map[string]string)
	}
	return ImportBatchError{
		tableName:         tableName,
		batchFilePath:     batchFilePath,
		err:               err,
		flow:              flow,
		step:              step,
		dbSpecificContext: dbSpecificContext,
	}
}

func mapToString(m map[string]string) string {
	pairs := make([]string, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, ",")
}
