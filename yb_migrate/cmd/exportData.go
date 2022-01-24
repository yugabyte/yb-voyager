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
	"os/exec"
	"strings"
	"time"
	"yb_migrate/src/migration"
	"yb_migrate/src/utils"

	"github.com/fatih/color"
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

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
	},

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("export data command called")

		if startClean {
			utils.CleanDir(exportDir + "/data")
		}

		exportData()
	},
}

func init() {
	exportCmd.AddCommand(exportDataCmd)
}

func exportData() {
	var success bool
	if migrationMode == "offline" {
		success = exportDataOffline()
	} else {
		success = exportDataOnline()
	}

	if success {
		err := exec.Command("touch", exportDir+"/metainfo/flags/exportDone").Run() //to inform import data command
		utils.CheckError(err, "", "couldn't touch file exportDone in metainfo/data folder", true)
		color.Green("Export of data complete \u2705")
	} else {
		color.Red("Export of data failed, retry!! \u274C")
	}
}

func exportDataOffline() bool {
	ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	tableList := migration.GetTableList(exportDir)
	fmt.Printf("tables for data export: %v\n", tableList)

	exportDataStart := make(chan bool)
	quitChan := make(chan bool) //for checking failure/errors of the parallel goroutines
	go func() {
		q := <-quitChan
		if q {
			fmt.Println("cancel(), quitchan, main exportDataOffline()")
			cancel()                    //will cancel/stop both dump tool and progress bar
			time.Sleep(time.Second * 5) //give sometime for the cancel to complete before this function returns
			fmt.Println("Cancelled the context having dump command and progress bar")
		}
	}()

	switch source.DBType {
	case ORACLE:
		fmt.Printf("Preparing for data export from Oracle\n")
		utils.WaitGroup.Add(1)
		go migration.Ora2PgExportDataOffline(ctx, &source, exportDir, quitChan, exportDataStart)

	case POSTGRESQL:
		fmt.Printf("Preparing for data export from Postgres\n")
		utils.WaitGroup.Add(1)
		go migration.PgDumpExportDataOffline(ctx, &source, exportDir, quitChan, exportDataStart)

	case MYSQL:
		fmt.Printf("Preparing for data export from MySQL\n")
		utils.WaitGroup.Add(1)
		go migration.Ora2PgExportDataOffline(ctx, &source, exportDir, quitChan, exportDataStart)

	}

	//wait for the export data to start
	// fmt.Println("passing the exportDataStart channel receiver")
	<-exportDataStart
	// fmt.Println("passed the exportDataStart channel receiver")

	tablesMetadata := createExportTableMetadataSlice(exportDir, tableList)
	// fmt.Println(tableList, "\n", tablesMetadata)

	migration.UpdateDataFilePath(&source, exportDir, tablesMetadata)

	migration.UpdateTableRowCount(&source, exportDir, tablesMetadata)

	exportDataStatus(ctx, tablesMetadata, quitChan)

	utils.WaitGroup.Wait() //waiting for the dump to complete

	if ctx.Err() != nil {
		fmt.Printf("ctx error(exportData.go): %v\n", ctx.Err())
		return false
	}

	migration.ExportDataPostProcessing(exportDir, &tablesMetadata)

	return true
}

func exportDataOnline() bool {
	return true // empty function
}

func createExportTableMetadataSlice(exportDir string, tableList []string) []utils.TableProgressMetadata {
	numTables := len(tableList)
	tablesMetadata := make([]utils.TableProgressMetadata, numTables)

	for i := 0; i < numTables; i++ {
		tableInfo := strings.Split(tableList[i], ".")
		if source.DBType == POSTGRESQL { //format for every table: schema.tablename
			tablesMetadata[i].TableSchema = tableInfo[0]
			tablesMetadata[i].TableName = tableInfo[1]
		} else { //oracle,mysql
			tablesMetadata[i].TableSchema = source.Schema
			tablesMetadata[i].TableName = tableInfo[0]
		}

		//Initializing all the members of struct
		tablesMetadata[i].DataFilePath = ""         //will be updated when status changes to IN-PROGRESS
		tablesMetadata[i].CountTotalRows = int64(0) //will be updated by other func
		tablesMetadata[i].CountLiveRows = int64(0)
		tablesMetadata[i].Status = 0
		tablesMetadata[i].FileOffsetToContinue = int64(0)
	}

	return tablesMetadata
}
