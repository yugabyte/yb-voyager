/*
Copyright (c) YugabyteDB, Inc.

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
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func pgdumpExportDataOffline(ctx context.Context, source *Source, connectionUri string, exportDir string, tableList []sqlname.NameTuple, quitChan chan bool, exportDataStart chan bool, exportSuccessChan chan bool, snapshotName string) {
	defer utils.WaitGroup.Done()

	pgDumpPath, binaryCheckIssue, err := GetAbsPathOfPGCommandAboveVersion("pg_dump", source.DBVersion)
	if err != nil {
		utils.ErrExit("could not get absolute path of pg_dump command: %v", err)
	} else if binaryCheckIssue != "" {
		utils.ErrExit("could not get absolute path of pg_dump command: %s", binaryCheckIssue)
	}

	pgDumpArgs.DataDirPath = filepath.Join(exportDir, "data")
	pgDumpArgs.TablesListPattern = createTableListPatterns(tableList)
	pgDumpArgs.ParallelJobs = strconv.Itoa(source.NumConnections)
	pgDumpArgs.DataFormat = "directory"

	args := getPgDumpArgsFromFile("data")
	if snapshotName != "" {
		args = fmt.Sprintf("%s --snapshot=%s", args, snapshotName)
	}
	if config.IsLogLevelDebugOrBelow() {
		args = fmt.Sprintf("%s --verbose", args)
	}
	cmd := fmt.Sprintf(`%s '%s' %s`, pgDumpPath, connectionUri, args)
	log.Infof("Running command: %s", cmd)
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	proc := exec.CommandContext(ctx, "/bin/bash", "-c", cmd)
	proc.Env = append(os.Environ(), "PGPASSWORD="+source.Password)
	proc.Stderr = &outbuf
	proc.Stdout = &errbuf
	err = proc.Start()
	if outbuf.String() != "" {
		log.Infof("%s", outbuf.String())
	}
	if err != nil {
		fmt.Printf("pg_dump failed to start exporting data with error: %v. For more details check '%s/logs/yb-voyager.log'.\n", err, exportDir)
		log.Infof("pg_dump failed to start exporting data with error: %v\n%s", err, errbuf.String())
		quitChan <- true
		runtime.Goexit()
	}
	utils.PrintAndLog("Data export started.")
	exportDataStart <- true

	// Parsing the main toc.dat file in parallel.
	go parseAndCreateTocTextFile(pgDumpArgs.DataDirPath)

	// Wait for pg_dump to complete before renaming of data files.
	err = proc.Wait()
	if err != nil {
		fmt.Printf("pg_dump failed to export data with error: %v. For more details check '%s/logs/yb-voyager-export-data.log'.\n", err, exportDir)
		log.Infof("pg_dump failed to export data with output: %s", outbuf.String())
		log.Infof("pg_dump failed to export data with error: %v\n%s", err, errbuf.String())
		quitChan <- true
		runtime.Goexit()
	}
	exportSuccessChan <- true
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
		utils.ErrExit("parsing tocfile: %q: %v", tocFilePath, err)
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

func createTableListPatterns(tableList []sqlname.NameTuple) string {
	var tableListPattern string
	for _, table := range tableList {
		tableListPattern += fmt.Sprintf("--table='%s' ", table.ForKey())
	}

	return strings.TrimPrefix(tableListPattern, "--table=")
}

func renameDataFiles(tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	for _, tableProgressMetadata := range tablesProgressMetadata {
		oldFilePath := tableProgressMetadata.InProgressFilePath
		newFilePath := tableProgressMetadata.FinalFilePath
		if utils.FileOrFolderExists(oldFilePath) {
			log.Infof("Renaming %q -> %q", oldFilePath, newFilePath)
			err := os.Rename(oldFilePath, newFilePath)
			if err != nil {
				utils.ErrExit("renaming data file: for table %q after data export: %v", tableProgressMetadata.TableName, err)
			}
		} else {
			log.Infof("File %q to rename doesn't exists!", oldFilePath)
		}
	}
}
