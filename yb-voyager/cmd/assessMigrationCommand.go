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
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var supportedAssessmentReportFormats = []string{"json", "html"}
var assessmentParamsFpath string

var metadataAndStatsDir string

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: "Assess the migration from source database to YugabyteDB.",
	Long:  `Assess the migration from source database to YugabyteDB.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		setSourceDefaultPort()
	},

	Run: func(cmd *cobra.Command, args []string) {
		CreateMigrationProjectIfNotExists(source.DBType, exportDir)

		err := assessMigration()
		if err != nil {
			utils.ErrExit("failed to assess migration: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(assessMigrationCmd)
	registerCommonGlobalFlags(assessMigrationCmd)
	registerSourceDBConnFlags(assessMigrationCmd, false)

	// TODO: clarity on whether this flag should be a mandatory or not
	assessMigrationCmd.Flags().StringVar(&assessmentParamsFpath, "assessment-params", "",
		"TOML file path to the user provided assessment params.")

	BoolVar(assessMigrationCmd.Flags(), &startClean, "start-clean", false,
		"cleans up the project directory for schema or data files depending on the export command (default false)")

	// optional flag to take metadata and stats directory path in case it is not in exportDir
	assessMigrationCmd.Flags().StringVar(&metadataAndStatsDir, "metadata-and-stats-dir", "",
		"Directory path where metadata and stats are stored. Optional flag, if not provided, "+
			"it will be assumed to be present at default path inside the export directory.")
}

//go:embed report.template
var bytesTemplate []byte

func assessMigration() error {
	checkStartCleanForAssessMigration()

	err := gatherMetadataAndStats()
	if err != nil {
		return fmt.Errorf("failed to gather metadata and stats: %w", err)
	}

	log.Infof("Assessing migration from source database to YugabyteDB...")
	if metadataAndStatsDir != "" {
		migassessment.AssessmentDataDir = metadataAndStatsDir
	} else {
		migassessment.AssessmentDataDir = filepath.Join(exportDir, "assessment", "data")
	}

	// load and sets 'assessmentParams' from the user input file
	err = migassessment.LoadAssessmentParams(assessmentParamsFpath)
	if err != nil {
		log.Errorf("failed to load assessment parameters: %v", err)
		return fmt.Errorf("failed to load assessment parameters: %w", err)

	}

	err = migassessment.ShardingAssessment()
	if err != nil {
		log.Errorf("failed to perform sharding assessment: %v", err)
		return fmt.Errorf("failed to perform sharding assessment: %w", err)
	}

	// err := migassessment.SizingAssessment()

	err = generateAssessmentReport()
	if err != nil {
		log.Errorf("failed to generate assessment report: %v", err)
		return fmt.Errorf("failed to generate assessment report: %w", err)
	}

	utils.PrintAndLog("Migration assessment completed successfully.")
	return nil
}

func generateAssessmentReport() error {
	utils.PrintAndLog("Generating assessment reports...")
	reportsDir := filepath.Join(exportDir, "assessment", "reports")
	for _, assessmentReportFormat := range supportedAssessmentReportFormats {
		reportFilePath := filepath.Join(reportsDir, "report."+assessmentReportFormat)
		var assessmentReportContent bytes.Buffer
		switch assessmentReportFormat {
		case "json":
			strReport, err := json.MarshalIndent(&migassessment.FinalReport, "", "\t")
			if err != nil {
				log.Errorf("failed to marshal the assessment report: %v", err)
				return fmt.Errorf("failed to marshal the assessment report: %w", err)
			}

			_, err = assessmentReportContent.Write(strReport)
			if err != nil {
				log.Errorf("failed to write assessment report to buffer: %v", err)
				return fmt.Errorf("failed to write assessment report to buffer: %w", err)
			}
		case "html":
			templ := template.Must(template.New("report").Parse(string(bytesTemplate)))
			err := templ.Execute(&assessmentReportContent, migassessment.FinalReport)
			if err != nil {
				log.Errorf("failed to render the assessment report: %v", err)
				return fmt.Errorf("failed to render the assessment report: %w", err)
			}
		}

		log.Infof("writing assessment report to file: %s", reportFilePath)
		err := os.WriteFile(reportFilePath, assessmentReportContent.Bytes(), 0644)
		if err != nil {
			log.Errorf("failed to write the assessment report: %v", err)
			return fmt.Errorf("failed to write the assessment report: %w", err)
		}
	}
	utils.PrintAndLog("Generated assessment reports at '%s'", reportsDir)
	return nil
}

func checkStartCleanForAssessMigration() {
	assessmentDir := filepath.Join(exportDir, "assessment")
	dataFilesPattern := filepath.Join(assessmentDir, "data", "*.csv")
	reportsFilePattern := filepath.Join(assessmentDir, "reports", "report.*")

	if utils.FileOrFolderExistsWithGlobPattern(dataFilesPattern) || utils.FileOrFolderExistsWithGlobPattern(reportsFilePattern) {
		if startClean {
			utils.CleanDir(filepath.Join(exportDir, "assessment", "data"))
			utils.CleanDir(filepath.Join(exportDir, "assessment", "reports"))
		} else {
			utils.ErrExit("metadata or reports files already exist in the assessment directory at '%s'. ", assessmentDir)
		}
	}
}
