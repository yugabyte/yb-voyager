//go:build integration_live_migration

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
package testlivemigration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const exportDataDriftReport = "drift_analysis_report_export_data.json"

// startStreamingExportForDriftCheck starts a live export and returns once the
// source exporter has recorded that Debezium is streaming changes. Parallel
// tests share the source container, so each passes its own database.
func startStreamingExportForDriftCheck(t *testing.T, databaseName string) *LiveMigrationTest {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: databaseName,
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (id SERIAL PRIMARY KEY, amount NUMERIC);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;`,
		},
		// export data skips empty tables, so seed one row for streaming to start.
		InitialDataSQL: []string{
			`INSERT INTO test_schema.orders (amount) VALUES (100);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	t.Cleanup(lm.Cleanup)

	testutils.FatalIfError(t, lm.SetupContainers(context.Background()), "failed to setup containers")
	testutils.FatalIfError(t, lm.SetupSchema(), "failed to setup schema")
	// The check follows capture, so pin it on rather than rely on its default.
	testutils.FatalIfError(t, lm.StartExportData(true, map[string]string{
		"--disable-schema-snapshot-capture": "false",
	}), "failed to start export data")

	// Set right after the exporter sees the switch to streaming, before it next
	// checks whether Debezium is still running.
	require.Eventually(t, func() bool {
		started := false
		err := lm.WithMetaDB(0, func(m *metadb.MetaDB) error {
			msr, err := m.GetMigrationStatusRecord()
			if err != nil {
				return err
			}
			if msr != nil {
				started = msr.ExportDataFromSourceStarted
			}
			return nil
		})
		if err != nil {
			t.Logf("reading the migration status record: %v", err)
		}
		return started
	}, 4*time.Minute, 2*time.Second, "export data never started streaming changes")
	return lm
}

// assertNoDriftCheck pins that export data neither ran the check nor wrote its report.
func assertNoDriftCheck(t *testing.T, lm *LiveMigrationTest) {
	assert.NotContains(t, lm.GetExportCommandStdout(), "Checking the source schema for drift")
	assert.NoFileExists(t, filepath.Join(lm.GetCurrentExportDir(), "reports", exportDataDriftReport))
}

// TestLiveExportDataChecksSchemaDriftWhenDebeziumDiesWhileStreaming pins the
// drift check on export failure: a column added mid-stream, then Debezium dying,
// leaves the drift summary after "Export of data failed!" and its own report,
// and the export still exits 1. Debezium is SIGKILLed, so it exits with neither
// 130 nor 143 and counts as a failure rather than an interrupt.
func TestLiveExportDataChecksSchemaDriftWhenDebeziumDiesWhileStreaming(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_failure")

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")
	lm.KillDebezium(cmd.SOURCE_DB_EXPORTER_ROLE)
	require.Error(t, lm.WaitForExportDataExitTimeout(3*time.Minute), "export data must fail once Debezium dies")
	assert.Equal(t, testutils.ExitCode(1), lm.exportCmd.ExitCode(), "the check must not change the export's exit code")

	stdout := lm.GetExportCommandStdout()
	failedAt := strings.Index(stdout, "Export of data failed!")
	require.GreaterOrEqual(t, failedAt, 0, "export output: %s", stdout)
	afterFailure := stdout[failedAt:]
	checkAt := strings.Index(afterFailure, "Checking the source schema for drift...")
	require.GreaterOrEqual(t, checkAt, 0, "export output: %s", stdout)
	summary := afterFailure[checkAt:]
	assert.Contains(t, summary, "Changes detected  : 1")
	assert.Contains(t, summary, "drift_analysis_report_export_data.html")
	assert.Contains(t, summary, exportDataDriftReport)

	reportsDir := filepath.Join(lm.GetCurrentExportDir(), "reports")
	raw, err := os.ReadFile(filepath.Join(reportsDir, exportDataDriftReport))
	require.NoError(t, err)
	var report schemadrift.Report
	require.NoError(t, json.Unmarshal(raw, &report))
	assert.Equal(t, 1, report.Summary.ChangeCount)
	assert.False(t, report.Summary.LiveCompared, "the check takes no live read")
	require.Len(t, report.Drifts, 1)
	assert.Equal(t, schemadiff.ColumnAdded, report.Drifts[0].Type)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: "test_schema", Name: "orders"}, report.Drifts[0].Object)
	assert.Equal(t, "note", report.Drifts[0].SubObject)
	assert.NoFileExists(t, filepath.Join(reportsDir, cmd.DRIFT_REPORT_FILE_NAME+".json"),
		"the check must leave the schema detect-drift report alone")
}

// TestLiveExportDataSkipsSchemaDriftCheckWhenStopped pins the end-to-end
// property that stopping export data is never reported as drift, even with drift
// on the source. StopExportData sends SIGTERM to Voyager, as end migration does;
// Voyager's own signal handling then ends the run before either interrupt guard
// in the drift path is reached. The guards themselves are pinned by
// TestLiveExportDataSkipsSchemaDriftCheckWhenDebeziumIsTerminated and by
// TestDebeziumFailure.
func TestLiveExportDataSkipsSchemaDriftCheckWhenStopped(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_stop")

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")
	require.NoError(t, lm.StopExportData())

	assertNoDriftCheck(t, lm)
}

// TestLiveExportDataSkipsSchemaDriftCheckWhenDebeziumIsTerminated pins the
// exit-code guard: Ctrl-C reaches Debezium too, and it can exit with 143 (or
// 130) before Voyager's own handler runs. SIGTERM to Debezium alone reproduces
// that ordering, and the export must fail without checking for drift.
func TestLiveExportDataSkipsSchemaDriftCheckWhenDebeziumIsTerminated(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_dbz_sigterm")

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")
	pidStr, err := dbzm.GetPIDOfDebeziumOnExportDir(lm.GetCurrentExportDir(), cmd.SOURCE_DB_EXPORTER_ROLE)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	require.NoError(t, err)
	require.NoError(t, syscall.Kill(pid, syscall.SIGTERM))

	require.Error(t, lm.WaitForExportDataExitTimeout(3*time.Minute), "export data must fail once Debezium is terminated")
	assert.Equal(t, testutils.ExitCode(1), lm.exportCmd.ExitCode())
	logBytes, err := os.ReadFile(filepath.Join(lm.GetCurrentExportDir(), "logs", "yb-voyager-export-data.log"))
	require.NoError(t, err)
	assert.Contains(t, string(logBytes), "exit status 143", "Debezium must have exited with Java's SIGTERM code")
	assertNoDriftCheck(t, lm)
}
