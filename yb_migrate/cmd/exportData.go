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
	"yb_migrate/migration"

	"github.com/spf13/cobra"
)

// exportDataCmd represents the exportData command
var exportDataCmd = &cobra.Command{
	Use:   "data",
	Short: "This commands is used to export table's data from source database to *.sql files",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("export data command called")

		// if migrationutil.FileOrFolderExists(migrationutil.GetProjectDirPath(&source, ExportDir)) {
		// 	fmt.Println("Project already exists")
		// 	if StartClean == "YES" {
		// 		fmt.Printf("Deleting it before continue...\n")
		// 		migrationutil.DeleteProjectDirIfPresent(&source, ExportDir)
		// 	} else {
		// 		fmt.Printf("Either remove the project or use start-clean flag as 'YES'\n")
		// 		fmt.Println("Aborting...")
		//		os.Exit(1)
		// 	}
		// }

		exportData()
	},
}

func init() {
	exportCmd.AddCommand(exportDataCmd)
}

func exportData() {

	/*
		TODO: Check and Ask if want to use the existing project directory or recreate it

		projectDirName := migrationutil.GetProjectDirName(&source)
		if source.DBType == "oracle" {
			migrationutil.CreateMigrationProjectIfNotExists(ExportDir, projectDirName, source.Schema)
		} else {
			migrationutil.CreateMigrationProjectIfNotExists(ExportDir, projectDirName, source.DBName)
		}
	*/

	if MigrationMode == "offline" {
		exportDataOffline()
	} else {
		exportDataOnline()
	}
}

func exportDataOffline() {
	/*
		TODO: check and clean subdirs under the data dir, before exportData everytime
		Also if the project is not created then create it
	*/

	switch source.DBType {
	case "oracle":
		fmt.Printf("Prepare Ora2Pg for data export from Oracle\n")
		if source.Port == "" {
			source.Port = "1521"
		}
		migration.Ora2PgExportDataOffline(&source, ExportDir)
	case "postgres":
		fmt.Printf("Prepare pg_dump for data export from PG\n")
		if source.Port == "" {
			source.Port = "5432"
		}
		migration.PgDumpExportDataOffline(&source, ExportDir)
	case "mysql":
		fmt.Printf("Prepare Ora2Pg for data export from MySQL\n")
		if source.Port == "" {
			source.Port = "3306"
		}
		migration.Ora2PgExportDataOffline(&source, ExportDir)
	}

}

func exportDataOnline() {}
