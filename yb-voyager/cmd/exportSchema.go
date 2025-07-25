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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
var assessmentRecommendationsApplied bool
var assessSchemaBeforeExport utils.BoolStr

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
		markFlagsRequired(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		source.ApplyExportSchemaObjectListFilter()
		err := exportSchema(cmd)
		if err != nil {
			utils.ErrExit("%v", err)
		}
	},
}

func exportSchema(cmd *cobra.Command) error {
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

	utils.PrintAndLog("\nexport of schema for source type as '%s'\n", source.DBType)
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

	err = runAssessMigrationCmdBeforExportSchemaIfRequired(cmd)
	if err != nil {
		log.Warnf("failed to run assess-migration command before export schema: %v", err)
	}

	exportSchemaStartEvent := createExportSchemaStartedEvent()
	controlPlane.ExportSchemaStarted(&exportSchemaStartEvent)

	source.DB().ExportSchema(exportDir, schemaDir)

	err = updateIndexesInfoInMetaDB()
	if err != nil {
		return fmt.Errorf("failed to update indexes info metadata db: %w", err)
	}

	err = applyMigrationAssessmentRecommendations()
	if err != nil {
		return fmt.Errorf("failed to apply migration assessment recommendation to the schema files: %w", err)
	}

	// continue after logging the error; since this transformation is only for performance improvement
	err = applyMergeConstraintsTransformations()
	if err != nil {
		log.Warnf("failed to apply merge constraints transformation to the schema files: %v", err)
	}

	utils.PrintAndLog("\nExported schema files created under directory: %s\n\n", filepath.Join(exportDir, "schema"))

	packAndSendExportSchemaPayload(COMPLETE, nil)

	saveSourceDBConfInMSR()
	setSchemaIsExported()

	exportSchemaCompleteEvent := createExportSchemaCompletedEvent()
	controlPlane.ExportSchemaCompleted(&exportSchemaCompleteEvent)
	return nil
}

func runAssessMigrationCmdBeforExportSchemaIfRequired(exportSchemaCmd *cobra.Command) error {
	if !bool(assessSchemaBeforeExport) {
		log.Infof("skipping running assess-migration command before export schema as flag --assess-schema-before-export is set as false.")
		return nil
	}

	if source.DBType != POSTGRESQL {
		log.Infof("skipping running assess-migration command before export schema as source DB type is not PostgreSQL.")
		return nil
	}

	if ok, _ := IsMigrationAssessmentDoneDirectly(metaDB); ok {
		log.Infof("migration assessment is already done, skipping running assess-migration command.")
		return nil
	} else if ok, _ := IsMigrationAssessmentDoneViaExportSchema(); ok {
		log.Infof("migration assessment is already done via export schema, skipping running assess-migration command.")
		return nil
	}

	var assessFlagsWithValues []string
	commonFlags := utils.GetCommonFlags(exportSchemaCmd, assessMigrationCmd)
	for _, flag := range commonFlags {
		// don't pass start-clean flag to assess-migration command here
		if flag.Name == "start-clean" || !flag.Changed {
			continue
		}

		// bool flags: --flag=value
		// everything else: --flag value
		if flag.Value.Type() == "bool" {
			assessFlagsWithValues = append(assessFlagsWithValues,
				fmt.Sprintf("--%s=%s", flag.Name, flag.Value.String()),
			)
		} else {
			assessFlagsWithValues = append(assessFlagsWithValues,
				"--"+flag.Name,
				flag.Value.String(),
			)
		}
	}

	// Append --yes=true(irrespective) at the end to override any --yes=false if set in export schema cmd
	assessFlagsWithValues = append(assessFlagsWithValues,
		"--invoked-by-export-schema", "true",
		"--iops-capture-interval", "0", // TODO: any small but significant duration will be better than 0
		"--yes",
	)

	// locate voyager binary
	voyagerExecutable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate yb-voyager executable, skipping assessment: %v", err)
	}

	var stderrBuf, stdoutBuf bytes.Buffer

	utils.PrintAndLog("Assessing migration before exporting schema...")

	// Invoke the assess-migration command as a subprocess
	cmd := exec.Command(voyagerExecutable, append([]string{"assess-migration"}, assessFlagsWithValues...)...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// run and ignore exit status
	if err := cmd.Run(); err != nil {
		utils.PrintAndLog("Failed to assess the migration, continuing with export schema...\n")
		return fmt.Errorf("assess migration cmd exit err: %s and stderr: %s", err.Error(), stderrBuf.String())
	}

	utils.PrintAndLog("Migration assessment completed successfully.")

	// fetching assessment report path output line from stdout of assess-migration command process
	re := regexp.MustCompile(`^\s*generated (?:JSON|HTML) assessment report at: .+`)
	scanner := bufio.NewScanner(strings.NewReader(stdoutBuf.String()))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if re.MatchString(line) {
			fmt.Println(line)
		}
	}

	fmt.Println()
	return nil
}

func packAndSendExportSchemaPayload(status string, errorMsg error) {
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
		ControlPlaneType:       getControlPlaneType(),
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

	// temporary flag to disable this change if user encounters any issues
	BoolVar(exportSchemaCmd.Flags(), &assessSchemaBeforeExport, "assess-schema-before-export", true,
		"run migration assessment before exporting schema. (default true)")
	exportSchemaCmd.Flags().MarkHidden("assess-schema-before-export") // hide this flag from help output
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

func applyMigrationAssessmentRecommendations() error {
	if skipRecommendations {
		log.Infof("not apply recommendations due to flag --skip-recommendations=true")
		return nil
	} else if source.DBType == MYSQL {
		return nil
	}

	assessViaExportSchema, err := IsMigrationAssessmentDoneViaExportSchema()
	if err != nil {
		return fmt.Errorf("failed to check if migration assessment is done via export schema: %w", err)
	}

	if !bool(skipRecommendations) && assessViaExportSchema {
		utils.PrintAndLog(`Recommendations generated but not applied. Run the "assess-migration" command explicitly to produce precise recommendations and apply them.`)
		return nil
	}

	// TODO: copy the reports to "export-dir/assessment/reports" for further usage
	assessmentReportPath := lo.Ternary(assessmentReportPath != "", assessmentReportPath,
		filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME)))
	log.Infof("using assessmentReportPath: %s", assessmentReportPath)
	if !utils.FileOrFolderExists(assessmentReportPath) {
		utils.PrintAndLog("migration assessment report file doesn't exists at %q, skipping apply recommendations step...", assessmentReportPath)
		return nil
	}

	log.Infof("parsing assessment report json file for applying recommendations")
	report, err := ParseJSONToAssessmentReport(assessmentReportPath)
	if err != nil {
		return fmt.Errorf("failed to parse json report file %q: %w", assessmentReportPath, err)
	}

	shardedTables, err := report.GetShardedTablesRecommendation()
	if err != nil {
		return fmt.Errorf("failed to fetch sharded tables recommendation: %w", err)
	} else {
		err := applyShardedTablesRecommendation(shardedTables, TABLE)
		if err != nil {
			return fmt.Errorf("failed to apply colocated vs sharded table recommendation: %w", err)
		}
		err = applyShardedTablesRecommendation(shardedTables, MVIEW)
		if err != nil {
			return fmt.Errorf("failed to apply colocated vs sharded table recommendation: %w", err)
		}
	}

	assessmentRecommendationsApplied = true
	SetAssessmentRecommendationsApplied()

	utils.PrintAndLog("Applied assessment recommendations.")
	return nil
}

// TODO: merge this function with applying sharded/colocated recommendation
func applyMergeConstraintsTransformations() error {
	if utils.GetEnvAsBool("YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS", false) {
		log.Infof("skipping applying merge constraints transformation due to env var YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS=true")
		return nil
	}

	utils.PrintAndLog("Applying merge constraints transformation to the exported schema")
	transformer := sqltransformer.NewTransformer()

	fileName := utils.GetObjectFilePath(schemaDir, TABLE)
	if !utils.FileOrFolderExists(fileName) { // there are no tables in exported schema
		log.Infof("table.sql file doesn't exists, skipping applying merge constraints transformation")
		return nil
	}

	rawStmts, err := queryparser.ParseSqlFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to parse table.sql file: %w", err)
	}

	transformedRawStmts, err := transformer.MergeConstraints(rawStmts.Stmts)
	if err != nil {
		return fmt.Errorf("failed to merge constraints: %w", err)
	}

	sqlStmts, err := queryparser.DeparseRawStmts(transformedRawStmts)
	if err != nil {
		return fmt.Errorf("failed to deparse transformed raw stmts: %w", err)
	}

	fileContent := strings.Join(sqlStmts, "\n\n")

	// rename the old file to table_before_merge_constraints.sql
	// replace filepath base with new name
	renamedFileName := filepath.Join(filepath.Dir(fileName), "table_before_merge_constraints.sql")
	err = os.Rename(fileName, renamedFileName)
	if err != nil {
		return fmt.Errorf("failed to rename table.sql file to table_before_merge_constraints.sql: %w", err)
	}

	err = os.WriteFile(fileName, []byte(fileContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write transformed table.sql file: %w", err)
	}

	return nil
}

func applyShardedTablesRecommendation(shardedTables []string, objType string) error {
	if shardedTables == nil {
		log.Info("list of sharded tables is null hence all the tables are recommended as colocated")
		return nil
	}

	filePath := utils.GetObjectFilePath(schemaDir, objType)
	if !utils.FileOrFolderExists(filePath) {
		// Report if the file does not exist for tables. No need to report it for mviews
		if objType == TABLE {
			utils.PrintAndLog("Required schema file %s does not exists, "+
				"returning without applying colocated/sharded tables recommendation", filePath)
		}
		return nil
	}

	log.Infof("applying colocated vs sharded tables recommendation")
	var newSQLFileContent strings.Builder
	sqlInfoArr := parseSqlFileForObjectType(filePath, objType)

	for _, sqlInfo := range sqlInfoArr {
		/*
			We can rely on pg_query to detect if it is CreateTable and also table name
			but due to time constraint this module can't be tested thoroughly so relying on the existing as much as possible

			We can pass the whole .sql file as a string also to pg_query.Parse() all the statements at once.
			But avoiding that also specially for cases where the SQL syntax can be invalid
		*/
		modifiedSqlStmt, match, err := applyShardingRecommendationIfMatching(&sqlInfo, shardedTables, objType)
		if err != nil {
			log.Errorf("failed to apply sharding recommendation for table=%q: %v", sqlInfo.objName, err)
			if match {
				utils.PrintAndLog("Unable to apply sharding recommendation for table=%q, continuing without applying...\n", sqlInfo.objName)
				utils.PrintAndLog("Please manually add the clause \"WITH (colocation = false)\" to the CREATE TABLE DDL of the '%s' table.\n", sqlInfo.objName)
			}
		} else {
			if match {
				log.Infof("original ddl - %s", sqlInfo.stmt)
				log.Infof("modified ddl - %s", modifiedSqlStmt)
			}
		}

		_, err = newSQLFileContent.WriteString(modifiedSqlStmt + "\n\n")
		if err != nil {
			return fmt.Errorf("write SQL string to string builder: %w", err)
		}
	}

	// rename existing table.sql file to table.sql.orig
	backupPath := filePath + ".orig"
	log.Infof("renaming existing file '%s' --> '%s.orig'", filePath, backupPath)
	err := os.Rename(filePath, filePath+".orig")
	if err != nil {
		return fmt.Errorf("error renaming file %s: %w", filePath, err)
	}

	// create new table.sql file for modified schema
	log.Infof("creating file %q to store the modified recommended schema", filePath)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file '%q' storing the modified recommended schema: %w", filePath, err)
	}
	if _, err = file.WriteString(newSQLFileContent.String()); err != nil {
		return fmt.Errorf("error writing to file '%q' storing the modified recommended schema: %w", filePath, err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("error closing file '%q' storing the modified recommended schema: %w", filePath, err)
	}
	var objTypeName = ""
	switch objType {
	case MVIEW:
		objTypeName = "MATERIALIZED VIEW"
	case TABLE:
		objTypeName = "TABLE"
	default:
		panic(fmt.Sprintf("Object type not supported %s", objType))
	}

	utils.PrintAndLog("Modified CREATE %s statements in %q according to the colocation and sharding recommendations of the assessment report.",
		objTypeName,
		utils.GetRelativePathFromCwd(filePath))
	utils.PrintAndLog("The original DDLs have been preserved in %q for reference.", utils.GetRelativePathFromCwd(backupPath))
	return nil
}

/*
applyShardingRecommendationIfMatching uses pg_query module to parse the given SQL stmt
In case of any errors or unexpected behaviour it return the original DDL
so in worse case, only recommendation of that table won't be followed.

# It can handle cases like multiple options in WITH clause

returns:
modifiedSqlStmt: original stmt if not sharded else modified stmt with colocation clause
match: true if its a sharded table and should be modified
error: nil/non-nil

Drawback: pg_query module doesn't have functionality to format the query after parsing
so the CREATE TABLE for sharding recommended tables will be one-liner
*/
func applyShardingRecommendationIfMatching(sqlInfo *sqlInfo, shardedTables []string, objType string) (string, bool, error) {

	stmt := sqlInfo.stmt
	formattedStmt := sqlInfo.formattedStmt
	parseTree, err := pg_query.Parse(stmt)
	if err != nil {
		return formattedStmt, false, fmt.Errorf("error parsing the stmt-%s: %v", stmt, err)
	}

	if len(parseTree.Stmts) == 0 {
		log.Warnf("parse tree is empty for stmt=%s for table '%s'", stmt, sqlInfo.objName)
		return formattedStmt, false, nil
	}

	relation := &pg_query.RangeVar{}
	switch objType {
	case MVIEW:
		createMViewNode, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateTableAsStmt)
		if !ok || createMViewNode.CreateTableAsStmt.Objtype != pg_query.ObjectType_OBJECT_MATVIEW {
			// return the original sql if it's not a Create Materialized view statement
			log.Infof("stmt=%s is not create materialized view as per the parse tree,"+
				" expected tablename=%s", stmt, sqlInfo.objName)
			return formattedStmt, false, nil
		}
		relation = createMViewNode.CreateTableAsStmt.Into.Rel
	case TABLE:
		createStmtNode, ok := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateStmt)
		if !ok { // return the original sql if it's not a CreateStmt
			log.Infof("stmt=%s is not createTable as per the parse tree, expected tablename=%s", stmt, sqlInfo.objName)
			return formattedStmt, false, nil
		}
		relation = createStmtNode.CreateStmt.Relation
	default:
		panic(fmt.Sprintf("Object type not supported %s", objType))
	}

	// true -> oracle, false -> PG
	parsedObjectName := utils.BuildObjectName(relation.Schemaname, relation.Relname)

	match := false
	switch source.DBType {
	case POSTGRESQL:
		match = slices.Contains(shardedTables, parsedObjectName)
	case ORACLE:
		// TODO: handle case-sensitivity properly
		for _, shardedTable := range shardedTables {
			// in case of oracle, shardedTable is unqualified.
			if strings.ToLower(shardedTable) == parsedObjectName {
				match = true
				break
			}
		}
	default:
		panic(fmt.Sprintf("unsupported source db type %s for applying sharding recommendations", source.DBType))
	}
	if !match {
		log.Infof("%q not present in the sharded table list", parsedObjectName)
		return formattedStmt, false, nil
	} else {
		log.Infof("%q present in the sharded table list", parsedObjectName)
	}

	colocationOption := &pg_query.DefElem{
		Defname: COLOCATION_CLAUSE,
		Arg:     pg_query.MakeStrNode("false"),
	}

	nodeForColocationOption := &pg_query.Node_DefElem{
		DefElem: colocationOption,
	}

	log.Infof("adding colocation option in the parse tree for table %s", sqlInfo.objName)
	switch objType {
	case MVIEW:
		createMViewNode, _ := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateTableAsStmt)

		if createMViewNode.CreateTableAsStmt.Into.Options == nil {
			createMViewNode.CreateTableAsStmt.Into.Options =
				[]*pg_query.Node{{Node: nodeForColocationOption}}
		} else {
			createMViewNode.CreateTableAsStmt.Into.Options = append(
				createMViewNode.CreateTableAsStmt.Into.Options,
				&pg_query.Node{Node: nodeForColocationOption})
		}
	case TABLE:
		createStmtNode, _ := parseTree.Stmts[0].Stmt.Node.(*pg_query.Node_CreateStmt)
		if createStmtNode.CreateStmt.Options == nil {
			createStmtNode.CreateStmt.Options =
				[]*pg_query.Node{{Node: nodeForColocationOption}}
		} else {
			createStmtNode.CreateStmt.Options = append(
				createStmtNode.CreateStmt.Options,
				&pg_query.Node{Node: nodeForColocationOption})
		}
	default:
		panic(fmt.Sprintf("Object type not supported %s", objType))
	}

	log.Infof("deparsing the updated parse tre into a stmt for table '%s'", parsedObjectName)
	modifiedQuery, err := pg_query.Deparse(parseTree)
	if err != nil {
		return formattedStmt, true, fmt.Errorf("error deparsing the parseTree into the query: %w", err)
	}

	// adding semi-colon at the end
	return fmt.Sprintf("%s;", modifiedQuery), true, nil
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
