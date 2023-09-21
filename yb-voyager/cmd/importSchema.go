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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var importSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command imports schema into the target YugabyteDB database",

	PreRun: func(cmd *cobra.Command, args []string) {
		if tconf.TargetDBType == "" {
			tconf.TargetDBType = YUGABYTEDB
		}
		validateImportFlags(cmd, TARGET_DB_IMPORTER_ROLE)
	},

	Run: func(cmd *cobra.Command, args []string) {
		tconf.ImportMode = true
		sourceDBType = ExtractMetaInfo(exportDir).SourceDBType
		importSchema()
	},
}

func init() {
	importCmd.AddCommand(importSchemaCmd)
	registerCommonGlobalFlags(importSchemaCmd)
	registerCommonImportFlags(importSchemaCmd)
	registerTargetDBConnFlags(importSchemaCmd)
	registerImportSchemaFlags(importSchemaCmd)
}

var flagPostImportData utils.BoolStr
var importObjectsInStraightOrder utils.BoolStr
var flagRefreshMViews utils.BoolStr

func importSchema() {
	err := retrieveMigrationUUID(exportDir)
	if err != nil {
		utils.ErrExit("failed to get migration UUID: %w", err)
	}
	tconf.Schema = strings.ToLower(tconf.Schema)

	conn, err := pgx.Connect(context.Background(), tconf.GetConnectionUri())
	if err != nil {
		utils.ErrExit("Unable to connect to target YugabyteDB database: %v", err)
	}
	defer conn.Close(context.Background())

	targetDBVersion := ""
	query := "SELECT setting FROM pg_settings WHERE name = 'server_version'"
	err = conn.QueryRow(context.Background(), query).Scan(&targetDBVersion)
	if err != nil {
		utils.ErrExit("get target db version: %s", err)
	}
	utils.PrintAndLog("YugabyteDB version: %s\n", targetDBVersion)

	payload := callhome.GetPayload(exportDir, migrationUUID)
	payload.TargetDBVersion = targetDBVersion

	if !flagPostImportData {
		filePath := filepath.Join(exportDir, "schema", "uncategorized.sql")
		if utils.FileOrFolderExists(filePath) {
			color.Red("\nIMPORTANT NOTE: Please, review and manually import the DDL statements from the %q\n", filePath)
		}

		createTargetSchemas(conn)

		if sourceDBType == ORACLE && enableOrafce {
			// Install Orafce extension in target YugabyteDB.
			installOrafce(conn)
		}
	}
	var objectList []string

	objectsToImportAfterData := []string{"INDEX", "FTS_INDEX", "PARTITION_INDEX", "TRIGGER"}
	if !flagPostImportData { // Pre data load.
		// This list also has defined the order to create object type in target YugabyteDB.
		objectList = utils.GetSchemaObjectList(sourceDBType)
		objectList = utils.SetDifference(objectList, objectsToImportAfterData)
		if len(objectList) == 0 {
			utils.ErrExit("No schema objects to import! Must import at least 1 of the supported schema object types: %v", utils.GetSchemaObjectList(sourceDBType))
		}
	} else { // Post data load.
		objectList = objectsToImportAfterData
	}
	objectList = applySchemaObjectFilterFlags(objectList)
	log.Infof("List of schema objects to import: %v", objectList)

	// Import some statements only after importing everything else
	isSkipStatement := func(objType, stmt string) bool {
		stmt = strings.ToUpper(strings.TrimSpace(stmt))
		switch objType {
		case "SEQUENCE":
			// ALTER TABLE table_name ALTER COLUMN column_name ... ('sequence_name');
			// ALTER SEQUENCE sequence_name OWNED BY table_name.column_name;
			return strings.HasPrefix(stmt, "ALTER TABLE") || strings.HasPrefix(stmt, "ALTER SEQUENCE")
		case "TABLE":
			// skips the ALTER TABLE table_name ADD CONSTRAINT constraint_name FOREIGN KEY (column_name) REFERENCES another_table_name(another_column_name);
			return strings.Contains(stmt, "ALTER TABLE") && strings.Contains(stmt, "FOREIGN KEY")
		case "UNIQUE INDEX":
			// skips all the INDEX DDLs, Except CREATE UNIQUE INDEX index_name ON table ... (column_name);
			return !strings.Contains(stmt, objType)
		case "INDEX":
			// skips all the CREATE UNIQUE INDEX index_name ON table ... (column_name);
			return strings.Contains(stmt, "UNIQUE INDEX")
		}
		return false
	}
	skipFn := isSkipStatement
	importSchemaInternal(exportDir, objectList, skipFn)

	// Import the skipped ALTER TABLE statements from sequence.sql and table.sql if it exists
	skipFn = func(objType, stmt string) bool {
		return !isSkipStatement(objType, stmt)
	}
	if slices.Contains(objectList, "SEQUENCE") {
		importSchemaInternal(exportDir, []string{"SEQUENCE"}, skipFn)
	}
	if slices.Contains(objectList, "TABLE") {
		importSchemaInternal(exportDir, []string{"TABLE"}, skipFn)
	}

	importDefferedStatements()
	log.Info("Schema import is complete.")

	dumpStatements(failedSqlStmts, filepath.Join(exportDir, "schema", "failed.sql"))

	if flagPostImportData {
		if flagRefreshMViews {
			refreshMViews(conn)
		}
	} else {
		utils.PrintAndLog("\nNOTE: Materialised Views are not populated by default. To populate them, pass --refresh-mviews while executing `import schema --post-import-data`.")
	}

	callhome.PackAndSendPayload(exportDir)
}

func dumpStatements(stmts []string, filePath string) {
	if len(stmts) == 0 {
		if utils.FileOrFolderExists(filePath) {
			err := os.Remove(filePath)
			if err != nil {
				utils.ErrExit("remove file: %v", err)
			}
		}
		log.Infof("no failed sql statements to dump")
		return
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		utils.ErrExit("open file: %v", err)
	}

	for i := 0; i < len(stmts); i++ {
		_, err = file.WriteString(stmts[i] + "\n\n")
		if err != nil {
			utils.ErrExit("failed writing in file %s: %v", filePath, err)
		}
	}

	msg := fmt.Sprintf("\nSQL statements failed during migration are present in %q file\n", filePath)
	color.Red(msg)
	log.Info(msg)
}

func installOrafce(conn *pgx.Conn) {
	utils.PrintAndLog("Installing Orafce extension in target YugabyteDB")
	_, err := conn.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS orafce")
	if err != nil {
		utils.ErrExit("failed to install Orafce extension: %v", err)
	}
}

func refreshMViews(conn *pgx.Conn) {
	utils.PrintAndLog("\nRefreshing Materialised Views..\n\n")
	var mViewNames []string
	mViewsSqlInfoArr := getDDLStmts("MVIEW")
	for _, eachMviewSql := range mViewsSqlInfoArr {
		if strings.Contains(strings.ToUpper(eachMviewSql.stmt), "CREATE MATERIALIZED VIEW") {
			mViewNames = append(mViewNames, eachMviewSql.objName)
		}
	}
	log.Infof("List of Mviews Imported to refresh - %v", mViewNames)
	for _, mViewName := range mViewNames {
		query := fmt.Sprintf("REFRESH MATERIALIZED VIEW %s", mViewName)
		_, err := conn.Exec(context.Background(), query)
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "has not been populated") {
			utils.ErrExit("error in refreshing the materialised view %s: %v", mViewName, err)
		}
	}
	log.Infof("Checking if mviews are refreshed or not - %v", mViewNames)
	var mviewsNotRefreshed []string
	for _, mViewName := range mViewNames {
		query := fmt.Sprintf("SELECT * from %s LIMIT 1;", mViewName)
		rows, err := conn.Query(context.Background(), query)
		if err != nil {
			utils.ErrExit("error in checking whether mview %s is refreshed or not: %v", mViewName, err)
		}
		if !rows.Next() {
			mviewsNotRefreshed = append(mviewsNotRefreshed, mViewName)
		}
		rows.Close()
	}
	if len(mviewsNotRefreshed) > 0 {
		utils.PrintAndLog("\nNOTE: Following Materialised Views might not be refreshed - %v, Please verify and refresh them manually if required!", mviewsNotRefreshed)
	}
}

func getDDLStmts(objType string) []sqlInfo {
	var sqlInfoArr []sqlInfo
	schemaDir := filepath.Join(exportDir, "schema")
	importMViewFilePath := utils.GetObjectFilePath(schemaDir, objType)
	if utils.FileOrFolderExists(importMViewFilePath) {
		sqlInfoArr = createSqlStrInfoArray(importMViewFilePath, objType)
	}
	return sqlInfoArr
}

func createTargetSchemas(conn *pgx.Conn) {
	var targetSchemas []string
	tconf.Schema = strings.ToLower(strings.Trim(tconf.Schema, "\"")) //trim case sensitivity quotes if needed, convert to lowercase
	switch sourceDBType {
	case "postgresql": // in case of postgreSQL as source, there can be multiple schemas present in a database
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "SCHEMA")
	case "oracle": // ORACLE PACKAGEs are exported as SCHEMAs
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = append(targetSchemas, tconf.Schema)
		targetSchemas = append(targetSchemas, utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "PACKAGE")...)
	case "mysql":
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = append(targetSchemas, tconf.Schema)

	}
	targetSchemas = utils.ToCaseInsensitiveNames(targetSchemas)

	utils.PrintAndLog("schemas to be present in target database %q: %v\n", tconf.DBName, targetSchemas)

	for _, targetSchema := range targetSchemas {
		//check if target schema exists or not
		schemaExists := checkIfTargetSchemaExists(conn, targetSchema)
		dropSchemaQuery := fmt.Sprintf("DROP SCHEMA %s CASCADE", targetSchema)

		if schemaExists {
			if startClean {
				promptMsg := fmt.Sprintf("do you really want to drop the '%s' schema", targetSchema)
				if !utils.AskPrompt(promptMsg) {
					continue
				}

				utils.PrintAndLog("dropping schema '%s' in target database", targetSchema)
				_, err := conn.Exec(context.Background(), dropSchemaQuery)
				if err != nil {
					utils.ErrExit("Failed to drop schema %q: %s", targetSchema, err)
				}
			} else {
				utils.PrintAndLog("schema '%s' already present in target database, continuing with it..\n", targetSchema)
			}
		}
	}

	if sourceDBType != POSTGRESQL { // with the new schema list flag, pg_dump takes care of all schema creation DDLs
		schemaExists := checkIfTargetSchemaExists(conn, tconf.Schema)
		createSchemaQuery := fmt.Sprintf("CREATE SCHEMA %s", tconf.Schema)
		/* --target-db-schema(or target.Schema) flag valid for Oracle & MySQL
		only create target.Schema, other required schemas are created via .sql files */
		if !schemaExists {
			utils.PrintAndLog("creating schema '%s' in target database...", tconf.Schema)
			_, err := conn.Exec(context.Background(), createSchemaQuery)
			if err != nil {
				utils.ErrExit("Failed to create %q schema in the target DB: %s", tconf.Schema, err)
			}
		}

		if tconf.Schema == YUGABYTEDB_DEFAULT_SCHEMA &&
			!utils.AskPrompt("do you really want to import into 'public' schema") {
			utils.ErrExit("User selected not to import in the `public` schema. Exiting.")
		}
	}
}

func checkIfTargetSchemaExists(conn *pgx.Conn, targetSchema string) bool {
	checkSchemaExistQuery := fmt.Sprintf("SELECT schema_name FROM information_schema.schemata WHERE schema_name = '%s'", targetSchema)

	var fetchedSchema string
	err := conn.QueryRow(context.Background(), checkSchemaExistQuery).Scan(&fetchedSchema)
	log.Infof("check if schema %q exists: fetchedSchema: %q, err: %s", targetSchema, fetchedSchema, err)
	if err != nil && (strings.Contains(err.Error(), "no rows in result set") && fetchedSchema == "") {
		return false
	} else if err != nil {
		utils.ErrExit("Failed to check if schema %q exists: %s", targetSchema, err)
	}

	return fetchedSchema == targetSchema
}

func missingRequiredSchemaObject(err error) bool {
	return strings.Contains(err.Error(), "does not exist")
}

func isAlreadyExists(errString string) bool {
	alreadyExistsErrors := []string{"already exists",
		"multiple primary keys",
		"already a partition"}
	for _, subStr := range alreadyExistsErrors {
		if strings.Contains(errString, subStr) {
			return true
		}
	}
	return false
}
