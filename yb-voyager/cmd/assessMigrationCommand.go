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
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var supportedAssessmentReportFormats = []string{"json", "html"}
var assessmentParamsFpath string

var assessmentDataDirFlag string

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: "Assess the migration from source database to YugabyteDB.",
	Long:  `Assess the migration from source database to YugabyteDB.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateSourceDBTypeForAssessMigration()
		setExportFlagsDefaults()
		validateSourceSchema()
		validatePortRange()
		validateSSLMode()
		validateAssessmentDataDirFlag()
	},

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
	registerSourceDBConnFlags(assessMigrationCmd, false, false)

	// TODO: clarity on whether this flag should be a mandatory or not
	assessMigrationCmd.Flags().StringVar(&assessmentParamsFpath, "assessment-params-file", "",
		"TOML file path to the user provided assessment params.")

	BoolVar(assessMigrationCmd.Flags(), &startClean, "start-clean", false,
		"cleans up the project directory for schema or data files depending on the export command (default false)")

	// optional flag to take metadata and stats directory path in case it is not in exportDir
	assessMigrationCmd.Flags().StringVar(&assessmentDataDirFlag, "assessment-data-dir", "",
		"Directory path where metadata and stats of source DB are stored. Optional flag, if not provided, "+
			"it will be assumed to be present at default path inside the export directory.")
}

//go:embed report.template
var bytesTemplate []byte

func assessMigration() (err error) {
	checkStartCleanForAssessMigration()
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	err = gatherAssessmentData()
	if err != nil {
		return fmt.Errorf("failed to gather assessment data: %w", err)
	}

	// parse the schema.sql file and put it into folders
	schemaDir = lo.Ternary(assessmentDataDirFlag != "", filepath.Join(assessmentDataDirFlag, "schema"),
		filepath.Join(exportDir, "assessment", "data", "schema"))
	source.DB().ExportSchema(exportDir, schemaDir)

	err = runAssessment()
	if err != nil {
		return fmt.Errorf("failed to run assessment: %w", err)
	}

	err = generateAssessmentReport()
	if err != nil {
		return fmt.Errorf("failed to generate assessment report: %w", err)
	}

	utils.PrintAndLog("Migration assessment completed successfully.")
	return nil
}

func runAssessment() error {
	log.Infof("running assessment for migration from '%s' to YugabyteDB", source.DBType)
	migassessment.AssessmentDataDir = lo.Ternary(assessmentDataDirFlag != "",
		assessmentDataDirFlag, filepath.Join(exportDir, "assessment", "data"))

	// load and sets 'assessmentParams' from the user input file
	err := migassessment.LoadAssessmentParams(assessmentParamsFpath)
	if err != nil {
		return fmt.Errorf("failed to load assessment parameters: %w", err)
	}

	err = migassessment.ShardingAssessment()
	if err != nil {
		return fmt.Errorf("failed to perform sharding assessment: %w", err)
	}

	// migassessment.SizingAssessment()
	return nil
}

func checkStartCleanForAssessMigration() {
	assessmentDir := filepath.Join(exportDir, "assessment")
	dataFilesPattern := filepath.Join(assessmentDir, "data", "*.csv")
	reportsFilePattern := filepath.Join(assessmentDir, "reports", "report.*")
	schemaFilesPattern := filepath.Join(assessmentDir, "data", "schema", "*", "*.sql")

	if utils.FileOrFolderExistsWithGlobPattern(dataFilesPattern) ||
		utils.FileOrFolderExistsWithGlobPattern(reportsFilePattern) ||
		utils.FileOrFolderExistsWithGlobPattern(schemaFilesPattern) {
		if startClean {
			utils.CleanDir(filepath.Join(exportDir, "assessment", "data"))
			utils.CleanDir(filepath.Join(exportDir, "assessment", "reports"))
		} else {
			utils.ErrExit("assessment data or reports files already exist in the assessment directory at '%s'. ", assessmentDir)
		}
	}
}

func gatherAssessmentData() (err error) {
	if assessmentDataDirFlag != "" {
		return nil // assessment data files are provided by the user inside assessmentDataDir
	}

	assessmentDataDir := filepath.Join(exportDir, "assessment", "data")

	// setting schema objects types to export before creating the project directories
	source.ExportObjectTypeList = utils.GetExportSchemaObjectList(source.DBType)
	schemaDir = filepath.Join(assessmentDataDir, "schema")
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	if source.Password == "" {
		source.Password, err = askPassword("source DB", source.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			return fmt.Errorf("failed to get source DB password: %w", err)
		}
	}

	utils.PrintAndLog("gathering metadata and stats from '%s' source database...", source.DBType)
	switch source.DBType {
	case POSTGRESQL:
		err := gatherAssessmentDataFromPG()
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source PG database: %w", err)
		}
	default:
		return fmt.Errorf("source DB Type %s is not yet supported for metadata and stats gathering", source.DBType)
	}
	utils.PrintAndLog("gathered assessment data files at '%s'", assessmentDataDir)
	return nil
}

func gatherAssessmentDataFromPG() (err error) {
	if assessmentDataDirFlag != "" {
		return nil
	}

	homebrewVoyagerDir := fmt.Sprintf("yb-voyager@%s", utils.YB_VOYAGER_VERSION)
	gatherAssessmentDataScriptPath := filepath.Join("/", "etc", "yb-voyager", "gather-assessment-data", "postgresql", "yb-voyager-gather-assessment-data.sh")

	possiblePathsForScript := []string{
		gatherAssessmentDataScriptPath,
		filepath.Join("/", "opt", "homebrew", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, gatherAssessmentDataScriptPath),
		filepath.Join("/", "usr", "local", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, gatherAssessmentDataScriptPath),
	}

	scriptPath := ""
	for _, path := range possiblePathsForScript {
		if utils.FileOrFolderExists(path) {
			scriptPath = path
			break
		}
	}

	if scriptPath == "" {
		return fmt.Errorf("script not found in possible paths: %v", possiblePathsForScript)
	}

	log.Infof("using script: %s", scriptPath)
	assessmentDataDir := filepath.Join(exportDir, "assessment", "data")
	scriptArgs := []string{
		source.DB().GetConnectionUriWithoutPassword(),
		source.Schema,
		assessmentDataDir,
	}

	preparedScriptCmd := exec.Command(scriptPath, scriptArgs...)
	log.Infof("running script: %s", preparedScriptCmd.String())
	preparedScriptCmd.Env = append(preparedScriptCmd.Env, "PGPASSWORD="+source.Password)
	preparedScriptCmd.Dir = assessmentDataDir
	stdout, err := preparedScriptCmd.CombinedOutput()
	log.Infof("output of gather assessment data script for PG: %s\n", string(stdout))
	if err != nil {
		return fmt.Errorf("error running script: %w", err)
	}
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
				return fmt.Errorf("failed to marshal the assessment report: %w", err)
			}

			_, err = assessmentReportContent.Write(strReport)
			if err != nil {
				return fmt.Errorf("failed to write assessment report to buffer: %w", err)
			}
		case "html":
			templ := template.Must(template.New("report").Parse(string(bytesTemplate)))
			err := templ.Execute(&assessmentReportContent, migassessment.FinalReport)
			if err != nil {
				return fmt.Errorf("failed to render the assessment report: %w", err)
			}
		}

		log.Infof("writing assessment report to file: %s", reportFilePath)
		err := os.WriteFile(reportFilePath, assessmentReportContent.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("failed to write the assessment report: %w", err)
		}
	}
	utils.PrintAndLog("Generated assessment reports at '%s'", reportsDir)
	return nil
}

func validateSourceDBTypeForAssessMigration() {
	switch source.DBType {
	case POSTGRESQL:
		return
	default:
		utils.ErrExit("source DB Type %q is not yet supported for migration assessment", source.DBType)
	}
}

func validateAssessmentDataDirFlag() {
	if assessmentDataDirFlag != "" {
		if !utils.FileOrFolderExists(assessmentDataDirFlag) {
			utils.ErrExit("assessment data directory '%s' provided with `--assessment-data-dir` flag does not exist", assessmentDataDirFlag)
		} else {
			log.Infof("using provided assessment data directory: %s", assessmentDataDirFlag)
		}
	}
}
