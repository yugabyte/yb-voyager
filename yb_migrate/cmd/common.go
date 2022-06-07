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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/srcdb"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"

	_ "github.com/godror/godror"
	log "github.com/sirupsen/logrus"
)

func UpdateFilePaths(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	var requiredMap map[string]string

	// TODO: handle the case if table name has double quotes/case sensitive

	sortedKeys := utils.GetSortedKeys(&tablesProgressMetadata)
	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(exportDir + "/data")
		for _, key := range sortedKeys {
			tableName := tablesProgressMetadata[key].TableName
			fullTableName := tablesProgressMetadata[key].FullTableName

			if _, ok := requiredMap[fullTableName]; ok { // checking if toc/dump has data file for table
				tablesProgressMetadata[key].InProgressFilePath = exportDir + "/data/" + requiredMap[fullTableName]
				if tablesProgressMetadata[key].TableSchema == "public" {
					tablesProgressMetadata[key].FinalFilePath = exportDir + "/data/" + tableName + "_data.sql"
				} else {
					tablesProgressMetadata[key].FinalFilePath = exportDir + "/data/" + fullTableName + "_data.sql"
				}
			} else {
				log.Infof("deleting an entry %q from tablesProgressMetadata: ", key)
				delete(tablesProgressMetadata, key)
			}
		}
	} else if source.DBType == "oracle" || source.DBType == "mysql" {
		for _, key := range sortedKeys {
			targetTableName := tablesProgressMetadata[key].TableName
			// required if PREFIX_PARTITION is set in ora2pg.conf file
			// if tablesProgressMetadata[key].IsPartition {
			// 	targetTableName = tablesProgressMetadata[key].ParentTable + "_" + targetTableName
			// }
			tablesProgressMetadata[key].InProgressFilePath = exportDir + "/data/tmp_" + targetTableName + "_data.sql"
			tablesProgressMetadata[key].FinalFilePath = exportDir + "/data/" + targetTableName + "_data.sql"
		}
	}

	logMsg := "After updating data file paths, TablesProgressMetadata:"
	for _, key := range sortedKeys {
		logMsg += fmt.Sprintf("%+v\n", tablesProgressMetadata[key])
	}
	log.Infof(logMsg)
}

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

func UpdateTableRowCount(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	fmt.Println("calculating num of rows to export for each table...")
	if !source.VerboseMode {
		go utils.Wait()
	}

	utils.PrintIfTrue(fmt.Sprintf("+%s+\n", strings.Repeat("-", 65)), source.VerboseMode)
	utils.PrintIfTrue(fmt.Sprintf("| %30s | %30s |\n", "Table", "Row Count"), source.VerboseMode)

	sortedKeys := utils.GetSortedKeys(&tablesProgressMetadata)
	for _, key := range sortedKeys {
		utils.PrintIfTrue(fmt.Sprintf("|%s|\n", strings.Repeat("-", 65)), source.VerboseMode)

		utils.PrintIfTrue(fmt.Sprintf("| %30s ", key), source.VerboseMode)

		if source.VerboseMode {
			go utils.Wait()
		}

		rowCount := source.DB().GetTableRowCount(tablesProgressMetadata[key].FullTableName)

		if source.VerboseMode {
			utils.WaitChannel <- 0
			<-utils.WaitChannel
		}

		tablesProgressMetadata[key].CountTotalRows = rowCount
		utils.PrintIfTrue(fmt.Sprintf("| %30d |\n", rowCount), source.VerboseMode)
	}
	utils.PrintIfTrue(fmt.Sprintf("+%s+\n", strings.Repeat("-", 65)), source.VerboseMode)
	if !source.VerboseMode {
		utils.WaitChannel <- 0
		<-utils.WaitChannel
	}

	log.Tracef("After updating total row count, TablesProgressMetadata: %+v", tablesProgressMetadata)
}

func GetTableRowCount(filePath string) map[string]int64 {
	tableRowCountMap := make(map[string]int64)

	fileBytes, err := ioutil.ReadFile(filePath)
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

func ExportDataPostProcessing(source *srcdb.Source, exportDir string, tablesProgressMetadata *map[string]*utils.TableProgressMetadata) {
	if source.DBType == "oracle" || source.DBType == "mysql" {
		// empty - in case of oracle and mysql, the renaming is handled by tool(ora2pg)
	} else if source.DBType == "postgresql" {
		renameDataFiles(tablesProgressMetadata)
	}

	saveExportedRowCount(exportDir, tablesProgressMetadata)
	utils.UpdateDataSize(exportDir)
}

func renameDataFiles(tablesProgressMetadata *map[string]*utils.TableProgressMetadata) {
	for _, tableProgressMetadata := range *tablesProgressMetadata {
		oldFilePath := tableProgressMetadata.InProgressFilePath
		newFilePath := tableProgressMetadata.FinalFilePath
		if utils.FileOrFolderExists(oldFilePath) {
			err := os.Rename(oldFilePath, newFilePath)
			if err != nil {
				utils.ErrExit("renaming data file for table %q after data export: %v", tableProgressMetadata.TableName, err)
			}
		}
	}
}

func saveExportedRowCount(exportDir string, tablesMetadata *map[string]*utils.TableProgressMetadata) {
	var maxTableLines, totalTableLines int64
	filePath := exportDir + "/metainfo/flags/tablesrowcount"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fmt.Println("exported num of rows for each table")
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
	fmt.Printf("| %30s | %30s |\n", "Table", "Row Count")
	sortedKeys := utils.GetSortedKeys(tablesMetadata)
	for _, key := range sortedKeys {
		tableMetadata := (*tablesMetadata)[key]
		fmt.Printf("|%s|\n", strings.Repeat("-", 65))

		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		actualRowCount := tableMetadata.CountLiveRows
		line := targetTableName + "," + strconv.FormatInt(actualRowCount, 10) + "\n"
		file.WriteString(line)
		fmt.Printf("| %30s | %30d |\n", key, actualRowCount)
		if maxTableLines < actualRowCount {
			maxTableLines = actualRowCount
		}
		totalTableLines += actualRowCount
	}
	utils.InitJSON(exportDir)
	payload := utils.GetPayload()
	payload.LargestTableRows = maxTableLines
	payload.TotalRows = totalTableLines
	utils.PackPayload(exportDir)
	utils.SendPayload()
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
}

//setup a project having subdirs for various database objects IF NOT EXISTS
func CreateMigrationProjectIfNotExists(source *srcdb.Source, exportDir string) {
	var projectSubdirs = []string{"schema", "data", "reports", "metainfo", "metainfo/data", "metainfo/schema", "metainfo/flags", "temp"}

	// log.Debugf("Creating a project directory...")
	//Assuming export directory as a project directory
	projectDirPath := exportDir

	for _, subdir := range projectSubdirs {
		err := exec.Command("mkdir", "-p", projectDirPath+"/"+subdir).Run()
		if err != nil {
			utils.ErrExit("couldn't create sub-directories under %q: %v", projectDirPath, err)
		}
	}

	// Put info to metainfo/schema about the source db
	sourceInfoFile := projectDirPath + "/metainfo/schema/" + "source-db-" + source.DBType
	_, err := exec.Command("touch", sourceInfoFile).CombinedOutput()
	if err != nil {
		utils.ErrExit("coludn't touch file %q: %v", sourceInfoFile, err)
	}

	schemaObjectList := utils.GetSchemaObjectList(source.DBType)

	// creating subdirs under schema dir
	for _, schemaObjectType := range schemaObjectList {
		if schemaObjectType == "INDEX" { //no separate dir for indexes
			continue
		}
		databaseObjectDirName := strings.ToLower(schemaObjectType) + "s"

		err := exec.Command("mkdir", "-p", projectDirPath+"/schema/"+databaseObjectDirName).Run()
		if err != nil {
			utils.ErrExit("couldn't create sub-directories under %q: %v", projectDirPath+"/schema", err)
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
