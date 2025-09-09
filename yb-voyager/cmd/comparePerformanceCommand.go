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
	"os"

	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var comparePerformanceCmd = &cobra.Command{
	Use:   "compare-performance",
	Short: "Compare query performance between source PostgreSQL and target YugabyteDB",
	Long: `Compare query performance between source PostgreSQL and target YugabyteDB using pg_stat_statements data.

This command analyzes pg_stat_statements data collected during assess-migration from the source database
and compares it with current pg_stat_statements data from the target YugabyteDB database.

Prerequisites:
  - assess-migration command must have been run with PGSS data collection
  - Source workload should have been executed on both source and target databases
  - pg_stat_statements extension must be enabled on the target YugabyteDB database`,

	Hidden: true, // Hide command until fully implemented

	PreRun: func(cmd *cobra.Command, args []string) {
		validatePrerequisites()
	},

	Run: comparePerformanceHandler,
}

func comparePerformanceHandler(cmd *cobra.Command, args []string) {
	utils.PrintAndLog("Starting performance comparison...")

	err := collectTargetPgssData()
	if err != nil {
		utils.ErrExit("Target PGSS collection failed: %v", err)
	}

	err = performAnalysisAndGenerateReport()
	if err != nil {
		utils.ErrExit("Performance analysis failed: %v", err)
	}

	utils.PrintAndLog("Performance comparison completed successfully!")
}

// collectTargetPgssData collects current PGSS data from target database (STUB)
func collectTargetPgssData() error {
	// TODO: Implement target PGSS data collection
	// This should:
	// 1. Connect to target YugabyteDB database
	// 2. Query pg_stat_statements view
	// 3. Handle multi-node cluster data aggregation
	// 4. Return PGSS data as pgss.QueryStats slice

	return nil
}

// performAnalysisAndGenerateReport performs performance analysis and generates reports (STUB)
func performAnalysisAndGenerateReport() error {
	// TODO: Implement performance analysis and report generation
	// This should:
	// 1. Match queries between source and target
	// 2. Calculate performance metrics (slowdown ratios, impact)
	// 3. Generate HTML and JSON reports with multiple views
	// 4. Save reports to output directory

	return nil
}

func validatePrerequisites() {
	utils.PrintAndLog("Validating prerequisites...")

	// Check 1: Assessment database exists and is accessible
	utils.PrintAndLog("Checking assessment database...")
	assessmentDBPath := migassessment.GetSourceMetadataDBFilePath()
	if _, err := os.Stat(assessmentDBPath); os.IsNotExist(err) {
		utils.ErrExit("Assessment database not found. Please run 'assess-migration' command first.")
	}

	adb, err := migassessment.NewAssessmentDB("")
	if err != nil {
		utils.ErrExit("Failed to open assessment database: %v", err)
	}

	// Check 2: Source PGSS data exists in assessment DB
	utils.PrintAndLog("Checking source pg_stat_statements data...")
	hasData, err := adb.HasPgssData()
	if err != nil {
		utils.ErrExit("Failed to verify pg_stat_statements data in assessment database: %v", err)
	}
	if !hasData {
		utils.ErrExit("No pg_stat_statements data found in assessment database. Please ensure pg_stat_statements extension was enabled during assess-migration and workload was executed on source database.")
	}

	// Check 3: Target database is reachable
	utils.PrintAndLog("Checking target database connection...")
	tdb := tgtdb.NewTargetDB(&tconf)
	err = tdb.Init()
	if err != nil {
		utils.ErrExit("Failed to initialize target database: %v", err)
	}
	defer tdb.Finalize()

	// Check 4: pg_stat_statements extension is enabled on target database
	utils.PrintAndLog("Checking pg_stat_statements extension on target database...")
	_, err = tdb.Query("SELECT 1 FROM pg_stat_statements LIMIT 1")
	if err != nil {
		utils.ErrExit("pg_stat_statements extension is not available on target database: %v\n"+
			"Please ensure the extension is installed and enabled.", err)
	}

	utils.PrintAndLog("Prerequisites validated successfully for performance comparison\n")
}

func init() {
	// Register the command with root
	rootCmd.AddCommand(comparePerformanceCmd)

	registerCommonGlobalFlags(comparePerformanceCmd)
	registerTargetDBConnFlags(comparePerformanceCmd)
}
