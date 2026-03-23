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

	err = lm.WaitForCutoverComplete(0, 120)
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
		tableName: {Inserts: deltaInserts + 5},
	}, 60, 1)
	require.NoError(t, err, "forward streaming did not complete on new iteration")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "data mismatch after new iteration forward migration")

	err = lm.InitiateCutoverToTarget(false, nil)
	require.NoError(t, err, "failed to initiate final cutover to target")

	err = lm.WaitForCutoverComplete(1, 120)
	require.NoError(t, err, "final cutover to target did not complete")
}

// removeFailpointMarkers deletes the failpoints directory so stale markers
// from a previous injection do not confuse subsequent waits.
func removeFailpointMarkers(exportDir string) {
	_ = os.RemoveAll(filepath.Join(exportDir, markerDir))
}

// setupToForwardStreaming drives a migration from start through snapshot and
// forward streaming. On return both export-data and import-data are running
// and source deltas have been streamed to target. Ready for cutover-to-target.
func setupToForwardStreaming(t *testing.T, lm *LiveMigrationTest) {
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
}

// verifyFallbackAfterCutoverToTarget verifies the migration completed the
// cutover-to-target with fallback: executes target delta, waits for fallback
// streaming, and validates data consistency.
func verifyFallbackAfterCutoverToTarget(t *testing.T, lm *LiveMigrationTest) {
	err := lm.ExecuteTargetDelta()
	require.NoError(t, err, "failed to execute target delta")

	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		tableName: {Inserts: deltaInserts},
	}, 60, 1)
	require.NoError(t, err, "fallback streaming did not complete")

	err = lm.ValidateDataConsistency([]string{tableName}, "id")
	require.NoError(t, err, "data mismatch after fallback streaming")
}

// ===========================================================================
// Cutover-to-TARGET failpoint tests

//Test plan with failure injections below
//1. export-data-from-source crashes after Debezium completes, before deletePGReplicationSlotAndPublication
//2. export-data-from-source crashes after deleting replication slot and publication, before markCutoverProcessed
//3. export-data-from-source crashes after marking cutover processed, before startFallBackSetupIfRequired
//4. import data to source crashes before marking cutover processed
//5. import data to source crashes after marking cutover processed
// ===========================================================================

// ---------------------------------------------------------------------------
// export-data-from-source crashes after Debezium completes, before
// deletePGReplicationSlotAndPublication. On resume:
// isCutoverInitiatedAndCutoverDetected returns true, skips Debezium,
// proceeds to delete slot and mark cutover processed.
// ---------------------------------------------------------------------------

func TestCutoverToTargetResumption_ExporterCrashAfterDebeziumComplete(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_ct_resumption_exporter_debezium"))
	defer lm.Cleanup()

	setupToForwardStreaming(t, lm)

	err := lm.StopExportData()
	require.NoError(t, err, "failed to stop export data")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToTarget(true, nil)
	require.NoError(t, err, "failed to initiate cutover to target")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "afterCompletingDebezium=return(true)",
	)
	err = lm.StartExportDataWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start export data with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-after-completing-debezium.log")
	err = lm.WaitForExportFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "export data did not crash after completing debezium")
	t.Log("export data crashed after completing debezium — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to resume export data")

	err = lm.WaitForCutoverComplete(0, 180)
	require.NoError(t, err, "cutover-to-target did not complete after resume")

	verifyFallbackAfterCutoverToTarget(t, lm)
	t.Log("TestCutoverToTargetResumption_ExporterCrashAfterDebeziumComplete passed")
}

// ---------------------------------------------------------------------------
// export-data-from-source crashes after deleting PG replication slot, before
// markCutoverProcessed. On resume: skips Debezium (cutover detected), slot
// already gone, proceeds to markCutoverProcessed.
// ---------------------------------------------------------------------------

func TestCutoverToTargetResumption_ExporterCrashAfterDeletingReplicationSlot(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_ct_resumption_exporter_pg_slot"))
	defer lm.Cleanup()

	setupToForwardStreaming(t, lm)

	err := lm.StopExportData()
	require.NoError(t, err, "failed to stop export data")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToTarget(true, nil)
	require.NoError(t, err, "failed to initiate cutover to target")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "afterDeletingReplicationSlotAndPublication=return(true)",
	)
	err = lm.StartExportDataWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start export data with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-after-deleting-replication-slot-and-publication.log")
	err = lm.WaitForExportFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "export data did not crash after deleting replication slot")
	t.Log("export data crashed after deleting PG replication slot — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to resume export data")

	err = lm.WaitForCutoverComplete(0, 180)
	require.NoError(t, err, "cutover-to-target did not complete after resume")

	verifyFallbackAfterCutoverToTarget(t, lm)
	t.Log("TestCutoverToTargetResumption_ExporterCrashAfterDeletingReplicationSlot passed")
}

// ---------------------------------------------------------------------------
// export-data-from-source crashes after markCutoverProcessed
// (CutoverProcessedBySourceExporter = true) but before
// startFallBackSetupIfRequired exec. On resume:
// handleCutoverAlreadyProcessedForExportData detects already-processed,
// calls startFurtherCommandsAfterCurrentExportData.

//Can fail in scenarios where export data from target is started and cutover is completed but the exporter
//failed after marking cutover processed and before starting import data to source
// ---------------------------------------------------------------------------

func TestCutoverToTargetResumption_ExporterCrashAfterMarkProcessed(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_ct_resumption_exporter_post_mark"))
	defer lm.Cleanup()

	setupToForwardStreaming(t, lm)

	err := lm.StopExportData()
	require.NoError(t, err, "failed to stop export data")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToTarget(true, nil)
	require.NoError(t, err, "failed to initiate cutover to target")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "cutoverToTargetExporterPostMarkProcessed=return(true)",
	)
	err = lm.StartExportDataWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start export data with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-cutover-to-target-exporter-post-mark.log")
	err = lm.WaitForExportFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "export data did not crash after markCutoverProcessed")
	t.Log("export data crashed after markCutoverProcessed — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to resume export data")

	err = lm.WaitForCutoverComplete(0, 180)
	require.NoError(t, err, "cutover-to-target did not complete after resume")

	verifyFallbackAfterCutoverToTarget(t, lm)
	t.Log("TestCutoverToTargetResumption_ExporterCrashAfterMarkProcessed passed")
}

// ---------------------------------------------------------------------------
// import-data-to-target crashes after sequence/identity restore but BEFORE
// markCutoverProcessed. On resume: handleCutoverAlreadyProcessedForImportData
// sees NOT processed, re-runs streamChanges + postCutoverProcessing.
// Sequence/identity restore is idempotent.
// ---------------------------------------------------------------------------

func TestCutoverToTargetResumption_ImporterCrashBeforeMarkProcessed(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_ct_resumption_importer_pre_mark"))
	defer lm.Cleanup()

	setupToForwardStreaming(t, lm)

	err := lm.StopImportData()
	require.NoError(t, err, "failed to stop import data")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToTarget(true, nil)
	require.NoError(t, err, "failed to initiate cutover to target")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "cutoverToTargetImporterPreMarkProcessed=return(true)",
	)

	err = lm.StartImportDataWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import data with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-cutover-to-target-importer-pre-mark.log")
	err = lm.WaitForImportFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import data did not crash before markCutoverProcessed")
	t.Log("import data crashed before markCutoverProcessed — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "failed to resume import data")

	err = lm.WaitForCutoverComplete0, (180)
	require.NoError(t, err, "cutover-to-target did not complete after resume")

	verifyFallbackAfterCutoverToTarget(t, lm)
	t.Log("TestCutoverToTargetResumption_ImporterCrashBeforeMarkProcessed passed")
}

// ---------------------------------------------------------------------------
// import-data-to-target crashes after markCutoverProcessed
// (CutoverProcessedByTargetImporter = true) but before
// waitUntilCutoverProcessedByCorrespondingExporterForImporter. On resume:
// handleCutoverAlreadyProcessedForImportData detects already-processed,
// calls startFurtherCommandsAfterCurrentImportData →
// startExportDataFromTargetIfRequired.
// ---------------------------------------------------------------------------

func TestCutoverToTargetResumption_ImporterCrashAfterMarkProcessed(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_ct_resumption_importer_post_mark"))
	defer lm.Cleanup()

	setupToForwardStreaming(t, lm)

	err := lm.StopImportData()
	require.NoError(t, err, "failed to stop import data")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToTarget(true, nil)
	require.NoError(t, err, "failed to initiate cutover to target")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "cutoverToTargetImporterPostMarkProcessed=return(true)",
	)

	err = lm.StartImportDataWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import data with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-cutover-to-target-importer-post-mark.log")
	err = lm.WaitForImportFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import data did not crash after markCutoverProcessed")
	t.Log("import data crashed after markCutoverProcessed — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartExportData(true, nil)
	require.NoError(t, err, "failed to resume export data")

	err = lm.StartImportData(true, nil)
	require.NoError(t, err, "failed to resume import data")

	err = lm.WaitForCutoverComplete(0, 180)
	require.NoError(t, err, "cutover-to-target did not complete after resume")

	verifyFallbackAfterCutoverToTarget(t, lm)
	t.Log("TestCutoverToTargetResumption_ImporterCrashAfterMarkProcessed passed")
}

// ===========================================================================
// Cutover-to-SOURCE failpoint tests

//Test plan with failure injections below
//1. import-to-source crashes before markCutoverProcessed
//2. import-to-source crashes after markCutoverProcessed
//3. import-to-source crashes during initializeNextIteration
//4. import-to-source crashes after initializeNextIteration
//5. import-to-source crashes before initializeNextIteration
//6. import-to-source crashes during setUpNextIterationMSR
//7. export-data-from-source crashes after markCutoverProcessed
//8. export-data-from-source crashes after deleting replication slot and publication
//9. export-data-from-source crashes after completing debezium
// ===========================================================================

// ---------------------------------------------------------------------------
// import-to-source crashes before markCutoverProcessed
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ImporterCrashBeforeMarkProcessed(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_importer_before_mark"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "cutoverToSourceImporterPreMarkProcessed=return(true)",
	)
	err = lm.StartImportDataToSourceWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import-to-source with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-cutover-to-source-importer-pre-mark.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash before markCutoverProcessed")
	t.Log("import-to-source crashed before markCutoverProcessed — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized")

	err = lm.WaitForCutoverSourceComplete(0,180)
	require.NoError(t, err, "cutover-to-source did not complete")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashBeforeMarkProcessed passed")
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

	// Stop import data to source processes before initiating cutover so we can restart them
	// with deterministic failpoint injection.
	err := lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")

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

	// Wait for import-to-source to hit the failpoint and crash.
	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-cutover-to-source-importer-post-mark.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash at failpoint")
	t.Log("import-to-source crashed after markCutoverProcessed — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	// Resume both without failpoints.
	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized")

	err = lm.WaitForCutoverSourceComplete(0,180)
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
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ImporterCrashDuringInitNextIteration(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_importer_during_init"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")

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

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-during-init-next-iteration.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash during initializeNextIteration")
	t.Log("import-to-source crashed during initializeNextIteration — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized after resume")

	err = lm.WaitForCutoverSourceComplete(0,180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashDuringInitNextIteration passed")
}

// ---------------------------------------------------------------------------
// import-to-source crashes AFTER
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

	err := lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")

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

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-after-init-next-iteration.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash after initializeNextIteration")
	t.Log("import-to-source crashed after initializeNextIteration — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized after resume")

	err = lm.WaitForCutoverSourceComplete(0,180)
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

	err := lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")

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

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-before-init-next-iteration.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash before initializeNextIteration")
	t.Log("import-to-source crashed before initializeNextIteration — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized after resume")

	err = lm.WaitForCutoverSourceComplete(0,180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashBeforeInitNextIteration passed")
}

// ---------------------------------------------------------------------------
// import-to-source crashes during setUpNextIterationMSR
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ImporterCrashDuringSetUpNextIterationMSR(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_importer_during_set_up_next_iteration_msr"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopImportDataToSource()
	require.NoError(t, err, "failed to stop import data to source")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "duringSetUpNextIterationMSR=return(true)",
	)
	err = lm.StartImportDataToSourceWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start import-to-source with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-during-set-up-next-iteration-msr.log")
	err = lm.WaitForImportToSourceFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "import-to-source did not crash during setUpNextIterationMSR")
	t.Log("import-to-source crashed during setUpNextIterationMSR — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartImportDataToSource(true, nil)
	require.NoError(t, err, "failed to resume import-to-source")

	err = lm.WaitForNextIterationInitialized(120, 0)
	require.NoError(t, err, "next iteration was not initialized after resume")

	err = lm.WaitForCutoverSourceComplete(0,180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ImporterCrashDuringSetUpNextIterationMSR passed")
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

	err = lm.WaitForCutoverSourceComplete(0,180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ExporterCrashAfterMarkProcessed passed")
}

// ---------------------------------------------------------------------------
//export data from target crashes after deleting replication slot and publication
//on resume, it should resume the migration for starting the next iteration.
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ExporterCrashAfterDeletingReplicationSlotAndPublication(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_after_exporter_yb_slot_delete"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export data from target")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "afterDeletingReplicationSlotAndPublication=return(true)",
	)
	err = lm.StartExportDataFromTargetWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start export data from target with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-after-deleting-replication-slot-and-publication.log")
	err = lm.WaitForExportFromTargetFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "export-from-target did not crash after deleting replication slot and publication")
	t.Log("export-from-target crashed after deleting replication slot and publication — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to resume export-from-target")

	err = lm.WaitForCutoverSourceComplete(0,180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ExporterCrashAfterDeletingReplicationSlotAndPublication passed")
}

// ---------------------------------------------------------------------------
//export data from target crashes after completing debezium once the cutover is initiatted
//on resume, it should resume continue with post processing and next iteration initialization.
// ---------------------------------------------------------------------------

func TestCutoverToSourceResumption_ExporterCrashAfterCompletingDebezium(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, newCutoverResumptionTestConfig(
		"test_fb_resumption_after_exporter_debezium_complete"))
	defer lm.Cleanup()

	setupToFallbackStreaming(t, lm)

	err := lm.StopExportDataFromTarget()
	require.NoError(t, err, "failed to stop export data from target")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	require.NoError(t, err, "failed to initiate cutover to source")

	fpEnv := testutils.GetFailpointEnvVar(
		fpPkgPrefix + "afterCompletingDebezium=return(true)",
	)
	err = lm.StartExportDataFromTargetWithEnv(true, nil, []string{fpEnv})
	require.NoError(t, err, "failed to start export data from target with failpoint")

	markerPath := filepath.Join(lm.GetCurrentExportDir(), markerDir,
		"failpoint-after-completing-debezium.log")
	err = lm.WaitForExportFromTargetFailpointAndProcessCrash(
		t, markerPath, 180*time.Second, 60*time.Second)
	require.NoError(t, err, "export-from-target did not crash after completing debezium")
	t.Log("export-from-target crashed after completing debezium — resuming")

	removeFailpointMarkers(lm.GetCurrentExportDir())

	err = lm.StartExportDataFromTarget(true, nil)
	require.NoError(t, err, "failed to resume export-from-target")

	err = lm.WaitForCutoverSourceComplete(0,180)
	require.NoError(t, err, "cutover-to-source did not complete after resume")

	verifyNewIterationForward(t, lm)
	t.Log("TestCutoverToSourceResumption_ExporterCrashAfterCompletingDebezium passed")
}
