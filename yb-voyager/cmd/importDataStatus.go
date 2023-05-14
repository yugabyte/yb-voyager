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
	"strings"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// var debugFile *os.File

func importDataStatus() {
	// debugFile, _ = os.OpenFile(exportDir+"/temp/debug.txt", os.O_CREATE|os.O_WRONLY, 0644)
	// fmt.Printf("TablesProgressMetadata: %v\n", tablesProgressMetadata)
	importProgressContainer = ProgressContainer{
		container: mpb.New(),
	}
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
	log.Infof("Initializing import data status")
	tablesProgressMetadata = make(map[string]*utils.TableProgressMetadata)
	importedRowCount := getImportedRowsCount(exportDir, tables)
	var totalRowCountMap map[string]int64
	if dataFileDescriptor.TableRowCount != nil {
		totalRowCountMap = dataFileDescriptor.TableRowCount
	} else {
		totalRowCountMap = dataFileDescriptor.TableFileSize
	}

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
			TableName:      sqlname.NewSourceName(schema, table), // TODO: Use sqlname.TargetName instead.
			Status:         0,
			CountLiveRows:  importedRowCount[fullTableName],
			CountTotalRows: totalRowCountMap[fullTableName],
		}
	}

}

// TODO: rename it and corresponding refactoring so that RowCount & ByteCount both are signified
func getImportedRowsCount(exportDir string, tables []string) map[string]int64 {
	metaInfoDataDir := exportDir + "/metainfo/data"
	importedRowCounts := make(map[string]int64)

	for _, table := range tables {
		pattern := fmt.Sprintf("%s/%s.%s.[D]", metaInfoDataDir, table, SPLIT_INFO_PATTERN)
		matches, _ := filepath.Glob(pattern)
		importedRowCounts[table] = 0

		for _, filePath := range matches {
			importedRowCounts[table] += getProgressAmount(filePath)
		}

		if importedRowCounts[table] == 0 { //if it zero, then its import not started yet
			importedRowCounts[table] = -1
		}
		// fmt.Printf("Previous count %s = %d\n", table, importedRowCounts[table])
	}
	log.Infof("importedRowCount: %v", importedRowCounts)
	return importedRowCounts
}
