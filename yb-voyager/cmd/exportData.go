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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var exportDataCmd = &cobra.Command{
	Use:   "data",
	Short: "This command is used to export table's data from source database to *.sql files",
	Long:  ``,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
	},

	Run: func(cmd *cobra.Command, args []string) {
		checkDataDirs()
		exportData()
	},
}

func init() {
	exportCmd.AddCommand(exportDataCmd)
}

func exportData() {
	utils.PrintAndLog("export of data for source type as '%s'", source.DBType)

	var success bool
	if migrationMode == "offline" {
		success = exportDataOffline()
	} else {
		success = exportDataOnline()
	}

	if success {
		createExportDataDoneFlag()
		color.Green("Export of data complete \u2705")
		log.Info("Export of data completed.")
	} else {
		color.Red("Export of data failed, retry!! \u274C")
		log.Error("Export of data failed.")
	}
}

func exportDataOffline() bool {
	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the source db: %s", err)
	}

	source.DB().CheckRequiredToolsAreInstalled()

	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	var tableList []string
	if source.TableList != "" {
		userTableList := utils.CsvStringToSlice(source.TableList)

		if source.DBType == POSTGRESQL {
			// in postgres format should be schema.table, public is default and other parts of code assume schema.table format
			for _, table := range userTableList {
				parts := strings.Split(table, ".")
				if len(parts) == 1 {
					tableList = append(tableList, "public."+table)
				} else if len(parts) == 2 {
					tableList = append(tableList, table)
				} else {
					utils.ErrExit("invalid table name %q in the --table-list flag.", table)
				}
			}
		} else {
			tableList = userTableList
		}

		log.Infof("table list for data export: %v", tableList)
		if source.VerboseMode {
			fmt.Printf("table list flag values: %v\n", tableList)
		}
	} else {
		tableList = source.DB().GetAllTableNames()
		fmt.Printf("Num tables to export: %d\n", len(tableList))
		utils.PrintAndLog("table list for data export: %v", tableList)
	}
	if len(tableList) == 0 {
		fmt.Println("no tables present to export, exiting...")
		os.Exit(0)
	}

	exportDataStart := make(chan bool)
	quitChan := make(chan bool) //for checking failure/errors of the parallel goroutines
	go func() {
		q := <-quitChan
		if q {
			log.Infoln("Cancel() being called, within exportDataOffline()")
			cancel()                    //will cancel/stop both dump tool and progress bar
			time.Sleep(time.Second * 5) //give sometime for the cancel to complete before this function returns
			utils.ErrExit("yb-voyager encountered internal error. "+
				"Check %s/yb-voyager.log for more details.", exportDir)
		}
	}()

	initializeExportTableMetadata(tableList)
	initializeExportTablePartitionMetadata(tableList)

	log.Infof("Export table metadata: %s", spew.Sdump(tablesProgressMetadata))
	UpdateTableApproxRowCount(&source, exportDir, tablesProgressMetadata)

	if source.DBType == POSTGRESQL {
		//need to export setval() calls to resume sequence value generation
		sequenceList := utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "SEQUENCE")
		tableList = append(tableList, sequenceList...)
	}
	fmt.Printf("Initiating data export.\n")
	utils.WaitGroup.Add(1)
	go source.DB().ExportData(ctx, exportDir, tableList, quitChan, exportDataStart)
	// Wait for the export data to start.
	<-exportDataStart

	UpdateFilePaths(&source, exportDir, tablesProgressMetadata)
	utils.WaitGroup.Add(1)
	exportDataStatus(ctx, tablesProgressMetadata, quitChan)

	utils.WaitGroup.Wait() // waiting for the dump and progress bars to complete

	if ctx.Err() != nil {
		fmt.Printf("ctx error(exportData.go): %v\n", ctx.Err())
		return false
	}

	source.DB().ExportDataPostProcessing(exportDir, tablesProgressMetadata)
	printExportedRowCount(datafile.OpenDescriptor(exportDir).TableRowCount)

	callhome.UpdateDataSize(exportDir)
	callhome.PackAndSendPayload(exportDir)
	return true
}

func exportDataOnline() bool {
	errMsg := "online migration not supported yet\n"
	utils.ErrExit(errMsg)

	return false
}

func validateTableListFlag(tableListString string) {
	if tableListString == "" {
		return
	}

	tableList := utils.CsvStringToSlice(tableListString)
	// TODO: update regexp once table name with double quotes are allowed/supported
	tableNameRegex := regexp.MustCompile("[a-zA-Z0-9_.]+")
	for _, table := range tableList {
		if !tableNameRegex.MatchString(table) {
			utils.ErrExit("Error: Invalid table name '%v' provided wtih --table-list flag", table)
		}
	}
}

func checkDataDirs() {
	exportDataDir := filepath.Join(exportDir, "data")
	flagFilePath := filepath.Join(exportDir, "metainfo", "flags", "exportDataDone")
	dfdFilePath := exportDir + datafile.DESCRIPTOR_PATH
	if startClean {
		utils.CleanDir(exportDataDir)
		os.Remove(flagFilePath)
		os.Remove(dfdFilePath)
	} else {
		if !utils.IsDirectoryEmpty(exportDataDir) {
			utils.ErrExit("%s/data directory is not empty, use --start-clean flag to clean the directories and start", exportDir)
		}
	}
}

func createExportDataDoneFlag() {
	exportDoneFlagPath := filepath.Join(exportDir, "metainfo", "flags", "exportDataDone")
	_, err := os.Create(exportDoneFlagPath)
	if err != nil {
		utils.ErrExit("creating exportDataDone flag: %v", err)
	}
}
