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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
		"--suppress-schema-snapshot-capture": "false",
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
}
