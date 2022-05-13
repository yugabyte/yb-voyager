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
package migration

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/srcdb"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func PgDumpExportDataOffline(ctx context.Context, source *srcdb.Source, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	defer utils.WaitGroup.Done()

	dataDirPath := exportDir + "/data"

	tableListPatterns := createTableListPatterns(tableList)

	SSLQueryString := generateSSLQueryStringIfNotExists(source)

	// Using pgdump for exporting data in directory format.
	cmd := ""
	if source.Uri != "" {
		cmd = fmt.Sprintf(`pg_dump "%s" --no-blobs --data-only --compress=0 %s -Fd --file %s --jobs %d`, source.Uri, tableListPatterns, dataDirPath, source.NumConnections)
	} else {
		cmd = fmt.Sprintf(`pg_dump "postgresql://%s:%s@%s:%d/%s?%s" --no-blobs --data-only --compress=0 %s -Fd --file %s --jobs %d`, source.User, source.Password,
			source.Host, source.Port, source.DBName, SSLQueryString, tableListPatterns, dataDirPath, source.NumConnections)
	}
	log.Infof("Running command: %s", cmd)
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	proc := exec.CommandContext(ctx, "/bin/bash", "-c", cmd)
	proc.Stderr = &outbuf
	proc.Stdout = &errbuf
	err := proc.Start()
	if outbuf.String() != "" {
		log.Infof("%s", outbuf.String())
	}
	if err != nil {
		fmt.Printf("pg_dump failed to start exporting data with error: %v. Refer to Refer to'%s/yb_migrate.log' for further details.", err, exportDir)
		log.Infof("pg_dump failed to start exporting data with error: %v\n%s", err, errbuf.String())
		quitChan <- true
		runtime.Goexit()
	}
	utils.PrintAndLog("Data export started.")
	exportDataStart <- true

	// Parsing the main toc.dat file in parallel.
	go parseAndCreateTocTextFile(dataDirPath)

	// Wait for pg_dump to complete before renaming of data files.
	err = proc.Wait()
	if err != nil {
		fmt.Printf("pg_dump failed to export data with error: %v. Refer to Refer to'%s/yb_migrate.log' for further details.", err, exportDir)
		log.Infof("pg_dump failed to export data with error: %v\n%s", err, errbuf.String())
		quitChan <- true
		runtime.Goexit()
	}
}

//The function might be error prone rightnow, will need to verify with other possible toc files. Testing needs to be done
func getMappingForTableNameVsTableFileName(dataDirPath string) map[string]string {
	tocTextFilePath := dataDirPath + "/toc.txt"
	// waitingFlag := 0
	for !utils.FileOrFolderExists(tocTextFilePath) {
		// waitingFlag = 1
		time.Sleep(time.Second * 1)
	}

	pgRestoreCmd := exec.Command("pg_restore", "-l", dataDirPath)
	stdOut, err := pgRestoreCmd.Output()
	if err != nil {
		utils.ErrExit("Couldn't parse the TOC file to collect the tablenames for data files: %s", err)
	}

	tableNameVsFileNameMap := make(map[string]string)
	var sequencesPostData strings.Builder

	lines := strings.Split(string(stdOut), "\n")
	for _, line := range lines {
		// example of line: 3725; 0 16594 TABLE DATA public categories ds2
		parts := strings.Split(line, " ")

		if len(parts) < 8 { // those lines don't contain table/sequences related info
			continue
		} else if parts[3] == "TABLE" && parts[4] == "DATA" {
			fileName := strings.Trim(parts[0], ";") + ".dat"
			schemaName := parts[5]
			tableName := parts[6]
			if nameContainsCapitalLetter(tableName) {
				// Surround the table name with double quotes.
				tableName = fmt.Sprintf("\"%s\"", tableName)
			}
			fullTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
			tableNameVsFileNameMap[fullTableName] = fileName
		}
	}

	tocTextFileDataBytes, err := ioutil.ReadFile(tocTextFilePath)
	if err != nil {
		utils.ErrExit("Failed to read file %q: %v", tocTextFilePath, err)
	}

	tocTextFileData := strings.Split(string(tocTextFileDataBytes), "\n")
	numLines := len(tocTextFileData)
	setvalRegex := regexp.MustCompile("(?i)SELECT.*setval")

	for i := 0; i < numLines; i++ {
		if setvalRegex.MatchString(tocTextFileData[i]) {
			sequencesPostData.WriteString(tocTextFileData[i])
			sequencesPostData.WriteString("\n")
		}
	}

	//extracted SQL for setval() and put it into a postexport.sql file
	ioutil.WriteFile(dataDirPath+"/postdata.sql", []byte(sequencesPostData.String()), 0644)
	return tableNameVsFileNameMap
}

func nameContainsCapitalLetter(name string) bool {
	for _, c := range name {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
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

func generateSSLQueryStringIfNotExists(s *srcdb.Source) string {

	if s.Uri == "" {
		SSLQueryString := ""
		if s.SSLQueryString == "" {

			if s.SSLMode == "disable" || s.SSLMode == "allow" || s.SSLMode == "prefer" || s.SSLMode == "require" || s.SSLMode == "verify-ca" || s.SSLMode == "verify-full" {
				SSLQueryString = "sslmode=" + s.SSLMode
				if s.SSLMode == "require" || s.SSLMode == "verify-ca" || s.SSLMode == "verify-full" {
					SSLQueryString = fmt.Sprintf("sslmode=%s", s.SSLMode)
					if s.SSLCertPath != "" {
						SSLQueryString += "&sslcert=" + s.SSLCertPath
					}
					if s.SSLKey != "" {
						SSLQueryString += "&sslkey=" + s.SSLKey
					}
					if s.SSLRootCert != "" {
						SSLQueryString += "&sslrootcert=" + s.SSLRootCert
					}
					if s.SSLCRL != "" {
						SSLQueryString += "&sslcrl=" + s.SSLCRL
					}
				}
			} else {
				utils.ErrExit("Invalid sslmode entered.")
			}
		} else {
			SSLQueryString = s.SSLQueryString
		}
		return SSLQueryString
	} else {
		return ""
	}
}
