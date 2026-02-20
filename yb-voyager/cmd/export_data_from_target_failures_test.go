//go:build failpoint

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
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const tableName = "test_schema_ff.ff_cutover_test"

// TestExportFromTargetStartupFailureAndCutoverResume verifies that cutover is only
// marked as complete once export-data-from-target starts up properly.
//
// Flow (real end-to-end fall-forward migration with CDC events):
//  1. PG source → export data (snapshot-and-changes) — 20 snapshot rows
//  2. YB target ← import data (with failpoint env for the post-cutover exec)
//  3. Wait for snapshot import to YB target (20 rows)
//  4. Insert 5 CDC rows on PG source → wait for forward streaming to YB target (25 rows)
//  5. PG source-replica ← import data to source-replica (sets FallForwardEnabled)
//  6. Initiate cutover to target
//  7. After cutover: import exec's into export-from-target → failpoint fires → crash
//  8. Verify cutover is NOT complete
//  9. Resume export-data-from-target without failpoint
//  10. Insert 5 rows on YB target → wait for fall-forward streaming to PG source-replica
//  11. Verify cutover IS complete
func TestExportFromTargetStartupFailureAndCutoverResume(t *testing.T) {
	ctx := context.Background()

	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// --- Container setup ---

	pgSourceContainer := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
		ForLive: true,
	})
	err := pgSourceContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PG source container")
	defer pgSourceContainer.Stop(ctx)

	ybTargetContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = ybTargetContainer.Start(ctx)
	require.NoError(t, err, "Failed to start YB target container")
	defer ybTargetContainer.Stop(ctx)

	pgSourceReplicaContainer := testcontainers.NewTestContainer("postgresql", nil)
	err = pgSourceReplicaContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PG source-replica container")
	defer pgSourceReplicaContainer.Stop(ctx)

	// --- Schema and data setup (20 initial rows in PG source, empty YB + source-replica) ---

	setupExportFromTargetTestData(t, pgSourceContainer, ybTargetContainer, pgSourceReplicaContainer)
	defer pgSourceContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_ff CASCADE;")
	defer ybTargetContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_ff CASCADE;")
	defer pgSourceReplicaContainer.ExecuteSqls("DROP SCHEMA IF EXISTS test_schema_ff CASCADE;")

	// --- Step 1: Start export data from PG source (snapshot-and-changes, async) ---

	logTest(t, "Starting export data from PG source (snapshot-and-changes)...")
	waitForExportInit := func() {
		time.Sleep(5 * time.Second)
	}
	exportRunner := testutils.NewVoyagerCommandRunner(pgSourceContainer, "export data", []string{
		"--export-dir", exportDir,
		"--export-type", "snapshot-and-changes",
		"--source-db-schema", "test_schema_ff",
		"--disable-pb", "true",
		"--yes",
	}, waitForExportInit, true)
	err = exportRunner.Run()
	require.NoError(t, err, "Failed to start export data")
	defer exportRunner.Kill()

	// --- Step 2: Start import data to YB target (async, with failpoint env) ---

	logTest(t, "Starting import data to YB target (with failpoint env for export-from-target)...")
	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/exportFromTargetStartupError=1*return()",
	)
	waitForImportInit := func() {
		time.Sleep(5 * time.Second)
	}
	importRunner := testutils.NewVoyagerCommandRunner(ybTargetContainer, "import data", []string{
		"--export-dir", exportDir,
		"--disable-pb", "true",
		"--yes",
	}, waitForImportInit, true).WithEnv(failpointEnv)
	err = importRunner.Run()
	require.NoError(t, err, "Failed to start import data")

	// --- Wait for snapshot to be imported to YB target (20 rows) ---

	logTest(t, "Waiting for snapshot import to YB target (20 rows)...")
	snapshotDone := waitForRowCount(t, ybTargetContainer, tableName, 20, 120*time.Second, 3*time.Second)
	require.True(t, snapshotDone, "Snapshot import to YB target should complete (20 rows)")

	// --- Forward CDC: Insert 5 rows on PG source, wait for them on YB target ---

	logTest(t, "Inserting 5 CDC rows on PG source...")
	pgSourceContainer.ExecuteSqls(
		`INSERT INTO test_schema_ff.ff_cutover_test (name, value)
		SELECT 'cdc_forward_' || i, 1000 + i FROM generate_series(1, 5) i;`,
	)

	logTest(t, "Waiting for forward CDC streaming to YB target (25 rows)...")
	forwardCDCDone := waitForRowCount(t, ybTargetContainer, tableName, 25, 60*time.Second, 3*time.Second)
	require.True(t, forwardCDCDone, "Forward CDC rows should appear on YB target")
	logTest(t, "Forward CDC streaming verified: 25 rows on YB target")

	// --- Step 3: Start import data to source-replica (sets FallForwardEnabled) ---

	logTest(t, "Starting import data to source-replica (sets FallForwardEnabled)...")
	srConfig := pgSourceReplicaContainer.GetConfig()
	srHost, srPort, err := pgSourceReplicaContainer.GetHostPort()
	require.NoError(t, err, "Failed to get source-replica host:port")

	importToSRRunner := testutils.NewVoyagerCommandRunner(nil, "import data to source-replica", []string{
		"--export-dir", exportDir,
		"--source-replica-db-host", srHost,
		"--source-replica-db-port", strconv.Itoa(srPort),
		"--source-replica-db-user", srConfig.User,
		"--source-replica-db-password", srConfig.Password,
		"--source-replica-db-name", srConfig.DBName,
		"--disable-pb", "true",
		"--start-clean", "true",
		"--yes",
	}, nil, true)
	err = importToSRRunner.Run()
	require.NoError(t, err, "Failed to start import data to source-replica")
	defer importToSRRunner.Kill()

	logTest(t, "Waiting for FallForwardEnabled to be set in metaDB...")
	ffEnabled := waitForFallForwardEnabled(t, exportDir, 60*time.Second, 2*time.Second)
	require.True(t, ffEnabled, "FallForwardEnabled should be set by import-to-source-replica")
	logTest(t, "FallForwardEnabled is now true")

	logTest(t, "Waiting for TargetDBConf to be set in metaDB by import...")
	tdbConfSet := waitForTargetDBConfInMetaDB(t, exportDir, 120*time.Second, 3*time.Second)
	require.True(t, tdbConfSet, "TargetDBConf should be set by import data")

	// --- Step 4: Initiate cutover to target ---

	logTest(t, "Initiating cutover to target...")
	initiateCutoverToTarget(t, exportDir)
	logTest(t, "Cutover initiated")

	// Export detects cutover and shuts down gracefully (exit code 0).
	logTest(t, "Waiting for export process to exit after cutover...")
	exportExitErr := exportRunner.Wait()
	require.NoError(t, exportExitErr, "Export should exit cleanly after processing cutover")

	// Import processes cutover, then exec's into export-data-from-target.
	// The exec'd process inherits GO_FAILPOINTS and crashes before setting the flag.
	logTest(t, "Waiting for import→export-from-target to crash via failpoint...")
	_, importExitErr := waitForProcessExitOrKill(importRunner, exportDir, 180*time.Second)
	require.Error(t, importExitErr, "Export-from-target should exit with error due to failpoint")

	time.Sleep(3 * time.Second)

	// --- Step 5: Verify cutover is NOT complete ---

	logTest(t, "Verifying cutover is NOT complete after failed export-from-target startup...")
	verifyCutoverIsNotComplete(t, exportDir)

	// --- Step 6: Resume export-data-from-target WITHOUT failpoint ---

	logTest(t, "Starting export-data-from-target manually (no failpoint)...")
	ybConfig := ybTargetContainer.GetConfig()
	exportFromTargetRunner := testutils.NewVoyagerCommandRunner(nil, "export data from target", []string{
		"--export-dir", exportDir,
		"--target-ssl-mode", "disable",
		"--disable-pb", "true",
		"--yes",
	}, nil, true).WithEnv(fmt.Sprintf("TARGET_DB_PASSWORD=%s", ybConfig.Password))

	err = exportFromTargetRunner.Run()
	require.NoError(t, err, "Failed to start resumed export-data-from-target")
	defer exportFromTargetRunner.Kill()

	// --- Step 7: Wait for export-from-target to start and set the flag ---

	logTest(t, "Waiting for ExportFromTargetFallForwardStarted to become true...")
	started := waitForExportFromTargetStarted(t, exportDir, 120*time.Second, 3*time.Second)
	require.True(t, started, "ExportFromTargetFallForwardStarted should become true after successful startup")

	// --- Step 8: Verify cutover IS complete ---

	logTest(t, "Verifying cutover IS complete after successful export-from-target startup...")
	verifyCutoverIsComplete(t, exportDir)

	// --- Step 9: Fall-forward CDC: Insert 5 rows on YB target, verify on source-replica ---

	logTest(t, "Inserting 5 CDC rows on YB target (post-cutover, fall-forward streaming)...")
	ybTargetContainer.ExecuteSqls(
		`INSERT INTO test_schema_ff.ff_cutover_test (name, value)
		SELECT 'cdc_fallforward_' || i, 2000 + i FROM generate_series(1, 5) i;`,
	)

	// YB target should now have 30 rows (20 snapshot + 5 forward CDC + 5 fall-forward)
	logTestf(t, "YB target row count: %d", queryRowCount(t, ybTargetContainer, tableName))

	// Wait for the 5 fall-forward CDC rows to appear on source-replica.
	// Source-replica gets: 20 snapshot + 5 forward CDC (from initial export) + 5 fall-forward.
	// But the snapshot rows on source-replica come from the PG source export, so it should have
	// at least the 5 fall-forward rows beyond what it already had.
	logTest(t, "Waiting for fall-forward CDC rows to appear on PG source-replica...")
	ffCDCDone := waitForRowCount(t, pgSourceReplicaContainer, tableName, 30, 120*time.Second, 3*time.Second)
	require.True(t, ffCDCDone, "Fall-forward CDC rows should appear on PG source-replica (30 rows)")
	logTestf(t, "Source-replica row count: %d", queryRowCount(t, pgSourceReplicaContainer, tableName))

	logTest(t, "Test passed: cutover is only COMPLETED once export-data-from-target starts properly, "+
		"and fall-forward CDC streaming works end-to-end")
}
