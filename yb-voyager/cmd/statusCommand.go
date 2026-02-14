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

package cmd

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status and progress",
	Long: `Display the current state of a migration project.

Shows two sections:
  1. Migration Details — source, target, export directory, UUID, workflow
  2. Migration Progress — phase progress tree, last completed command, next step`,

	Run: func(cmd *cobra.Command, args []string) {
		runStatus()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	registerCommonGlobalFlags(statusCmd)
}

func runStatus() {
	// ── Load config file to read source/target details ──
	var v *viper.Viper
	if cfgFile != "" {
		v = viper.New()
		v.SetConfigType("yaml")
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			log.Warnf("failed to read config file for status display: %v", err)
			v = nil
		}
	}

	// ── Load MSR if metaDB is available ──
	var msr *metadb.MigrationStatusRecord
	if metaDB != nil {
		var err error
		msr, err = metaDB.GetMigrationStatusRecord()
		if err != nil {
			log.Warnf("failed to get migration status record: %v", err)
		}
	}

	// ── Section 1: Migration Details ──
	printMigrationDetails(v, msr)

	// ── Section 2: Migration Progress ──
	printMigrationProgress(v, msr)

	fmt.Println()
}

// printMigrationDetails renders Section 1 with source, target, and migration metadata.
func printMigrationDetails(v *viper.Viper, msr *metadb.MigrationStatusRecord) {
	var lines []string

	// Export directory
	lines = append(lines, formatKeyValue("Export Dir:", displayPath(exportDir), kvWidth))

	// UUID (from MSR only)
	if msr != nil && msr.MigrationUUID != "" {
		lines = append(lines, formatKeyValue("UUID:", msr.MigrationUUID, kvWidth))
	}

	// Workflow
	wfName := resolveWorkflowDisplayName(v, msr)
	if wfName != "" {
		lines = append(lines, formatKeyValue("Workflow:", wfName, kvWidth))
	}

	// Source details — prefer MSR, fall back to config file
	sourceLine := buildSourceLine(v, msr)
	if sourceLine != "" {
		lines = append(lines, formatKeyValue("Source:", sourceLine, kvWidth))
	}

	// Source schema
	sourceSchema := buildSourceSchema(v, msr)
	if sourceSchema != "" {
		lines = append(lines, formatKeyValue("Schema:", dimStyle.Render(sourceSchema), kvWidth))
	}

	// Target details — prefer MSR, fall back to config file
	targetLine := buildTargetLine(v, msr)
	if targetLine != "" {
		lines = append(lines, formatKeyValue("Target:", targetLine, kvWidth))
	}

	printSection("Migration Details", lines...)
}

// printMigrationProgress renders Section 2 with phase tree, last command, and next step.
func printMigrationProgress(v *viper.Viper, msr *metadb.MigrationStatusRecord) {
	wf := resolveWorkflow(msr)

	// Use empty currentStepID — we want status purely from MSR, not from a running command.
	phases := computePhaseStatuses(wf, msr, "")

	var progress []string

	// Phase tree
	if len(phases) > 0 {
		progress = append(progress, formatPhaseLines(phases)...)
	}

	padding := strings.Repeat(" ", kvWidth+1)

	// Last completed command
	lastStep := findLastCompletedStep(wf, msr)
	if lastStep != nil {
		if len(progress) > 0 {
			progress = append(progress, "")
		}
		progress = append(progress,
			formatKeyValue("Last Command:", fmt.Sprintf("%s — %s", lastStep.DisplayName, successStyle.Render("Completed")), kvWidth))
	}

	// Next step — resolve from workflow, but override with start-migration when
	// assessment is done and the target hasn't been configured yet.
	nextStepName, nextStepCmd := resolveNextStepForStatus(wf, msr, v)
	if nextStepName != "" {
		if len(progress) > 0 {
			progress = append(progress, "")
		}
		progress = append(progress, formatKeyValue("Next Step:", nextStepName, kvWidth))
		progress = append(progress, padding+cmdStyle.Render(nextStepCmd))
	}

	if msr == nil {
		progress = append(progress, "")
		progress = append(progress, dimStyle.Render("Migration has not started yet. Run assess-migration to begin."))
	}

	printSection("Migration Progress", progress...)
}

// resolveNextStepForStatus determines the next step to display.
// It checks the workflow for the next undone migration step, but interposes
// the administrative "start-migration" step when assessment is done and the
// target database has not yet been configured.
func resolveNextStepForStatus(wf *Workflow, msr *metadb.MigrationStatusRecord, v *viper.Viper) (name string, cmd string) {
	configFlag := buildConfigFlag()

	nextStep := findNextStep(wf, msr)
	if nextStep == nil {
		return "", ""
	}

	// If the next workflow step is export-schema (first Schema step) and the target
	// is not yet configured, the user needs to run start-migration first.
	if nextStep.ID == StepExportSchema && !targetConfigured(v, msr) {
		return "Start Migration", fmt.Sprintf("yb-voyager start-migration%s", configFlag)
	}

	return nextStep.DisplayName, fmt.Sprintf("yb-voyager %s%s", nextStep.Command, configFlag)
}

// buildConfigFlag returns the --config-file or --export-dir flag string for CLI commands.
func buildConfigFlag() string {
	if cfgFile != "" {
		return fmt.Sprintf(" --config-file %s", displayPath(cfgFile))
	}
	return fmt.Sprintf(" --export-dir %s", displayPath(exportDir))
}

// targetConfigured checks whether the target database has been configured beyond
// the template defaults. Used to detect whether start-migration has been run.
func targetConfigured(v *viper.Viper, msr *metadb.MigrationStatusRecord) bool {
	// If export schema (or any later step) is already done, target must be configured.
	if msr != nil && msr.ExportSchemaDone {
		return true
	}
	// Check the config file for target details that differ from template defaults.
	if v != nil {
		host := v.GetString("target.db-host")
		user := v.GetString("target.db-user")
		if host != "" && host != "127.0.0.1" && user != "" && user != "test_user" {
			return true
		}
	}
	return false
}

// resolveWorkflowDisplayName returns a human-readable workflow name from MSR or config.
func resolveWorkflowDisplayName(v *viper.Viper, msr *metadb.MigrationStatusRecord) string {
	wf := resolveWorkflow(msr)
	return workflowDisplayName(wf.Name)
}

// buildSourceLine creates a formatted "DBType @ host:port/dbname" string for the source.
func buildSourceLine(v *viper.Viper, msr *metadb.MigrationStatusRecord) string {
	// Prefer MSR source details (set after assess/export)
	if msr != nil && msr.SourceDBConf != nil {
		src := msr.SourceDBConf
		if src.DBType != "" {
			return fmt.Sprintf("%s @ %s:%d/%s", titleCase(src.DBType), src.Host, src.Port, src.DBName)
		}
	}
	// Fall back to config file
	if v != nil {
		dbType := v.GetString("source.db-type")
		host := v.GetString("source.db-host")
		port := v.GetInt("source.db-port")
		dbName := v.GetString("source.db-name")
		if dbType != "" {
			return fmt.Sprintf("%s @ %s:%d/%s", titleCase(dbType), host, port, dbName)
		}
	}
	return ""
}

// buildSourceSchema returns the source schema list string.
func buildSourceSchema(v *viper.Viper, msr *metadb.MigrationStatusRecord) string {
	if msr != nil && msr.SourceDBConf != nil && msr.SourceDBConf.SchemaConfig != "" {
		return msr.SourceDBConf.SchemaConfig
	}
	if v != nil {
		schema := v.GetString("source.db-schema")
		if schema != "" {
			return schema
		}
	}
	return ""
}

// buildTargetLine creates a formatted "YugabyteDB @ host:port/dbname" string for the target.
func buildTargetLine(v *viper.Viper, msr *metadb.MigrationStatusRecord) string {
	// Prefer MSR target details (set after import data starts)
	if msr != nil && msr.TargetDBConf != nil {
		tgt := msr.TargetDBConf
		if tgt.Host != "" {
			return fmt.Sprintf("YugabyteDB @ %s:%d/%s", tgt.Host, tgt.Port, tgt.DBName)
		}
	}
	// Fall back to config file
	if v != nil {
		host := v.GetString("target.db-host")
		port := v.GetInt("target.db-port")
		dbName := v.GetString("target.db-name")
		if host != "" {
			return fmt.Sprintf("YugabyteDB @ %s:%d/%s", host, port, dbName)
		}
	}
	return ""
}
