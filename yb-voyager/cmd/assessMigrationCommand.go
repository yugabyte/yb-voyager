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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var supportedMigrationReportFormats = []string{"json", "html"}
var userInputFpath string
var assessmentReportFormat string
var metadataAndStatsDir string

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: "Assess the migration from source database to YugabyteDB.",
	Long:  `Assess the migration from source database to YugabyteDB.`,

	Run: func(cmd *cobra.Command, args []string) {
		err := assessMigration()
		if err != nil {
			utils.ErrExit("failed to assess migration: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(assessMigrationCmd)
	registerCommonGlobalFlags(assessMigrationCmd)

	// TODO: clarity on whether this flag should be a mandatory or not
	assessMigrationCmd.Flags().StringVar(&userInputFpath, "user-input", "",
		"File path for user input to the plugins. This file should contain the user input for the plugins in TOML format.")

	assessMigrationCmd.Flags().StringVar(&assessmentReportFormat, "report-format", "json",
		fmt.Sprintf("Output format for migration assessment report. Supported formats are: %s.",
			strings.Join(supportedMigrationReportFormats, ", ")))

	// optional flag to take metadata and stats directory path in case it is not in exportDir
	assessMigrationCmd.Flags().StringVar(&metadataAndStatsDir, "metadata-and-stats-dir", "",
		"Directory path where metadata and stats are stored. Optional flag, if not provided, "+
			"it will be assumed to be present at default path inside the export directory.")

}

// TODO embed a template file
var bytesTemplate []byte

func assessMigration() error {
	log.Infof("Assessing migration from source database to YugabyteDB...")
	migassessment.ExportDir = exportDir

	// err := loadAssessmentParams() // Sets `params`

	err := migassessment.ShardingAssessment()
	if err != nil {
		log.Errorf("failed to perform sharding assessment: %v", err)
		return fmt.Errorf("failed to perform sharding assessment: %w", err)
	}

	err = migassessment.SizingAssessment()
	if err != nil {
		log.Errorf("failed to perform sizing assessment: %v", err)
		return fmt.Errorf("failed to perform sizing assessment: %w", err)
	}

	reportFilePath := filepath.Join(exportDir, "assessment", "reports", "report."+assessmentReportFormat)
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

	// Write strReport to report.*
	err = os.WriteFile(reportFilePath, assessmentReportContent.Bytes(), 0644)
	if err != nil {
		log.Errorf("failed to write the assessment report: %v", err)
		return fmt.Errorf("failed to write the assessment report: %w", err)
	}

	return nil
}
