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
	"os"
	"path/filepath"

	goerrors "github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/pingcap/failpoint"
)

// injectImportCDCTransformFailure simulates a failure during CDC event
// transformation on the import side. Used by tests to verify resumability
// and idempotency of the CDC apply path.
func injectImportCDCTransformFailure() error {
	var fpErr error
	failpoint.Inject("importCDCTransformFailure", func(val failpoint.Value) {
		if val != nil {
			_ = os.MkdirAll(filepath.Join(exportDir, "logs"), 0755)
			_ = os.WriteFile(filepath.Join(exportDir, "logs", "failpoint-import-cdc-transform.log"), []byte("hit\n"), 0644)
			fpErr = goerrors.Errorf("failpoint: import CDC transform failure")
		}
	})
	return fpErr
}

// injectImportCDCBatchDBError simulates a non-retryable DB error after a
// successful CDC batch execution. Tests can use a hit-counter expression
// (e.g. 100*off->return(true)) to crash after N successful batches.
func injectImportCDCBatchDBError() error {
	var fpErr error
	failpoint.Inject("importCDCBatchDBError", func(val failpoint.Value) {
		if val != nil {
			_ = os.MkdirAll(filepath.Join(exportDir, "logs"), 0755)
			_ = os.WriteFile(
				filepath.Join(exportDir, "logs", "failpoint-import-cdc-db-error.log"),
				[]byte("hit\n"),
				0644,
			)
			fpErr = &pgconn.PgError{
				Code:    "23505",
				Message: "failpoint: duplicate key value violates unique constraint",
			}
		}
	})
	return fpErr
}

// injectImportSnapshotTransformError simulates a per-row transform failure
// during snapshot batch production. Tests can use a hit-counter expression
// (e.g. 20*off->return(true)) to crash after N rows are processed.
// Black-box tests can set YB_VOYAGER_FAILPOINT_MARKER_DIR to write a marker file.
func injectImportSnapshotTransformError() error {
	var fpErr error
	failpoint.Inject("importSnapshotTransformError", func(val failpoint.Value) {
		if val != nil {
			if markerDir := os.Getenv("YB_VOYAGER_FAILPOINT_MARKER_DIR"); markerDir != "" {
				_ = os.MkdirAll(markerDir, 0755)
				_ = os.WriteFile(filepath.Join(markerDir, "failpoint-import-snapshot-transform-error.log"), []byte("hit\n"), 0644)
			}
			fpErr = goerrors.Errorf("failpoint: snapshot row transform failed")
		}
	})
	return fpErr
}
