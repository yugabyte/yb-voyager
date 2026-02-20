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
	"strconv"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/pingcap/failpoint"
)

// injectPgDumpSnapshotFailure simulates a snapshot failure before pg_dump.
// It optionally delays (via YB_VOYAGER_PGDUMP_FAIL_DELAY_MS), writes a marker file,
// and returns a non-nil error when the failpoint is triggered.
func injectPgDumpSnapshotFailure() error {
	var fpErr error
	failpoint.Inject("pgDumpSnapshotFailure", func(val failpoint.Value) {
		if val != nil {
			if delayMs := os.Getenv("YB_VOYAGER_PGDUMP_FAIL_DELAY_MS"); delayMs != "" {
				if delay, err := strconv.Atoi(delayMs); err == nil && delay > 0 {
					time.Sleep(time.Duration(delay) * time.Millisecond)
				}
			}
			_ = os.MkdirAll(filepath.Join(exportDir, "logs"), 0755)
			_ = os.WriteFile(filepath.Join(exportDir, "logs", "failpoint-pg-dump-snapshot.log"), []byte("hit\n"), 0644)
			fpErr = goerrors.Errorf("failpoint: pg_dump snapshot failure")
		}
	})
	return fpErr
}

// injectSnapshotToCDCTransitionError simulates a failure during the snapshot-to-CDC transition.
// Returns (triggered, error). When triggered is true, the caller should return (false, err).
func injectSnapshotToCDCTransitionError() (bool, error) {
	var triggered bool
	var fpErr error
	failpoint.Inject("snapshotToCDCTransitionError", func(val failpoint.Value) {
		if val != nil {
			_ = os.MkdirAll(filepath.Join(exportDir, "logs"), 0755)
			_ = os.WriteFile(filepath.Join(exportDir, "logs", "failpoint-snapshot-to-cdc.log"), []byte("hit\n"), 0644)
			triggered = true
			fpErr = goerrors.Errorf("failpoint: snapshot->CDC transition failure")
		}
	})
	return triggered, fpErr
}

// injectExportFromTargetStartupError simulates a failure during export-from-target startup.
// Returns (triggered, error). When triggered is true, the caller should return (false, err).
func injectExportFromTargetStartupError() (bool, error) {
	var triggered bool
	var fpErr error
	failpoint.Inject("exportFromTargetStartupError", func(val failpoint.Value) {
		if val != nil {
			_ = os.MkdirAll(filepath.Join(exportDir, "logs"), 0755)
			_ = os.WriteFile(filepath.Join(exportDir, "logs", "failpoint-export-from-target-startup.log"), []byte("hit\n"), 0644)
			triggered = true
			fpErr = goerrors.Errorf("failpoint: export-from-target startup failure")
		}
	})
	return triggered, fpErr
}
