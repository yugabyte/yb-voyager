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
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var assessmentParamsFpath string
var assessmentDataDirFlag string
var assessmentReport AssessmentReport

type AssessmentReport struct {
	SchemaSummary utils.SchemaSummary `json:"SchemaSummary"`

	UnsupportedDataTypes []utils.TableColumnsDataTypes `json:"UnsupportedDataTypes"`

	UnsupportedFeatures []string `json:"UnsupportedFeatures"`

	Recommendations migassessment.AssessmentRecommendations `json:"Recommendations"`
}

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

func assessMigration() (err error) {
	checkStartCleanForAssessMigration()
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	// setting schemaDir to use later on - gather assessment data, segregating into schema files per object etc..
	schemaDir = lo.Ternary(assessmentDataDirFlag != "", filepath.Join(assessmentDataDirFlag, "schema"),
		filepath.Join(exportDir, "assessment", "data", "schema"))

	err = gatherAssessmentData()
	if err != nil {
		return fmt.Errorf("failed to gather assessment data: %w", err)
	}

	parseExportedSchemaFileForAssessment()

	err = runAssessment()
	if err != nil {
		return fmt.Errorf("failed to run assessment: %w", err)
	}

	// err = generateAssessmentReport()
	err = generateConsolidatedAssessmentReport()
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
	gatherAssessmentDataScriptPath := filepath.Join("/", "etc", "yb-voyager", "gather-assessment-data", "postgresql", "yb-voyager-pg-gather-assessment-data.sh")

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

func parseExportedSchemaFileForAssessment() {
	log.Infof("set 'schemaDir' as: %s", schemaDir)
	source.ApplyExportSchemaObjectListFilter()
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	source.DB().ExportSchema(exportDir, schemaDir)
}

//go:embed assessmentReport.template
var bytesTemplate []byte

func generateConsolidatedAssessmentReport() (err error) {
	utils.PrintAndLog("Generating consolidated assessment report...")

	err = getAssessmentReportContentFromAnalyzeSchema()
	if err != nil {
		return fmt.Errorf("failed to generate assessment report content from analyze schema: %w", err)
	}

	assessmentReport.UnsupportedDataTypes, err = fetchColumnsWithUnsupportedDataTypes()
	if err != nil {
		return fmt.Errorf("failed to fetch columns with unsupported data types: %w", err)
	}

	assessmentReport.Recommendations = migassessment.Recommendations

	assessmentReportDir := filepath.Join(exportDir, "assessment", "reports")
	err = generateAssessmentReportJson(assessmentReportDir)
	if err != nil {
		return fmt.Errorf("failed to generate assessment report JSON: %w", err)
	}

	err = generateAssessmentReportHtml(assessmentReportDir)
	if err != nil {
		return fmt.Errorf("failed to generate assessment report HTML: %w", err)
	}
	return nil
}

func getAssessmentReportContentFromAnalyzeSchema() (err error) {
	analyzeSchemaReport := analyzeSchemaInternal(&source)
	assessmentReport.SchemaSummary = analyzeSchemaReport.SchemaSummary

	// set invalidCount to zero so that it doesn't show up in the report
	for i := 0; i < len(assessmentReport.SchemaSummary.DBObjects); i++ {
		assessmentReport.SchemaSummary.DBObjects[i].InvalidCount = 0
	}

	unsupportedFeatures, err := fetchUnsupportedFeaturesForPG(analyzeSchemaReport)
	if err != nil {
		return fmt.Errorf("failed to fetch unsupported features: %w", err)
	}
	assessmentReport.UnsupportedFeatures = unsupportedFeatures

	return nil
}

func fetchUnsupportedFeaturesForPG(analyzeSchemaReport utils.SchemaReport) ([]string, error) {
	var unsupportedFeatures []string
	log.Infof("fetching unsupported features for PG...")

	lookForIssuesMatching := func(issueReason string) []utils.Issue {
		var matchingIssues []utils.Issue
		for _, issue := range analyzeSchemaReport.Issues {
			if strings.Contains(issue.Reason, issueReason) {
				matchingIssues = append(matchingIssues, issue)
			}
		}
		return matchingIssues
	}

	log.Infof("looking for gin indexes(unsupported)...")
	gistIndexIssues := lookForIssuesMatching(GIST_INDEX_ISSUE_REASON)
	var tablesWithGistIndexes []string
	for _, issue := range gistIndexIssues {
		tablesWithGistIndexes = append(tablesWithGistIndexes, issue.ObjectName)
	}
	if len(tablesWithGistIndexes) > 0 {
		unsupportedFeatures = append(unsupportedFeatures, fmt.Sprintf("GIST indexes names: %v", tablesWithGistIndexes))
	}

	log.Infof("looking for constraint triggers(unsupported)...")
	constraintTriggerIssues := lookForIssuesMatching(CONSTRAINT_TRIGGER_ISSUE_REASON)
	var constraintTriggerNames []string
	for _, issue := range constraintTriggerIssues {
		constraintTriggerNames = append(constraintTriggerNames, issue.ObjectName)
	}
	if len(constraintTriggerNames) > 0 {
		unsupportedFeatures = append(unsupportedFeatures, fmt.Sprintf("Constraint triggers: %v", constraintTriggerNames))
	}

	log.Infof("looking for inherited tables(unsupported)...")
	inheritanceIssues := lookForIssuesMatching(INHERITANCE_ISSUE_REASON)
	var tablesWithInheritance []string
	for _, issue := range inheritanceIssues {
		tablesWithInheritance = append(tablesWithInheritance, issue.ObjectName)
	}
	if len(tablesWithInheritance) > 0 {
		unsupportedFeatures = append(unsupportedFeatures, fmt.Sprintf("Inherited(child) Tables are: %v", tablesWithInheritance))
	}

	log.Infof("looking for stored generated columns(unsupported)...")
	storedGeneratedColumnIssues := lookForIssuesMatching(STORED_GENERATED_COLUMN_ISSUE_REASON)
	var tablesWithStoredGeneratedColumns []string
	for _, issue := range storedGeneratedColumnIssues {
		tablesWithStoredGeneratedColumns = append(tablesWithStoredGeneratedColumns, issue.ObjectName)
	}
	if len(tablesWithStoredGeneratedColumns) > 0 {
		unsupportedFeatures = append(unsupportedFeatures, fmt.Sprintf("Tables with stored generated columns: %v", tablesWithStoredGeneratedColumns))
	}

	return unsupportedFeatures, nil
}

func fetchColumnsWithUnsupportedDataTypes() ([]utils.TableColumnsDataTypes, error) {
	var unsupportedDataTypes []utils.TableColumnsDataTypes

	// load file with all column data types
	assessmentDataDir := lo.Ternary(assessmentDataDirFlag != "", assessmentDataDirFlag,
		filepath.Join(exportDir, "assessment", "data"))
	filePath := filepath.Join(assessmentDataDir, "table-columns-data-types.csv")

	allColumnsDataTypes, err := migassessment.LoadCSVDataFile[utils.TableColumnsDataTypes](filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load table columns data types file: %w", err)
	}

	// filter columns with unsupported data types using srcdb.PostgresUnsupportedDataTypesForDbzm
	pgUnsupportedDataTypes := srcdb.PostgresUnsupportedDataTypesForDbzm
	for i := 0; i < len(allColumnsDataTypes); i++ {
		if utils.ContainsAnySubstringFromSlice(pgUnsupportedDataTypes, allColumnsDataTypes[i].DataType) {
			unsupportedDataTypes = append(unsupportedDataTypes, *allColumnsDataTypes[i])
		}
	}

	return unsupportedDataTypes, nil
}

func generateAssessmentReportJson(reportDir string) error {
	jsonReportFilePath := filepath.Join(reportDir, "assessmentReport.json")
	log.Infof("writing assessment report to file: %s", jsonReportFilePath)
	strReport, err := json.MarshalIndent(assessmentReport, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal the assessment report: %w", err)
	}

	err = os.WriteFile(jsonReportFilePath, strReport, 0644)
	if err != nil {
		return fmt.Errorf("failed to write assessment report to file: %w", err)
	}

	utils.PrintAndLog("generated JSON assessment report at: %s", jsonReportFilePath)
	return nil
}

func generateAssessmentReportHtml(reportDir string) error {
	htmlReportFilePath := filepath.Join(reportDir, "assessmentReport.html")
	log.Infof("writing assessment report to file: %s", htmlReportFilePath)

	file, err := os.Create(htmlReportFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file for %q: %w", filepath.Base(htmlReportFilePath), err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Errorf("failed to close file %q: %v", htmlReportFilePath, err)
		}
	}()

	log.Infof("creating template for assessment report...")
	tmpl := template.Must(template.New("report").Parse(string(bytesTemplate)))

	log.Infof("execute template for assessment report...")
	err = tmpl.Execute(file, assessmentReport)
	if err != nil {
		return fmt.Errorf("failed to render the assessment report: %w", err)
	}

	utils.PrintAndLog("generated HTML assessment report at: %s", htmlReportFilePath)
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
			utils.ErrExit("assessment data directory %q provided with `--assessment-data-dir` flag does not exist", assessmentDataDirFlag)
		} else {
			log.Infof("using provided assessment data directory: %s", assessmentDataDirFlag)
		}
	}
}
