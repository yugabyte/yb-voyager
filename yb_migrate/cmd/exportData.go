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
	"yb_migrate/migrationutil"

	"github.com/spf13/cobra"
)

var exportMode string

// exportDataCmd represents the exportData command
var exportDataCmd = &cobra.Command{
	Use:   "data",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("export data called")
		exportData()
	},
}

func init() {
	exportCmd.AddCommand(exportDataCmd)

	exportDataCmd.PersistentFlags().StringVar(&exportMode, "export mode", "offline",
		"offline | online")
}

func exportData() {
	projectDirName := migrationutil.GetProjectDirName(source.DBType, source.Schema, source.DBName)

	if source.DBType == "oracle" {
		migrationutil.CreateMigrationProject(exportDir, projectDirName, source.Schema)
	} else {
		migrationutil.CreateMigrationProject(exportDir, projectDirName, source.DBName)
	}

	if exportMode == "offline" {
		exportDataOffline()
	} else {
		exportDataOnline()
	}
}

func exportDataOffline() {
	/*
		TODO: check and clean the data directory before export everytime this command
	*/

	switch source.DBType {
	case "oracle":
		fmt.Printf("Prepare Ora2Pg for data export from Oracle\n")
		if source.Port == "" {
			source.Port = "1521"
		}
		migration.OracleExportDataOffline(&source, exportDir)
	case "postgres":
		fmt.Printf("Prepare ysql_dump for data export from PG\n")
		if source.Port == "" {
			source.Port = "5432"
		}
		migration.PostgresExportDataOffline(&source, exportDir)
	case "mysql":
		fmt.Printf("Prepare Ora2Pg for data export from MySQL\n")
		if source.Port == "" {
			source.Port = "3306"
		}
		// migration.mysqlDataExportOffline()
	}

}

func exportDataOnline() {}
