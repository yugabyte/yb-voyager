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
package srcdb

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func pgdumpExportDataOffline(ctx context.Context, source *Source, connectionUri string, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool, exportSuccessChan chan bool) {
	defer utils.WaitGroup.Done()

	dataDirPath := exportDir + "/data"

	tableListPatterns := createTableListPatterns(tableList)

	// Using pgdump for exporting data in directory format.

	pgDumpPath, err := GetAbsPathOfPGCommand("pg_dump")
	if err != nil {
		utils.ErrExit("could not get absolute path of pg_dump command: %v", err)
	}

	pgDumpArgs := fmt.Sprintf(`--no-blobs --data-only --no-owner --compress=0 %s -Fd --file %s --jobs %d --no-privileges --no-tablespaces --load-via-partition-root`,
		tableListPatterns, dataDirPath, source.NumConnections)
	os.Setenv("PGPASSWORD", source.Password)
	cmd := fmt.Sprintf(`%s '%s' %s`, pgDumpPath, connectionUri, pgDumpArgs)
	log.Infof("Running command: %s", cmd)

	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	proc := exec.CommandContext(ctx, "/bin/bash", "-c", cmd)
	proc.Stderr = &outbuf
	proc.Stdout = &errbuf
	err = proc.Start()
	if outbuf.String() != "" {
		log.Infof("%s", outbuf.String())
	}
	if err != nil {
		fmt.Printf("pg_dump failed to start exporting data with error: %v. For more details check '%s/yb-voyager.log'.\n", err, exportDir)
		log.Infof("pg_dump failed to start exporting data with error: %v\n%s", err, errbuf.String())
		quitChan <- true
		os.Unsetenv("PGPASSWORD")
		runtime.Goexit()
	}
	utils.PrintAndLog("Data export started.")
	exportDataStart <- true

	// Parsing the main toc.dat file in parallel.
	go parseAndCreateTocTextFile(dataDirPath)

	// Wait for pg_dump to complete before renaming of data files.
	err = proc.Wait()
	if err != nil {
		fmt.Printf("pg_dump failed to export data with error: %v. For more details check '%s/yb-voyager.log'.\n", err, exportDir)
		log.Infof("pg_dump failed to export data with error: %v\n%s", err, errbuf.String())
		quitChan <- true
		os.Unsetenv("PGPASSWORD")
		runtime.Goexit()
	}
	exportSuccessChan <- true
	os.Unsetenv("PGPASSWORD")
}

func parseAndCreateTocTextFile(dataDirPath string) {
	tocFilePath := dataDirPath + "/toc.dat"
	var waitingFlag int
	for !utils.FileOrFolderExists(tocFilePath) {
		waitingFlag = 1
		time.Sleep(time.Second * 3)
	}

	if waitingFlag == 1 {
		log.Info("toc.dat file successfully created.")
	}

	parseTocFileCommand := exec.Command("strings", tocFilePath)
	cmdOutput, err := parseTocFileCommand.CombinedOutput()
	if err != nil {
		utils.ErrExit("parsing tocfile %q: %v", tocFilePath, err)
	}

	//Put the data into a toc.txt file
	tocTextFilePath := dataDirPath + "/toc.txt"
	tocTextFile, err := os.Create(tocTextFilePath)
	if err != nil {
		utils.ErrExit("create toc.txt: %s", err)
	}
	writer := bufio.NewWriter(tocTextFile)
	_, err = writer.Write(cmdOutput)
	if err != nil {
		utils.ErrExit("write to toc.txt: %s", err)
	}
	err = writer.Flush()
	if err != nil {
		utils.ErrExit("flush toc.txt: %s", err)
	}
	tocTextFile.Close()
}

func createTableListPatterns(tableList []string) string {
	var tableListPattern string

	for _, table := range tableList {
		tableListPattern += fmt.Sprintf("-t '%s' ", table)
	}

	return tableListPattern
}

func renameDataFiles(tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	for _, tableProgressMetadata := range tablesProgressMetadata {
		oldFilePath := tableProgressMetadata.InProgressFilePath
		newFilePath := tableProgressMetadata.FinalFilePath
		if utils.FileOrFolderExists(oldFilePath) {
			log.Infof("Renaming %q -> %q", oldFilePath, newFilePath)
			err := os.Rename(oldFilePath, newFilePath)
			if err != nil {
				utils.ErrExit("renaming data file for table %q after data export: %v", tableProgressMetadata.TableName, err)
			}
		} else {
			log.Infof("File %q to rename doesn't exists!", oldFilePath)
		}
	}
}
