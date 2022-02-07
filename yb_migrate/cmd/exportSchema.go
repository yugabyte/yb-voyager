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

	"yb_migrate/src/migration"
	"yb_migrate/src/utils"

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

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
	},

	Run: func(cmd *cobra.Command, args []string) {
		// fmt.Println("export schema command called")

		if startClean {
			utils.CleanDir(exportDir + "/schema")
		}

		exportSchema()
		/*
			// if sourceStruct and exportDir etc are nil or undefined
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
	utils.PrintIfTrue(fmt.Sprintf("export of schema for source type as '%s'\n", source.DBType), !source.GenerateReportMode)

	switch source.DBType {
	case ORACLE:
		utils.PrintIfTrue("Prepare Ora2Pg for schema export from Oracle\n", source.VerboseMode, !source.GenerateReportMode)

		if source.SSLMode == "" {
			//TODO: findout and add default mode
			source.SSLMode = ""
		}
		oracleExportSchema()
	case POSTGRESQL:
		utils.PrintIfTrue("Prepare pg_dump for schema export from PG\n", source.VerboseMode, !source.GenerateReportMode)

		if source.SSLMode == "" {
			source.SSLMode = "prefer"
		}
		postgresExportSchema()
	case MYSQL:
		utils.PrintIfTrue("Prepare Ora2Pg for schema export from MySQL\n", source.VerboseMode, !source.GenerateReportMode)
		if source.SSLMode == "" {
			source.SSLMode = "DISABLED"
		}
		mysqlExportSchema()
	default:
		fmt.Printf("Invalid source database type for export\n")
	}

	if !source.GenerateReportMode { //check is to avoid report generation twice via generateReport command
		generateReport()
	}
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)

	// Hide num-connections flag from help description from Export Schema command
	exportSchemaCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().MarkHidden("num-connections")
		command.Parent().HelpFunc()(command, strings)
	})
}

func oracleExportSchema() {
	utils.CheckToolsRequiredInstalledOrNot(&source)

	utils.CheckSourceDbAccessibility(&source)

	//Optional
	migration.PrintOracleSourceDBVersion(&source)

	//[TODO]: Project Name can be based on user input or some other rules.
	utils.CreateMigrationProjectIfNotExists(&source, exportDir)

	migration.Ora2PgExtractSchema(&source, exportDir)
}

func postgresExportSchema() {
	utils.CheckToolsRequiredInstalledOrNot(&source)

	utils.CheckSourceDbAccessibility(&source)

	//Optional
	migration.PrintPostgresSourceDBVersion(&source)

	utils.CreateMigrationProjectIfNotExists(&source, exportDir)

	migration.PgDumpExtractSchema(&source, exportDir)
}

func mysqlExportSchema() {
	utils.CheckToolsRequiredInstalledOrNot(&source)

	utils.CheckSourceDbAccessibility(&source)

	//TODO: TEST/DEBUG this , causing error
	migration.PrintMySQLSourceDBVersion(&source) //Optional step

	utils.CreateMigrationProjectIfNotExists(&source, exportDir)

	migration.Ora2PgExtractSchema(&source, exportDir)
}
