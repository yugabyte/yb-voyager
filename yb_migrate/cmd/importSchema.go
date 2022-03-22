/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/spf13/cobra"
	"github.com/yugabyte/ybm/yb_migrate/src/utils"
)

// importSchemaCmd represents the importSchema command
var importSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command imports schema into the destination YugabyteDB database",
	Long:  `Long version This command imports schema into the destination YUgabyteDB database.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
		// fmt.Println("Import Schema PersistentPreRun")
	},

	Run: func(cmd *cobra.Command, args []string) {
		target.ImportMode = true
		importSchema()
	},
}

func init() {
	importCmd.AddCommand(importSchemaCmd)
}

func importSchema() {
	fmt.Printf("import of schema in '%s' database started\n", target.DBName)

	targetConnectionURIWithGivenDB := generateTargetDBUri(&target)

	bgCtx := context.Background()
	conn, err := pgx.Connect(bgCtx, targetConnectionURIWithGivenDB)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer conn.Close(bgCtx)

	PrintTargetYugabyteDBVersion(&target)

	// in case of postgreSQL as source, there can be multiple schemas present in a database
	targetSchemas := []string{target.Schema}
	sourceDBType := ExtractMetaInfo(exportDir).SourceDBType
	if sourceDBType == "postgresql" {
		source = utils.Source{DBType: sourceDBType}
		targetSchemas = append(targetSchemas, utils.GetObjectNameListFromReport(generateReportHelper(), "SCHEMA")...)
	}

	utils.PrintIfTrue(fmt.Sprintf("schemas to be present in target database: %v\n", targetSchemas), target.VerboseMode)

	for _, targetSchema := range targetSchemas {
		//check if target schema exists or not
		schemaExists := checkIfTargetSchemaExists(conn, targetSchema)

		dropSchemaQuery := fmt.Sprintf("DROP SCHEMA %s CASCADE", targetSchema)
		createSchemaQuery := fmt.Sprintf("CREATE SCHEMA %s", targetSchema)

		// schema dropping or creating based on startClean and schemaExists boolean flags
		if startClean {
			if schemaExists {
				promptMsg := fmt.Sprintf("do you really want to drop the '%s' schema", targetSchema)
				if !utils.AskPrompt(promptMsg) {
					continue
				}

				fmt.Printf("dropping schema '%s' in target database\n", targetSchema)
				_, err := conn.Exec(bgCtx, dropSchemaQuery)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

			} else {
				fmt.Printf("schema '%s' in target database doesn't exist\n", targetSchema)
			}

			//in case of postgres, CREATE SCHEMA DDLs for non-public schemas are already present in .sql files
			if sourceDBType != "postgresql" || targetSchema == "public" {
				fmt.Printf("creating schema '%s' in target database...\n", targetSchema)
				_, err := conn.Exec(bgCtx, createSchemaQuery)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
			}
		} else {
			if schemaExists {
				fmt.Printf("already present schema '%s' in target database, continuing with it..\n", targetSchema)
			} else {
				fmt.Printf("creating schema '%s' in target database...\n", targetSchema)
				_, err := conn.Exec(bgCtx, createSchemaQuery)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
			}
		}
	}

	if sourceDBType != POSTGRESQL && target.Schema == "public" && !utils.AskPrompt("do you really want to import into 'public' schema") {
		os.Exit(1)
	}

	YugabyteDBImportSchema(&target, exportDir)
}

func checkIfTargetSchemaExists(conn *pgx.Conn, targetSchema string) bool {
	checkSchemaExistQuery := fmt.Sprintf("SELECT schema_name FROM information_schema.schemata WHERE schema_name = '%s'", targetSchema)

	var fetchedSchema string
	err := conn.QueryRow(context.Background(), checkSchemaExistQuery).Scan(&fetchedSchema)

	if err != nil && (strings.Contains(err.Error(), "no rows in result set") && fetchedSchema == "") {
		return false
	} else if err != nil {
		// fmt.Println(err)
		// os.Exit(1)
		panic(err)
	}

	return fetchedSchema == targetSchema
}

func generateSSLQueryStringIfNotExists(t *utils.Target) string {
	SSLQueryString := ""
	if t.SSLMode == "" {
		t.SSLMode = "prefer"
	}
	if t.SSLQueryString == "" {

		if t.SSLMode == "disable" || t.SSLMode == "allow" || t.SSLMode == "prefer" || t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
			SSLQueryString = "sslmode=" + t.SSLMode
			if t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
				SSLQueryString = fmt.Sprintf("sslmode=%s", t.SSLMode)
				if t.SSLCertPath != "" {
					SSLQueryString += "&sslcert=" + t.SSLCertPath
				}
				if t.SSLKey != "" {
					SSLQueryString += "&sslkey=" + t.SSLKey
				}
				if t.SSLRootCert != "" {
					SSLQueryString += "&sslrootcert=" + t.SSLRootCert
				}
				if t.SSLCRL != "" {
					SSLQueryString += "&sslcrl=" + t.SSLCRL
				}
			}
		} else {
			fmt.Println("Invalid sslmode entered")
		}
	} else {
		SSLQueryString = t.SSLQueryString
	}
	return SSLQueryString
}

func generateTargetDBUri(t *utils.Target) string {
	if t.Uri != "" {
		return t.Uri
	} else {
		return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?%s", target.User,
			target.Password, target.Host, target.Port, target.DBName, generateSSLQueryStringIfNotExists(&target))
	}
}
