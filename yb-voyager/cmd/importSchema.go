/*
Copyright (c) YugaByte, Inc.

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
	"strings"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
)

var importSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command imports schema into the destination YugabyteDB database",
	Long:  ``,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateImportFlags()
	},

	Run: func(cmd *cobra.Command, args []string) {
		target.ImportMode = true
		sourceDBType = ExtractMetaInfo(exportDir).SourceDBType
		importSchema()
	},
}

func init() {
	importCmd.AddCommand(importSchemaCmd)
	registerCommonImportFlags(importSchemaCmd)
	registerImportSchemaFlags(importSchemaCmd)
}

var flagPostImportData bool

func importSchema() {
	err := target.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to target YB cluster: %s", err)
	}
	conn := target.DB().Conn()
	targetDBVersion := target.DB().GetVersion()
	utils.PrintAndLog("YugabyteDB version: %s\n", targetDBVersion)

	payload := callhome.GetPayload(exportDir)
	payload.TargetDBVersion = targetDBVersion

	if !flagPostImportData {
		createTargetSchemas(conn)
	}
	var objectList []string
	if !flagPostImportData { // Pre data load.
		// This list also has defined the order to create object type in target YugabyteDB.
		objectList = utils.GetSchemaObjectList(sourceDBType)
		objectList = utils.SetDifference(objectList, []string{"TRIGGER", "INDEX"})
		if len(objectList) == 0 {
			utils.ErrExit("No schema objects to import! Must import at least 1 of the supported schema object types: %v", utils.GetSchemaObjectList(sourceDBType))
		}
	} else { // Post data load.
		objectList = []string{"INDEX", "TRIGGER"}
	}
	objectList = applySchemaObjectFilterFlags(objectList)
	log.Infof("List of schema objects to import: %v", objectList)
	importSchemaInternal(&target, exportDir, objectList, nil)
	callhome.PackAndSendPayload(exportDir)
}

func createTargetSchemas(conn *pgx.Conn) {
	var targetSchemas []string
	target.Schema = strings.ToLower(strings.Trim(target.Schema, "\"")) //trim case sensitivity quotes if needed, convert to lowercase
	switch sourceDBType {
	case "postgresql": // in case of postgreSQL as source, there can be multiple schemas present in a database
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "SCHEMA")
	case "oracle": // ORACLE PACKAGEs are exported as SCHEMAs
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = append(targetSchemas, target.Schema)
		targetSchemas = append(targetSchemas, utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "PACKAGE")...)
	case "mysql":
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = append(targetSchemas, target.Schema)

	}
	targetSchemas = utils.ToCaseInsensitiveNames(targetSchemas)

	utils.PrintAndLog("schemas to be present in target database: %v\n", targetSchemas)

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
		schemaExists := checkIfTargetSchemaExists(conn, target.Schema)
		createSchemaQuery := fmt.Sprintf("CREATE SCHEMA %s", target.Schema)
		/* --target-db-schema(or target.Schema) flag valid for Oracle & MySQL
		only create target.Schema, other required schemas are created via .sql files */
		if !schemaExists {
			utils.PrintAndLog("creating schema '%s' in target database...", target.Schema)
			_, err := conn.Exec(context.Background(), createSchemaQuery)
			if err != nil {
				utils.ErrExit("Failed to create %q schema in the target DB: %s", target.Schema, err)
			}
		}

		if target.Schema == YUGABYTEDB_DEFAULT_SCHEMA &&
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
