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
	"os/exec"
	"strings"
	"yb_migrate/src/migration"
	"yb_migrate/src/utils"

	"github.com/spf13/cobra"
	"github.com/tevino/abool/v2"
)

var ExportDataDone = abool.New()

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

		exportData()
	},
}

func init() {
	exportCmd.AddCommand(exportDataCmd)
}

func exportData() {

	if migrationMode == "offline" {
		exportDataOffline()
	} else {
		exportDataOnline()
	}

	err := exec.Command("touch", exportDir+"/metainfo/data/"+"exportDone").Run()
	utils.CheckError(err, "", "couldn't touch file exportDone in metainfo/data folder", true)
}

func exportDataOffline() {

	switch source.DBType {
	case "oracle":
		fmt.Printf("Prepare Ora2Pg for data export from Oracle\n")
		if source.Port == "" {
			source.Port = "1521"
		}
		migration.Ora2PgExportDataOffline(&source, exportDir)
	case "postgres":
		fmt.Printf("Prepare pg_dump for data export from PG\n")
		if source.Port == "" {
			source.Port = "5432"
		}
		go migration.PgDumpExportDataOffline(&source, exportDir, ExportDataDone)
	case "mysql":
		fmt.Printf("Prepare Ora2Pg for data export from MySQL\n")
		if source.Port == "" {
			source.Port = "3306"
		}
		migration.Ora2PgExportDataOffline(&source, exportDir)
	}

	tableList := migration.GetTableList(exportDir)
	tablesMetadata := createExportTableMetadataSlice(exportDir, tableList)

	migration.UpdateDataFilePath(&source, exportDir, tablesMetadata)

	migration.UpdateTableRowCount(&source, exportDir, tablesMetadata)

	exportDataStatus(tablesMetadata)

	if source.DBType == "postgres" { //not required for oracle, mysql
		migration.ExportDataPostProcessing(exportDir)
	}
}

func exportDataOnline() {}

func createExportTableMetadataSlice(exportDir string, tableList []string) []utils.ExportTableMetadata {
	numTables := len(tableList)
	tablesMetadata := make([]utils.ExportTableMetadata, numTables)

	for i := 0; i < numTables; i++ {
		tableInfo := strings.Split(tableList[i], ".")
		if len(tableInfo) > 1 { //postgres
			tablesMetadata[i].TableSchema = tableInfo[0]
			tablesMetadata[i].TableName = tableInfo[1]
		} else { //oracle
			tablesMetadata[i].TableSchema = source.Schema
			tablesMetadata[i].TableName = tableInfo[0]
		}

		// tablesMetadata[i].DataFilePath; will be updated when status changes to IN-PROGRESS
		// tablesMetadata[i].CountTotalRows; will be done by other func
		tablesMetadata[i].CountLiveRows = int64(0)
		tablesMetadata[i].Status = "NOT-STARTED"
		tablesMetadata[i].FileOffsetToContinue = int64(0)
	}

	return tablesMetadata
}
