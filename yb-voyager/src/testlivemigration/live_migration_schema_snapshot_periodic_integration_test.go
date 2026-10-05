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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestLiveExportDataCapturesPeriodicSchemaSnapshot proves that the periodic
// schema-snapshot capture ticker fires during a REAL snapshot-and-changes (live)
// export from a PostgreSQL source. This exercises the production wiring at
// exportDataDebezium.go (startPeriodicSourceSchemaSnapshotCapture) end-to-end;
// the offline command test and the function-level integration test do not reach
// the streaming call site.
//
// No target/import is involved: a snapshot-and-changes export streams to the
// export dir on its own, so a source container is all that's needed.
//
// Timing: --schema-snapshot-capture-interval is in MINUTES with a floor of 1,
// and the ticker fires only after a full interval (there is no immediate tick —
// the export-data start snapshot covers t=0). So the export is held open
// (streaming, idle) until the first periodic snapshot lands ~60s after the
// ticker starts; we poll with generous margin for container/Debezium startup.
// One tick is asserted to bound runtime — the per-tick "persist on every tick,
// no dedup" behavior is covered by the faster function-level integration test.
func TestLiveExportDataCapturesPeriodicSchemaSnapshot(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test1",
		},
		// No TargetDB: this test only needs the source export streaming.
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.orders (id SERIAL PRIMARY KEY, amount NUMERIC);
			CREATE TABLE test_schema.customers (id SERIAL PRIMARY KEY, name TEXT);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.orders REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.customers REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// export data skips empty tables entirely; seed a row in each so both
			// are part of the export and the streaming phase actually starts.
			`INSERT INTO test_schema.orders (amount) VALUES (100);`,
			`INSERT INTO test_schema.customers (name) VALUES ('alice');`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	// interval=1 (minute, the floor) and capture explicitly enabled — it defaults
	// off until detect-drift ships, so it must be turned on for this test.
	err = lm.StartExportData(true, map[string]string{
		"--schema-snapshot-capture-interval": "1",
		"--disable-schema-snapshot-capture":  "false",
	})
	testutils.FatalIfError(t, err, "failed to start export data")

	// The export has created the metaDB by now (StartExportData waits on startup);
	// open a read handle to it.
	err = lm.InitMetaDB()
	testutils.FatalIfError(t, err, "failed to initialize meta db")

	countPeriodic := func() int {
		headers, err := schemasnapshot.ListSnapshots(lm.GetMetaDB())
		testutils.FatalIfError(t, err, "failed to list snapshots")
		n := 0
		for _, h := range headers {
			if h.Label == schemasnapshot.LabelExportDataFromSourcePeriodic {
				n++
			}
		}
		return n
	}

	// First tick lands ~60s after the ticker starts; allow generous margin for
	// container + Debezium startup before the export's ticker even begins. The
	// loop breaks as soon as a periodic snapshot appears, so this ceiling only
	// applies on failure — on the happy path the test finishes in ~90s.
	const pollTimeout = 4 * time.Minute
	const pollInterval = 5 * time.Second
	deadline := time.Now().Add(pollTimeout)
	periodic := 0
	for time.Now().Before(deadline) {
		if periodic = countPeriodic(); periodic >= 1 {
			break
		}
		time.Sleep(pollInterval)
	}

	// Tear the export down before asserting so the process is stopped even on
	// failure (Cleanup would also catch it, but stop explicitly and promptly).
	if stopErr := lm.StopExportData(); stopErr != nil {
		t.Logf("WARNING: failed to stop export data: %v", stopErr)
	}

	assert.GreaterOrEqual(t, periodic, 1,
		"expected at least one periodic schema snapshot (label %q) to be captured during a live snapshot-and-changes export within %s",
		schemasnapshot.LabelExportDataFromSourcePeriodic, pollTimeout)

	// The ticker must be started EXACTLY once for the whole export. It used to be
	// started in both exportDataOffline and debeziumExportData, so PG
	// snapshot-and-changes ran two tickers concurrently and recorded two periodic
	// snapshots per interval. Asserted from the log rather than by counting rows: the
	// two tickers are offset by however long the snapshot phase takes, so a row-rate
	// check is timing-dependent, while the start count is exact.
	logPath := filepath.Join(lm.GetCurrentExportDir(), "logs", "yb-voyager-export-data.log")
	logBytes, err := os.ReadFile(logPath)
	if assert.NoError(t, err, "failed to read export-data log at %s", logPath) {
		starts := strings.Count(string(logBytes), "starting periodic schema-snapshot capture every")
		assert.Equal(t, 1, starts,
			"periodic schema-snapshot capture must be started exactly once per export; %d starts means concurrent tickers", starts)
	}
}

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
	testutils.FatalIfError(t, lm.StartExportData(true, nil), "failed to start export data")

	// Set right after the exporter sees the switch to streaming, before it next
	// checks whether Debezium is still running.
	require.Eventually(t, func() bool {
		started := false
		_ = lm.WithMetaDB(0, func(m *metadb.MetaDB) error {
			msr, err := m.GetMigrationStatusRecord()
			if err == nil && msr != nil {
				started = msr.ExportDataFromSourceStarted
			}
			return nil
		})
		return started
	}, 4*time.Minute, 2*time.Second, "export data never started streaming changes")
	return lm
}

// TestLiveExportDataChecksSchemaDriftWhenDebeziumDiesWhileStreaming pins the
// drift check on export failure: a column added mid-stream, then Debezium dying,
// leaves the drift summary after "Export of data failed!" and its own report.
// Debezium is SIGKILLed, so it exits with neither 130 nor 143 and counts as a
// failure rather than an interrupt.
func TestLiveExportDataChecksSchemaDriftWhenDebeziumDiesWhileStreaming(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_failure")

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")
	lm.KillDebezium(cmd.SOURCE_DB_EXPORTER_ROLE)
	require.Error(t, lm.WaitForExportDataExitTimeout(3*time.Minute), "export data must fail once Debezium dies")

	stdout := lm.GetExportCommandStdout()
	failedAt := strings.Index(stdout, "Export of data failed!")
	require.GreaterOrEqual(t, failedAt, 0, "export output: %s", stdout)
	afterFailure := stdout[failedAt:]
	assert.Contains(t, afterFailure, "Checking the source schema for drift...")
	assert.Contains(t, afterFailure, "Changes detected  : 1")

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

// TestLiveExportDataSkipsSchemaDriftCheckWhenStopped pins that a stop is never
// reported as drift, even with drift on the source. StopExportData sends
// SIGTERM, as end migration does, and Voyager then stops Debezium.
func TestLiveExportDataSkipsSchemaDriftCheckWhenStopped(t *testing.T) {
	t.Parallel()
	lm := startStreamingExportForDriftCheck(t, "drift_on_stop")

	testutils.FatalIfError(t, lm.ExecuteOnSource(`ALTER TABLE test_schema.orders ADD COLUMN note TEXT;`), "failed to add a column")
	require.NoError(t, lm.StopExportData())

	assert.NotContains(t, lm.GetExportCommandStdout(), "Checking the source schema for drift")
	assert.NoFileExists(t, filepath.Join(lm.GetCurrentExportDir(), "reports", exportDataDriftReport))
}
