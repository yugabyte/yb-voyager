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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestExportFromTargetStartupFailureAndCutoverResume verifies that live migration cutover
// is only marked as complete once `export-data-from-target` starts up properly, and that
// a startup failure can be recovered by resuming.
//
// Scenario (end-to-end fall-forward migration with CDC events):
//  1. PG source -> `export data` (snapshot-and-changes) with 20 snapshot rows.
//  2. YB target <- `import data` (with failpoint env for the post-cutover exec).
//  3. Wait for snapshot import to YB target (20 rows).
//  4. Insert 5 CDC rows on PG source -> wait for forward streaming to YB target (25 rows).
//  5. PG source-replica <- `import data` to source-replica (sets FallForwardEnabled).
//  6. Initiate cutover to target.
//  7. After cutover: import exec's into `export-from-target` -> failpoint fires -> crash.
//  8. Verify cutover is NOT complete.
//  9. Resume `export-data-from-target` without failpoint.
//  10. Insert 5 rows on YB target -> wait for fall-forward streaming to PG source-replica.
//  11. Verify cutover IS complete.
//
// This test validates:
// - Cutover completeness gate: cutover is not marked complete if export-from-target fails to start
// - Resume of export-data-from-target correctly completes the cutover
// - Fall-forward streaming works after recovery
//
// Injection point:
//   - Go failpoint in `cmd/exportDataFromTarget.go` at `exportFromTargetStartupError`,
//     triggered via GO_FAILPOINTS env var passed through import data's post-cutover exec.
func TestExportFromTargetStartupFailureAndCutoverResume(t *testing.T) {
	tableName := "test_schema_ff.ff_cutover_test"
	ctx := context.Background()

	createSchemaSQL := []string{
		"DROP SCHEMA IF EXISTS test_schema_ff CASCADE;",
		"CREATE SCHEMA test_schema_ff;",
		`CREATE TABLE test_schema_ff.ff_cutover_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_ff.ff_cutover_test REPLICA IDENTITY FULL;`,
	}

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:                    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:                    ContainerConfig{Type: "yugabytedb", DatabaseName: "yugabyte"},
		SourceReplicaDB:             ContainerConfig{Type: "postgresql", DatabaseName: "postgres"},
		SchemaNames:                 []string{"test_schema_ff"},
		SchemaSQL:                   createSchemaSQL,
		SourceReplicaSetupSchemaSQL: createSchemaSQL,
		InitialDataSQL: []string{
			`INSERT INTO test_schema_ff.ff_cutover_test (name, value)
			SELECT 'initial_' || i, i * 10 FROM generate_series(1, 20) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_ff.ff_cutover_test (name, value)
			SELECT 'cdc_forward_' || i, 1000 + i FROM generate_series(1, 5) i;`,
		},
		TargetDeltaSQL: []string{
			`INSERT INTO test_schema_ff.ff_cutover_test (name, value)
			SELECT 'cdc_fallforward_' || i, 2000 + i FROM generate_series(1, 5) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_ff CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	// --- Step 1: Start export data from PG source (snapshot-and-changes, async) ---

	err := lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export data")

	// --- Step 2: Start import data to YB target (async, with failpoint env) ---

	failpointEnv := testutils.GetFailpointEnvVar(
		"github.com/yugabyte/yb-voyager/yb-voyager/cmd/exportFromTargetStartupError=1*return()",
	)
	err = lm.StartImportDataWithEnv(true, nil, []string{failpointEnv})
	require.NoError(t, err, "Failed to start import data")

	// --- Wait for snapshot, then forward CDC ---

	err = lm.WaitForSnapshotComplete(map[string]int64{
		reportTableName(tableName): 20,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")

	lm.ExecuteSourceDelta()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 5},
	}, 60, 3)
	require.NoError(t, err, "forward streaming did not complete")

	// --- Step 3: Start import data to source-replica (sets FallForwardEnabled) ---

	err = lm.StartImportDataToSourceReplica(true, nil)
	require.NoError(t, err, "Failed to start import data to source-replica")

	err = lm.WaitForFallForwardEnabled(60)
	require.NoError(t, err, "FallForwardEnabled should be set by import-to-source-replica")

	// --- Step 4: Initiate cutover to target ---

	require.NoError(t, lm.InitiateCutoverToTarget(false, nil), "Failed to initiate cutover")

	// Export detects cutover and shuts down gracefully (exit code 0).
	require.NoError(t, lm.WaitForExportDataExit(), "Export should exit cleanly after processing cutover")

	// Import processes cutover, then exec's into export-data-from-target.
	// The exec'd process inherits GO_FAILPOINTS and crashes before setting the flag.
	failMarkerPath := filepath.Join(lm.GetCurrentExportDir(), "logs", "failpoint-export-from-target-startup.log")
	err = lm.WaitForImportFailpointAndProcessCrash(t, failMarkerPath, 120*time.Second, 60*time.Second)
	require.NoError(t, err, "Export-from-target should crash via failpoint")

	// --- Step 5: Verify cutover is NOT complete ---

	require.NoError(t, lm.AssertCutoverIsNotComplete())

	// --- Step 6: Resume export-data-from-target WITHOUT failpoint ---

	ybConfig := lm.GetTargetContainer().GetConfig()
	lm.WithEnv(fmt.Sprintf("TARGET_DB_PASSWORD=%s", ybConfig.Password))
	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "Failed to start resumed export-data-from-target")

	// --- Step 7: Wait for export-from-target to start and set the flag ---

	err = lm.WaitForExportFromTargetStarted(120)
	require.NoError(t, err, "ExportFromTargetFallForwardStarted should become true after successful startup")

	// --- Step 8: Verify cutover IS complete ---

	require.NoError(t, lm.AssertCutoverIsComplete())

	// --- Step 9: Fall-forward CDC: Insert 5 rows on YB target, verify on source-replica ---

	lm.ExecuteTargetDelta()

	err = lm.WaitForFallForwardStreamingComplete([]string{tableName}, 120, 3)
	require.NoError(t, err, "fall-forward streaming did not complete")
}

// TestFallForwardCDCStreamingFailureAndResume verifies that export-data-from-target
// can recover from a Byteman-injected failure during active fall-forward CDC streaming,
// and that after resume, CDC events from the target are correctly streamed to the
// source-replica.
//
// Scenario (end-to-end fall-forward migration with mid-stream crash):
//  1. PG source -> `export data` (snapshot-and-changes) with 20 snapshot rows.
//  2. Write Byteman rule: crash on 1st streaming batch (before-batch-streaming).
//  3. YB target <- `import data` with DEBEZIUM_OPTS (inherited by export-from-target).
//  4. Wait for snapshot import (20 rows), insert 5 CDC rows, wait for forward streaming.
//  5. PG source-replica <- `import data` to source-replica (sets FallForwardEnabled).
//  6. Initiate cutover to target.
//  7. Export exits gracefully, import exec's into export-from-target with Byteman.
//  8. Wait for export-from-target startup (cutover complete).
//  9. Insert 5 rows on YB target -> triggers CDC batch -> Byteman crashes Debezium.
//  10. Wait for crash.
//  11. Resume export-data-from-target WITHOUT Byteman.
//  12. Insert 5 more rows on YB target -> wait for fall-forward streaming to source-replica.
//
// This test validates:
// - export-data-from-target can recover from a mid-stream Debezium crash
// - fall-forward streaming resumes correctly after crash recovery
// - data inserted before and after crash eventually reaches the source-replica
//
// Injection point:
//   - Byteman rule on `before-batch-streaming` in YbExporterConsumer (via DEBEZIUM_OPTS
//     inherited by the export-from-target process from import data's exec).
func TestFallForwardCDCStreamingFailureAndResume(t *testing.T) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Skip("Skipping test: BYTEMAN_JAR environment variable not set. Install Byteman to run this test.")
	}

	tableName := "test_schema_ff.ff_stream_test"
	ctx := context.Background()

	createSchemaSQL := []string{
		"DROP SCHEMA IF EXISTS test_schema_ff CASCADE;",
		"CREATE SCHEMA test_schema_ff;",
		`CREATE TABLE test_schema_ff.ff_stream_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_ff.ff_stream_test REPLICA IDENTITY FULL;`,
	}

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:                    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "postgres"},
		TargetDB:                    ContainerConfig{Type: "yugabytedb", DatabaseName: "yugabyte"},
		SourceReplicaDB:             ContainerConfig{Type: "postgresql", DatabaseName: "postgres"},
		SchemaNames:                 []string{"test_schema_ff"},
		SchemaSQL:                   createSchemaSQL,
		SourceReplicaSetupSchemaSQL: createSchemaSQL,
		InitialDataSQL: []string{
			`INSERT INTO test_schema_ff.ff_stream_test (name, value)
			SELECT 'initial_' || i, i * 10 FROM generate_series(1, 20) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema_ff.ff_stream_test (name, value)
			SELECT 'cdc_forward_' || i, 1000 + i FROM generate_series(1, 5) i;`,
		},
		TargetDeltaSQL: []string{
			`INSERT INTO test_schema_ff.ff_stream_test (name, value)
			SELECT 'cdc_fallforward_' || i, 2000 + i FROM generate_series(1, 5) i;`,
		},
		CleanupSQL: []string{"DROP SCHEMA IF EXISTS test_schema_ff CASCADE;"},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(ctx))
	require.NoError(t, lm.SetupSchema())

	exportDir := lm.GetCurrentExportDir()

	// --- Step 1: Start export data from PG source (no Byteman) ---

	err := lm.StartExportData(true, nil)
	require.NoError(t, err, "Failed to start export data")

	// --- Step 2: Set up Byteman to crash the 1st streaming batch on the target exporter ---
	// The rule file is written to exportDir; DEBEZIUM_OPTS referencing it will be
	// passed to the import process and inherited when import exec's into export-from-target.

	bytemanHelper, err := testutils.NewBytemanHelper(exportDir)
	require.NoError(t, err, "Failed to create Byteman helper")

	bytemanHelper.AddRuleFromBuilder(
		testutils.NewRule("fail_ff_stream_batch").
			AtMarker(testutils.MarkerCDC, "before-batch-streaming").
			If(`incrementCounter("ff_batch") == 1`).
			ThrowException("java.lang.RuntimeException", "TEST: Simulated fall-forward batch streaming failure"),
	)
	require.NoError(t, bytemanHelper.WriteRules())

	// --- Step 3: Start import data to YB target with Byteman DEBEZIUM_OPTS ---
	// The source exporter (started above) is unaffected — only the exec'd export-from-target
	// Debezium will load the Byteman agent.

	err = lm.StartImportDataWithEnv(true, nil, bytemanHelper.GetEnv())
	require.NoError(t, err, "Failed to start import data")

	// --- Step 4: Wait for snapshot, then forward CDC ---

	err = lm.WaitForSnapshotComplete(map[string]int64{
		reportTableName(tableName): 20,
	}, 120)
	require.NoError(t, err, "snapshot phase did not complete")

	lm.ExecuteSourceDelta()

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		reportTableName(tableName): {Inserts: 5},
	}, 60, 3)
	require.NoError(t, err, "forward streaming did not complete")

	// --- Step 5: Start import data to source-replica (sets FallForwardEnabled) ---

	err = lm.StartImportDataToSourceReplica(true, nil)
	require.NoError(t, err, "Failed to start import data to source-replica")

	err = lm.WaitForFallForwardEnabled(60)
	require.NoError(t, err, "FallForwardEnabled should be set by import-to-source-replica")

	// --- Step 6: Initiate cutover to target ---

	require.NoError(t, lm.InitiateCutoverToTarget(false, nil), "Failed to initiate cutover")

	require.NoError(t, lm.WaitForExportDataExit(), "Export should exit cleanly after cutover")

	// --- Step 7: Wait for export-from-target to start and complete cutover ---
	// Import exec's into export-from-target. Debezium starts with Byteman loaded but
	// the startup path succeeds (Byteman only fires on before-batch-streaming, not at startup).

	err = lm.WaitForExportFromTargetStarted(120)
	require.NoError(t, err, "Export-from-target should start successfully before Byteman fires")

	require.NoError(t, lm.AssertCutoverIsComplete())

	// --- Step 8: Insert rows on target -> triggers streaming batch -> Byteman crashes ---

	lm.ExecuteTargetDelta()

	matched, err := bytemanHelper.WaitForInjection(">>> BYTEMAN: fail_ff_stream_batch", 90*time.Second)
	require.NoError(t, err, "Should be able to read debezium logs")
	require.True(t, matched, "Byteman injection should have fired on the fall-forward batch")

	// The import process was replaced by export-from-target via exec.
	// When Debezium crashes, the export-from-target process exits with an error.
	err = lm.WaitForImportDataExit()
	require.Error(t, err, "Export-from-target should crash after Byteman injection")

	// --- Step 9: Resume export-data-from-target WITHOUT Byteman ---

	lm.ClearEnv()
	ybConfig := lm.GetTargetContainer().GetConfig()
	lm.WithEnv(fmt.Sprintf("TARGET_DB_PASSWORD=%s", ybConfig.Password))
	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "Failed to start resumed export-data-from-target")

	// --- Step 10: Insert more rows on target, wait for fall-forward streaming ---

	lm.ExecuteOnTarget(
		`INSERT INTO test_schema_ff.ff_stream_test (name, value)
		SELECT 'cdc_ff_resume_' || i, 3000 + i FROM generate_series(1, 5) i;`,
	)

	err = lm.WaitForFallForwardStreamingComplete([]string{tableName}, 120, 3)
	require.NoError(t, err, "fall-forward streaming did not complete after resume")
}
