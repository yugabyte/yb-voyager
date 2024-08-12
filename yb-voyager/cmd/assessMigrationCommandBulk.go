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
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var bulkAssessmentDir string
var fleetConfigPath string
var continueOnError utils.BoolStr
var bulkAssessmentReport BulkAssessmentReport

type AssessMigrationDBConfig struct {
	DbType      string
	Host        string
	Port        string
	ServiceName string
	SID         string
	TnsAlias    string
	User        string
	Password    string
	Schema      string
}

var assessMigrationBulkCmd = &cobra.Command{
	Use:   "assess-migration-bulk",
	Short: "Bulk Assessment of multiple schemas across one or more Oracle database instances",
	Long:  "Bulk Assessment of multiple schemas across one or more Oracle database instances",

	Run: func(cmd *cobra.Command, args []string) {
		assessMigrationBulk()
	},
}

func init() {
	rootCmd.AddCommand(assessMigrationBulkCmd)

	// defining flags
	assessMigrationBulkCmd.Flags().StringVar(&fleetConfigPath, "fleet-config-file", "", "File containing the connection params for schema(s) to be assessed (required)")
	BoolVar(assessMigrationBulkCmd.Flags(), &continueOnError, "continue-on-error", true, "If true, it will print the error message on console and continue to next schema’s assessment")
	assessMigrationBulkCmd.Flags().Bool("ignore-exists", true, "If true, skip assessment if for a schema it's already done")
	assessMigrationBulkCmd.Flags().StringVar(&bulkAssessmentDir, "bulk-assessment-dir", "", "Top-level directory storing the export-dir of each schema (default: pwd)")
	BoolVar(assessMigrationBulkCmd.Flags(), &startClean, "start-clean", false, "Cleans up all the export-dirs in bulk assessment directory to start everything from scratch")

	// marking mandatory flags
	assessMigrationBulkCmd.MarkFlagRequired("fleet-config-file")
	assessMigrationBulkCmd.MarkFlagRequired("bulk-assessment-dir")
}

func assessMigrationBulk() {
	if startClean {
		proceed := utils.AskPrompt(
			"CAUTION: Using --start-clean will delete all progress in each export directory present inside the bulk-assessment-dir. " +
				"Do you want to proceed")
		if !proceed {
			return
		}

		// cleaning all export-dir present inside bulk-assessment-dir
		matches, err := filepath.Glob(fmt.Sprintf("%s/*-export-dir", bulkAssessmentDir))
		if err != nil {
			utils.ErrExit("error while cleaning up export directories: %s", err)
		}
		for _, match := range matches {
			utils.CleanDir(match)
		}

		err = os.RemoveAll(filepath.Join(bulkAssessmentDir, "bulkAssessmentReport.html"))
		if err != nil {
			utils.ErrExit("failed to remove bulk assessment report: %s", err)
		}
	}

	dbConfigs, err := parseFleetConfigFile(fleetConfigPath)
	if err != nil {
		utils.ErrExit("failed to parse fleet config file: %v", err)
	}

	for _, dbConfig := range dbConfigs {
		utils.PrintAndLog("\nAssessing '%s' schema\n", dbConfig.Schema)
		err = executeAssessment(dbConfig)
		if err != nil {
			log.Errorf("failed to assess migration for schema %s: %v", dbConfig.Schema, err)
			fmt.Printf("failed to assess migration for schema %s: %v\n", dbConfig.Schema, err)
			if !continueOnError {
				break
			}
		}
		if ProcessShutdownRequested {
			log.Info("Shutting down as SIGINT/SIGTERM received...")
			log.Infof("sleep for 10 seconds for exit handlers to execute")
			time.Sleep(time.Second * 10)
		}
	}

	err = generateBulkAssessmentReport(dbConfigs)
	if err != nil {
		utils.ErrExit("failed to generate bulk assessment report: %s", err)
	}
}

func executeAssessment(dbConfig AssessMigrationDBConfig) error {
	log.Infof("executing assessment for schema %q", dbConfig.Schema)
	exportDirPath := getAssessmentExportDirPath(dbConfig)
	cmdArgs := buildCommandArguments(dbConfig, exportDirPath)
	if err := os.MkdirAll(exportDirPath, 0755); err != nil {
		return fmt.Errorf("creating export-directory %q for schema %q: %w", exportDirPath, dbConfig.Schema, err)
	}

	execCmd := exec.Command(os.Args[0], cmdArgs...)
	// password either has to be provided via fleet_config_file or can be provided at run-time by the user.
	// user setting the env var route is not supported for assess-migration-bulk command
	execCmd.Env = append(os.Environ(), "SOURCE_DB_PASSWORD="+dbConfig.Password)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin
	log.Infof("executing the cmd: %s", execCmd.String())
	err := execCmd.Run()
	if err != nil {
		return fmt.Errorf("error while assess migration of schema-%s: %v", dbConfig.Schema, err)
	}
	return nil
}

func buildCommandArguments(dbConfig AssessMigrationDBConfig, exportDirPath string) []string {
	log.Infof("building assess-migration command arguments for schema %q", dbConfig.Schema)
	args := []string{"assess-migration",
		"--source-db-type", dbConfig.DbType,
		"--source-db-schema", dbConfig.Schema,
		"--export-dir", exportDirPath,
	}

	if dbConfig.User != "" {
		args = append(args, "--source-db-user", dbConfig.User)
	}
	if dbConfig.TnsAlias != "" {
		args = append(args, "--oracle-tns-alias", dbConfig.TnsAlias)
	}
	if dbConfig.SID != "" {
		args = append(args, "--source-db-name", dbConfig.SID)
	}
	if dbConfig.Host != "" {
		args = append(args, "--source-db-host", dbConfig.Host)
	}
	if dbConfig.Port != "" {
		args = append(args, "--source-db-port", dbConfig.Port)
	}
	return args
}

func parseFleetConfigFile(filePath string) ([]AssessMigrationDBConfig, error) {
	log.Infof("parsing fleet config file %q", filePath)
	var dbConfigs []AssessMigrationDBConfig
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		dbConfig := parseFleetConfigLine(line)
		dbConfigs = append(dbConfigs, dbConfig)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dbConfigs, nil
}

/*
Format: covers both SSL and non-SSL cases

	<dbtype>,<hostname>,<port>,<service_name>,<sid>,<tns_alias>,<username>,<password>,<schema>
*/
func parseFleetConfigLine(line string) AssessMigrationDBConfig {
	config := strings.Split(line, ",")
	return AssessMigrationDBConfig{
		DbType:      config[0],
		Host:        config[1],
		Port:        config[2],
		ServiceName: config[3],
		SID:         config[4],
		TnsAlias:    config[5],
		User:        config[6],
		Password:    config[7],
		Schema:      config[8],
	}
}

func getAssessmentExportDirPath(dbConfig AssessMigrationDBConfig) string {
	switch {
	case dbConfig.SID != "":
		return fmt.Sprintf("%s/%s-%s-export-dir", bulkAssessmentDir, dbConfig.SID, dbConfig.Schema)
	case dbConfig.ServiceName != "":
		return fmt.Sprintf("%s/%s-%s-export-dir", bulkAssessmentDir, dbConfig.ServiceName, dbConfig.Schema)
	case dbConfig.TnsAlias != "":
		return fmt.Sprintf("%s/%s-%s-export-dir", bulkAssessmentDir, dbConfig.TnsAlias, dbConfig.Schema)
	default:
		panic(fmt.Sprintf("unable to get export dir for assessment of schema-%s", dbConfig.Schema))
	}
}

func getDatabaseIdentifier(dbConfig AssessMigrationDBConfig) string {
	switch {
	case dbConfig.SID != "":
		return dbConfig.SID
	case dbConfig.ServiceName != "":
		return dbConfig.ServiceName
	case dbConfig.TnsAlias != "":
		return dbConfig.TnsAlias
	default:
		return ""
	}
}

//go:embed templates/bulkAssessmentReport.template
var bulkAssessmentHtmlTmpl string

func generateBulkAssessmentReport(dbConfigs []AssessMigrationDBConfig) error {
	log.Infof("generating bulk assessment report")
	for _, dbConfig := range dbConfigs {
		exportDirPath := getAssessmentExportDirPath(dbConfig)
		assessmentReportPath := filepath.Join(exportDirPath, "assessment", "reports", "assessmentReport.html")
		assessmentReportRelPath, err := filepath.Rel(bulkAssessmentDir, assessmentReportPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s schema assessment report: %w", dbConfig.Schema, err)
		}
		var assessmentDetail = AssessmentDetail{
			Schema:             dbConfig.Schema,
			DatabaseIdentifier: getDatabaseIdentifier(dbConfig),
			ReportPath:         assessmentReportRelPath,
			Status:             COMPLETE,
		}
		if !utils.FileOrFolderExists(assessmentReportPath) {
			assessmentDetail.Status = ERROR
		}
		bulkAssessmentReport.Details = append(bulkAssessmentReport.Details, assessmentDetail)
	}

	tmpl, err := template.New("bulk-assessement-report").Parse(bulkAssessmentHtmlTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse the bulkAssessmentReport template: %w", err)
	}

	reportPath := filepath.Join(bulkAssessmentDir, "bulkAssessmentReport.html")
	file, err := os.Create(reportPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	err = tmpl.Execute(file, bulkAssessmentReport)
	if err != nil {
		return fmt.Errorf("failed to execute parsed template file: %w", err)
	}
	return nil
}

func validateBulkAssessmentDirFlag() {
	if bulkAssessmentDir == "" {
		utils.ErrExit(`ERROR: required flag "bulk-assessment-dir" not set`)
	}
	if !utils.FileOrFolderExists(bulkAssessmentDir) {
		utils.ErrExit("bulk-assessment-dir %q doesn't exists.\n", bulkAssessmentDir)
	} else {
		if bulkAssessmentDir == "." {
			fmt.Println("Note: Using current directory as bulk-assessment-dir")
		}
		var err error
		bulkAssessmentDir, err = filepath.Abs(bulkAssessmentDir)
		if err != nil {
			utils.ErrExit("Failed to get absolute path for bulk-assessment-dir %q: %v\n", exportDir, err)
		}
		bulkAssessmentDir = filepath.Clean(bulkAssessmentDir)
	}
}

/*
	TODO:
		check if value valid or not,
		expected/mandatory params are passed
		strip any trailing spaces
	func validateFleetConfigFilePath() {}
*/

func isBulkAssessmentCommand(cmd *cobra.Command) bool {
	return cmd.Name() == assessMigrationBulkCmd.Name()
}
