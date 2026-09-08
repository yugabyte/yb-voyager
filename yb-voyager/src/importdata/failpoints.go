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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	goerrors "github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/pingcap/failpoint"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

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

// injectImportCDCTransformFailure simulates a failure during CDC event
// transformation on the import side. Used by tests to verify resumability
// and idempotency of the CDC apply path.
func injectImportCDCTransformFailure(exportDir string) error {
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
func injectImportCDCNonRetryableBatchDBError(exportDir string) error {
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

// injectUniqueKeyConflictDetected supports two test modes via GO_FAILPOINTS:
//   - return(true): crash on first conflict (false-positive validation)
//   - return("count"): deduped conflict counting in unique-key-conflict-stats.json
func injectUniqueKeyConflictDetectedFailpoint(exportDir string, cachedEvent, incomingEvent *tgtdb.Event) {
	failpoint.Inject("uniqueKeyConflictDetected", func(val failpoint.Value) {
		if val == nil {
			return
		}
		if mode, ok := val.(string); ok && mode == "count" {
			recordUniqueKeyConflictCount(exportDir, cachedEvent, incomingEvent)
			return
		}
		writeFailpointMarker(exportDir, "failpoint-unique-key-conflict-detected.log")
		utils.ErrExit("failpoint: unexpected unique key conflict detected")
	})
}

const uniqueKeyConflictStatsFileName = "unique-key-conflict-stats.json"

// UniqueKeyConflictStats is written to <exportDir>/failpoints/unique-key-conflict-stats.json.
type UniqueKeyConflictStats struct {
	Total   int            `json:"total"`
	ByTable map[string]int `json:"by_table"`
}

var (
	ukConflictStatsMu sync.Mutex
	ukConflictStats   UniqueKeyConflictStats
	ukConflictSeen    map[string]struct{}
)

func initUniqueKeyConflictStatsLocked() {
	if ukConflictSeen == nil {
		ukConflictSeen = make(map[string]struct{})
	}
	if ukConflictStats.ByTable == nil {
		ukConflictStats.ByTable = make(map[string]int)
	}
}

func uniqueKeyConflictPairKey(table string, cachedVsn, incomingVsn int64) string {
	vsn1, vsn2 := cachedVsn, incomingVsn
	if vsn1 > vsn2 {
		vsn1, vsn2 = vsn2, vsn1
	}
	return fmt.Sprintf("%s:%d:%d", table, vsn1, vsn2)
}

func recordUniqueKeyConflictCount(exportDir string, cachedEvent, incomingEvent *tgtdb.Event) {
	table := cachedEvent.TableNameTup.ForKey()
	pairKey := uniqueKeyConflictPairKey(table, cachedEvent.Vsn, incomingEvent.Vsn)

	ukConflictStatsMu.Lock()
	defer ukConflictStatsMu.Unlock()

	initUniqueKeyConflictStatsLocked()
	if _, seen := ukConflictSeen[pairKey]; seen {
		return
	}
	ukConflictSeen[pairKey] = struct{}{}
	ukConflictStats.Total++
	ukConflictStats.ByTable[table]++
	_ = writeUniqueKeyConflictStatsLocked(exportDir)
}

func writeUniqueKeyConflictStatsLocked(exportDir string) error {
	if exportDir == "" {
		return nil
	}
	failpointsDir := filepath.Join(exportDir, "failpoints")
	if err := os.MkdirAll(failpointsDir, 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(ukConflictStats, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(failpointsDir, uniqueKeyConflictStatsFileName), payload, 0644)
}

// writeFailpointMarker mirrors cmd's helper of the same name for the injectors that
// moved here; the marker files live under <exportDir>/failpoints as before.
func writeFailpointMarker(exportDir string, filename string) {
	_ = os.MkdirAll(filepath.Join(exportDir, "failpoints"), 0755)
	_ = os.WriteFile(filepath.Join(exportDir, "failpoints", filename), []byte("hit\n"), 0644)
}
