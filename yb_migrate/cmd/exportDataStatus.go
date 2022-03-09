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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

// exportDataStatusCmd represents the exportDataStatus command
var exportDataStatusCmd = &cobra.Command{
	Use:   "exportDataStatus",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("exportDataStatus called")
	},
}

// var debugFile *os.File

func exportDataStatus(ctx context.Context, tablesMetadata []utils.TableProgressMetadata, quitChan chan bool) {
	quitChan2 := make(chan bool)
	quit := false
	go func() {
		quit = <-quitChan2
		if quit {
			quitChan <- true
		}
	}()

	numTables := len(tablesMetadata)
	// progressContainer := mpb.NewWithContext(ctx, mpb.WithWaitGroup(&utils.WaitGroup))
	progressContainer := mpb.NewWithContext(ctx)

	doneCount := 0
	var exportedTables []string
	// debugFile, _ = os.OpenFile("/home/centos/temp/err.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	// defer debugFile.Close()

	for doneCount < numTables && !quit { //TODO: wait for export data to start
		for i := 0; i < numTables && !quit; i++ {
			if tablesMetadata[i].Status == 0 && utils.FileOrFolderExists(tablesMetadata[i].InProgressFilePath) {
				tablesMetadata[i].Status = 1
				// utils.WaitGroup.Add(1)
				// fmt.Fprintf(debugFile, "start table=%s, file=%s\n", tablesMetadata[i].TableName, tablesMetadata[i].InProgressFilePath)
				go startExportPB(progressContainer, &tablesMetadata[i], quitChan2)

			} else if tablesMetadata[i].Status == 2 {
				tablesMetadata[i].Status = 3

				exportedTables = append(exportedTables, tablesMetadata[i].FullTableName)
				// fmt.Fprintf(debugFile, "done table= %s, liverow count: %d, expectedTotal: %d\n", tablesMetadata[i].TableName, tablesMetadata[i].CountLiveRows, tablesMetadata[i].CountTotalRows)
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
	}

	progressContainer.Wait() //shouldn't be needed as the previous loop is doing the same

	printExportedTables(exportedTables)

	//TODO: print remaining/unable-to-export tables
}

func startExportPB(progressContainer *mpb.Progress, tableMetadata *utils.TableProgressMetadata, quitChan chan bool) {
	// defer utils.WaitGroup.Done()

	var tableName string
	if source.DBType == POSTGRESQL && tableMetadata.TableSchema != "public" {
		tableName = tableMetadata.TableSchema + "." + tableMetadata.TableName
	} else {
		tableName = tableMetadata.TableName
	}

	total := int64(100)

	bar := progressContainer.AddBar(total,
		mpb.BarFillerClearOnComplete(),
		// mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(tableName),
		),
		mpb.AppendDecorators(
			// decor.Percentage(decor.WCSyncSpaceR),
			decor.OnComplete(
				decor.NewPercentage("%.2f", decor.WCSyncSpaceR), "completed",
			),
			decor.OnComplete(
				//TODO: default feature by package, need to verify the correctness/algorithm for ETA
				decor.AverageETA(decor.ET_STYLE_GO), "",
			),
		),
	)

	if tableMetadata.CountTotalRows == 0 { // if row count = 0, then return
		bar.IncrInt64(100)
		tableMetadata.Status = 2
		return
	}

	tableDataFile, err := os.Open(tableMetadata.InProgressFilePath)
	if err != nil {
		fmt.Println(err)
		quitChan <- true
		runtime.Goexit()
	}
	defer tableDataFile.Close()

	reader := bufio.NewReader(tableDataFile)
	// var prevLine string

	go func() { //for continuously increasing PB percentage
		for !bar.Completed() {
			PercentageValueFloat := float64(tableMetadata.CountLiveRows) / float64(tableMetadata.CountTotalRows) * 100
			PercentageValueInt64 := int64(PercentageValueFloat)
			incrementValue := (PercentageValueInt64) - bar.Current()
			bar.IncrInt64(incrementValue)
			time.Sleep(time.Millisecond * 500)
		}
	}()

	var line string
	var readLineErr error
	for !checkForEndOfFile(&source, tableMetadata, line) {
		for {
			line, readLineErr = reader.ReadString('\n')
			if readLineErr == io.EOF {
				// fmt.Fprintf(debugFile, "counting rows for tname=%s, prevLine='%s', line='%s', liverow count=%d\n", tableName, prevLine, line, tableMetadata.CountLiveRows)
				break
			} else if readLineErr != nil { //error other than EOF
				panic(readLineErr)
			}

			if strings.HasPrefix(line, "\\.") { //break loop to execute checkForEndOfFile()
				break
			} else if isDataLine(line) {
				tableMetadata.CountLiveRows += 1
				// prevLine = line
			}
		}
	}

	/*
		Below extra step to count rows because there may be still a possibility that some rows left uncounted before EOF
		1. if previous loop breaks because of fileName changes and before counting all rows.
		2. Based on - even after file rename the file access with reader stays and can count remaining lines in the file
		(Mainly for Oracle, MySQL)
	*/

	// if !bar.Completed() && utils.FileOrFolderExists(tableMetadata.FinalFilePath) {
	// 	tableDataFile, err := os.Open(tableMetadata.FinalFilePath)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		quitChan <- true
	// 		runtime.Goexit()
	// 	}
	// 	reader = bufio.NewReader(tableDataFile)
	// 	count := int64(0)
	// 	for {
	// 		line, readLineErr := reader.ReadString('\n')
	// 		if readLineErr == io.EOF {
	// 			// fmt.Fprintf(debugFile, "LAST counting rows for tname=%s, len(line)=%d, line='%s', liverow count=%d\n", tableName, len(line), line, tableMetadata.CountLiveRows)
	// 			break
	// 		} else if readLineErr != nil { //error other than EOF
	// 			panic(readLineErr)
	// 		}
	// 		// fmt.Fprintf(errFile, "LINE=%s, tname=%s, count=%d\n", line, name, count)
	// 		if isDataLine(line) {
	// 			count++
	// 		}
	// 	}
	// 	tableMetadata.CountLiveRows = count //updating with new and final count

	// 	// fmt.Fprintf(debugFile, "PB is not complete, completing the remaining bar.\nFinal %s, lastLine:'%s', RowCount:%d\n", tableName, prevLine, count)
	// 	bar.IncrBy(100)
	// }

	for {
		line, readLineErr := reader.ReadString('\n')
		if readLineErr == io.EOF {
			// fmt.Fprintf(debugFile, "LAST counting rows for tname=%s, len(line)=%d, line='%s', liverow count=%d\n", tableName, len(line), line, tableMetadata.CountLiveRows)
			break
		} else if readLineErr != nil { //error other than EOF
			panic(readLineErr)
		}
		// fmt.Fprintf(errFile, "LINE=%s, tname=%s, count=%d\n", line, name, count)
		if isDataLine(line) {
			tableMetadata.CountLiveRows += 1
		}
	}
	if !bar.Completed() {
		bar.IncrBy(100) //completing remaining progress bar to continue the execution
	}

	// fmt.Fprintf(debugFile, "startExportPB() complete for %s\n", tableName)
	tableMetadata.Status = 2 //before return
}

func checkForEndOfFile(source *utils.Source, tableMetadata *utils.TableProgressMetadata, line string) bool {
	if source.DBType == "postgresql" {
		if strings.HasPrefix(line, "\\.") {
			// fmt.Fprintf(debugFile, "checkForEOF done for table:%s line:%s, tablefile: %s\n", tableMetadata.TableName, line, tableMetadata.FinalFilePath)
			return true
		}
	} else if source.DBType == "oracle" || source.DBType == "mysql" {
		if !utils.FileOrFolderExists(tableMetadata.InProgressFilePath) && utils.FileOrFolderExists(tableMetadata.FinalFilePath) {
			// fmt.Fprintf(debugFile, "checkForEOF done for table:%s line:%s, tablefile: %s\n", tableMetadata.TableName, line, tableMetadata.FinalFilePath)
			return true
		}
	}
	return false
}

func init() {
	exportDataCmd.AddCommand(exportDataStatusCmd)
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
