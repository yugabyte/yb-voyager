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
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func ora2pgExportDataOffline(ctx context.Context, source *Source, exportDir string, tableNameList []*sqlname.SourceName,
	tablesColumnList map[*sqlname.SourceName][]string, quitChan chan bool, exportDataStart chan bool, exportSuccessChan chan bool) {
	defer utils.WaitGroup.Done()

	//ora2pg does accepts table names in format of SCHEMA_NAME.TABLE_NAME
	tableList := []string{}
	for _, tableName := range tableNameList {
		tableList = append(tableList, tableName.ObjectName.Unquoted)
	}
	conf := getDefaultOra2pgConfig(source)
	conf.DisablePartition = "1"
	conf.Allow = fmt.Sprintf("TABLE%v", tableList)
	// providing column list for tables having unsupported column types
	for tableName, columnList := range tablesColumnList {
		allColumns := "*"
		if len(columnList) == 1 && columnList[0] == allColumns {
			continue
		}
		log.Infof("Modifying struct for table %s, columnList: %v\n", tableName.ObjectName.Unquoted, columnList)
		conf.ModifyStruct += fmt.Sprintf("%s(%s) ", tableName.ObjectName.Unquoted, strings.Join(columnList, ","))
	}
	configFilePath := filepath.Join(exportDir, "temp", ".ora2pg.conf")
	populateOra2pgConfigFile(configFilePath, conf)

	tempDirPath := filepath.Join(exportDir, "temp", "ora2pg_temp_dir")
	exportDataCommandString := fmt.Sprintf("ora2pg -q -t COPY -P %d -o data.sql -b %s/data -c %s --no_header -T %s",
		source.NumConnections, exportDir, configFilePath, tempDirPath)

	os.Setenv("ORA2PG_PASSWD", source.Password)
	defer os.Unsetenv("ORA2PG_PASSWD")
	//Exporting all the tables in the schema
	log.Infof("Executing command: %s", exportDataCommandString)
	exportDataCommand := exec.CommandContext(ctx, "/bin/bash", "-c", exportDataCommandString)

	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	exportDataCommand.Stdout = &outbuf
	exportDataCommand.Stderr = &errbuf

	err := exportDataCommand.Start()
	if err != nil {
		utils.ErrExit("Failed to initiate data export: %v\n%s", err, errbuf.String())
	}
	fmt.Println("Data export started.")
	exportDataStart <- true

	err = exportDataCommand.Wait()
	log.Infof(`ora2pg STDOUT: "%s"`, outbuf.String())
	log.Errorf(`ora2pg STDERR: "%s"`, errbuf.String())
	if err != nil {
		utils.ErrExit("Data export failed: %v\n%s", err, errbuf.String())
	}

	// move to ALTER SEQUENCE commands to postdata.sql file
	extractAlterSequenceStatements(exportDir)
	exportSuccessChan <- true
}

func extractAlterSequenceStatements(exportDir string) {
	alterSequenceRegex := regexp.MustCompile(`(?)ALTER SEQUENCE`)
	filePath := filepath.Join(exportDir, "data", "data.sql")
	var requiredLines strings.Builder

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		if alterSequenceRegex.MatchString(line) {
			requiredLines.WriteString(line + "\n")
		}
	}

	os.WriteFile(filepath.Join(exportDir, "data", "postdata.sql"), []byte(requiredLines.String()), 0644)
}

// extract all identity column names from ALTER SEQUENCE IF EXISTS statements in postdata.sql
func getIdentityColumnSequences(exportDir string) []string {
	var identityColumns []string
	alterSequenceRegex := regexp.MustCompile(`ALTER SEQUENCE (IF EXISTS )?(.*)? RESTART WITH`)

	filePath := filepath.Join(exportDir, "data", "postdata.sql")
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		utils.ErrExit("unable to read file %q: %v\n", filePath, err)
	}

	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		if matches := alterSequenceRegex.FindStringSubmatch(line); matches != nil && strings.HasPrefix(matches[2], "iseq$$_") {
			identityColumns = append(identityColumns, matches[2])
		}
	}

	log.Infof("extracted identity columns list: %v", identityColumns)
	return identityColumns
}

// replace all identity column names from ALTER SEQUENCE IF EXISTS statements in postdata.sql
func replaceAllIdentityColumns(exportDir string, sourceTargetIdentitySequenceNames map[string]string) {
	filePath := filepath.Join(exportDir, "data", "postdata.sql")
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		utils.ErrExit("unable to read file %q: %v\n", filePath, err)
	}

	lines := strings.Split(string(bytes), "\n")
	for sourceSeq, targetSeq := range sourceTargetIdentitySequenceNames {
		for i, line := range lines {
			// extra space to avoid matching iseq$$_81022 with iseq$$_810220
			if strings.Contains(line, sourceSeq+" ") {
				lines[i] = strings.ReplaceAll(line, sourceSeq, targetSeq)
			}
		}
	}

	var bytesToWrite []byte
	for _, line := range lines {
		// ignore the ALTER stmts having iseq$$_ and keep the rest
		if !strings.Contains(line, "iseq$$_") {
			bytesToWrite = append(bytesToWrite, []byte(line+"\n")...)
		}
	}

	err = os.WriteFile(filePath, bytesToWrite, 0644)
	if err != nil {
		utils.ErrExit("unable to write file %q: %v\n", filePath, err)
	}
}

// renaming data files having pgReservedKeywords to include quotes
func renameDataFilesForReservedWords(tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	log.Infof("renaming data files for tables with reserved words in them")
	for _, tableProgressMetadata := range tablesProgressMetadata {
		tblNameUnquoted := tableProgressMetadata.TableName.ObjectName.Unquoted
		if !sqlname.IsReservedKeyword(tblNameUnquoted) {
			continue
		}

		tblNameQuoted := fmt.Sprintf(`"%s"`, tblNameUnquoted)
		oldFilePath := tableProgressMetadata.FinalFilePath
		newFilePath := filepath.Join(filepath.Dir(oldFilePath), tblNameQuoted+"_data.sql")
		if utils.FileOrFolderExists(oldFilePath) {
			log.Infof("Renaming %q -> %q", oldFilePath, newFilePath)
			err := os.Rename(oldFilePath, newFilePath)
			if err != nil {
				utils.ErrExit("renaming data file for table %q after data export: %v", tblNameQuoted, err)
			}
			tableProgressMetadata.FinalFilePath = newFilePath
		} else {
			utils.PrintAndLog("File %q to rename doesn't exists!", oldFilePath)
		}
	}
}

func getOra2pgExportedColumnsMap(exportDir string, tablesMetadata map[string]*utils.TableProgressMetadata) map[string][]string {

	result := make(map[string][]string)
	for _, tableMetadata := range tablesMetadata {
		if tableMetadata.CountLiveRows == 0 {
			continue
		}
		tableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		result[tableName] = getOra2pgExportedColumnsListForTable(exportDir, tableName, tableMetadata.FinalFilePath)
	}
	return result
}

func getOra2pgExportedColumnsListForTable(exportDir, tableName, filePath string) []string {
	var columnsList []string

	re := regexp.MustCompile(`(?i)COPY .*[\s]+\((.*)\) FROM STDIN`)
	err := utils.ForEachMatchingLineInFile(filePath, re, func(matches []string) bool {
		columnsList = strings.Split(matches[1], ",")
		for i, column := range columnsList {
			columnsList[i] = strings.TrimSpace(column)
		}
		return false // stop reading file
	})
	if err != nil {
		utils.ErrExit("error in reading file %q: %v", filePath, err)
	}
	log.Infof("columns list for table %s: %v", tableName, columnsList)
	return columnsList
}
