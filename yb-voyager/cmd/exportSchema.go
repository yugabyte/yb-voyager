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
	"strings"

	"github.com/fatih/color"
	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/sqltransformer"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var skipRecommendations utils.BoolStr
var assessmentReportPath string
var assessmentRecommendationsApplied = false
var errorApplyingAssessmentRecommendations = false

var exportSchemaCmd = &cobra.Command{
	Use: "schema",
	Short: "Export schema from source database into export-dir as .sql files\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/schema-migration/export-schema/",
	Long: ``,

	PreRun: func(cmd *cobra.Command, args []string) {
		if source.StrExportObjectTypeList != "" && source.StrExcludeObjectTypeList != "" {
			utils.ErrExit("Error only one of --object-type-list and --exclude-object-type-list is allowed")
		}
		setExportFlagsDefaults()
		err := validateExportFlags(cmd, SOURCE_DB_EXPORTER_ROLE)
		if err != nil {
			utils.ErrExit("Error validating export schema flags: %s", err.Error())
		}

		validateAssessmentReportPathFlag()
		markFlagsRequired(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		source.ApplyExportSchemaObjectListFilter()
		err := exportSchema()
		if err != nil {
			utils.ErrExit("%v", err)
		}
	},
}

func exportSchema() error {
	if metaDBIsCreated(exportDir) && schemaIsExported() {
		if startClean {
			proceed := utils.AskPrompt(
				"CAUTION: Using --start-clean will overwrite any manual changes done to the " +
					"exported schema. Do you want to proceed")
			if !proceed {
				return nil
			}

			for _, dirName := range []string{"schema", "reports", "temp", "metainfo/schema"} {
				utils.CleanDir(filepath.Join(exportDir, dirName))
			}
			clearSchemaIsExported()
			clearAssessmentRecommendationsApplied()
		} else {
			fmt.Fprintf(os.Stderr, "Schema is already exported. "+
				"Use --start-clean flag to export schema again -- "+
				"CAUTION: Using --start-clean will overwrite any manual changes done to the exported schema.\n")
			return nil
		}
	} else if startClean {
		utils.PrintAndLog("Schema is not exported yet. Ignoring --start-clean flag.\n\n")
	}
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	err := retrieveMigrationUUID()
	if err != nil {
		log.Errorf("failed to get migration UUID: %v", err)
		return fmt.Errorf("failed to get migration UUID during export schema: %w", err)
	}

	utils.PrintAndLog("export of schema for source type as '%s'\n", source.DBType)
	// Check connection with source database.
	err = source.DB().Connect()
	if err != nil {
		log.Errorf("failed to connect to the source db: %s", err)
		return fmt.Errorf("failed to connect to the source db during export schema: %w", err)
	}
	defer source.DB().Disconnect()

	if source.RunGuardrailsChecks {
		// Check source database version.
		log.Info("checking source DB version")
		err = source.DB().CheckSourceDBVersion(exportType)
		if err != nil {
			return fmt.Errorf("failed to check source db version during export schema: %w", err)
		}

		// Check if required binaries are installed.
		binaryCheckIssues, err := checkDependenciesForExport()
		if err != nil {
			return fmt.Errorf("failed to check dependencies for export schema: %w", err)
		} else if len(binaryCheckIssues) > 0 {
			return fmt.Errorf("\n%s\n%s", color.RedString("\nMissing dependencies for export schema:"), strings.Join(binaryCheckIssues, "\n"))
		}
	}

	checkSourceDBCharset()
	sourceDBVersion := source.DB().GetVersion()
	source.DBVersion = sourceDBVersion
	source.DBSize, err = source.DB().GetDatabaseSize()
	if err != nil {
		log.Errorf("error getting database size: %v", err) //can just log as this is used for call-home only
	}
	utils.PrintAndLog("%s version: %s\n", source.DBType, sourceDBVersion)

	res := source.DB().CheckSchemaExists()
	if !res {
		return fmt.Errorf("failed to check if source schema exist during export schema: %q", source.Schema)
	}

	// Check if the source database has the required permissions for exporting schema.
	if source.RunGuardrailsChecks {
		checkIfSchemasHaveUsagePermissions()
		missingPerms, err := source.DB().GetMissingExportSchemaPermissions("")
		if err != nil {
			return fmt.Errorf("failed to get missing export schema permissions: %w", err)
		}
		if len(missingPerms) > 0 {
			color.Red("\nPermissions missing in the source database for export schema:\n")
			output := strings.Join(missingPerms, "\n")
			fmt.Printf("%s\n\n", output)

			link := "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
			fmt.Println("Check the documentation to prepare the database for migration:", color.BlueString(link))

			reply := utils.AskPrompt("\nDo you want to continue anyway")
			if !reply {
				return fmt.Errorf("grant the required permissions and try again")
			}
		}
	}

	exportSchemaStartEvent := createExportSchemaStartedEvent()
	controlPlane.ExportSchemaStarted(&exportSchemaStartEvent)

	source.DB().ExportSchema(exportDir, schemaDir)

	err = updateIndexesInfoInMetaDB()
	if err != nil {
		return fmt.Errorf("failed to update indexes info metadata db: %w", err)
	}

	applySchemaTransformations()

	utils.PrintAndLog("\nExported schema files created under directory: %s\n\n", filepath.Join(exportDir, "schema"))

	packAndSendExportSchemaPayload(COMPLETE, "")

	saveSourceDBConfInMSR()
	setSchemaIsExported()

	exportSchemaCompleteEvent := createExportSchemaCompletedEvent()
	controlPlane.ExportSchemaCompleted(&exportSchemaCompleteEvent)
	return nil
}

func packAndSendExportSchemaPayload(status string, errorMsg string) {
	if !shouldSendCallhome() {
		return
	}
	payload := createCallhomePayload()
	payload.MigrationPhase = EXPORT_SCHEMA_PHASE
	payload.Status = status
	sourceDBDetails := callhome.SourceDBDetails{
		DBType:    source.DBType,
		DBVersion: source.DBVersion,
		DBSize:    source.DBSize,
	}
	payload.SourceDBDetails = callhome.MarshalledJsonString(sourceDBDetails)
	exportSchemaPayload := callhome.ExportSchemaPhasePayload{
		StartClean:             bool(startClean),
		AppliedRecommendations: assessmentRecommendationsApplied,
		UseOrafce:              bool(source.UseOrafce),
		CommentsOnObjects:      bool(source.CommentsOnObjects),
		Error:                  callhome.SanitizeErrorMsg(errorMsg),
	}

	payload.PhasePayload = callhome.MarshalledJsonString(exportSchemaPayload)

	err := callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)
	registerCommonGlobalFlags(exportSchemaCmd)
	registerCommonExportFlags(exportSchemaCmd)
	registerSourceDBConnFlags(exportSchemaCmd, false, true)
	BoolVar(exportSchemaCmd.Flags(), &source.UseOrafce, "use-orafce", true,
		"enable using orafce extension in export schema")

	BoolVar(exportSchemaCmd.Flags(), &source.CommentsOnObjects, "comments-on-objects", false,
		"enable export of comments associated with database objects (default false)")

	exportSchemaCmd.Flags().StringVar(&source.StrExportObjectTypeList, "object-type-list", "",
		"comma separated list of objects to export. ")

	exportSchemaCmd.Flags().StringVar(&source.StrExcludeObjectTypeList, "exclude-object-type-list", "",
		"comma separated list of objects to exclude from export. ")

	BoolVar(exportSchemaCmd.Flags(), &skipRecommendations, "skip-recommendations", false,
		"disable applying recommendations in the exported schema suggested by the migration assessment report")

	exportSchemaCmd.Flags().StringVar(&assessmentReportPath, "assessment-report-path", "",
		"path to the generated assessment report file(JSON format) to be used for applying recommendation to exported schema")
}

func validateAssessmentReportPathFlag() {
	if assessmentReportPath == "" {
		return
	}

	if !utils.FileOrFolderExists(assessmentReportPath) {
		utils.ErrExit("assessment report file doesn't exists at path provided in --assessment-report-path flag: %q", assessmentReportPath)
	}
	if !strings.HasSuffix(assessmentReportPath, ".json") {
		utils.ErrExit("assessment report file should be in JSON format, path provided in --assessment-report-path flag: %q", assessmentReportPath)
	}
}

func schemaIsExported() bool {
	if !metaDBIsCreated(exportDir) {
		return false
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("check if schema is exported: load migration status record: %s", err)
	}

	return msr.ExportSchemaDone
}

func setSchemaIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportSchemaDone = true
	})
	if err != nil {
		utils.ErrExit("set schema is exported: update migration status record: %s", err)
	}
}

func clearSchemaIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportSchemaDone = false
	})
	if err != nil {
		utils.ErrExit("clear schema is exported: update migration status record: %s", err)
	}
}

func updateIndexesInfoInMetaDB() error {
	log.Infof("updating indexes info in metaDB")
	if !utils.ContainsString(source.ExportObjectTypeList, "TABLE") {
		log.Infof("skipping updating indexes info in metaDB since TABLE object type is not being exported")
		return nil
	}
	indexesInfo := source.DB().GetIndexesInfo()
	if indexesInfo == nil {
		return nil
	}
	err := metadb.UpdateJsonObjectInMetaDB(metaDB, metadb.SOURCE_INDEXES_INFO_KEY, func(record *[]utils.IndexInfo) {
		*record = indexesInfo
	})
	if err != nil {
		return err
	}
	return nil
}

/*
applySchemaTransformations applies the following transformations to the exported schema one by one
and saves the transformed schema in the same file.

In case of any failure in applying any transformation, it logs the error, keep the original file and continues with the next transformation.
*/
func applySchemaTransformations() {
	// 1. Transform table.sql
	{
		tableFilePath := utils.GetObjectFilePath(schemaDir, TABLE)
		var transformations []func([]*pg_query.RawStmt, string) ([]*pg_query.RawStmt, error)
		if !skipRecommendations {
			transformations = append(transformations, applyShardedTableTransformation)     // transform #1
			transformations = append(transformations, applyMergeConstraintsTransformation) // transform #2
		} else {
			transformations = append(transformations, applyMergeConstraintsTransformation) // transform #1
		}

		err := transformSchemaFile(tableFilePath, transformations, "table")
		if err != nil {
			log.Warnf("Error transforming %q: %v", tableFilePath, err)
		}
	}

	// 2. Transform mview.sql
	{
		mviewFilePath := utils.GetObjectFilePath(schemaDir, MVIEW)
		var transformations []func([]*pg_query.RawStmt, string) ([]*pg_query.RawStmt, error)
		if !skipRecommendations {
			transformations = append(transformations, applyShardedTableTransformation) // only transformation for mview
		}

		err := transformSchemaFile(mviewFilePath, transformations, "mview")
		if err != nil {
			log.Warnf("Error transforming %q: %v", mviewFilePath, err)
		}
	}

	// Check the flag to message the user about the recommendations applied and ask to apply manually
	if errorApplyingAssessmentRecommendations {
		utils.PrintAndLog("\nUnable to apply assessment recommendations(sharded/colocated tables) to the exported schema. Please check the logs for more details.")
		utils.PrintAndLog("You can apply the recommendations manually by referring to the assessment report.")
	} else if assessmentRecommendationsApplied {
		SetAssessmentRecommendationsApplied()
	}
	// else case will be whether neither applied nor errored, but rather schema file was not present.

	// There is corner case: when recommmendations applied on table.sql but not on mview.sql or vice versa
	// In this case, there is no definite answer whether assessmentRecommendationsApplied should be true or false; Assuming false.
}

// transformSchemaFile applies a sequence of transformations to the given schema file
// and writes the transformed result back. If the file doesn't exist, logs a message and returns nil.
func transformSchemaFile(filePath string, transformations []func(raw []*pg_query.RawStmt, filePath string) ([]*pg_query.RawStmt, error), objectType string) error {
	if !utils.FileOrFolderExists(filePath) || len(transformations) == 0 {
		log.Infof("schema file %q for object type %s doesn't exist or no transformations to apply", filePath, objectType)
		return nil
	}

	var rawStmts []*pg_query.RawStmt
	var err error
	defer func() {
		if err != nil {
			errorApplyingAssessmentRecommendations = true
			utils.PrintAndLog("Failed to apply any transformation to the exported schema file %q: %v\n", filePath, err)
		}
	}()

	log.Infof("applying transformations to the schema file %q for object type %s", filePath, objectType)
	rawStmts, err = queryparser.ParseSqlFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse sql statements from %s object type in schema file %q: %w", objectType, filePath, err)
	}

	var beforeSqlStmts []string
	beforeSqlStmts, err = queryparser.DeparseRawStmts(rawStmts)
	if err != nil {
		return fmt.Errorf("failed to deparse raw stmts for %s object type in schema file %q: %w", objectType, filePath, err)
	}

	transformedStmts := rawStmts
	// Apply transformations in order
	for _, transformFn := range transformations {
		transformFuncName := utils.GetFuncName(transformFn)
		log.Infof("applying transformation: %s on %s", filepath.Base(transformFuncName), filePath)

		newStmts, err2 := transformFn(transformedStmts, filePath)
		if err2 != nil {
			// Log and continue using the unmodified statements slice for subsequent transformations in case of error
			log.Warnf("failed to apply transformation %s on the exported schema file %q: %v",
				filepath.Base(transformFuncName), filePath, err)
			continue
		}
		transformedStmts = newStmts
	}

	var sqlStmts []string
	sqlStmts, err = queryparser.DeparseRawStmts(transformedStmts)
	if err != nil {
		return fmt.Errorf("failed to deparse transformed raw stmts for %s object type in schema file %q: %w", objectType, filePath, err)
	}

	// Below Check for if transformations changed anything is WRONG
	// here we are dealing with pointers - *pg_query.RawStmt so underlying elements of slices point to same memory
	// if slices.Equal(originalStmts, transformedStmts) {
	// 	log.Infof("no change in the schema for object type %s after applying all transformations", objectType)
	// 	return nil
	// }
	if slices.Equal(beforeSqlStmts, sqlStmts) {
		log.Infof("no change in the schema for object type %s after applying all transformations", objectType)
		return nil
	}

	// Backup original
	backupFile := filePath + ".orig"
	err = os.Rename(filePath, backupFile)
	if err != nil {
		return fmt.Errorf("failed to rename %s file to %s: %w", filePath, backupFile, err)
	}
	utils.PrintAndLog("The original DDLs(without transformation) for %q object type are backed up at %s\n", strings.ToUpper(objectType), backupFile)

	// Write updated file
	fileContent := strings.Join(sqlStmts, "\n\n")
	err = os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write transformed schema file %q: %w", filePath, err)
	}

	return nil
}

func applyShardedTableTransformation(stmts []*pg_query.RawStmt, filePath string) ([]*pg_query.RawStmt, error) {
	log.Infof("applying sharded tables transformation to the exported schema file %q", filePath)
	if bool(skipRecommendations) || !slices.Contains(assessMigrationSupportedDBTypes, source.DBType) {
		log.Info("skipping applying sharded tables transformation due to --skip-recommendations flag or assessment unsupported source db type")
		return stmts, nil
	}

	var transformedRawStmts []*pg_query.RawStmt
	var err error
	// defer func to inspect err and set global flag for recommendations application
	defer func() {
		if err != nil {
			errorApplyingAssessmentRecommendations = true
			assessmentRecommendationsApplied = false
			utils.PrintAndLog("Failed to apply assessment recommendations to the exported schema file %q: %v\n", filepath.Base(filePath), err)
		} else {
			utils.PrintAndLog("Applied assessment recommendations to %s schema\n", filepath.Base(filePath))
			assessmentRecommendationsApplied = true
		}
	}()

	assessmentReportPath = lo.Ternary(assessmentReportPath != "", assessmentReportPath,
		filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME)))

	var assessmentReport *AssessmentReport
	assessmentReport, err = ParseJSONToAssessmentReport(assessmentReportPath)
	if err != nil {
		return stmts, fmt.Errorf("failed to parse json report file %q: %w", assessmentReportPath, err)
	}

	var shardedObjects []string
	shardedObjects, err = assessmentReport.GetShardedTablesRecommendation()
	if err != nil {
		return stmts, fmt.Errorf("failed to fetch sharded tables recommendation: %w", err)
	}

	isObjectSharded := func(objectName string) bool {
		switch source.DBType {
		case POSTGRESQL:
			return slices.Contains(shardedObjects, objectName)
		case ORACLE:
			// TODO: handle case-sensitivity properly
			for _, shardedObject := range shardedObjects {
				// in case of oracle, shardedTable is unqualified.
				if strings.ToLower(shardedObject) == objectName {
					return true
				}
			}
		default:
			panic(fmt.Sprintf("unsupported source db type %s for applying sharded table transformation", source.DBType))
		}
		return false
	}

	transformer := sqltransformer.NewTransformer()
	transformedRawStmts, err = transformer.ConvertToShardedTables(stmts, isObjectSharded)
	if err != nil {
		return stmts, fmt.Errorf("failed to convert to sharded tables: %w", err)
	}

	assessmentRecommendationsApplied = true
	return transformedRawStmts, nil
}

func applyMergeConstraintsTransformation(rawStmts []*pg_query.RawStmt, filePath string) ([]*pg_query.RawStmt, error) {
	if utils.GetEnvAsBool("YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS", false) {
		log.Infof("skipping applying merge constraints transformation due to env var YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS=true")
		return rawStmts, nil
	}

	log.Infof("applying merge constraints transformation to the exported schema file %q", filePath)
	transformer := sqltransformer.NewTransformer()
	transformedRawStmts, err := transformer.MergeConstraints(rawStmts)
	if err != nil {
		return rawStmts, fmt.Errorf("failed to merge constraints: %w", err)
	}

	return transformedRawStmts, nil
}

func createExportSchemaStartedEvent() cp.ExportSchemaStartedEvent {
	result := cp.ExportSchemaStartedEvent{}
	initBaseSourceEvent(&result.BaseEvent, "EXPORT SCHEMA")
	return result
}

func createExportSchemaCompletedEvent() cp.ExportSchemaCompletedEvent {
	result := cp.ExportSchemaCompletedEvent{}
	initBaseSourceEvent(&result.BaseEvent, "EXPORT SCHEMA")
	return result
}

func SetAssessmentRecommendationsApplied() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.AssessmentRecommendationsApplied = true
	})
	if err != nil {
		utils.ErrExit("failed to update migration status record with assessment recommendations applied flag: %w", err)
	}
}

func clearAssessmentRecommendationsApplied() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.AssessmentRecommendationsApplied = false
	})
	if err != nil {
		utils.ErrExit("clear assessment recommendations applied: update migration status record: %s", err)
	}
}
