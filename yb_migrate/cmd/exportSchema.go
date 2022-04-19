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
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/migration"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"

	"github.com/spf13/cobra"
)

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
		checkSchemaDirs()
		exportSchema()
	},
}

func exportSchema() {
	utils.PrintIfTrue(fmt.Sprintf("export of schema for source type as '%s'\n", source.DBType), !source.GenerateReportMode)

	utils.CheckToolsRequiredInstalledOrNot(&source)

	migration.CheckSourceDBAccessibility(&source)

	if !source.GenerateReportMode {
		migration.PrintSourceDBVersion(&source)
	}

	utils.CreateMigrationProjectIfNotExists(&source, exportDir)

	switch source.DBType {
	case ORACLE:
		utils.PrintIfTrue("preparing Ora2Pg for schema export from Oracle\n", source.VerboseMode, !source.GenerateReportMode)

		migration.Ora2PgExtractSchema(&source, exportDir)
	case POSTGRESQL:
		utils.PrintIfTrue("preparing pg_dump for schema export from PG\n", source.VerboseMode, !source.GenerateReportMode)

		migration.PgDumpExtractSchema(&source, exportDir)
	case MYSQL:
		utils.PrintIfTrue("preparing Ora2Pg for schema export from MySQL\n", source.VerboseMode, !source.GenerateReportMode)

		migration.Ora2PgExtractSchema(&source, exportDir)
	default:
		fmt.Printf("Invalid source database type for export\n")
	}

	if !source.GenerateReportMode { //check is to avoid report generation twice via generateReport command
		fmt.Printf("\nexported schema files created under directory: %s\n", exportDir+"/schema")
		generateReport()
	}
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)

	// Hide num-connections flag from help description from Export Schema command
	exportSchemaCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().MarkHidden("parallel-jobs")
		command.Parent().HelpFunc()(command, strings)
	})

}

func checkSchemaDirs() {
	schemaDir := exportDir + "/schema"
	tempDir := exportDir + "/temp"
	reportDir := exportDir + "/reports"
	metainfoSchemaDir := exportDir + "/metainfo/schema"
	if startClean {
		utils.CleanDir(schemaDir)
		utils.CleanDir(tempDir)
		utils.CleanDir(reportDir)
		utils.CleanDir(metainfoSchemaDir)
	} else {
		if !utils.IsDirectoryEmpty(schemaDir) {
			fmt.Fprintf(os.Stderr, "schema directory is not empty, use --start-clean flag to clean the directories and start")
			log.Fatalf("schema directory is not empty, use --start-clean flag to clean the directories and start")
		}
		if !utils.IsDirectoryEmpty(metainfoSchemaDir) {
			fmt.Fprintf(os.Stderr, "metainfo/schema directory is not empty, use --start-clean flag to clean the directories and start")
			log.Fatalf("metainfo/schema directory is not empty, use --start-clean flag to clean the directories and start")
		}
	}
}
