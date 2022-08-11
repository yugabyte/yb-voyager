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
	exportDataCmd.Flags().BoolVar(&disablePb, "disable-pb", false,
		"true - to disable progress bar during data export(default false)")
	exportDataCmd.Flags().StringVar(&source.ExcludeTableList, "exclude-table-list", "",
		"List of tables to exclude while exporting data(no-op if --table-list is used)")
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
	var finalTableList []string
	excludeTableList := extractTableListFromString(source.ExcludeTableList)
	if source.TableList != "" {
		finalTableList = extractTableListFromString(source.TableList)
	} else {
		tableList = source.DB().GetAllTableNames()
		finalTableList = removeExcludeTables(tableList, excludeTableList)
		fmt.Printf("Num tables to export: %d\n", len(finalTableList))
		utils.PrintAndLog("table list for data export: %v", finalTableList)
	}
	if len(finalTableList) == 0 {
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

	initializeExportTableMetadata(finalTableList)
	initializeExportTablePartitionMetadata(finalTableList)

	log.Infof("Export table metadata: %s", spew.Sdump(tablesProgressMetadata))
	UpdateTableApproxRowCount(&source, exportDir, tablesProgressMetadata)

	if source.DBType == POSTGRESQL {
		//need to export setval() calls to resume sequence value generation
		sequenceList := utils.GetObjectNameListFromReport(analyzeSchemaInternal(), "SEQUENCE")
		finalTableList = append(finalTableList, sequenceList...)
	}
	fmt.Printf("Initiating data export.\n")
	utils.WaitGroup.Add(1)
	go source.DB().ExportData(ctx, exportDir, finalTableList, quitChan, exportDataStart)
	// Wait for the export data to start.
	<-exportDataStart

	updateFilePaths(&source, exportDir, tablesProgressMetadata)
	if !disablePb {
		utils.WaitGroup.Add(1)
		exportDataStatus(ctx, tablesProgressMetadata, quitChan)
	}

	utils.WaitGroup.Wait() // waiting for the dump and progress bars to complete
	if ctx.Err() != nil {
		fmt.Printf("ctx error(exportData.go): %v\n", ctx.Err())
		return false
	}

	source.DB().ExportDataPostProcessing(exportDir, tablesProgressMetadata)

	tableRowCount := datafile.OpenDescriptor(exportDir).TableRowCount
	printExportedRowCount(tableRowCount)
	callhome.UpdateDataStats(exportDir, tableRowCount)
	callhome.PackAndSendPayload(exportDir)
	return true
}

func exportDataOnline() bool {
	errMsg := "online migration not supported yet\n"
	utils.ErrExit(errMsg)

	return false
}

//flagName can be "exclude-table-list" or "table-list"
func validateTableListFlag(tableListString string, flagName string) {
	if tableListString == "" {
		return
	}
	tableList := utils.CsvStringToSlice(tableListString)
	// TODO: update regexp once table name with double quotes are allowed/supported
	tableNameRegex := regexp.MustCompile("[a-zA-Z0-9_.]+")
	for _, table := range tableList {
		if !tableNameRegex.MatchString(table) {
			utils.ErrExit("Error: Invalid table name '%v' provided wtih --%s flag", table, flagName)
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

func extractTableListFromString(flagTableList string) []string {
	var finalTableList []string
	if flagTableList == "" {
		return []string{}
	}
	tableList := utils.CsvStringToSlice(flagTableList)
	if source.DBType != POSTGRESQL {
		return tableList
	}
	// in postgres format should be schema.table, public is default and other parts of code assume schema.table format
	for _, table := range tableList {
		parts := strings.Split(table, ".")
		if len(parts) == 1 {
			finalTableList = append(finalTableList, "public."+table)
		} else if len(parts) == 2 {
			finalTableList = append(finalTableList, table)
		} else {
			utils.ErrExit("invalid table name %q in the --table-list flag.", table)
		}
	}
	return finalTableList
}

func createExportDataDoneFlag() {
	exportDoneFlagPath := filepath.Join(exportDir, "metainfo", "flags", "exportDataDone")
	_, err := os.Create(exportDoneFlagPath)
	if err != nil {
		utils.ErrExit("creating exportDataDone flag: %v", err)
	}
}
