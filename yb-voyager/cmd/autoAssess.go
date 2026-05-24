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

// Auto-assess: runs `yb-voyager assess` as a subprocess at the end of
// `yb-voyager new` so users only need to invoke one command up to the
// assessment report. The subprocess approach is intentional — it
// inherits assess's PersistentPreRun (metaDB init, locking, callhome,
// etc.) without us re-wiring it, and lets `new` survive an assess
// failure without being killed by utils.ErrExit.
//
// To revert auto-assess and go back to "new tells you to run assess
// manually," delete this file and the single block in newCommand.go
// (handleConnectionString / handleConnectionStringDirect) that calls
// runAutoAssess.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// runAutoAssess shells out to `yb-voyager assess --migration-name <name> --yes`
// with all output captured behind a spinner. On success, it prints the
// Assessment Summary footer that assess itself emitted (extracted from
// captured stdout) and a hint about re-running assess with custom options.
// On failure, it writes the captured output to a log file and prints a
// short pointer for retry.
//
// Returns true on success.
func runAutoAssess(migrationName, exportDirPath string) bool {
	// --iops-capture-interval 5 keeps the auto-assess fast and demo-friendly.
	// A freshly-initialized project has no workload to sample anyway; users who
	// want a real IOPS reading can re-run `yb-voyager assess` (the standalone
	// command keeps its 120s default).
	args := []string{
		"assess",
		"--migration-name", migrationName,
		"--iops-capture-interval", "5",
		"--yes",
	}

	cmd := exec.Command("yb-voyager", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	// Preserve styling — without this, lipgloss/charm detect "not a TTY"
	// on the piped stdout and strip colors from the captured footer.
	cmd.Env = append(os.Environ(), "CLICOLOR_FORCE=1")

	stop := startSpinner("Assessing migration")
	err := cmd.Run()
	stop()

	if err != nil {
		logPath := writeAutoAssessLog(exportDirPath, &outBuf, &errBuf)
		printAutoAssessFailure(migrationName, logPath, err)
		return false
	}

	printAutoAssessSuccess(&outBuf, migrationName)
	return true
}

// startSpinner prints a rotating glyph next to msg until the returned stop
// function is called. Safe to call from any goroutine; idempotent stop.
func startSpinner(msg string) func() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan struct{})
	var once sync.Once

	go func() {
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				// Clear the line on exit.
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r  %s %s...", frames[i%len(frames)], msg)
				i++
			}
		}
	}()

	return func() {
		once.Do(func() { close(done) })
	}
}

// printAutoAssessSuccess extracts the "Assessment Summary" footer from
// the captured assess stdout and re-emits it, followed by a hint line.
// Falls back to a minimal one-liner if the marker isn't present.
func printAutoAssessSuccess(out *bytes.Buffer, migrationName string) {
	const marker = "Assessment Summary"

	captured := out.String()
	idx := strings.Index(captured, marker)
	if idx == -1 {
		// Shouldn't happen on a successful assess, but degrade gracefully.
		fmt.Println()
		fmt.Println("  " + successStyle.Render("✓") + " Assessment complete.")
		fmt.Println()
		return
	}

	// The footer rendering helper adds two-space leading indent. Re-emit
	// captured lines starting from the section banner (which is preceded
	// by leading spaces). Walk back to the start of that line so we keep
	// the indentation column aligned.
	lineStart := strings.LastIndex(captured[:idx], "\n")
	if lineStart == -1 {
		lineStart = 0
	} else {
		lineStart++ // skip the newline itself
	}
	footer := captured[lineStart:]

	// Strip the generic "Tip: yb-voyager status ..." line that
	// printCommandFooter appends to every assess footer. At this stage
	// (right after `new`), nothing has started — `status` is a redundant
	// pointer. Our own re-assess hint replaces it below.
	footer = stripStatusTip(footer)

	// Strip trailing blank lines for a tidy hand-off to our hint line.
	footer = strings.TrimRight(footer, "\n ")

	fmt.Println()
	fmt.Println(footer)
	fmt.Println()
	fmt.Println("  " + dimStyle.Render(fmt.Sprintf(
		"Tip: Re-assess with custom options (e.g. specific schemas, read replicas): yb-voyager assess --migration-name %s",
		migrationName)))
	fmt.Println()
}

// stripStatusTip removes any line that is the generic
// "Tip: yb-voyager status ..." that printCommandFooter unconditionally
// appends. Matches both the colored and plain forms.
func stripStatusTip(footer string) string {
	lines := strings.Split(footer, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		trimmed := strings.TrimSpace(stripANSI(ln))
		if strings.HasPrefix(trimmed, "Tip:") && strings.Contains(trimmed, "yb-voyager status") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

// stripANSI removes ANSI escape sequences from s. Used for pattern-matching
// captured styled output without depending on the active color palette.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip until terminator (anything in @-~ range, typically 'm').
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// printAutoAssessFailure prints a short pointer to the log file containing
// the full captured stdout/stderr from the failed assess subprocess.
func printAutoAssessFailure(migrationName, logPath string, runErr error) {
	fmt.Println()
	fmt.Println("  " + color.RedString("✗") + " Assessment failed: " + runErr.Error())
	if logPath != "" {
		fmt.Println("    " + dimStyle.Render("Details: "+displayPath(logPath)))
	}
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Retry:"))
	fmt.Println("    " + cmdStyle.Render(fmt.Sprintf("yb-voyager assess --migration-name %s", migrationName)))
	fmt.Println()
}

// writeAutoAssessLog dumps captured stdout/stderr to a timestamped file under
// <exportDir>/logs/ and returns its path. Returns empty string if writing
// fails — failure-path UX shouldn't itself fail.
func writeAutoAssessLog(exportDirPath string, outBuf, errBuf *bytes.Buffer) string {
	logsDir := filepath.Join(exportDirPath, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return ""
	}
	logPath := filepath.Join(logsDir, fmt.Sprintf("auto-assess-%s.log", time.Now().Format("20060102-150405")))
	f, err := os.Create(logPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	fmt.Fprintln(w, "=== stdout ===")
	w.Write(outBuf.Bytes())
	fmt.Fprintln(w, "\n=== stderr ===")
	w.Write(errBuf.Bytes())
	return logPath
}
