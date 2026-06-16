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

// `data migrate` orchestrates `export data` -> `import data` as one durable
// snapshot-only data-migration workflow.
//
// Output strategy:
//
//	Children run as subprocesses with their stdout/stderr piped straight
//	through to the orchestrator so the user sees the live per-table progress
//	tables in real time (unlike schema-migrate, where phases are short and
//	output is captured behind a spinner). The child's repeated startup
//	banners, Migration Progress block, and Next-step rows are suppressed via
//	the same env-var gates used by schema-migrate; the orchestrator prints
//	one consolidated Migration Progress + Next-step at the end.
//
// Scope: snapshot-only migrations only. Live (snapshot-and-changes) is
// rejected up-front because the parent->child sequential model doesn't fit a
// streaming concurrent flow; live users stay on the split `export data` /
// `import data` commands.
//
// Durability: skip export if ExportDataDone, skip import if ImportDataDone.
// If both done -> "data already migrated". --start-clean resets both flags
// before running and forwards --start-clean=true to both children.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var dataMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run export and import of data as one durable workflow (snapshot-only).",
	Run: func(cmd *cobra.Command, args []string) {
		dataMigrateRunningCmd = cmd
		runDataMigrate()
	},
}

// Captured in Run before runDataMigrate dispatches to child phases. Lets
// buildChildDataArgs walk Flags().Visit() without forming a package-init
// reference cycle through dataMigrateCmd.
var dataMigrateRunningCmd *cobra.Command

func init() {
	dataCmd.AddCommand(dataMigrateCmd)
	registerCommonGlobalFlags(dataMigrateCmd)
	// Source/target/import/export flags are added later via wireDataMigrateFlagUnion()
	// — by Execute() time, exportDataCmd and importDataCmd have their own flag
	// sets fully populated, and we copy the union into dataMigrateCmd.
}

// Populated by wireDataMigrateFlagUnion(); used by runChildDataPhase to decide
// whether a given flag should be forwarded to the export child, the import
// child, or both.
var (
	dataMigrateExportFlagNames = map[string]bool{}
	dataMigrateImportFlagNames = map[string]bool{}
)

// wireDataMigrateFlagUnion copies the union of flags from exportDataCmd and
// importDataCmd into dataMigrateCmd. Same backing pflag.Value pointers — so a
// `--target-db-host=...` on `data migrate` writes through to the same global
// var that the import child reads when invoked directly. Called from
// cmd.Execute() after all package init()s have run.
func wireDataMigrateFlagUnion() {
	copyFlagsInto(dataMigrateCmd, exportDataCmd, dataMigrateExportFlagNames)
	copyFlagsInto(dataMigrateCmd, importDataCmd, dataMigrateImportFlagNames)
}

func copyFlagsInto(dst, src *cobra.Command, capturedNames map[string]bool) {
	src.Flags().VisitAll(func(f *pflag.Flag) {
		capturedNames[f.Name] = true
		if dst.Flags().Lookup(f.Name) != nil {
			return // already present (manually registered or from prior copy)
		}
		// Copy without the "required" annotation — `data migrate` accepts these
		// values from the config file, so marking them required on the parent
		// would force users to repeat them on the command line. The child
		// subprocess still enforces required-ness in its own pre-run.
		copied := *f
		if copied.Annotations != nil {
			a := make(map[string][]string, len(copied.Annotations))
			for k, v := range copied.Annotations {
				if k == cobra.BashCompOneRequiredFlag {
					continue
				}
				a[k] = v
			}
			copied.Annotations = a
		}
		dst.Flags().AddFlag(&copied)
	})
}

func runDataMigrate() {
	if metaDB == nil {
		utils.ErrExit("meta DB not initialized — run `yb-voyager new` first to set up this migration project.")
	}

	// Refuse live mode up-front. Users on live migrations should stay on the
	// split export-data / import-data commands which run concurrently.
	rejectIfLiveModeConfigured()

	if bool(startClean) {
		resetDataMigrateMSRFlags()
	}

	msr := initialReadOfDataMSR()

	if msr.ImportDataDone && !bool(startClean) {
		printDataAlreadyMigrated()
		return
	}

	// Phase 1: Export data
	if msr.ExportDataDone && !bool(startClean) {
		printPhaseSkipped("export data", "already exported")
	} else {
		if err := runChildDataPhase("export"); err != nil {
			printDataMigrateFailure("export data", err)
			os.Exit(exitCodeFrom(err))
		}
	}

	// Phase 2: Import data
	if err := runChildDataPhase("import"); err != nil {
		printDataMigrateFailure("import data", err)
		os.Exit(exitCodeFrom(err))
	}

	printDataMigrateComplete()
}

// rejectIfLiveModeConfigured errors out if the user has configured live
// migration (snapshot-and-changes) via flag or config file. data migrate
// always hardcodes snapshot-only when invoking the children.
func rejectIfLiveModeConfigured() {
	rec, err := metaDB.GetMigrationStatusRecord()
	if err == nil && rec != nil && rec.ExportType != "" && rec.ExportType != utils.SNAPSHOT_ONLY {
		utils.ErrExit(
			"`yb-voyager data migrate` only supports snapshot-only migrations.\n" +
				"  For live migrations (snapshot-and-changes / cutover), use the split commands:\n" +
				"    yb-voyager export data   # in one terminal\n" +
				"    yb-voyager import data   # in another terminal",
		)
	}
}

// runChildDataPhase invokes `yb-voyager <phase> data` as a subprocess with
// stdout+stderr connected straight through to the orchestrator so live
// progress UIs render in real time. Output is also tee'd to an in-memory
// buffer so we can dump it to a log file on failure.
func runChildDataPhase(phase string) error {
	args := buildChildDataArgs(phase)

	binary, err := os.Executable()
	if err != nil {
		binary = "yb-voyager"
	}

	cmd := exec.Command(binary, args...)
	// Tee child stdout/stderr to the user's terminal AND an in-memory buffer
	// (so we have something to dump to a log file on failure).
	var outBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &outBuf)
	cmd.Stdin = os.Stdin
	cmd.Env = orchestratorChildEnv()

	printDataPhaseHeader(phase)

	runErr := cmd.Run()
	if runErr != nil {
		logPath := writeOrchestratorPhaseLog("data-migrate", phase, &outBuf)
		if logPath != "" {
			fmt.Println()
			fmt.Println("  " + dimStyle.Render("Full log: "+displayPath(logPath)))
		}
		return runErr
	}
	return nil
}

func buildChildDataArgs(phase string) []string {
	// `yb-voyager data export` and `yb-voyager data import` are the registered
	// invocation paths (under the `data` parent).
	args := []string{"data", phase, "--yes"}

	if migrationName != "" {
		args = append(args, "--migration-name", migrationName)
	}
	if exportDir != "" {
		args = append(args, "--export-dir", exportDir)
	}
	if cfgFile != "" {
		args = append(args, "--config-file", cfgFile)
	}
	// Always hardcode snapshot-only on the export child. Import doesn't have
	// an --export-type flag.
	if phase == "export" {
		args = append(args, "--export-type", utils.SNAPSHOT_ONLY)
	}

	// Forward every user-set flag to the child(ren) that accept it. Flags
	// already covered manually above (migration-name, export-dir, config-file,
	// yes, export-type) are excluded so they don't appear twice in argv.
	manuallyForwarded := map[string]bool{
		"migration-name": true,
		"export-dir":     true,
		"config-file":    true,
		"yes":            true,
		"export-type":    true,
	}
	dataMigrateRunningCmd.Flags().Visit(func(f *pflag.Flag) {
		if manuallyForwarded[f.Name] {
			return
		}
		var belongs bool
		if phase == "export" {
			belongs = dataMigrateExportFlagNames[f.Name]
		} else {
			belongs = dataMigrateImportFlagNames[f.Name]
		}
		if !belongs {
			return
		}
		args = append(args, fmt.Sprintf("--%s=%s", f.Name, f.Value.String()))
	})

	return args
}

func printDataPhaseHeader(phase string) {
	fmt.Println()
	switch phase {
	case "export":
		fmt.Println("  " + nextStepLabelStyle.Render("→ Exporting data from source"))
	case "import":
		fmt.Println("  " + nextStepLabelStyle.Render("→ Importing data into target"))
	}
}

// initialReadOfDataMSR returns a narrow view of MSR for data-phase decisions.
func initialReadOfDataMSR() *DataMSRSnapshot {
	rec, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("read migration status record: %w", err)
	}
	if rec == nil {
		return &DataMSRSnapshot{}
	}
	return &DataMSRSnapshot{
		ExportDataDone: rec.ExportDataDone,
		ImportDataDone: rec.ImportDataDone,
		ExportType:     rec.ExportType,
	}
}

type DataMSRSnapshot struct {
	ExportDataDone bool
	ImportDataDone bool
	ExportType     string
}

// resetDataMigrateMSRFlags clears the two data-phase flags so a subsequent
// invocation doesn't short-circuit on stale state when --start-clean was
// passed but a child paused or errored before reaching the import phase.
func resetDataMigrateMSRFlags() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportDataDone = false
		record.ImportDataDone = false
	})
	if err != nil {
		utils.ErrExit("reset data phase flags in MSR: %w", err)
	}
}

func printDataAlreadyMigrated() {
	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓") + " Data already migrated.")
	fmt.Println("    " + dimStyle.Render("Use --start-clean to re-run the entire data migration."))
	printClosingProgress(StepImportData)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("End the migration and clean up metadata:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager end"+buildMigrationNameFlag()))
	fmt.Println()
}

func printDataMigrateComplete() {
	fmt.Println()
	fmt.Println("  " + successStyle.Render("✓") + " Data migration complete (export + import).")
	printClosingProgress(StepImportData)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Next step:") + " " + nextStepLabelStyle.Render("End the migration and clean up metadata:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager end"+buildMigrationNameFlag()))
	fmt.Println()
}

func printDataMigrateFailure(phase string, runErr error) {
	fmt.Println()
	fmt.Println("  " + color.RedString("✗") + " " + phase + " failed: " + runErr.Error())
	fmt.Println("    " + dimStyle.Render("Review the output above for details."))
	stepID := StepExportData
	if phase == "import data" {
		stepID = StepImportData
	}
	printClosingProgress(stepID)
	fmt.Println()
	fmt.Println("  " + nextStepLabelStyle.Render("Retry:"))
	fmt.Println("    " + cmdStyle.Render("yb-voyager data migrate"+buildMigrationNameFlag()))
	fmt.Println()
}
