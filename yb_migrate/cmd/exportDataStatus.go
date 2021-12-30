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
	"os"
	"runtime"
	"time"
	"yb_migrate/src/utils"

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

func exportDataStatus(ctx context.Context, tablesMetadata []utils.ExportTableMetadata, quitChan chan bool) {
	quitChan2 := make(chan bool)
	go func() {
		<-quitChan2
		quitChan <- true
	}()

	numTables := len(tablesMetadata)
	// progressContainer := mpb.NewWithContext(ctx, mpb.WithWaitGroup(&utils.WaitGroup))
	progressContainer := mpb.NewWithContext(ctx)

	doneCount := 0
	var exportedTables []string

	for doneCount < numTables {
		for i := 0; i < numTables; i++ {
			if tablesMetadata[i].Status == "NOT-STARTED" &&
				utils.FileOrFolderExists(tablesMetadata[i].DataFilePath) {
				tablesMetadata[i].Status = "IN-PROGRESS"
				// utils.WaitGroup.Add(1)

				go startProgressBar(progressContainer, &tablesMetadata[i], quitChan2)

			} else if tablesMetadata[i].Status == "IN-PROGRESS" &&
				tablesMetadata[i].CountLiveRows >= tablesMetadata[i].CountTotalRows {
				tablesMetadata[i].Status = "DONE"

				exportedTables = append(exportedTables, tablesMetadata[i].TableName)
				doneCount++

				if doneCount == numTables {
					break
				}
			}
		}
	}

	defer progressContainer.Wait()

	fmt.Printf("Exported tables:- ")
	for _, table := range exportedTables {
		fmt.Printf("%s, ", table)
	}
	fmt.Println()

	//TODO: print remaining/unable-to-export tables
}

func startProgressBar(progressContainer *mpb.Progress, tableMetadata *utils.ExportTableMetadata, quitChan chan bool) {
	// defer utils.WaitGroup.Done()

	name := tableMetadata.TableName
	total := int64(100)
	bar := progressContainer.AddBar(total,
		mpb.PrependDecorators(
			// display our name with one space on the right
			decor.Name(name),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
		),
	)

	tableDataFile, err := os.Open(tableMetadata.DataFilePath)
	if err != nil {
		fmt.Println(err)
		quitChan <- true
		runtime.Goexit()
	}

	reader := bufio.NewReader(tableDataFile)

	if tableMetadata.CountTotalRows == 0 { // if row count = 0, then return
		bar.IncrInt64(100)
		return
	}

	//TODO: Decide buffer size dynamically
	BUFFER_SIZE := 20
	for tableMetadata.CountLiveRows < tableMetadata.CountTotalRows {
		buf := make([]byte, BUFFER_SIZE*1024) //reading x MB size buffer each time

		n, _ := reader.Read(buf)

		tableMetadata.FileOffsetToContinue += int64(n) //update the current read location of file

		if n == 0 {
			// tableDataFile.Seek(tableMetadata.FileOffsetToContinue, 0)
			time.Sleep(time.Second * 2)
		} else {
			//counting number of rows
			for _, byteinfo := range buf {
				if string(byteinfo) == "\n" {
					tableMetadata.CountLiveRows += 1
				}
			}
		}
		// fmt.Printf("%s - %d\n", tableMetadata.TableName, tableMetadata.CountLiveRows)

		PercentageValueFloat := float64(tableMetadata.CountLiveRows) / float64(tableMetadata.CountTotalRows) * 100
		PercentageValueInt64 := int64(PercentageValueFloat)
		incrementValue := (PercentageValueInt64) - bar.Current()
		bar.IncrInt64(incrementValue)

		buf = nil //making it eligible for GC
	}

}

func init() {
	exportDataCmd.AddCommand(exportDataStatusCmd)
}
