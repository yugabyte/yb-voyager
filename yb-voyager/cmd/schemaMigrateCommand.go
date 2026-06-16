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

// `schema migrate` orchestrates `schema export` -> `schema analyze` -> `schema import`
// as a single durable, iterative workflow.
//
// Output strategy:
//
//	Children are run as subprocesses with three env-var gates set so their
//	output is the body content + the one-section "Command Summary" footer,
//	without the per-phase Migration Progress tree or duplicated startup
//	banners that would otherwise repeat 3x in a single `schema migrate` run.
//
//	  VOYAGER_SUPPRESS_NEXT_STEP=1  — child skips the Next-step + Tip rows.
//	  VOYAGER_SUPPRESS_PROGRESS=1   — child skips the entire Migration
//	                                  Progress section (phase tree + next
//	                                  step + tip).
//	  VOYAGER_QUIET_STARTUP=1       — child skips "Using config file:",
//	                                  "Using export-dir:", and demotes
//	                                  "migrationID:" to log-only.
//
//	The orchestrator prints the startup banner once at the top and the
//	consolidated Migration Progress once at the end (in whichever terminal
//	state — success / paused-on-issues / auto-approve-stop / failure).
//
// Durability is read from MSR before each phase: skip export if
// ExportSchemaDone, skip analyze if AnalyzeSchemaDone && LastAnalyzeIssueCount
// == 0, skip everything if ImportSchemaDone.
//
// To revert: delete this file and re-expose the three subcommands (un-set
// Hidden:true). Re-point startMigrationCommand and statusCommand next-step
// suggestions back to "schema export". The footer-suppress / quiet-startup
// env hooks are harmless if left in place.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var schemaMigrateAutoApprove utils.BoolStr = true

// Set in Run after cobra parses flags; consulted by runChildSchemaPhase to decide
// whether to forward import-only knobs to the schema import child.
var (
	schemaMigrateContinueOnErrorSet bool
	schemaMigrateIgnoreExistSet     bool
)

var schemaMigrateCmd = &cobra.Command{
	Use:     "migrate",
	Short:   "Run schema export, analyze, and import as one durable iterative workflow.",
	Aliases: []string{},

	Run: func(cmd *cobra.Command, args []string) {
		schemaMigrateContinueOnErrorSet = cmd.Flags().Changed("continue-on-error")
		schemaMigrateIgnoreExistSet = cmd.Flags().Changed("ignore-exist")
		runSchemaMigrate()
	},
}

func init() {
	schemaCmd.AddCommand(schemaMigrateCmd)
	registerCommonGlobalFlags(schemaMigrateCmd)

	BoolVar(schemaMigrateCmd.Flags(), &startClean, "start-clean", false,
		"forwarded to schema export and schema import — wipes the schema directory and target schemas before re-running.")

	BoolVar(schemaMigrateCmd.Flags(), &schemaMigrateAutoApprove, "auto-approve", true,
		"automatically continue to schema import when analyze reports 0 issues. Pass --auto-approve=false to stop after analyze.")

	// Forwarded to the schema import child only.
	BoolVar(schemaMigrateCmd.Flags(), &tconf.ContinueOnError, "continue-on-error", false,
		"ignore errors during schema import and continue (forwarded to schema import).")
	BoolVar(schemaMigrateCmd.Flags(), &tconf.IgnoreIfExists, "ignore-exist", false,
		"ignore errors during schema import if the object already exists (forwarded to schema import).")
}

func runSchemaMigrate() {
	if metaDB == nil {
		utils.ErrExit("meta DB not initialized — run `yb-voyager new` first to set up this migration project.")
	}

	// Parent's PersistentPreRun already prints "Using config file" and
	// "Using export-dir" once. Children inherit VOYAGER_QUIET_STARTUP=1 so
	// they don't repeat.

	if bool(startClean) {
		// --start-clean means "wipe the slate". Children take care of their
		// own state (export clears ExportSchemaDone before re-exporting,
		// import drops + recreates target schemas), but if we pause partway
		// through (e.g. analyze finds issues and we never reach import), a
		// stale ImportSchemaDone from a previous full run would trick the
		// next invocation into the "already migrated" short-circuit. Reset
		// the phase flags up front to keep the durability state coherent.
		resetSchemaMigrateMSRFlags()
	}

	msr := initialReadOfMSR()

	if msr.ImportSchemaDone && !bool(startClean) {
		printSchemaAlreadyMigrated()
		return
	}

	// Phase 1: Export schema
	if msr.ExportSchemaDone && !bool(startClean) {
		printPhaseSkipped("schema export", "already exported")
	} else {
		if err := runChildSchemaPhase("export"); err != nil {
			printSchemaMigrateFailure("schema export", err)
			os.Exit(exitCodeFrom(err))
		}
		msr = initialReadOfMSR()
	}

	// Phase 2: Analyze schema (always runnable; skip only if last run was clean)
	if msr.AnalyzeSchemaDone && msr.LastAnalyzeIssueCount == 0 && !bool(startClean) {
		printPhaseSkipped("schema analyze", "last run had 0 issues")
	} else {
		if err := runChildSchemaPhase("analyze"); err != nil {
			printSchemaMigrateFailure("schema analyze", err)
			os.Exit(exitCodeFrom(err))
		}
		msr = initialReadOfMSR()
	}

	if msr.LastAnalyzeIssueCount > 0 {
		printAnalyzeIssuesHint(msr.LastAnalyzeIssueCount)
		return
	}

	if !bool(schemaMigrateAutoApprove) {
		printAutoApproveStop()
		return
	}

	// Phase 3: Import schema
	if err := runChildSchemaPhase("import"); err != nil {
		printSchemaMigrateFailure("schema import", err)
		os.Exit(exitCodeFrom(err))
	}

	printSchemaMigrateComplete()
}

// phaseStage describes one segment of a spinner shown while a child subprocess
// is running. The first stage starts as soon as the subprocess starts. Each
// subsequent stage's marker is matched (substring) against streamed output
// lines; when a match fires, the spinner's label switches to that stage's
// label. This lets a single subprocess (e.g. `schema export`) show two visual
// stages (exporting → optimizing) without splitting the subprocess.
type phaseStage struct {
	label  string
	marker string // empty for the initial stage; non-empty for transitions
}

var (
	exportStages = []phaseStage{
		{label: "Exporting schema"},
		{label: "Optimizing schema for YugabyteDB", marker: "Applying schema modifications"},
	}
	analyzeStages = []phaseStage{{label: "Analyzing schema"}}
	importStages  = []phaseStage{{label: "Importing schema"}}
)

// runChildSchemaPhase invokes `yb-voyager schema <phase>` as a subprocess with
// stdout+stderr captured behind a spinner. After the subprocess completes,
// the "Schema Summary" Section block is extracted from the captured output
// and printed. On failure, the full captured output is dumped to a log file
// and the tail printed for context.
func runChildSchemaPhase(phase string) error {
	args := []string{"schema", phase, "--yes"}

	if migrationName != "" {
		args = append(args, "--migration-name", migrationName)
	}
	if exportDir != "" {
		args = append(args, "--export-dir", exportDir)
	}
	if cfgFile != "" {
		args = append(args, "--config-file", cfgFile)
	}
	// --start-clean is meaningful for export and import (not analyze).
	// BoolStr flags require an explicit value — bare `--start-clean` errors with
	// "flag needs an argument", so always pass it as `--start-clean=true`.
	if bool(startClean) && (phase == "export" || phase == "import") {
		args = append(args, "--start-clean=true")
	}
	// Forward import-only knobs to the schema import child if the user set them.
	if phase == "import" {
		if schemaMigrateContinueOnErrorSet {
			args = append(args, fmt.Sprintf("--continue-on-error=%t", bool(tconf.ContinueOnError)))
		}
		if schemaMigrateIgnoreExistSet {
			args = append(args, fmt.Sprintf("--ignore-exist=%t", bool(tconf.IgnoreIfExists)))
		}
	}

	binary, err := os.Executable()
	if err != nil {
		binary = "yb-voyager"
	}

	cmd := exec.Command(binary, args...)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	cmd.Stdin = nil // --yes is set; no interactive prompts expected
	cmd.Env = orchestratorChildEnv()

	stages := stagesForPhase(phase)
	sp := startDynamicSpinner(stages[0].label)

	// Reader goroutine: buffer everything for later, and watch for stage-transition markers.
	var outBuf bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1<<20) // up to 1 MiB lines
		for scanner.Scan() {
			line := scanner.Text()
			outBuf.WriteString(line + "\n")
			for _, st := range stages[1:] {
				if st.marker != "" && strings.Contains(line, st.marker) {
					sp.setLabel(st.label)
				}
			}
		}
	}()

	startErr := cmd.Start()
	if startErr != nil {
		sp.stop()
		pw.Close()
		<-readDone
		return startErr
	}
	runErr := cmd.Wait()
	pw.Close()
	<-readDone
	sp.stop()

	if runErr != nil {
		logPath := writeOrchestratorPhaseLog("schema-migrate", phase, &outBuf)
		printOrchestratorFailureTail("schema "+phase, &outBuf, logPath)
		return runErr
	}

	printSchemaPhaseSummary(phase, outBuf.String())
	return nil
}

func stagesForPhase(phase string) []phaseStage {
	switch phase {
	case "export":
		return exportStages
	case "analyze":
		return analyzeStages
	case "import":
		return importStages
	default:
		return []phaseStage{{label: phase}}
	}
}

// printSchemaPhaseSummary extracts the "<Phase> Schema Summary" block from the
// captured subprocess stdout and prints it. Falls back to a minimal success
// line if the marker isn't found (shouldn't happen on a successful child run).
func printSchemaPhaseSummary(phase, captured string) {
	marker := schemaSummaryMarker(phase)
	idx := strings.Index(captured, marker)
	if idx == -1 {
		fmt.Println()
		fmt.Println("  " + successStyle.Render("✓") + " schema " + phase + " completed.")
		return
	}
	// Walk back to the start of the line containing the marker so we preserve
	// the section's leading indentation.
	lineStart := strings.LastIndex(captured[:idx], "\n")
	if lineStart == -1 {
		lineStart = -1
	}
	footer := captured[lineStart+1:]
	footer = strings.TrimRight(footer, "\n ")

	fmt.Println()
	fmt.Println(footer)
}

func schemaSummaryMarker(phase string) string {
	switch phase {
	case "export":
		return "Export Schema Summary"
	case "analyze":
		return "Analyze Schema Summary"
	case "import":
		return "Import Schema Summary"
	default:
		return "Schema Summary"
	}
}

// dynamicSpinner is a spinner whose label can be swapped while it's running.
// Used by `schema migrate` to transition the displayed label mid-subprocess
// (e.g. from "Exporting schema" to "Optimizing schema for YugabyteDB").
type dynamicSpinner struct {
	label atomic.Value // string
	done  chan struct{}
	once  sync.Once
}

func startDynamicSpinner(initial string) *dynamicSpinner {
	s := &dynamicSpinner{done: make(chan struct{})}
	s.label.Store(initial)
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.done:
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r  %s %s...", frames[i%len(frames)], s.label.Load().(string))
				i++
			}
		}
	}()
	return s
}

func (s *dynamicSpinner) setLabel(label string) {
	s.label.Store(label)
}

func (s *dynamicSpinner) stop() {
	s.once.Do(func() { close(s.done) })
}

// initialReadOfMSR returns a (never-nil) MSR; callers should not modify it.
func initialReadOfMSR() *MSRSnapshot {
	rec, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("read migration status record: %w", err)
	}
	if rec == nil {
		return &MSRSnapshot{}
	}
	return &MSRSnapshot{
		ExportSchemaDone:      rec.ExportSchemaDone,
		AnalyzeSchemaDone:     rec.AnalyzeSchemaDone,
		LastAnalyzeIssueCount: rec.LastAnalyzeIssueCount,
		ImportSchemaDone:      rec.ImportSchemaDone,
	}
}

// MSRSnapshot is a narrow view into MSR for the orchestrator's decision logic.
type MSRSnapshot struct {
	ExportSchemaDone      bool
	AnalyzeSchemaDone     bool
	LastAnalyzeIssueCount int
	ImportSchemaDone      bool
}

// resetSchemaMigrateMSRFlags clears the three schema-phase flags. Called when
// --start-clean is passed so that subsequent invocations (with or without
// --start-clean) start from a coherent "nothing done" state until each child
// updates its own flag.
func resetSchemaMigrateMSRFlags() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportSchemaDone = false
		record.AnalyzeSchemaDone = false
		record.LastAnalyzeIssueCount = 0
		record.ImportSchemaDone = false
	})
	if err != nil {
		utils.ErrExit("reset schema phase flags in MSR: %w", err)
	}
}

func printSchemaAlreadyMigrated() {
	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓") + " Schema already migrated.")
	fmt.Println("    " + dimStyle.Render("Use --start-clean to re-run the entire schema migration."))
	printClosingProgress(StepImportSchema)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("Migrate data (export + import in one command):"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager data migrate"+buildMigrationNameFlag()))
	fmt.Println()
}

func printAnalyzeIssuesHint(count int) {
	fmt.Println()
	fmt.Println("  " + color.YellowString("⚠") + fmt.Sprintf(" Schema analyze reported %d issue(s). Schema migration is paused.", count))
	printClosingProgress(StepAnalyzeSchema)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:"))
	fmt.Println("    " + nextStepLabelStyle.Render("1. Review the analyze report above and fix the listed issues in the exported schema files."))
	fmt.Println("    " + nextStepLabelStyle.Render("   Schema directory: ") + dimStyle.Render(displayPath(fmt.Sprintf("%s/schema", exportDir))))
	fmt.Println("    " + nextStepLabelStyle.Render("2. Re-run schema migrate to continue:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager schema migrate"+buildMigrationNameFlag()))
	fmt.Println()
}

func printAutoApproveStop() {
	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓") + " Analyze reported 0 issues.")
	fmt.Println("    " + dimStyle.Render("--auto-approve=false — stopping before import."))
	printClosingProgress(StepAnalyzeSchema)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("Import the schema to your target YugabyteDB:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager schema migrate"+buildMigrationNameFlag()))
	fmt.Println()
}

func printSchemaMigrateComplete() {
	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓") + " Schema migration complete (export + analyze + import).")
	printClosingProgress(StepImportSchema)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("Migrate data (export + import in one command):"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager data migrate"+buildMigrationNameFlag()))
	fmt.Println()
}

func printSchemaMigrateFailure(phase string, runErr error) {
	fmt.Println()
	fmt.Println("  " + color.RedString("✗") + " " + phase + " failed: " + runErr.Error())
	fmt.Println("    " + dimStyle.Render("Review the output above for details."))
	// Mapping phase string -> step ID so the in-progress marker lands on the right phase.
	stepID := StepExportSchema
	switch phase {
	case "schema analyze":
		stepID = StepAnalyzeSchema
	case "schema import":
		stepID = StepImportSchema
	}
	printClosingProgress(stepID)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Retry:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager schema migrate"+buildMigrationNameFlag()))
	fmt.Println()
}
