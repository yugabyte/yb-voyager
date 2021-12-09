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
	"fmt"
	"os"
	"yb_migrate/migration"
	"yb_migrate/migrationutil"

	"github.com/spf13/cobra"
)

// exportSchemaCmd represents the exportSchema command
var exportSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command is used to export the schema from source database into .sql files",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		if startClean != "NO" && startClean != "YES" {
			fmt.Printf("Invalid value of flag start-clean as '%s'\n", startClean)
			os.Exit(1)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("export schema command called")

		if migrationutil.FileOrFolderExists(migrationutil.GetProjectDirPath(&source, ExportDir)) {
			fmt.Println("Project already exists")
			if startClean == "YES" {
				fmt.Printf("Deleting it before continue...\n")
				migrationutil.DeleteProjectDirIfPresent(&source, ExportDir)
			} else {
				fmt.Printf("Either remove the project or use start-clean flag as 'YES'\n")
				fmt.Println("Aborting...")
			}
		}

		exportSchema()

		/*
			// if sourceStruct and ExportDir etc are nil or undefined
			// then read the config file and create source struct from config file values
			// possibly export dir value as well and then call export Schema with the created arguments
			// else call the exportSchema directly
			if len(cfgFile) == 0 {
				exportSchema()
			} else {
				// read from config // prepare the structs and then call exportSchema
				fmt.Printf("Config path called")
			}
		*/
	},
}

func exportSchema() {
	switch source.DBType {
	case "oracle":
		fmt.Printf("Prepare Ora2Pg for schema export from Oracle\n")
		if source.Port == "" {
			source.Port = ORACLE_DEFAULT_PORT
		}
		if source.SSLMode == "" {
			//findout and add default mode
			source.SSLMode = ""
		}
		oracleExportSchema()
	case "postgres":
		fmt.Printf("Prepare ysql_dump for schema export from PG\n")
		if source.Port == "" {
			source.Port = POSTGRES_DEFAULT_PORT
		}
		if source.SSLMode == "" {
			source.SSLMode = "disable"
		}
		postgresExportSchema()
	case "mysql":
		fmt.Printf("Prepare Ora2Pg for schema export from MySQL\n")
		if source.Port == "" {
			source.Port = MYSQL_DEFAULT_PORT
		}
		if source.SSLMode == "" {
			source.SSLMode = "DISABLED"
		}
		mysqlExportSchema()
	default:
		fmt.Printf("Invalid source database type for export\n")
	}
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)

	// Hide num-connections flag from help description for this command
	exportSchemaCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().MarkHidden("num-connections")
		command.Parent().HelpFunc()(command, strings)
	})
}

func oracleExportSchema() {
	migrationutil.CheckToolsRequiredInstalledOrNot(source.DBType)

	migrationutil.CheckSourceDbAccessibility(&source)

	//Optional
	migration.PrintOracleSourceDBVersion(&source, ExportDir)

	//[TODO]: Project Name can be based on user input or some other rules.
	migrationutil.CreateMigrationProjectIfNotExists(&source, ExportDir)

	migration.OracleExtractSchema(&source, ExportDir)
}

func postgresExportSchema() {
	migrationutil.CheckToolsRequiredInstalledOrNot(source.DBType)

	migrationutil.CheckSourceDbAccessibility(&source)

	//Optional
	migration.PrintPostgresSourceDBVersion(&source, ExportDir)

	migrationutil.CreateMigrationProjectIfNotExists(&source, ExportDir)

	migration.PostgresExtractSchema(&source, ExportDir)
}

func mysqlExportSchema() {
	migrationutil.CheckToolsRequiredInstalledOrNot(source.DBType)

	migrationutil.CheckSourceDbAccessibility(&source)

	//Optional
	//TODO: causing error, debug this
	// migration.PrintMySQLSourceDBVersion(&source, ExportDir)

	migrationutil.CreateMigrationProjectIfNotExists(&source, ExportDir)

	migration.MySQLExtractSchema(&source, ExportDir)
}
