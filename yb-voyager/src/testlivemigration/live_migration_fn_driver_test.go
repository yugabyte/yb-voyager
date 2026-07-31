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

/*
Shared driver for the conflict-detection false-negative (FN) hunting suite.

See conflict-detection-fn-test-plan.md at the repo root for the full plan. The driver runs
one real live migration per case and classifies the outcome:

  - SKIP  "premise does not hold"     — the equality premise the case relies on is false in PG
                                        (proven by a preflight query, never assumed).
  - SKIP  "UNREACHABLE on YB target"  — the target rejects the schema (live error captured,
                                        never assumed from docs). Same for the source.
  - FAIL  "FALSE NEGATIVE CONFIRMED"  — import-data died with a duplicate-key error: detection
                                        missed a genuine conflict and it raced to the target.
  - FAIL  "STALL"                     — events never fully applied and import never exited
                                        (possible conflict-wait deadlock).
  - PASS  survived                    — all delta events applied, no duplicate-key error.

Design notes (learned the hard way):
  - Never use ExecuteSqlsOnDB / ExecuteOnSource / ExecuteOnTarget / SetupSchema for DDL that
    can fail: the container helper calls os.Exit on SQL errors and kills the whole test binary.
    All fallible SQL goes through WithSourceConn / WithTargetConn + database/sql Exec.
  - No t.Parallel(): the suite is run sequentially in isolated processes; the shared containers
    and the data-migration-report poller are unreliable under parallel load.
  - Report keys use the quoted `"schema"."table"` form.
*/

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fnCase describes one false-negative hunting scenario.
type fnCase struct {
	// name is a short lowercase slug; it becomes the per-test database name ("fn_<name>").
	name string

	// preflightSQL runs on the SOURCE before anything else. Use it to prove the equality
	// premise of the case (e.g. that '-0.0'::float8 = '0.0'::float8). Any error skips the
	// test with "premise does not hold". Use DO $$ ... RAISE EXCEPTION ... $$ blocks.
	preflightSQL []string

	// targetDDL runs on the TARGET first (dynamic probe). Any error skips the test with the
	// live error ("UNREACHABLE on YB target"). If nil, sourceDDL is used for the target too.
	targetDDL []string

	// sourceDDL runs on the SOURCE (include REPLICA IDENTITY FULL statements explicitly).
	// Any error skips the test ("UNREACHABLE on source PG").
	sourceDDL []string

	// seedSQL runs on the source after DDL; failures are test bugs (fatal).
	seedSQL []string

	// deltaSQL is the conflict-generating workload, executed statement by statement on the
	// source after the snapshot completes; failures are fatal.
	deltaSQL []string

	// expectedSnapshotRows: report-key ("schema"."table" quoted form) -> row count, used to
	// wait for snapshot completion.
	expectedSnapshotRows map[string]int64

	// expectedChanges: exact per-table I/U/D counts of deltaSQL. When set, the verdict loop
	// exits as soon as the report shows all events applied. When nil, the case is observed
	// for observationSeconds instead (for workloads whose emitted event counts are uncertain,
	// e.g. PK updates that Debezium may split into delete+insert).
	expectedChanges map[string]ChangesCount

	// afterSnapshotTargetSQL runs on the TARGET after the snapshot completes AND after the
	// import process has logged that the streaming phase started (so the conflict cache has
	// been initialized). Used by the mid-stream index case. Errors are fatal.
	afterSnapshotTargetSQL []string

	// observationSeconds bounds the verdict loop when expectedChanges is nil (default 120).
	observationSeconds int

	// verdictTimeoutSeconds bounds the verdict loop when expectedChanges is set (default 240).
	verdictTimeoutSeconds int
}

// execStmtsOn runs each statement through the given connection provider, returning the
// failing statement and error (nil error if all succeeded). It never os.Exits.
func execStmtsOn(withConn func(func(*sql.DB) error) error, stmts []string) (string, error) {
	for _, stmt := range stmts {
		s := stmt
		err := withConn(func(db *sql.DB) error {
			_, execErr := db.Exec(s)
			return execErr
		})
		if err != nil {
			return s, err
		}
	}
	return "", nil
}

// pollImportStdoutContains waits until the import command's combined output contains substr.
func pollImportStdoutContains(lm *LiveMigrationTest, substr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out := lm.GetImportCommandStdout() + lm.GetImportCommandStderr()
		if strings.Contains(out, substr) {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

// runFNCase drives one case end to end. See the file header for the verdict semantics.
func runFNCase(t *testing.T, c fnCase) {
	t.Helper()
	dbName := "fn_" + c.name

	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: dbName},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: dbName},
		SchemaNames: []string{"test_schema"},
		SchemaSQL:   []string{`CREATE SCHEMA IF NOT EXISTS test_schema;`},
		CleanupSQL:  []string{`DROP SCHEMA IF EXISTS test_schema CASCADE;`},
	})
	defer lm.Cleanup()

	if err := lm.SetupContainers(context.Background()); err != nil {
		t.Fatalf("failed to setup containers: %v", err)
	}
	if err := lm.SetupSchema(); err != nil { // schema-create only; safe
		t.Fatalf("failed to setup schema: %v", err)
	}

	// 1. Preflight: prove the equality premise on the source. Never assume.
	if stmt, err := execStmtsOn(lm.WithSourceConn, c.preflightSQL); err != nil {
		t.Skipf("premise does not hold on source PG — no FN class to hunt.\nstatement: %s\nerror: %v", stmt, err)
	}

	// 2. Dynamic probe: target DDL first. A rejection is a *finding*, captured live.
	targetDDL := c.targetDDL
	if targetDDL == nil {
		targetDDL = c.sourceDDL
	}
	if stmt, err := execStmtsOn(lm.WithTargetConn, targetDDL); err != nil {
		t.Skipf("UNREACHABLE ON YB TARGET — schema rejected, FN class cannot arise on a YB target.\nstatement: %s\nerror: %v", stmt, err)
	}
	if stmt, err := execStmtsOn(lm.WithSourceConn, c.sourceDDL); err != nil {
		t.Skipf("UNREACHABLE ON SOURCE PG — cannot construct the scenario.\nstatement: %s\nerror: %v", stmt, err)
	}

	if stmt, err := execStmtsOn(lm.WithSourceConn, c.seedSQL); err != nil {
		t.Fatalf("seed failed (test bug) on %q: %v", stmt, err)
	}

	if err := lm.StartExportData(true, nil); err != nil {
		t.Fatalf("failed to start export data: %v", err)
	}
	if err := lm.StartImportData(true, nil); err != nil {
		t.Fatalf("failed to start import data: %v", err)
	}
	if err := lm.WaitForSnapshotComplete(c.expectedSnapshotRows, 120); err != nil {
		t.Fatalf("snapshot did not complete: %v\nimport output tail:\n%s", err,
			tail(lm.GetImportCommandStderr()+lm.GetImportCommandStdout(), 25))
	}

	// 3. Optional mid-stream hook: wait for the streaming phase (conflict cache init) first.
	if len(c.afterSnapshotTargetSQL) > 0 {
		if !pollImportStdoutContains(lm, "treaming", 90*time.Second) { // "Initializing streaming phase" / "streaming changes to"
			t.Fatalf("import never entered streaming phase; cannot run afterSnapshotTargetSQL")
		}
		time.Sleep(5 * time.Second) // let the conflict cache finish initializing
		if stmt, err := execStmtsOn(lm.WithTargetConn, c.afterSnapshotTargetSQL); err != nil {
			t.Fatalf("afterSnapshotTargetSQL failed on %q: %v", stmt, err)
		}
	}

	// 4. Conflict-generating workload.
	if stmt, err := execStmtsOn(lm.WithSourceConn, c.deltaSQL); err != nil {
		t.Fatalf("delta failed (test bug) on %q: %v", stmt, err)
	}

	// 5. Verdict loop.
	timeout := time.Duration(c.verdictTimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 240 * time.Second
	}
	observation := time.Duration(c.observationSeconds) * time.Second
	if observation == 0 {
		observation = 120 * time.Second
	}
	start := time.Now()
	for {
		if lm.GetImportRunner() != nil && lm.GetImportRunner().IsStopped() {
			out := lm.GetImportCommandStderr() + "\n" + lm.GetImportCommandStdout()
			if containsUniqueViolation(out) {
				t.Fatalf("FALSE NEGATIVE CONFIRMED (%s):\n"+
					"conflict detection missed a genuine unique-key conflict; the reclaiming event raced\n"+
					"ahead and the target raised a duplicate-key error, aborting import-data.\n"+
					"---- import-data output (tail) ----\n%s", c.name, tail(out, 25))
			}
			t.Fatalf("import-data stopped unexpectedly (not a unique violation) for %s:\n%s", c.name, tail(out, 25))
		}
		if c.expectedChanges != nil {
			done, err := lm.streamingPhaseCompleted(c.expectedChanges, "source", "target")
			if err == nil && done {
				t.Logf("%s: SURVIVED — all delta events applied, no duplicate-key error", c.name)
				return
			}
			if time.Since(start) > timeout {
				t.Fatalf("STALL (%s): import still running but delta events not fully applied after %v — "+
					"possible conflict-wait deadlock.\nimport output tail:\n%s", c.name, timeout,
					tail(lm.GetImportCommandStderr()+lm.GetImportCommandStdout(), 25))
			}
		} else if time.Since(start) > observation {
			t.Logf("%s: SURVIVED observation window (%v) — no duplicate-key error observed", c.name, observation)
			return
		}
		time.Sleep(3 * time.Second)
	}
}

// buildFreeReclaimDeltaStmts returns 2n statements alternating DELETE of the previous row and
// INSERT of a new row (new PK) whose unique-key literal is produced by valueFn(i). Equal-but-
// differently-written literals from valueFn are the FN bait: each iteration frees the value
// under one spelling and reclaims it under another.
func buildFreeReclaimDeltaStmts(schemaTable string, n int, insertCols string, valueFn func(i int) string) []string {
	out := make([]string, 0, 2*n)
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf(`DELETE FROM %s WHERE id = %d;`, schemaTable, i-1))
		out = append(out, fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%d, %s, 'p%d');`,
			schemaTable, insertCols, i, valueFn(i), i))
	}
	return out
}

// freeReclaimChanges returns the exact ChangesCount produced by buildFreeReclaimDeltaStmts(n).
func freeReclaimChanges(n int) ChangesCount {
	return ChangesCount{Inserts: int64(n), Updates: 0, Deletes: int64(n)}
}
