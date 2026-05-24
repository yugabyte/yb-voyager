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
// Design:
//   - Subprocess invocation of each underlying phase ("yb-voyager schema <phase>"),
//     stdout/stderr streamed through so the user sees each phase's own footer.
//   - VOYAGER_SUPPRESS_NEXT_STEP=1 is set on children so they skip the "Next step"
//     line — the orchestrator emits its own at the end.
//   - Durability is read from MSR before each phase: skip export if ExportSchemaDone,
//     skip analyze if AnalyzeSchemaDone && LastAnalyzeIssueCount == 0, skip everything
//     if ImportSchemaDone.
//   - If analyze finishes with > 0 issues, stop with a hint to fix and re-run.
//   - --auto-approve=false stops before import even on a clean analyze.
//
// To revert: delete this file and re-expose the three subcommands (un-set Hidden:true).
// Re-point startMigrationCommand and statusCommand next-step suggestions back to "schema export".

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var schemaMigrateAutoApprove utils.BoolStr = true

var schemaMigrateCmd = &cobra.Command{
	Use:     "migrate",
	Short:   "Run schema export, analyze, and import as one durable iterative workflow.",
	Aliases: []string{},

	Run: func(cmd *cobra.Command, args []string) {
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
}

func runSchemaMigrate() {
	if metaDB == nil {
		utils.ErrExit("meta DB not initialized — run `yb-voyager new` first to set up this migration project.")
	}

	msr := initialReadOfMSR()

	if msr.ImportSchemaDone {
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

// runChildSchemaPhase invokes `yb-voyager schema <phase>` as a subprocess with the
// orchestrator's known-good flags forwarded, --yes for unattended prompts, and
// VOYAGER_SUPPRESS_NEXT_STEP=1 so the child skips its per-phase "Next step" footer.
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
	if bool(startClean) && (phase == "export" || phase == "import") {
		args = append(args, "--start-clean")
	}

	binary, err := os.Executable()
	if err != nil {
		binary = "yb-voyager"
	}

	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"VOYAGER_SUPPRESS_NEXT_STEP=1",
		"CLICOLOR_FORCE=1",
	)

	return cmd.Run()
}

func exitCodeFrom(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
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

func printPhaseSkipped(phase, reason string) {
	fmt.Println()
	fmt.Println("  " + dimStyle.Render(fmt.Sprintf("Skipping %s — %s. (Pass --start-clean to re-run from scratch.)", phase, reason)))
}

func printSchemaAlreadyMigrated() {
	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓") + " Schema already migrated.")
	fmt.Println("    " + dimStyle.Render("Use --start-clean to re-run the entire schema migration, or run individual hidden subcommands for surgical fixes."))
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("Export data from your source database:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager data export"+buildMigrationNameFlag()))
	fmt.Println()
}

func printAnalyzeIssuesHint(count int) {
	fmt.Println()
	fmt.Println("  " + color.YellowString("⚠") + fmt.Sprintf(" Schema analyze reported %d issue(s). Schema migration is paused.", count))
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
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("Import the schema to your target YugabyteDB:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager schema migrate"+buildMigrationNameFlag()))
	fmt.Println()
}

func printSchemaMigrateComplete() {
	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓") + " Schema migration complete (export + analyze + import).")
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("Export data from your source database:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager data export"+buildMigrationNameFlag()))
	fmt.Println()
}

func printSchemaMigrateFailure(phase string, runErr error) {
	fmt.Println()
	fmt.Println("  " + color.RedString("✗") + " " + phase + " failed: " + runErr.Error())
	fmt.Println("    " + dimStyle.Render("Review the output above for details."))
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Retry:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager schema migrate"+buildMigrationNameFlag()))
	fmt.Println()
}
