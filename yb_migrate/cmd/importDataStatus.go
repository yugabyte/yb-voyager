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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yugabyte/ybm/yb_migrate/src/migration"
	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	"github.com/fatih/color"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

// var debugFile *os.File

func importDataStatus() {
	// debugFile, _ = os.OpenFile(exportDir+"/temp/debug.txt", os.O_CREATE|os.O_WRONLY, 0644)
	// fmt.Printf("TablesProgressMetadata: %v\n", tablesProgressMetadata)

	for Done.IsNotSet() {
		for _, table := range importTables {

			if tablesProgressMetadata[table].Status == 0 && tablesProgressMetadata[table].CountLiveRows >= 0 {
				tablesProgressMetadata[table].Status = 1
				go startImportPB(table)
			} else if tablesProgressMetadata[table].Status == 2 &&
				tablesProgressMetadata[table].CountLiveRows >= tablesProgressMetadata[table].CountTotalRows {

				tablesProgressMetadata[table].Status = 3
				// fmt.Fprintf(debugFile, "Completed: table:%s  LiveCount:%d, TotalCount:%d\n", tableProgressMetadata.TableName,
				// 	tableProgressMetadata.CountLiveRows, tableProgressMetadata.CountTotalRows)

			}
		}
		time.Sleep(time.Millisecond * 100)
	}

	importProgressContainer.container.Wait()

	//TODO: add a check for errors/failures before printing it
	color.Green("\nAll the tables are imported\n")
}

func startImportPB(table string) {
	name := table
	total := int64(100)

	importProgressContainer.mu.Lock()
	bar := importProgressContainer.container.AddBar(total,
		mpb.BarFillerClearOnComplete(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(name),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.NewPercentage("%.2f", decor.WCSyncSpaceR), "completed",
			),
			decor.OnComplete(
				decor.AverageETA(decor.ET_STYLE_GO), "",
			),
		),
	)
	importProgressContainer.mu.Unlock()

	//if its already done or row count = 0, then return
	if tablesProgressMetadata[table].CountLiveRows >= tablesProgressMetadata[table].CountTotalRows {
		// fmt.Fprintf(tempFile, "PBreturn: table:%s IncValue: %d, percent:%d, LiveCount:%d, TotalCount:%d\n", tableProgressMetadata.TableName,
		// 	100, 100, tableProgressMetadata.CountLiveRows, tableProgressMetadata.CountTotalRows)

		// bar.SetCurrent(tablesProgressMetadata[table].CountLiveRows)
		bar.IncrInt64(100)

		tablesProgressMetadata[table].Status = 2 //set Status=DONE, before return
		return
	}

	//TODO: break loop in error/failure conditions
	for {
		PercentageValueFloat := (float64(tablesProgressMetadata[table].CountLiveRows) / float64(tablesProgressMetadata[table].CountTotalRows)) * 100
		PercentageValueInt64 := int64(PercentageValueFloat)
		incrementValue := PercentageValueInt64 - bar.Current()

		bar.IncrInt64(incrementValue)
		// bar.SetCurrent(tablesProgressMetadata[table].CountLiveRows)

		if PercentageValueInt64 >= 100 {
			break
		} else {
			time.Sleep(time.Millisecond * 100)
		}
	}

	tablesProgressMetadata[table].Status = 2 //set Status=DONE, before return
}

func initializeImportDataStatus(exportDir string, tables []string) {
	// log.Infof("Initializing import data status\n")
	tablesProgressMetadata = make(map[string]*utils.TableProgressMetadata)
	importedRowCount := getImportedRowsCount(exportDir, tables)

	rowCountFilePath := exportDir + "/metainfo/flags/tablesrowcount"
	totalRowCountMap := migration.GetTableRowCount(rowCountFilePath)

	for _, fullTableName := range tables {
		parts := strings.Split(fullTableName, ".")
		var table, schema string
		if len(parts) == 2 {
			schema = parts[0]
			table = parts[1]
		} else {
			schema = target.Schema //so schema will be user provided or public(always for pg, since target-db-schema flag is not for pg)
			table = parts[0]
		}
		tablesProgressMetadata[fullTableName] = &utils.TableProgressMetadata{
			TableSchema:    schema,
			TableName:      table,
			Status:         0,
			CountLiveRows:  importedRowCount[fullTableName],
			CountTotalRows: totalRowCountMap[fullTableName],
		}
		// fmt.Printf("TableRowCountMap for %s: %d\n", fullTableName, totalRowCountMap[fullTableName])
		// fmt.Printf("TableProgressMetadata for '%s': %+v\n", fullTableName, tablesProgressMetadata[fullTableName])
	}

}

func getImportedRowsCount(exportDir string, tables []string) map[string]int64 {
	metaInfoDataDir := exportDir + "/metainfo/data"
	importedRowCounts := make(map[string]int64)

	for _, table := range tables {
		pattern := fmt.Sprintf("%s/%s.[0-9]*.[0-9]*.[0-9]*.[D]", metaInfoDataDir, table)
		matches, _ := filepath.Glob(pattern)
		importedRowCounts[table] = 0

		tableDoneSplitsPattern := fmt.Sprintf("%s\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.[D]$", table)
		tableDoneSplitsRegexp := regexp.MustCompile(tableDoneSplitsPattern)

		for _, filePath := range matches {
			fileName := filepath.Base(filePath)
			submatches := tableDoneSplitsRegexp.FindStringSubmatch(fileName)

			if len(submatches) > 0 {
				cnt, _ := strconv.ParseInt(submatches[3], 10, 64)
				importedRowCounts[table] += cnt
			}

		}

		if importedRowCounts[table] == 0 { //if it zero, then its import not started yet
			importedRowCounts[table] = -1
		}
		// fmt.Printf("Previous count %s = %d\n", table, importedRowCounts[table])
	}

	return importedRowCounts
}
