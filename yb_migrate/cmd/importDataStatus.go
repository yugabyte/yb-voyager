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
	"path/filepath"
	"regexp"
	"strconv"
	"yb_migrate/src/migration"
	"yb_migrate/src/utils"

	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

// importDataStatusCmd represents the importDataStatus command
var importDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Status of import of data can be found from this command",
	Long:  `This command will give a little different output for offline and online mode.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("import data status called")
	},
}

func init() {
	importDataCmd.AddCommand(importDataStatusCmd)
}

func importDataStatus(progressContainer *mpb.Progress, tablesProgressMetadata *[]utils.ExportTableMetadata) {
	numTables := len(*tablesProgressMetadata)
	// debugFile, _ := os.OpenFile(exportDir+"/temp/debug.txt", os.O_WRONLY, 0644)

	for Done.IsNotSet() {
		for i := 0; i < numTables; i++ {
			tableProgressMetadata := &(*tablesProgressMetadata)[i]
			if tableProgressMetadata.Status == "NOT-STARTED" && tableProgressMetadata.CountLiveRows > 0 {

				tableProgressMetadata.Status = "IN-PROGRESS"
				go startImportPB(progressContainer, tableProgressMetadata)

			} else if tableProgressMetadata.Status == "DONE" &&
				tableProgressMetadata.CountLiveRows >= tableProgressMetadata.CountTotalRows {

				tableProgressMetadata.Status = "COMPLETED"
				// fmt.Fprintf(debugFile, "Completed: table:%s  LiveCount:%d, TotalCount:%d\n", tableProgressMetadata.TableName,
				// 	tableProgressMetadata.CountLiveRows, tableProgressMetadata.CountTotalRows)

			}
		}
	}

	progressContainer.Wait()
}

func startImportPB(progressContainer *mpb.Progress, tableProgressMetadata *utils.ExportTableMetadata) {
	name := tableProgressMetadata.TableName
	total := int64(100)
	// tempFile, _ := os.OpenFile(exportDir+"/temp/debug.txt", os.O_WRONLY, 0644)

	bar := progressContainer.AddBar(total,
		mpb.BarFillerClearOnComplete(),
		// mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(name),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpaceR),
			decor.OnComplete(
				//TODO: default feature by package, need to verify the correctness/algorithm for ETA
				decor.AverageETA(decor.ET_STYLE_GO), "done",
			),
		),
	)

	if tableProgressMetadata.CountLiveRows >= tableProgressMetadata.CountTotalRows ||
		tableProgressMetadata.CountTotalRows == 0 { // if row count = 0, then return
		// fmt.Fprintf(tempFile, "PBreturn: table:%s IncValue: %d, percent:%d, LiveCount:%d, TotalCount:%d\n", tableProgressMetadata.TableName,
		// 	100, 100, tableProgressMetadata.CountLiveRows, tableProgressMetadata.CountTotalRows)

		bar.IncrInt64(100)
		tableProgressMetadata.Status = "DONE"
		return
	}

	for {
		PercentageValueFloat := (float64(tableProgressMetadata.CountLiveRows) / float64(tableProgressMetadata.CountTotalRows)) * 100
		PercentageValueInt64 := int64(PercentageValueFloat)
		incrementValue := (PercentageValueInt64) - bar.Current()
		bar.IncrInt64(incrementValue)

		// fmt.Fprintf(tempFile, "PB: table:%s IncValue: %d, percent:%d, LiveCount:%d, TotalCount:%d\n", tableProgressMetadata.TableName,
		// 	incrementValue, PercentageValueInt64, tableProgressMetadata.CountLiveRows, tableProgressMetadata.CountTotalRows)

		if PercentageValueInt64 >= 100 {
			break
		}
	}

	tableProgressMetadata.Status = "DONE" //before return
}

func initializeImportDataStatus(exportDir string, tables []string) {
	importedRowCount := getImportedRowsCount(exportDir, tables)

	for _, tableName := range tables {
		currentTableStruct := utils.ExportTableMetadata{
			TableSchema:   "",
			TableName:     tableName,
			Status:        "NOT-STARTED",
			CountLiveRows: importedRowCount[tableName],
		}
		tablesProgressMetadata = append(tablesProgressMetadata, currentTableStruct)
	}

	//Temporary: Till the migration checker code is not in-place
	tableRowCountMap := migration.GetTableRowCount("/home/centos/yb_migrate_projects/" + target.DBName + "_rc.txt")
	for i := 0; i < len(tablesProgressMetadata); i++ {
		tablesProgressMetadata[i].CountTotalRows = tableRowCountMap[tablesProgressMetadata[i].TableName]
	}
}

func getImportedRowsCount(exportDir string, tables []string) map[string]int64 {
	metaInfoDataDir := exportDir + "/metainfo/data"
	importedRowCount := make(map[string]int64)

	for _, table := range tables {
		//[CPD] because there might be some garbage pattern.sql file created for execution
		pattern := fmt.Sprintf("%s/%s.[0-9]*.[0-9]*.[0-9]*.[CPD]", metaInfoDataDir, table)
		matches, _ := filepath.Glob(pattern)
		importedRowCount[table] = 0

		tableDoneSplitsPattern := fmt.Sprintf("%s\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.[D]$", table)
		tableDoneSplitsRegexp := regexp.MustCompile(tableDoneSplitsPattern)

		for _, filePath := range matches {
			fileName := filepath.Base(filePath)
			submatches := tableDoneSplitsRegexp.FindStringSubmatch(fileName)
			// if table == "film_actor" {
			// 	fmt.Printf("submatches len = %d\n", len(submatches))
			// }

			if len(submatches) > 0 {
				cnt, _ := strconv.ParseInt(submatches[3], 10, 64)
				importedRowCount[table] += cnt
			}

		}
		// fmt.Printf("Previous count %s = %d\n", table, importedRowCount[table])
	}

	return importedRowCount
}
