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
What happens to the fall-back flow's unsupported-column prompt when nobody can answer it?

At cutover, `import data` reaches `export data from target` through syscall.Exec
(cmd/importData.go, startExportDataFromTargetIfRequired), so fd 0 is inherited EXACTLY as
import data had it. generateExportDataFromTargetCommand appends --yes only when the
parent had --yes, so a user who ran `import data` without --yes gets a reverse-direction
export that will prompt:

	The following columns data export is unsupported:
	sweep_schema.p_fbprompt: [v]

	Do you want to continue with the export by ignoring just these columns' data? [Y/N]:

utils.AskPrompt (src/utils/utils.go) then does `fmt.Scan(&input)` followed by
`panic(err)`. So the outcome depends entirely on what fd 0 is:

	A. stdin at EOF (closed, or /dev/null - a backgrounded or CI-run import data)
	   -> Scan returns io.ErrUnexpectedEOF -> panic
	B. stdin an open pipe nobody ever writes to
	   -> Scan blocks -> the migration hangs with the prompt on screen

This test runs both and records which. It is an observation, not an assertion: it logs
STDIN-EXPERIMENT lines and never fails on the product's behaviour.
*/

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	fbPromptTable = "p_fbprompt"
	// How long to watch after cutover before calling it a hang.
	fbPromptObserveWindow = 200 * time.Second
	fbPromptPoll          = 5 * time.Second
)

func TestFallbackPromptStdinBehaviour(t *testing.T) {
	t.Run("A_stdin_at_eof", func(t *testing.T) {
		// An empty reader: exec.Cmd copies nothing and closes the write end, so the
		// child sees EOF immediately. Same as < /dev/null.
		runFallbackPromptCase(t, "A: stdin at EOF (closed / /dev/null)", func() (io.Reader, func()) {
			return strings.NewReader(""), func() {}
		})
	})

	t.Run("B_stdin_open_pipe_never_written", func(t *testing.T) {
		// A real os.Pipe read end is passed straight through as the child's fd 0, with
		// no copying goroutine. Keeping the write end open means the child's read never
		// returns: the closest analogue to a background process with no terminal.
		runFallbackPromptCase(t, "B: stdin open pipe, never written", func() (io.Reader, func()) {
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatalf("os.Pipe: %v", err)
			}
			return pr, func() { pw.Close(); pr.Close() }
		})
	})
}

func runFallbackPromptCase(t *testing.T, label string, mkStdin func() (io.Reader, func())) {
	schema := "sweep_schema"
	dbName := "fbprompt"
	cfg := &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: dbName},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: dbName},
		SchemaNames: []string{schema},
		SchemaSQL: []string{
			fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", schema),
			fmt.Sprintf("CREATE SCHEMA %s;", schema),
		},
		CleanupSQL: []string{fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", schema)},
	}

	lm := NewLiveMigrationTest(t, cfg)
	defer lm.Cleanup()

	if err := lm.SetupContainers(context.Background()); err != nil {
		t.Fatalf("container setup failed: %v", err)
	}
	if err := lm.SetupSchema(); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}

	// int4range is the ideal excluded type: the forward and reverse guardrails both
	// exclude it, and it needs no extension.
	tbl := fmt.Sprintf("%s.%s", schema, fbPromptTable)
	ddl := []string{
		fmt.Sprintf("CREATE TABLE %s (id int PRIMARY KEY, filler text, v int4range)", tbl),
	}
	for _, side := range []dbSideLite{sideLiteSource, sideLiteTarget} {
		if err := execOnLite(lm, side, ddl...); err != nil {
			t.Fatalf("probe DDL failed on %s: %v", side, err)
		}
	}
	if err := execOnLite(lm, sideLiteSource,
		fmt.Sprintf("ALTER TABLE %s REPLICA IDENTITY FULL", tbl)); err != nil {
		t.Fatalf("replica identity failed: %v", err)
	}
	if err := execOnLite(lm, sideLiteSource, fmt.Sprintf(
		"INSERT INTO %s (id, filler, v) SELECT g, 'filler-'||g, int4range(g, g+5) FROM generate_series(1,10) g", tbl)); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	stdin, cleanupStdin := mkStdin()
	defer cleanupStdin()

	if err := lm.StartExportData(true, nil); err != nil {
		t.Fatalf("export data failed to start: %v", err)
	}
	// The whole point: import data's stdin becomes export-data-from-target's stdin.
	lm.SetImportDataStdin(stdin)
	if err := lm.StartImportData(true, map[string]string{"--log-level": "debug"}); err != nil {
		t.Fatalf("import data failed to start: %v", err)
	}
	if err := lm.WaitForSnapshotComplete(map[string]int64{
		fmt.Sprintf("%s.%s", schema, fbPromptTable): 10,
	}, 900); err != nil {
		t.Logf("STDIN-EXPERIMENT %s | snapshot wait did not complete: %v", label, err)
	}

	// Give streaming something to carry, then cut over with fall-back prepared. That is
	// what makes import data exec into export data from target.
	if err := execOnLite(lm, sideLiteSource,
		fmt.Sprintf("UPDATE %s SET v = int4range(100,200) WHERE id = 1", tbl)); err != nil {
		t.Logf("delta failed: %v", err)
	}

	if err := lm.InitiateCutoverToTarget(true, nil); err != nil {
		t.Logf("STDIN-EXPERIMENT %s | cutover initiation returned: %v", label, err)
	}

	runner := lm.GetImportRunner()
	start := time.Now()
	var stopped bool
	deadline := time.Now().Add(fbPromptObserveWindow)
	for time.Now().Before(deadline) {
		if runner.IsStopped() {
			stopped = true
			break
		}
		time.Sleep(fbPromptPoll)
	}
	elapsed := time.Since(start).Round(time.Second)

	out := runner.Stdout()
	errOut := runner.Stderr()
	combined := out + "\n" + errOut

	promptSeen := strings.Contains(combined, "Do you want to continue with the export by ignoring")
	noticeSeen := strings.Contains(combined, "The following columns data export is unsupported")
	panicSeen := strings.Contains(combined, "panic:")
	eofSeen := strings.Contains(combined, "EOF")

	outcome := "HUNG (still running)"
	if stopped {
		outcome = fmt.Sprintf("EXITED (exit code %v)", runner.ExitCode())
		if panicSeen {
			outcome = "PANICKED"
		}
	}

	t.Logf("STDIN-EXPERIMENT %s | outcome=%s | elapsed=%s | exclusion_notice=%v prompt=%v panic=%v eof_mentioned=%v",
		label, outcome, elapsed, noticeSeen, promptSeen, panicSeen, eofSeen)

	// Verbatim evidence: the guardrail notice, the prompt, and any panic with its top
	// frames. This is what a user would actually see.
	for _, marker := range []string{
		"The following columns data export is unsupported",
		"Do you want to continue with the export by ignoring",
		"panic:",
	} {
		if idx := strings.Index(combined, marker); idx >= 0 {
			end := idx + 1800
			if end > len(combined) {
				end = len(combined)
			}
			t.Logf("STDIN-EXPERIMENT %s | VERBATIM from %q:\n%s", label, marker, combined[idx:end])
		}
	}
	if !stopped {
		t.Logf("STDIN-EXPERIMENT %s | CONFIRMED STILL ALIVE after %s; last 1200 bytes of output:\n%s",
			label, elapsed, tailStr(combined, 1200))
	}
}

// --- tiny local helpers, kept separate from the sweep harness ---

type dbSideLite int

const (
	sideLiteSource dbSideLite = iota
	sideLiteTarget
)

func (s dbSideLite) String() string {
	if s == sideLiteSource {
		return "source"
	}
	return "target"
}

func execOnLite(lm *LiveMigrationTest, side dbSideLite, stmts ...string) error {
	if side == sideLiteSource {
		return lm.ExecuteOnSource(stmts...)
	}
	return lm.ExecuteOnTarget(stmts...)
}

func tailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
