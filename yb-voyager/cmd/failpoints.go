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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// injectImportCDCTransformFailure simulates a failure during CDC event
// transformation on the import side. Used by tests to verify resumability
// and idempotency of the CDC apply path.
func injectImportCDCTransformFailure() error {
	var fpErr error
	failpoint.Inject("importCDCTransformFailure", func(val failpoint.Value) {
		if val != nil {
			_ = os.MkdirAll(filepath.Join(exportDir, "failpoints"), 0755)
			_ = os.WriteFile(filepath.Join(exportDir, "failpoints", "failpoint-import-cdc-transform.log"), []byte("hit\n"), 0644)
			fpErr = goerrors.Errorf("failpoint: import CDC transform failure")
		}
	})
	return fpErr
}

// injectImportCDCNonRetryableBatchDBError simulates a non-retryable DB error after a
// successful CDC batch execution. Tests can use a hit-counter expression
// (e.g. 100*off->return(true)) to crash after N successful batches.
func injectImportCDCNonRetryableBatchDBError() error {
	var fpErr error
	failpoint.Inject("importCDCNonRetryableBatchDBError", func(val failpoint.Value) {
		if val != nil {
			_ = os.MkdirAll(filepath.Join(exportDir, "failpoints"), 0755)
			_ = os.WriteFile(
				filepath.Join(exportDir, "failpoints", "failpoint-import-cdc-non-retryable-batch-db-error.log"),
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

func writeFailpointMarker(filename string) {
	_ = os.MkdirAll(filepath.Join(exportDir, "failpoints"), 0755)
	_ = os.WriteFile(filepath.Join(exportDir, "failpoints", filename), []byte("hit\n"), 0644)
}

// injectCutoverToSourceExporterPostMarkProcessed crashes export-from-target after
// it marks CutoverToSourceProcessedByTargetExporter, but before it chains to
// startNextIterationImportDataToTarget. Tests verify the exporter resumes correctly
// on re-run: it detects cutover-already-processed and chains to the next iteration.
func injectCutoverToSourceExporterPostMarkProcessed() {
	failpoint.Inject("cutoverToSourceExporterPostMarkProcessed", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarker("failpoint-cutover-to-source-exporter-post-mark.log")
			utils.ErrExit("failpoint: crash after marking cutover-to-source processed by exporter")
		}
	})
}

// injectCutoverToSourceImporterPostMarkProcessed crashes import-to-source after
// it marks CutoverToSourceProcessedBySourceImporter in postCutoverProcessing,
// but before waitUntilCutoverProcessedByCorrespondingExporterForImporter.
// Tests verify the importer resumes correctly: it detects cutover-already-processed
// and chains to startExportDataFromSourceOnNextIteration.
func injectCutoverToSourceImporterPostMarkProcessed() {
	failpoint.Inject("cutoverToSourceImporterPostMarkProcessed", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarker("failpoint-cutover-to-source-importer-post-mark.log")
			utils.ErrExit("failpoint: crash after marking cutover-to-source processed by importer")
		}
	})
}

// injectBeforeInitializeNextIteration crashes import-to-source after
// waitUntilCutoverProcessedByCorrespondingExporterForImporter completes but before
// initializeNextIteration runs. Tests verify the importer resumes correctly:
// initializeNextIteration is fully idempotent (hasn't run yet in this case).
func injectBeforeInitializeNextIteration() {
	failpoint.Inject("beforeInitializeNextIteration", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarker("failpoint-before-init-next-iteration.log")
			utils.ErrExit("failpoint: crash before initializing next iteration")
		}
	})
}

// injectDuringInitializeNextIteration crashes import-to-source during
// initializeNextIteration after setUpNextIterationMSR but before setting
// NextIterationInitialized = true. Tests verify that partial iteration state
// is handled correctly: initializeNextIteration is fully idempotent.
func injectDuringInitializeNextIteration() {
	failpoint.Inject("duringInitializeNextIteration", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarker("failpoint-during-init-next-iteration.log")
			utils.ErrExit("failpoint: crash during initialize next iteration")
		}
	})
}

// injectAfterInitializeNextIteration crashes import-to-source after
// initializeNextIteration completes (NextIterationInitialized = true) but
// before syscall.Exec to export-data-from-source. Tests verify that on
// re-run, the process detects the iteration is already initialized and
// proceeds directly to exec.
func injectAfterInitializeNextIteration() {
	failpoint.Inject("afterInitializeNextIteration", func(val failpoint.Value) {
		if val != nil {
			writeFailpointMarker("failpoint-after-init-next-iteration.log")
			utils.ErrExit("failpoint: crash after initializing next iteration")
		}
	})
}
