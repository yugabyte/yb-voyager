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
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/compareperf"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var comparePerformanceCmd = &cobra.Command{
	Use:   "compare-performance",
	Short: "Compare query performance between source and target YugabyteDB",
	Long: `Compare query performance between source and target YugabyteDB.

This command analyzes stats collected during assess-migration from the source database
and compares it with stats collected from the target YugabyteDB database.

Prerequisites:
  - assess-migration command must have been run and collected the stats from the source database
  - Source workload should have been executed on both source and target database
  - stats collection(pg_stat_statements) must be enabled on the target YugabyteDB database`,

	Hidden: true, // Hide command until fully implemented

	PreRun: func(cmd *cobra.Command, args []string) {
		tconf.TargetDBType = YUGABYTEDB
		importerRole = TARGET_DB_IMPORTER_ROLE
		checkOrSetDefaultTargetSSLMode()
		validateTargetPortRange()  // Sets default port if needed
		validateTargetSchemaFlag() // Sets default schema based on DB type
		getTargetPassword(cmd)
		validateComparePerfPrerequisites()
	},

	Run: comparePerformanceCommandFn,
}

func comparePerformanceCommandFn(cmd *cobra.Command, args []string) {
	utils.PrintAndLog("Starting performance comparison...")

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %v", err)
	}

	// TODO scenario: what if user provided assessment-metadata-dir flag in assessment command if they manually ran the gather scripts?
	assessmentDirPath := filepath.Join(exportDir, "assessment")
	migassessment.AssessmentDir = assessmentDirPath
	assessmentDB, err := migassessment.NewAssessmentDB()
	if err != nil {
		utils.ErrExit("Failed to create assessment database: %v", err)
	}

	targetDB := tgtdb.NewTargetDB(&tconf)
	err = targetDB.Init()
	if err != nil {
		utils.ErrExit("Failed to initialize target database: %v", err)
	}
	defer targetDB.Finalize()
	ybTarget, ok := targetDB.(*tgtdb.TargetYugabyteDB)
	if !ok {
		utils.ErrExit("compare-performance: target database is not YugabyteDB")
	}

	comparator, err := compareperf.NewQueryPerformanceComparator(msr, assessmentDB, ybTarget)
	if err != nil {
		utils.ErrExit("Failed to create query performance comparator: %v", err)
	}
	err = comparator.Compare()
	if err != nil {
		utils.ErrExit("Failed to perform performance comparison: %v", err)
	}
	err = comparator.GenerateReport(exportDir)
	if err != nil {
		utils.ErrExit("Failed to generate performance reports: %v", err)
	}

	utils.PrintAndLog("Performance comparison completed successfully!")
}

// TODO: add check if report already exists(or MSR); ask for --start-clean
func validateComparePerfPrerequisites() {
	utils.PrintAndLog("Validating prerequisites...")

	// Check 0: source db type(postgres) by fetching from MetaDB
	dbType := GetSourceDBTypeFromMSR()
	if dbType != POSTGRESQL {
		utils.ErrExit("Only PostgreSQL is supported for performance comparison.")
	}

	// Check 1: Assessment database exists and is accessible
	utils.PrintAndLog("Checking assessment database...")
	// Set AssessmentDir before accessing assessment database functions
	migassessment.AssessmentDir = filepath.Join(exportDir, "assessment")
	assessmentDBPath := migassessment.GetSourceMetadataDBFilePath()
	if _, err := os.Stat(assessmentDBPath); os.IsNotExist(err) {
		utils.ErrExit("Assessment database not found. Please run 'assess-migration' command first.")
	}
	adb, err := migassessment.NewAssessmentDB()
	if err != nil {
		utils.ErrExit("Failed to open assessment database: %v", err)
	}

	// Check 2: Source PGSS data exists in assessment DB
	utils.PrintAndLog("Checking source pg_stat_statements data...")
	hasData, err := adb.HasSourceQueryStats()
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
