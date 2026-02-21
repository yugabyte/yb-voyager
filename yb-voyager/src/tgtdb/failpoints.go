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
package tgtdb

import (
	"os"
	"path/filepath"

	goerrors "github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/pingcap/failpoint"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/errs"
)

// injectImportBatchCommitError injects a commit error before the actual transaction commit
// in the snapshot batch import path. Returns (rowsAffected, err, triggered).
// When triggered=true, the caller should return immediately with the provided values.
func injectImportBatchCommitError(batch Batch) (int64, error, bool) {
	var triggered bool
	var batchErr error
	failpoint.Inject("importBatchCommitError", func(val failpoint.Value) {
		if val != nil {
			commitErr := goerrors.Errorf("failpoint: commit failed")
			batchErr = newImportBatchErrorPgYb(commitErr, batch,
				errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL,
				errs.IMPORT_BATCH_ERROR_STEP_COMMIT_TXN)
			triggered = true
		}
	})
	return 0, batchErr, triggered
}

// injectImportCDCRetryableExecuteBatchError injects a retryable target DB error at the start
// of CDC batch execution. Used to validate retry behavior without requiring a resume run.
// Tests can use a one-shot or hit-counter expression, e.g.:
//
//	importCDCRetryableExecuteBatchError=1*return(true)
func injectImportCDCRetryableExecuteBatchError() error {
	var fpErr error
	failpoint.Inject("importCDCRetryableExecuteBatchError", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarkerFromEnv("failpoint-import-cdc-retryable-exec-batch-error.log")
			fpErr = &pgconn.PgError{Code: "40001", Message: "failpoint: retryable execute batch error"}
		}
	})
	return fpErr
}

// injectImportCDCExecEventError injects a non-retryable DB error while applying a CDC event in a batch.
// Used by import-side streaming tests to force a deterministic crash mid-stream.
// Tests can use a hit-counter expression (e.g. `50*off->return(true)`) to fail on the (N+1)-th event.
func injectImportCDCExecEventError() error {
	var fpErr error
	failpoint.Inject("importCDCExecEventError", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarkerFromEnv("failpoint-import-cdc-exec-event-error.log")
			fpErr = &pgconn.PgError{
				Code:    "23505",
				Message: "failpoint: cdc event exec failed",
			}
		}
	})
	return fpErr
}

// injectImportCDCRetryableAfterCommitError simulates a "commit succeeded but ExecuteBatch
// returned a retryable error" scenario. The importer retry loop should detect that the batch
// is already imported (via last_applied_vsn) and avoid re-applying / double-counting.
// Tests can use a one-shot or hit-counter expression, e.g.:
//
//	importCDCRetryableAfterCommitError=1*return(true)
func injectImportCDCRetryableAfterCommitError() error {
	var fpErr error
	failpoint.Inject("importCDCRetryableAfterCommitError", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarkerFromEnv("failpoint-import-cdc-retryable-after-commit-error.log")
			fpErr = &pgconn.PgError{
				Code:    "40001",
				Message: "failpoint: retryable error after commit",
			}
		}
	})
	return fpErr
}

// writeFailpointMarkerFromEnv writes a marker file to the directory specified by
// YB_VOYAGER_FAILPOINT_MARKER_DIR, if set. This enables black-box tests that run
// yb-voyager as an external process to detect that a failpoint was triggered.
func writeFailpointMarkerFromEnv(filename string) {
	if markerDir := os.Getenv("YB_VOYAGER_FAILPOINT_MARKER_DIR"); markerDir != "" {
		_ = os.MkdirAll(markerDir, 0755)
		_ = os.WriteFile(filepath.Join(markerDir, filename), []byte("hit\n"), 0644)
	}
}
