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

// injectImportBatchCommitError simulates a commit failure during
// transactional COPY batch import. Returns (true, err) when the
// failpoint fires so the caller can skip the real commit.
func injectImportBatchCommitError(batch Batch) (triggered bool, importBatchErr error) {
	failpoint.Inject("importBatchCommitError", func(val failpoint.Value) {
		if val != nil {
			err2 := goerrors.Errorf("failpoint: commit failed")
			importBatchErr = newImportBatchErrorPgYb(err2, batch,
				errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL,
				errs.IMPORT_BATCH_ERROR_STEP_COMMIT_TXN, 
				nil)
			triggered = true
		}
	})
	return
}

// injectImportCDCRetryableExecuteBatchError simulates a retryable target DB
// error at the start of CDC batch execution. Used by tests to validate
// in-process retry behaviour without requiring a resume run.
// Black-box tests can set YB_VOYAGER_FAILPOINT_MARKER_DIR to write a marker file.
func injectImportCDCRetryableExecuteBatchError() error {
	var fpErr error
	failpoint.Inject("importCDCRetryableExecuteBatchError", func(val failpoint.Value) {
		if val != nil {
			if markerDir := os.Getenv("YB_VOYAGER_FAILPOINT_MARKER_DIR"); markerDir != "" {
				_ = os.MkdirAll(markerDir, 0755)
				_ = os.WriteFile(filepath.Join(markerDir, "failpoint-import-cdc-retryable-exec-batch-error.log"), []byte("hit\n"), 0644)
			}
			fpErr = &pgconn.PgError{Code: "40001", Message: "failpoint: retryable execute batch error"}
		}
	})
	return fpErr
}

// injectImportCDCExecEventError simulates a non-retryable DB error while
// applying a single CDC event inside a batch. Tests can use a hit-counter
// expression (e.g. 50*off->return(true)) to fail on the (N+1)-th event.
// Black-box tests can set YB_VOYAGER_FAILPOINT_MARKER_DIR to write a marker file.
func injectImportCDCExecEventError() error {
	var fpErr error
	failpoint.Inject("importCDCExecEventError", func(val failpoint.Value) {
		if val != nil {
			if markerDir := os.Getenv("YB_VOYAGER_FAILPOINT_MARKER_DIR"); markerDir != "" {
				_ = os.MkdirAll(markerDir, 0755)
				_ = os.WriteFile(filepath.Join(markerDir, "failpoint-import-cdc-exec-event-error.log"), []byte("hit\n"), 0644)
			}
			fpErr = &pgconn.PgError{
				Code:    "23505",
				Message: "failpoint: cdc event exec failed",
			}
		}
	})
	return fpErr
}

// injectImportCDCRetryableAfterCommitError simulates a retryable error that
// occurs after the CDC batch transaction actually commits on the server.
// This exercises the safety net that detects already-imported batches.
// Black-box tests can set YB_VOYAGER_FAILPOINT_MARKER_DIR to write a marker file.
func injectImportCDCRetryableAfterCommitError() error {
	var fpErr error
	failpoint.Inject("importCDCRetryableAfterCommitError", func(val failpoint.Value) {
		if val != nil {
			if markerDir := os.Getenv("YB_VOYAGER_FAILPOINT_MARKER_DIR"); markerDir != "" {
				_ = os.MkdirAll(markerDir, 0755)
				_ = os.WriteFile(filepath.Join(markerDir, "failpoint-import-cdc-retryable-after-commit-error.log"), []byte("hit\n"), 0644)
			}
			fpErr = &pgconn.PgError{
				Code:    "40001",
				Message: "failpoint: retryable error after commit",
			}
		}
	})
	return fpErr
}
