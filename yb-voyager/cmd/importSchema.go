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
}

func importSchema() {
	utils.PrintAndLog("import of schema in %q database started", target.DBName)
	bgCtx := context.Background()

	err := target.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to target YB cluster: %s", err)
	}

	conn := target.DB().Conn()
	targetDBVersion := target.DB().GetVersion()
	fmt.Printf("Target YugabyteDB version: %s\n", targetDBVersion)

	payload := callhome.GetPayload(exportDir)
	payload.TargetDBVersion = targetDBVersion

	var targetSchemas []string
	if sourceDBType == "postgresql" { // in case of postgreSQL as source, there can be multiple schemas present in a database
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "SCHEMA")
	} else if sourceDBType == "oracle" { // ORACLE PACKAGEs are exported as SCHEMAs
		source = srcdb.Source{DBType: sourceDBType}
		targetSchemas = append(targetSchemas, YUGABYTEDB_DEFAULT_SCHEMA)
		targetSchemas = append(targetSchemas, utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "PACKAGE")...)
	}

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
				_, err := conn.Exec(bgCtx, dropSchemaQuery)
				if err != nil {
					utils.ErrExit("Failed to drop schema %q: %s", targetSchema, err)
				}
			} else {
				utils.PrintAndLog("schema '%s' already present in target database, continuing with it..\n", targetSchema)
			}
		}
	}

	if sourceDBType != POSTGRESQL {
		schemaExists := checkIfTargetSchemaExists(conn, target.Schema)
		createSchemaQuery := fmt.Sprintf("CREATE SCHEMA %s", target.Schema)
		/* --target-db-schema(or target.Schema) flag valid for Oracle & MySQL
		only create target.Schema, other required schemas are created via .sql files */
		if !schemaExists {
			utils.PrintAndLog("creating schema '%s' in target database...", target.Schema)
			_, err := conn.Exec(bgCtx, createSchemaQuery)
			if err != nil {
				utils.ErrExit("Failed to create %q schema in the target DB: %s", target.Schema, err)
			}
		}

		if target.Schema == YUGABYTEDB_DEFAULT_SCHEMA &&
			!utils.AskPrompt("do you really want to import into 'public' schema") {
			utils.ErrExit("User selected not to import in the `public` schema. Exiting.")
		}
	}

	YugabyteDBImportSchema(&target, exportDir)
	callhome.PackAndSendPayload(exportDir)
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
