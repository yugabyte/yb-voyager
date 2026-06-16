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

// Shared helpers used by `schema migrate` and `data migrate` to orchestrate
// child phase subprocesses with deduplicated footers, consistent log-on-failure
// behavior, and a single consolidated closing Migration Progress block.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// orchestratorChildEnv returns env vars to pass to a child phase subprocess so
// that its own startup banner, Migration Progress, and Next-step rows are
// suppressed (the orchestrator prints those itself, once at the end).
//
//	VOYAGER_SUPPRESS_NEXT_STEP=1  — child skips the Next-step + Tip rows.
//	VOYAGER_SUPPRESS_PROGRESS=1   — child skips the entire Migration Progress
//	                                section (phase tree + next step + tip).
//	VOYAGER_QUIET_STARTUP=1       — child skips "Using config file:",
//	                                "Using export-dir:", and demotes
//	                                "migrationID:" to log-only.
//	CLICOLOR_FORCE=1              — preserves ANSI styling when stdout is a pipe.
func orchestratorChildEnv() []string {
	return append(os.Environ(),
		"VOYAGER_SUPPRESS_NEXT_STEP=1",
		"VOYAGER_SUPPRESS_PROGRESS=1",
		"VOYAGER_QUIET_STARTUP=1",
		"CLICOLOR_FORCE=1",
	)
}

// exitCodeFrom unwraps an *exec.ExitError into its numeric exit code, defaulting
// to 1 for non-exec errors (start failures, signals, etc.).
func exitCodeFrom(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

// printPhaseSkipped renders a one-line dim "Skipping <phase> — <reason>" message
// shown when MSR durability lets us skip a child invocation.
func printPhaseSkipped(phase, reason string) {
	fmt.Println()
	fmt.Println("  " + dimStyle.Render(fmt.Sprintf("Skipping %s — %s. (Pass --start-clean to re-run from scratch.)", phase, reason)))
}

// printClosingProgress emits one consolidated Migration Progress block at the
// end of a `<schema|data> migrate` run, computed from the latest MSR.
// currentStepID marks the last step we either ran or short-circuited on, so
// the in-progress indicator points to the right phase.
func printClosingProgress(currentStepID string) {
	rec, err := metaDB.GetMigrationStatusRecord()
	if err != nil || rec == nil {
		return
	}
	wf := resolveWorkflow(rec)
	phases := computePhaseStatuses(wf, rec, currentStepID)
	if len(phases) == 0 {
		return
	}

	migrationFlag := buildMigrationNameFlag()
	var lines []string
	lines = append(lines, formatPhaseLines(phases)...)
	lines = append(lines, formatKeyValue("Tip:", dimStyle.Render("yb-voyager status"+migrationFlag), kvWidth))
	printSection("Migration Progress", lines...)
}

// writeOrchestratorPhaseLog dumps captured subprocess output to a timestamped
// log file under <exportDir>/logs/<prefix>-<phase>-<ts>.log. Returns empty
// string on write failure — failure-path UX must not itself fail.
func writeOrchestratorPhaseLog(prefix, phase string, outBuf *bytes.Buffer) string {
	logsDir := filepath.Join(exportDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return ""
	}
	logPath := filepath.Join(logsDir,
		fmt.Sprintf("%s-%s-%s.log", prefix, phase, time.Now().Format("20060102-150405")))
	f, err := os.Create(logPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	_, _ = w.Write(outBuf.Bytes())
	return logPath
}

// printOrchestratorFailureTail prints the last ~60 lines of captured output (so
// the user sees the immediate error) and points to the full log file.
func printOrchestratorFailureTail(label string, outBuf *bytes.Buffer, logPath string) {
	const tailLines = 60
	lines := strings.Split(strings.TrimRight(outBuf.String(), "\n"), "\n")
	start := 0
	if len(lines) > tailLines {
		start = len(lines) - tailLines
		fmt.Println()
		fmt.Println("  " + dimStyle.Render(fmt.Sprintf("--- last %d lines of %s output ---", tailLines, label)))
	}
	for _, ln := range lines[start:] {
		fmt.Println(ln)
	}
	if logPath != "" {
		fmt.Println()
		fmt.Println("  " + dimStyle.Render("Full log: "+displayPath(logPath)))
	}
}
