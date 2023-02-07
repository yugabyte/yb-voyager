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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/pbreporter"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

	"github.com/fatih/color"
	"github.com/vbauerster/mpb/v7"
)

func initializeExportTableMetadata(tableList []*sqlname.SourceName) {
	tablesProgressMetadata = make(map[string]*utils.TableProgressMetadata)
	numTables := len(tableList)

	for i := 0; i < numTables; i++ {
		tableName := tableList[i]
		key := tableName.Qualified.MinQuoted
		tablesProgressMetadata[key] = &utils.TableProgressMetadata{} //initialzing with struct

		// Initializing all the members of struct
		tablesProgressMetadata[key].IsPartition = false
		tablesProgressMetadata[key].InProgressFilePath = ""
		tablesProgressMetadata[key].FinalFilePath = ""        //file paths will be updated when status changes to IN-PROGRESS by other func
		tablesProgressMetadata[key].CountTotalRows = int64(0) //will be updated by other func
		tablesProgressMetadata[key].CountLiveRows = int64(0)
		tablesProgressMetadata[key].Status = 0
		tablesProgressMetadata[key].FileOffsetToContinue = int64(0)
		tablesProgressMetadata[key].TableName = tableName
	}
}

func exportDataStatus(ctx context.Context, tablesProgressMetadata map[string]*utils.TableProgressMetadata, quitChan, exportSuccessChan chan bool, disablePb bool) {
	defer utils.WaitGroup.Done()
	// TODO: Figure out if we require quitChan2 (along with the entire goroutine below which updates quitChan).
	quitChan2 := make(chan bool)
	quit := false
	updateMetadataAndExit := false
	safeExit := false
	go func() {
		quit = <-quitChan2
		if quit {
			quitChan <- true
		}
	}()

	go func() {
		updateMetadataAndExit = <-exportSuccessChan
	}()

	numTables := len(tablesProgressMetadata)
	progressContainer := mpb.NewWithContext(ctx)

	doneCount := 0
	var exportedTables []string
	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	for doneCount < numTables && !quit { //TODO: wait for export data to start
		for _, key := range sortedKeys {
			if quit {
				break
			}
			if tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_NOT_STARTED && (utils.FileOrFolderExists(tablesProgressMetadata[key].InProgressFilePath) ||
				utils.FileOrFolderExists(tablesProgressMetadata[key].FinalFilePath)) {
				tablesProgressMetadata[key].Status = utils.TABLE_MIGRATION_IN_PROGRESS
				go startExportPB(progressContainer, key, quitChan2, disablePb)
			} else if tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_DONE || (tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_NOT_STARTED && safeExit) {
				tablesProgressMetadata[key].Status = utils.TABLE_MIGRATION_COMPLETED
				exportedTables = append(exportedTables, key)
				doneCount++
				if doneCount == numTables {
					break
				}
			}

			//for failure/error handling. TODO: test it more
			if ctx.Err() != nil {
				fmt.Println(ctx.Err())
				break
			}
		}

		if ctx.Err() != nil {
			fmt.Println(ctx.Err())
			break
		}
		// Wait for metadata to update and then exit (fix for empty partitions in MySQL, Oracle)
		if updateMetadataAndExit {
			safeExit = true
			for _, key := range sortedKeys {
				if tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_IN_PROGRESS || tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_DONE {
					safeExit = false
					break
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	progressContainer.Wait() //shouldn't be needed as the previous loop is doing the same
	printExportedTables(exportedTables)
	//TODO: print remaining/unable-to-export tables
}

func startExportPB(progressContainer *mpb.Progress, mapKey string, quitChan chan bool, disablePb bool) {
	tableName := mapKey
	tableMetadata := tablesProgressMetadata[mapKey]

	pbr := pbreporter.NewExportPB(progressContainer, tableName, disablePb)
	// initialize PB total with identified approx row count
	pbr.SetTotalRowCount(tableMetadata.CountTotalRows, false)

	// parallel goroutine to calculate and set total to actual row count
	go func() {
		actualRowCount := source.DB().GetTableRowCount(tableMetadata.TableName.Qualified.MinQuoted)
		log.Infof("Replacing actualRowCount=%d inplace of expectedRowCount=%d for table=%s",
			actualRowCount, tableMetadata.CountTotalRows, tableMetadata.TableName.Qualified.MinQuoted)
		pbr.SetTotalRowCount(actualRowCount, false)
	}()

	tableDataFileName := tableMetadata.InProgressFilePath
	if utils.FileOrFolderExists(tableMetadata.FinalFilePath) {
		tableDataFileName = tableMetadata.FinalFilePath
	}
	tableDataFile, err := os.Open(tableDataFileName)
	if err != nil {
		utils.PrintAndLog("failed to open the data file %s for progress reporting: %q", tableDataFileName, err)
		quitChan <- true
		runtime.Goexit()
	}
	defer tableDataFile.Close()

	reader := bufio.NewReader(tableDataFile)

	go func() { //for continuously increasing PB percentage
		for !pbr.IsComplete() {
			pbr.SetExportedRowCount(tableMetadata.CountLiveRows)
			time.Sleep(time.Millisecond * 500)
		}
	}()

	var line string
	insideCopyStmt := false
	prefix := ""

	readLines := func() {
		for {
			line, err = reader.ReadString('\n')
			if line != "" {
				if line[len(line)-1] == '\n' {
					if prefix != "" {
						line = prefix + line
						prefix = ""
					}
				} else {
					prefix = prefix + line
				}
			}
			if err == io.EOF {
				time.Sleep(100 * time.Millisecond)
				break
			} else if err != nil { //error other than EOF
				utils.ErrExit("Error while reading file %s: %v", tableDataFile, err)
			}
			if isDataLine(line, source.DBType, &insideCopyStmt) {
				tableMetadata.CountLiveRows += 1
			}
			if source.DBType == "postgresql" && strings.HasPrefix(line, "\\.") {
				break
			}
		}
	}

	for !checkForEndOfFile(&source, tableMetadata, line) {
		readLines()
	}
	/*
		Below extra step to count rows because there may be still a possibility that some rows left uncounted before EOF
		1. if previous loop breaks because of fileName changes and before counting all rows.
		2. Based on - even after file rename the file access with reader stays and can count remaining lines in the file
		(Mainly for Oracle, MySQL)
	*/
	readLines()

	// PB will not change from "100%" -> "completed" until this function call is made
	pbr.SetTotalRowCount(-1, true) // Completing remaining progress bar by setting current equal to total
	tableMetadata.Status = utils.TABLE_MIGRATION_DONE
}

func checkForEndOfFile(source *srcdb.Source, tableMetadata *utils.TableProgressMetadata, line string) bool {
	if source.DBType == "postgresql" {
		if strings.HasPrefix(line, "\\.") {
			return true
		}
	} else if source.DBType == "oracle" || source.DBType == "mysql" {
		if !utils.FileOrFolderExists(tableMetadata.InProgressFilePath) && utils.FileOrFolderExists(tableMetadata.FinalFilePath) {
			return true
		}
	}
	return false
}

func printExportedTables(exportedTables []string) {
	output := "Exported tables:- {"
	nt := len(exportedTables)
	for i := 0; i < nt; i++ {
		output += exportedTables[i]
		if i < nt-1 {
			output += ",  "
		}

	}

	output += "}"

	color.Yellow(output)
}

// Example: `COPY "Foo" ("v") FROM STDIN;`
var reCopy = regexp.MustCompile(`(?i)COPY .* FROM STDIN;`)

/*
This function checks for based structure of data file which can be different
for different source db type based on tool used for export
postgresql - file has only data lines with "\." at the end
oracle/mysql - multiple copy statements with each having specific count of rows
*/
func isDataLine(line string, sourceDBType string, insideCopyStmt *bool) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	if sourceDBType == "postgresql" {
		return !(emptyLine || newLineChar || endOfCopy)
	} else if sourceDBType == "oracle" || sourceDBType == "mysql" {
		if *insideCopyStmt {
			if endOfCopy {
				*insideCopyStmt = false
			}
			return !(emptyLine || newLineChar || endOfCopy)
		} else { // outside copy
			if reCopy.MatchString(line) {
				*insideCopyStmt = true
			}
			return false
		}
	} else {
		panic("Invalid source db type")
	}
}
