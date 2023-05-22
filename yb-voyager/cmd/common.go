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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/fatih/color"
	_ "github.com/godror/godror"
	"github.com/gosuri/uitable"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func updateFilePaths(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	var requiredMap map[string]string

	// TODO: handle the case if table name has double quotes/case sensitive

	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(filepath.Join(exportDir, "data"))
		for _, key := range sortedKeys {
			tableName := tablesProgressMetadata[key].TableName
			fullTableName := tableName.Qualified.MinQuoted

			if _, ok := requiredMap[fullTableName]; ok { // checking if toc/dump has data file for table
				tablesProgressMetadata[key].InProgressFilePath = filepath.Join(exportDir, "data", requiredMap[fullTableName])
				if tablesProgressMetadata[key].TableName.SchemaName.Unquoted == "public" {
					tablesProgressMetadata[key].FinalFilePath = filepath.Join(exportDir, "data", tableName.ObjectName.MinQuoted+"_data.sql")
				} else {
					tablesProgressMetadata[key].FinalFilePath = filepath.Join(exportDir, "data", fullTableName+"_data.sql")
				}
			} else {
				log.Infof("deleting an entry %q from tablesProgressMetadata: ", key)
				delete(tablesProgressMetadata, key)
			}
		}
	} else if source.DBType == "oracle" || source.DBType == "mysql" {
		for _, key := range sortedKeys {
			targetTableName := tablesProgressMetadata[key].TableName.ObjectName.Unquoted
			// required if PREFIX_PARTITION is set in ora2pg.conf file
			if tablesProgressMetadata[key].IsPartition {
				targetTableName = tablesProgressMetadata[key].ParentTable + "_" + targetTableName
			}
			tablesProgressMetadata[key].InProgressFilePath = filepath.Join(exportDir, "data", "tmp_"+targetTableName+"_data.sql")
			tablesProgressMetadata[key].FinalFilePath = filepath.Join(exportDir, "data", targetTableName+"_data.sql")
		}
	}

	logMsg := "After updating data file paths, TablesProgressMetadata:"
	for _, key := range sortedKeys {
		logMsg += fmt.Sprintf("%+v\n", tablesProgressMetadata[key])
	}
	log.Infof(logMsg)
}

func getMappingForTableNameVsTableFileName(dataDirPath string) map[string]string {
	tocTextFilePath := filepath.Join(dataDirPath, "toc.txt")
	// waitingFlag := 0
	for !utils.FileOrFolderExists(tocTextFilePath) {
		// waitingFlag = 1
		time.Sleep(time.Second * 1)
	}

	pgRestorePath, err := srcdb.GetAbsPathOfPGCommand("pg_restore")
	if err != nil {
		utils.ErrExit("could not get absolute path of pg_restore command: %v", pgRestorePath)
	}
	pgRestoreCmd := exec.Command(pgRestorePath, "-l", dataDirPath)
	stdOut, err := pgRestoreCmd.Output()
	log.Infof("cmd: %s", pgRestoreCmd.String())
	log.Infof("output: %s", string(stdOut))
	if err != nil {
		utils.ErrExit("ERROR: couldn't parse the TOC file to collect the tablenames for data files: %v", err)
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
			if nameContainsCapitalLetter(tableName) || sqlname.IsReservedKeyword(tableName) {
				// Surround the table name with double quotes.
				tableName = fmt.Sprintf("\"%s\"", tableName)
			}
			fullTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
			tableNameVsFileNameMap[fullTableName] = fileName
		}
	}

	tocTextFileDataBytes, err := os.ReadFile(tocTextFilePath)
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
	os.WriteFile(filepath.Join(dataDirPath, "postdata.sql"), []byte(sequencesPostData.String()), 0644)
	return tableNameVsFileNameMap
}

func UpdateTableApproxRowCount(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	utils.PrintAndLog("calculating approx num of rows to export for each table...")
	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	for _, key := range sortedKeys {
		approxRowCount := source.DB().GetTableApproxRowCount(tablesProgressMetadata[key].TableName)
		tablesProgressMetadata[key].CountTotalRows = approxRowCount
	}

	log.Tracef("After updating total approx row count, TablesProgressMetadata: %+v", tablesProgressMetadata)
}

func GetTableRowCount(filePath string) map[string]int64 {
	tableRowCountMap := make(map[string]int64)

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		utils.ErrExit("read file %q: %s", filePath, err)
	}

	lines := strings.Split(strings.Trim(string(fileBytes), "\n"), "\n")

	for _, line := range lines {
		tableName := strings.Split(line, ",")[0]
		rowCount := strings.Split(line, ",")[1]
		rowCountInt64, _ := strconv.ParseInt(rowCount, 10, 64)

		tableRowCountMap[tableName] = rowCountInt64
	}

	log.Infof("tableRowCountMap: %v", tableRowCountMap)
	return tableRowCountMap
}

func printExportedRowCount(exportedRowCount map[string]int64, useDebezium bool) {
	var keys []string
	for key := range exportedRowCount {
		keys = append(keys, key)
	}

	table := uitable.New()
	headerfmt := color.New(color.FgGreen, color.Underline).SprintFunc()

	if useDebezium {
		exportStatus, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
		if err != nil {
			utils.ErrExit("Failed to read export status during data export: %v", err)
		}
		for i, tableStatus := range exportStatus.Tables {
			if i == 0 {
				if tableStatus.SchemaName != "" {
					table.AddRow(headerfmt("SCHEMA"), headerfmt("TABLE"), headerfmt("ROW COUNT"))
				} else {
					table.AddRow(headerfmt("DATABASE"), headerfmt("TABLE"), headerfmt("ROW COUNT"))
				}
			}
			if tableStatus.SchemaName != "" {
				table.AddRow(tableStatus.SchemaName, tableStatus.TableName, tableStatus.ExportedRowCountSnapshot)
			} else {
				table.AddRow(tableStatus.DatabaseName, tableStatus.TableName, tableStatus.ExportedRowCountSnapshot)
			}
		}
	} else {
		table.AddRow(headerfmt("TABLE"), headerfmt("ROW COUNT"))
		sort.Strings(keys)
		for _, key := range keys {
			table.AddRow(key, exportedRowCount[key])
		}
	}

	fmt.Print("\n")
	fmt.Println(table)
	fmt.Print("\n")

}

// setup a project having subdirs for various database objects IF NOT EXISTS
func CreateMigrationProjectIfNotExists(dbType string, exportDir string) {
	// TODO: add a check/prompt if any directories apart from required ones are present in export-dir
	var projectSubdirs = []string{
		"schema", "data", "reports",
		"metainfo", "metainfo/data", "metainfo/schema", "metainfo/flags", "metainfo/conf",
		"temp", "temp/ora2pg_temp_dir",
	}

	// log.Debugf("Creating a project directory...")
	//Assuming export directory as a project directory
	projectDirPath := exportDir

	for _, subdir := range projectSubdirs {
		err := exec.Command("mkdir", "-p", filepath.Join(projectDirPath, subdir)).Run()
		if err != nil {
			utils.ErrExit("couldn't create sub-directories under %q: %v", projectDirPath, err)
		}
	}

	// Put info to metainfo/schema about the source db
	sourceInfoFile := filepath.Join(projectDirPath, "metainfo", "schema", "source-db-"+dbType)
	_, err := exec.Command("touch", sourceInfoFile).CombinedOutput()
	if err != nil {
		utils.ErrExit("coludn't touch file %q: %v", sourceInfoFile, err)
	}

	schemaObjectList := utils.GetSchemaObjectList(dbType)

	// creating subdirs under schema dir
	for _, schemaObjectType := range schemaObjectList {
		if schemaObjectType == "INDEX" { //no separate dir for indexes
			continue
		}
		databaseObjectDirName := strings.ToLower(schemaObjectType) + "s"

		err := exec.Command("mkdir", "-p", filepath.Join(projectDirPath, "schema", databaseObjectDirName)).Run()
		if err != nil {
			utils.ErrExit("couldn't create sub-directories under %q: %v", filepath.Join(projectDirPath, "schema"), err)
		}
	}

	// log.Debugf("Created a project directory...")
}

func nameContainsCapitalLetter(name string) bool {
	for _, c := range name {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
}
