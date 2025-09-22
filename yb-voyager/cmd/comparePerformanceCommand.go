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
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/compareperf"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
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
	utils.PrintAndLog("starting performance comparison...")

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
	utils.PrintAndLog("generating performance reports...\n")
	err = comparator.GenerateReport(exportDir)
	if err != nil {
		utils.ErrExit("Failed to generate performance reports: %v", err)
	}

	err = SetPerformanceComparisonDone()
	if err != nil {
		utils.ErrExit("Failed to mark performance comparison as done: %v", err)
	}

	utils.PrintAndLog("Performance comparison completed successfully!")
}

func validateComparePerfPrerequisites() {
	// Handle start-clean flag and existing reports before doing any other checks (TODO refactor to be the same in assess-migration start clean code)
	err := handleStartCleanForComparePerf()
	if err != nil {
		utils.ErrExit("Failed to handle start-clean: %v", err)
	}

	utils.PrintAndLog("validating the setup for performance comparison...")

	// Check 0: source db type(postgres) by fetching from MetaDB
	dbType := GetSourceDBTypeFromMSR()
	if dbType != POSTGRESQL {
		utils.ErrExit("Only PostgreSQL is supported for performance comparison.")
	}

	// Check 1: Assessment database exists and is accessible
	log.Infof("checking assessment database...")
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
	log.Infof("checking source pg_stat_statements data...")
	hasData, err := adb.HasSourceQueryStats()
	if err != nil {
		utils.ErrExit("Failed to verify pg_stat_statements data in assessment database: %v", err)
	}
	if !hasData {
		utils.ErrExit("No pg_stat_statements data found in assessment database. Please ensure pg_stat_statements extension was enabled during assess-migration and workload was executed on source database.")
	}

	// Check 3: Target database is reachable
	log.Infof("checking target database connection...")
	tdb := tgtdb.NewTargetDB(&tconf)
	err = tdb.Init()
	if err != nil {
		utils.ErrExit("Failed to initialize target database: %v", err)
	}
	defer tdb.Finalize()

	// Check 4: pg_stat_statements extension is enabled on target database
	log.Infof("checking pg_stat_statements extension on target database...")
	_, err = tdb.Query("SELECT 1 FROM pg_stat_statements LIMIT 1")
	if err != nil {
		utils.ErrExit("pg_stat_statements extension is not available on target database: %v\n"+
			"Please ensure the extension is installed and enabled.", err)
	}

	log.Infof("setup validated successfully for performance comparison\n")
}

// Helper functions for MSR tracking
func IsPerformanceComparisonDone() (bool, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return false, fmt.Errorf("failed to get migration status record: %w", err)
	}
	if msr == nil {
		return false, nil
	}
	return msr.PerformanceComparisonDone, nil
}

func ClearPerformanceComparisonDone() error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.PerformanceComparisonDone = false
	})
	if err != nil {
		return fmt.Errorf("failed to clear performance comparison done flag: %w", err)
	}
	return nil
}

func SetPerformanceComparisonDone() error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.PerformanceComparisonDone = true
	})
	if err != nil {
		return fmt.Errorf("failed to set performance comparison done flag: %w", err)
	}
	return nil
}

func handleStartCleanForComparePerf() error {
	reportsDir := filepath.Join(exportDir, "reports")
	htmlReportPath := filepath.Join(reportsDir, "performance-comparison-report.html")
	jsonReportPath := filepath.Join(reportsDir, "performance-comparison-report.json")

	reportsExist := utils.FileOrFolderExists(htmlReportPath) || utils.FileOrFolderExists(jsonReportPath)

	isComparisonDone, err := IsPerformanceComparisonDone()
	if err != nil {
		return fmt.Errorf("failed to check if performance comparison is done: %w", err)
	}

	needCleanupOfLeftoverFiles := reportsExist && !isComparisonDone
	if bool(startClean) || needCleanupOfLeftoverFiles {
		// just cleaning up the compare performance report files
		if err := os.Remove(htmlReportPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove HTML report: %w", err)
		}
		if err := os.Remove(jsonReportPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove JSON report: %w", err)
		}

		err = ClearPerformanceComparisonDone()
		if err != nil {
			return fmt.Errorf("failed to clear performance comparison done flag: %w", err)
		}

		if bool(startClean) {
			utils.PrintAndLog("cleaned up existing performance comparison reports")
		} else {
			utils.PrintAndLog("cleaned up leftover performance comparison files from previous incomplete run")
		}
	} else if reportsExist {
		return fmt.Errorf("performance comparison reports already exist. Use --start-clean flag to overwrite them")
	}

	return nil
}

func init() {
	// Register the command with root
	rootCmd.AddCommand(comparePerformanceCmd)

	registerCommonGlobalFlags(comparePerformanceCmd)
	registerTargetDBConnFlags(comparePerformanceCmd)

	BoolVar(comparePerformanceCmd.Flags(), &startClean, "start-clean", false,
		"Clean up existing performance comparison reports before generating new ones (default false)")
}
