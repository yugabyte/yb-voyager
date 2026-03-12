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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func newCutoverResumptionTestConfig(dbName string) *TestConfig {
	return &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: dbName,
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: dbName,
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_table (
				id SERIAL PRIMARY KEY,
				name TEXT,
				value INTEGER
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_table REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_table (name, value)
			SELECT 'initial_' || i, i FROM generate_series(1, 10) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.test_table (name, value)
			SELECT md5(random()::text), 100 + i FROM generate_series(1, 5) i;`,
		},
		TargetDeltaSQL: []string{
			`INSERT INTO test_schema.test_table (name, value)
			SELECT md5(random()::text), 200 + i FROM generate_series(1, 5) i;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	}
}

const (
	tableName    = `"test_schema"."test_table"`
	fpPkgPrefix  = "github.com/yugabyte/yb-voyager/yb-voyager/cmd/"
	markerDir    = "failpoints"
	snapshotRows = int64(10)
	deltaInserts = int64(5)
)

// setupToFallbackStreaming drives a migration from start through the fallback
// streaming phase (forward export+import → cutover-to-target → fallback
// streaming). On return both export-from-target and import-to-source are
// running and have streamed the target delta.
func setupToFallbackStreaming(t *testing.T, lm *LiveMigrationTest) {
	err := lm.SetupContainers(context.Background())
	require.NoError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	require.NoError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to start export data")

	time.Sleep(10 * time.Second)

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		tableName: snapshotRows,
	}, 60)
	require.NoError(t, err, "snapshot phase did not complete")

	err = lm.ExecuteSourceDelta()
	require.NoError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		tableName: {Inserts: deltaInserts},
	}, 60, 1)
	require.NoError(t, err, "forward streaming did not complete")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "data mismatch after forward migration")

	err = lm.InitiateCutoverToTarget(true, nil)
	require.NoError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(120)
	require.NoError(t, err, "cutover to target did not complete")

	err = lm.ExecuteTargetDelta()
	require.NoError(t, err, "failed to execute target delta")

	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		tableName: {Inserts: deltaInserts},
	}, 60, 1)
	require.NoError(t, err, "fallback streaming did not complete")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "data mismatch after fallback streaming")
}

// verifyNewIterationForward runs the new iteration through forward migration
// and a final cutover-to-target to confirm the overall migration completed.
func verifyNewIterationForward(t *testing.T, lm *LiveMigrationTest) {
	err := lm.ExecuteSourceDelta()
	require.NoError(t, err, "failed to execute source delta on new iteration")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		tableName: {Inserts: deltaInserts},
	}, 60, 1)
	require.NoError(t, err, "forward streaming did not complete on new iteration")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "data mismatch after new iteration forward migration")

	err = lm.InitiateCutoverToTarget(false, nil)
	require.NoError(t, err, "failed to initiate final cutover to target")

	err = lm.WaitForCutoverComplete(120)
	require.NoError(t, err, "final cutover to target did not complete")
}

// removeFailpointMarkers deletes the failpoints directory so stale markers
// from a previous injection do not confuse subsequent waits.
func removeFailpointMarkers(exportDir string) {
	_ = os.RemoveAll(filepath.Join(exportDir, markerDir))
}

// ---------------------------------------------------------------------------
// import-to-source crashes after markCutoverProcessed, before
// waitUntilCutoverProcessedByCorrespondingExporterForImporter.
//
// On resume, handleCutoverAlreadyProcessedForImportData detects
// CutoverToSourceProcessedBySourceImporter = true and chains to
// startExportDataFromSourceOnNextIteration → initializeNextIteration → exec.
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ImporterCrashAfterMarkProcessed(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_importer_post_mark"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	// Stop both processes before initiating cutover so we can restart them
	// with deterministic failpoint injection.
	err := lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export data from target")
	err = lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	// Restart import-to-source WITH failpoint that crashes after markCutoverProcessed.
	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "cutoverToSourceImporterPostMarkProcessed=return(true)",
	)
	err = lm.StartImportDataToSourceWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import-to-source with failpoint")

	// Restart export-from-target normally — it will process cutover and wait
	// for next iteration initialization.
	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to restart export-from-target")

	// Wait for import-to-source to hit the failpoint and crash.
	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-cutover-to-source-importer-post-mark.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash at failpoint")
	t.Log("import-to-source crashed after markCutoverProcessed — resuming")

	// Export-from-target is blocked in waitUntilNextIterationInitialized
	// because the importer never initialised the next iteration. Kill it.
	err = lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export-from-target after importer crash")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	// Resume both without failpoints.
	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to resume export-from-target")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized")

	err = lm.WaitForCutoverSourceComplete(180)
	require.NoError(t, err, "cutover-to-source did not complete")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashAfterMarkProcessed passed")
}

// ---------------------------------------------------------------------------
//  import-to-source crashes DURING initializeNextIteration (partial
// iteration state). All operations in initializeNextIteration are idempotent
// (os.MkdirAll, CreateMigrationProjectIfNotExists, setUpNextIterationMSR),
// so re-running it on resume must succeed.
//
// NOTE: This test may expose a pre-existing issue in
// resolveToActiveIterationIfRequired where the global metaDB is set to the
// (partially-created) iteration's metaDB while exportDir is reset to the
// parent. If the resume path reads cutover flags from the iteration metaDB
// (which doesn't have them), it will not detect the cutover-already-processed
// state and fall through to the normal import flow. See Phase A2/B3 in the
// failure analysis for details.
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ImporterCrashDuringInitNextIteration(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_importer_during_init"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export data from target")
	err = lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	// Restart import-to-source with failpoint that crashes during
	// initializeNextIteration (after setUpNextIterationMSR, before
	// NextIterationInitialized = true).
	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "duringInitializeNextIteration=return(true)",
	)
	err = lm.StartImportDataToSourceWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import-to-source with failpoint")

	// Export-from-target processes cutover first, then waits.
	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to start export-from-target")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-during-init-next-iteration.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash during initializeNextIteration")
	t.Log("import-to-source crashed during initializeNextIteration — resuming")

	err = lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export-from-target")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to resume export-from-target")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized after resume")

	err = lm.WaitForCutoverSourceComplete(180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashDuringInitNextIteration passed")
}

// ---------------------------------------------------------------------------
// Phase between B3 and C: import-to-source crashes AFTER
// initializeNextIteration (NextIterationInitialized = true) but BEFORE
// syscall.Exec to export-data-from-source.
//
// On resume the process detects cutover already processed, calls
// startExportDataFromSourceOnNextIteration, finds
// NextIterationInitialized = true (idempotent initializeNextIteration),
// and proceeds to exec.
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ImporterCrashAfterInitNextIteration(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_importer_after_init"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export data from target")
	err = lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "afterInitializeNextIteration=return(true)",
	)
	err = lm.StartImportDataToSourceWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import-to-source with failpoint")

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to start export-from-target")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-after-init-next-iteration.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash after initializeNextIteration")
	t.Log("import-to-source crashed after initializeNextIteration — resuming")

	err = lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export-from-target")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to resume export-from-target")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized after resume")

	err = lm.WaitForCutoverSourceComplete(180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashAfterInitNextIteration passed")
}


// ---------------------------------------------------------------------------
// import-to-source crashes BEFORE
// initializeNextIteration (after waitUntilCutoverProcessedByCorresponding
// Exporter completes). On resume initializeNextIteration runs fresh — this
// validates the happy path where no partial iteration state exists yet.
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ImporterCrashBeforeInitNextIteration(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_importer_before_init"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export data from target")
	err = lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "beforeInitializeNextIteration=return(true)",
	)
	err = lm.StartImportDataToSourceWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import-to-source with failpoint")

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to start export-from-target")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-before-init-next-iteration.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash before initializeNextIteration")
	t.Log("import-to-source crashed before initializeNextIteration — resuming")

	err = lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export-from-target")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to resume export-from-target")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized after resume")

	err = lm.WaitForCutoverSourceComplete(180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashBeforeInitNextIteration passed")
}



// ---------------------------------------------------------------------------
// export-from-target crashes after markCutoverProcessed
// (CutoverToSourceProcessedByTargetExporter = true) but before
// startNextIterationImportDataToTarget.
//
// On resume, handleCutoverAlreadyProcessedForExportData detects cutover
// already processed, getCutoverToSourceStatus returns INITIATED (not
// COMPLETED because the next iteration hasn't fully started), and the
// exporter chains to waitUntilNextIterationInitialized → exec.
//
// Meanwhile, import-to-source (running normally) processes cutover,
// initializes the next iteration, and execs to export-data-from-source.
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ExporterCrashAfterMarkProcessed(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_exporter_post_mark"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export data from target")
	err = lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")
	lm.KillDebezium(TARGET_DB_EXPORTER_FB_ROLE)

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	// Restart export-from-target WITH failpoint: crashes after marking
	// cutover processed but before chaining to the next iteration.
	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "cutoverToSourceExporterPostMarkProcessed=return(true)",
	)
	err = lm.StartExportDataFromTargetWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start export-from-target with failpoint")

	// Import-to-source runs normally.
	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to start import-to-source")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-cutover-to-source-exporter-post-mark.log")
	err = lm.WaitForExportFromTargetFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "export-from-target did not crash at failpoint")
	t.Log("export-from-target crashed after markCutoverProcessed — resuming")

	// Import-to-source should have processed cutover and initialized the
	// next iteration (since the exporter already marked processed before
	// crashing, the importer's waitUntilCutoverProcessedByCorrespondingExporter
	// will succeed). Give it time, then check iteration init.
	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized by import-to-source")
	t.Log("import-to-source initialized the next iteration while exporter was down")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	// Resume export-from-target. It will detect cutover already processed,
	// find NextIterationInitialized = true, and exec into import-data-to-target.
	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to resume export-from-target")

	err = lm.WaitForCutoverSourceComplete(180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ExporterCrashAfterMarkProcessed passed")
}
