//go:build integration

// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemasnapshot_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

// newSourceCapture builds a SourceCapture over the given handles. Every input is a
// field, which is the point: the capture policy is exercised here without any command
// state, so these tests live next to the code they test rather than in package cmd.
func newSourceCapture(db *sql.DB, meta schemasnapshot.DBMetadata, mdb *metadb.MetaDB, schemas ...string) schemasnapshot.SourceCapture {
	return schemasnapshot.SourceCapture{
		DB:     db,
		MetaDB: mdb,
		Params: schemasnapshot.CaptureParams{
			DatabaseType: constants.POSTGRESQL,
			DBMetadata:   meta,
			Schemas:      schemas,
		},
	}
}

// execAll runs each statement, failing the test on the first error.
func execAll(t *testing.T, db *sql.DB, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		_, err := db.Exec(s)
		require.NoError(t, err, "exec %q", s)
	}
}

func countLabel(t *testing.T, mdb *metadb.MetaDB, label string) int {
	t.Helper()
	headers, err := schemasnapshot.ListSnapshots(mdb)
	require.NoError(t, err)
	n := 0
	for _, h := range headers {
		if h.Label == label {
			n++
		}
	}
	return n
}

// TestSourceCapture exercises SourceCapture end-to-end against a REAL PostgreSQL
// database and a REAL metaDB: the capture policy and its gates, the budget at scale,
// and the periodic ticker.
func TestSourceCapture(t *testing.T) {
	// One container for every group below. Each group gets its own schema and its own
	// metaDB, but they must share the container: startCaptureTestDB keys the container
	// registry off the config, so two groups asking for the same key would have the
	// first group's cleanup terminate the container the second still needs.
	db, meta, stop := startCaptureTestDB(t, &testcontainers.ContainerConfig{})
	t.Cleanup(stop)

	t.Run("capture", func(t *testing.T) { testSourceCaptureBehaviour(t, db, meta) })
	t.Run("large schema", func(t *testing.T) { testSourceCaptureLargeSchema(t, db, meta) })
	t.Run("periodic ticker", func(t *testing.T) { testSourceCaptureStartPeriodic(t, db, meta) })
}

func testSourceCaptureBehaviour(t *testing.T, db *sql.DB, meta schemasnapshot.DBMetadata) {
	ctx := context.Background()
	const schemaName = "capture_test"

	execAll(t, db,
		`CREATE SCHEMA IF NOT EXISTS `+schemaName,
		`CREATE TABLE `+schemaName+`.orders(id int primary key, amount numeric)`,
		`CREATE TABLE `+schemaName+`.customers(id int primary key, name text)`,
	)
	t.Cleanup(func() { execAll(t, db, `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`) })

	mdb := newIntegrationTestMetaDB(t)
	sc := newSourceCapture(db, meta, mdb, schemaName)

	t.Run("happy path captures and persists a real snapshot", func(t *testing.T) {
		require.NoError(t, sc.Capture(ctx, schemasnapshot.LabelExportSchema, "", true))

		headers, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err)
		require.Len(t, headers, 1, "exactly one snapshot must have been persisted")

		h := headers[0]
		assert.Equal(t, schemasnapshot.LabelExportSchema, h.Label)
		assert.False(t, h.IsPlaceholder, "a successful capture must not be a placeholder")

		content, err := schemasnapshot.LoadSnapshotByName(mdb, h.Name())
		require.NoError(t, err)
		require.NotNil(t, content)

		tableNames := make(map[string]bool, len(content.Tables))
		for _, tb := range content.Tables {
			tableNames[tb.Name] = true
		}
		assert.True(t, tableNames["orders"], "captured content must include the seeded 'orders' table")
		assert.True(t, tableNames["customers"], "captured content must include the seeded 'customers' table")
	})

	t.Run("a disabled capture is honored and returns no error", func(t *testing.T) {
		before, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err)

		disabled := sc
		disabled.Disabled = true
		require.NoError(t, disabled.Capture(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", true),
			"a skip is not an error")

		after, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err)
		assert.Equal(t, len(before), len(after), "a disabled capture must not add a snapshot row")
	})

	t.Run("a non-PostgreSQL source is honored and returns no error", func(t *testing.T) {
		before, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err)

		other := sc
		other.Params.DatabaseType = constants.ORACLE
		require.NoError(t, other.Capture(ctx, schemasnapshot.LabelExportSchema, "", true),
			"a skip is not an error")

		after, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err)
		assert.Equal(t, len(before), len(after), "a non-PostgreSQL source must not add a snapshot row")
	})

	t.Run("RecordPlaceholder writes a metadata-only marker", func(t *testing.T) {
		before := countLabel(t, mdb, schemasnapshot.LabelExportDataFromSourceExit)

		sc.RecordPlaceholder(schemasnapshot.LabelExportDataFromSourceExit, schemasnapshot.ReasonError)

		headers, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err)

		var placeholder *schemasnapshot.SnapshotHeader
		for i := range headers {
			if headers[i].Label == schemasnapshot.LabelExportDataFromSourceExit {
				placeholder = &headers[i]
				break
			}
		}
		require.NotNil(t, placeholder, "a header with the exit label must be present")
		assert.Equal(t, before+1, countLabel(t, mdb, schemasnapshot.LabelExportDataFromSourceExit))
		assert.True(t, placeholder.IsPlaceholder, "the marker must be flagged as a placeholder")
		assert.Equal(t, schemasnapshot.ReasonError, placeholder.Reason)
	})

	t.Run("periodic capture persists on every tick, even for an unchanged schema (no dedup)", func(t *testing.T) {
		before := countLabel(t, mdb, schemasnapshot.LabelExportDataFromSourcePeriodic)

		// Every periodic capture is persisted unconditionally — no dedup — so the drift
		// timeline records the source schema at each interval even when it hasn't changed.
		// The schema is NOT altered between these captures, so under the old dedup logic the
		// 2nd and 3rd would have been skipped; here all three must persist.
		//
		// Snapshot names are second-granularity ({label}_{YYYYMMDDThhmmssZ}); this test fires
		// captures back-to-back, so a >=1s wait between them avoids a UNIQUE-name collision.
		// Not a real-run concern: periodic captures are >=1 minute apart.
		const captures = 3
		for i := 0; i < captures; i++ {
			if i > 0 {
				time.Sleep(time.Second)
			}
			require.NoError(t, sc.Capture(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false))
		}

		assert.Equal(t, before+captures, countLabel(t, mdb, schemasnapshot.LabelExportDataFromSourcePeriodic),
			"every periodic capture must persist a new snapshot, even with an unchanged schema")
	})

	t.Run("an expired context aborts the capture fast and falls back to a placeholder", func(t *testing.T) {
		countExitPlaceholders := func() int {
			headers, err := schemasnapshot.ListSnapshots(mdb)
			require.NoError(t, err)
			n := 0
			for _, h := range headers {
				if h.Label == schemasnapshot.LabelExportDataFromSourceExit && h.IsPlaceholder {
					n++
				}
			}
			return n
		}
		before := countExitPlaceholders()

		// A context whose deadline has already passed. The capture must not run a real
		// query and must not hang; with placeholderOnFailure=true (as the abnormal-exit
		// path uses) it falls back to a metadata-only marker.
		expiredCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond) // ensure the deadline has elapsed

		start := time.Now()
		err := sc.Capture(expiredCtx, schemasnapshot.LabelExportDataFromSourceExit, schemasnapshot.ReasonError, true)
		elapsed := time.Since(start)

		assert.Error(t, err, "an aborted capture must report why it failed, not swallow it")
		assert.Less(t, elapsed, 3*time.Second, "an expired context must abort the capture promptly, not hang")
		assert.Equal(t, before+1, countExitPlaceholders(),
			"an aborted capture with placeholderOnFailure must record exactly one exit placeholder")
	})

	t.Run("a nil DB records a placeholder and reports why", func(t *testing.T) {
		// Snapshot names are second-granularity, and the case above just wrote an exit
		// placeholder; without this wait the two collide on the UNIQUE name.
		time.Sleep(time.Second)
		before := countLabel(t, mdb, schemasnapshot.LabelExportDataFromSourceExit)

		gone := sc
		gone.DB = nil
		err := gone.Capture(ctx, schemasnapshot.LabelExportDataFromSourceExit, schemasnapshot.ReasonError, true)

		require.Error(t, err, "a missing handle must be reported, not swallowed")
		assert.Contains(t, err.Error(), "no active database handle")
		assert.Equal(t, before+1, countLabel(t, mdb, schemasnapshot.LabelExportDataFromSourceExit),
			"the timeline marker must still be recorded when the handle is gone")
	})
}

// TestSourceCaptureLargeSchema verifies that the capture budget
// (schemasnapshot.CaptureTimeout) is enough to capture AND persist a large schema — the
// only case where capture size matters (the fallback placeholder is metadata-only and
// does not scale). Because the capture is bounded by CaptureTimeout, a real snapshot is
// itself proof the budget sufficed for this size: had it been exceeded, the capture
// would have returned a deadline error. The elapsed time is logged so the headroom is
// visible.
//
// Note: on a healthy testcontainer this proves "the budget is ample for size", not "the
// budget survives a slow/loaded/high-latency source" — that adverse case isn't
// deterministically reproducible here.
func testSourceCaptureLargeSchema(t *testing.T, db *sql.DB, meta schemasnapshot.DBMetadata) {
	ctx := context.Background()

	const (
		schemaName   = "bigschema"
		numTables    = 1000
		colsPerTable = 10 // id + 9 columns => ~10k columns total
	)

	// One DO block creates all tables server-side (far faster than numTables round-trips).
	createTables := fmt.Sprintf(`DO $$ BEGIN
  FOR i IN 1..%d LOOP
    EXECUTE format('CREATE TABLE %s.t%%s (id int primary key, c1 text, c2 text, c3 int, c4 numeric, c5 timestamptz, c6 boolean, c7 text, c8 int)', i);
  END LOOP;
END $$;`, numTables, schemaName)

	execAll(t, db, `CREATE SCHEMA IF NOT EXISTS `+schemaName, createTables)
	t.Cleanup(func() { execAll(t, db, `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`) })

	mdb := newIntegrationTestMetaDB(t)
	sc := newSourceCapture(db, meta, mdb, schemaName)

	start := time.Now()
	require.NoError(t, sc.Capture(ctx, schemasnapshot.LabelExportSchema, "", true))
	elapsed := time.Since(start)
	t.Logf("captured %d-table / ~%d-column schema in %s (budget %s)",
		numTables, numTables*colsPerTable, elapsed, schemasnapshot.CaptureTimeout)

	headers, err := schemasnapshot.ListSnapshots(mdb)
	require.NoError(t, err)
	require.Len(t, headers, 1, "exactly one snapshot must be persisted")

	require.False(t, headers[0].IsPlaceholder,
		"capture of a %d-table schema must complete and persist within %s; got a placeholder (budget exceeded)",
		numTables, schemasnapshot.CaptureTimeout)

	content, err := schemasnapshot.LoadSnapshotByName(mdb, headers[0].Name())
	require.NoError(t, err)
	require.NotNil(t, content)
	assert.GreaterOrEqual(t, len(content.Tables), numTables,
		"captured snapshot must include all %d seeded tables", numTables)
}

// TestSourceCaptureStartPeriodic exercises the periodic-capture ticker directly, with
// the interval passed as a parameter (the reason the interval is an argument rather than
// a global read): a sub-minute value is impossible via the real
// --schema-snapshot-capture-interval flag, which is in minutes with a 1-minute floor.
// This covers the ticker's gating, that it fires at the injected interval, and that it
// stops when the context is cancelled — cheaply, without the ~90s live-migration path.
//
// The exporter-role gate is NOT covered here: it lives at cmd's call site, since the
// role is a command concern. It is exercised by the command-level and live E2E tests,
// both of which run as the source exporter.
func testSourceCaptureStartPeriodic(t *testing.T, db *sql.DB, meta schemasnapshot.DBMetadata) {
	const schemaName = "tickerschema"

	execAll(t, db,
		`CREATE SCHEMA IF NOT EXISTS `+schemaName,
		`CREATE TABLE `+schemaName+`.t1 (id int primary key, v text)`,
	)
	t.Cleanup(func() { execAll(t, db, `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`) })

	mdb := newIntegrationTestMetaDB(t)
	sc := newSourceCapture(db, meta, mdb, schemaName)

	countPeriodic := func() int {
		return countLabel(t, mdb, schemasnapshot.LabelExportDataFromSourcePeriodic)
	}

	t.Run("no-op when interval is non-positive", func(t *testing.T) {
		before := countPeriodic()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sc.StartPeriodic(ctx, 0)
		time.Sleep(250 * time.Millisecond) // more than a dozen 20ms ticks, had it started
		assert.Equal(t, before, countPeriodic(), "interval <= 0 must not start a ticker")
	})

	t.Run("no-op when disabled", func(t *testing.T) {
		before := countPeriodic()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		disabled := sc
		disabled.Disabled = true
		disabled.StartPeriodic(ctx, 20*time.Millisecond)
		time.Sleep(250 * time.Millisecond)
		assert.Equal(t, before, countPeriodic(), "a disabled capture must not start a ticker")
	})

	t.Run("fires at the injected interval and stops on context cancel", func(t *testing.T) {
		before := countPeriodic()
		ctx, cancel := context.WithCancel(context.Background())

		sc.StartPeriodic(ctx, 50*time.Millisecond)

		// Snapshot names are second-granularity, so at most ~one periodic snapshot persists
		// per wall-clock second (same-second ticks collide on the UNIQUE name and are
		// swallowed). So "fired repeatedly" is observed across seconds, not per-tick.
		require.Eventually(t, func() bool { return countPeriodic() >= before+2 },
			5*time.Second, 100*time.Millisecond,
			"ticker must fire and persist periodic snapshots at the injected interval")

		cancel()
		time.Sleep(300 * time.Millisecond) // let the goroutine observe cancellation and exit
		stopped := countPeriodic()
		// If the ticker were still running, crossing into the next wall-clock second would
		// persist a new (distinct-name) snapshot; a stable count proves it stopped.
		time.Sleep(1300 * time.Millisecond)
		assert.Equal(t, stopped, countPeriodic(), "ticker must stop after context cancel")
	})
}
