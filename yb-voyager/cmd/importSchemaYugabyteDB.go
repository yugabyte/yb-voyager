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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
)

func importSchemaInternal(target *tgtdb.Target, exportDir string, importObjectList []string,
	skipFn func(string, string) bool) {
	schemaDir := filepath.Join(exportDir, "schema")
	for _, importObjectType := range importObjectList {
		importObjectFilePath := utils.GetObjectFilePath(schemaDir, importObjectType)
		if !utils.FileOrFolderExists(importObjectFilePath) {
			continue
		}
		fmt.Printf("\nImporting %q\n\n", importObjectFilePath)
		executeSqlFile(importObjectFilePath, importObjectType, skipFn)
	}
	log.Info("Schema import is complete.")
}

func ExtractMetaInfo(exportDir string) utils.ExportMetaInfo {
	log.Infof("Extracting the metainfo about the source database.")
	var metaInfo utils.ExportMetaInfo

	metaInfoDirPath := exportDir + "/metainfo"

	metaInfoDir, err := os.ReadDir(metaInfoDirPath)
	if err != nil {
		utils.ErrExit("Failed to read directory %q: %v", metaInfoDirPath, err)
	}

	for _, metaInfoSubDir := range metaInfoDir {
		if !metaInfoSubDir.IsDir() {
			continue
		}
		subItemPath := metaInfoDirPath + "/" + metaInfoSubDir.Name()
		subItems, err := os.ReadDir(subItemPath)
		if err != nil {
			utils.ErrExit("Failed to read directory %q: %v", subItemPath, err)
		}
		for _, subItem := range subItems {
			subItemName := subItem.Name()
			if strings.HasPrefix(subItemName, "source-db-") {
				splits := strings.Split(subItemName, "-")
				metaInfo.SourceDBType = splits[len(splits)-1]
			}
		}
	}
	return metaInfo
}

func applySchemaObjectFilterFlags(importObjectOrderList []string) []string {
	var finalImportObjectList []string
	excludeObjectList := utils.CsvStringToSlice(target.ExcludeImportObjects)
	for i, item := range excludeObjectList {
		excludeObjectList[i] = strings.ToUpper(item)
	}
	if target.ImportObjects != "" {
		includeObjectList := utils.CsvStringToSlice(target.ImportObjects)
		for i, item := range includeObjectList {
			includeObjectList[i] = strings.ToUpper(item)
		}
		// Maintain the order of importing the objects.
		for _, supportedObject := range importObjectOrderList {
			if slices.Contains(includeObjectList, supportedObject) {
				finalImportObjectList = append(finalImportObjectList, supportedObject)
			}
		}
	} else {
		finalImportObjectList = utils.SetDifference(importObjectOrderList, excludeObjectList)
	}
	if sourceDBType == "postgresql" && !slices.Contains(finalImportObjectList, "SCHEMA") && !flagPostImportData { // Schema should be migrated by default.
		finalImportObjectList = append([]string{"SCHEMA"}, finalImportObjectList...)
	}
	return finalImportObjectList
}

func mergeSqlFilesIfNeeded(filePath string, objType string) {
	// Used for scenarios when we have statements exclusively of the type "\i <filename>.sql" in the file(s) used during import schema.
	rollbackTransaction := func(initFile *os.File, initFilePath string, copyFilePath string) {
		if err := initFile.Close(); err != nil {
			utils.ErrExit("Error while merging statements: %v. Replace the contents of file %s with contents of file %s before retrying", err, initFilePath, copyFilePath)
		}
		// Rename automatically overwrites contents of the new file location.
		if err := os.Rename(copyFilePath, initFilePath); err != nil {
			utils.ErrExit("Error while merging statements: %v. Rename file %s to %s before retrying", err, copyFilePath, initFilePath)
		}
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		utils.ErrExit("Error while opening %s: %v", filePath, err)
	}

	// Gather list of files to merge (if any).
	fileMergeQueue := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		currLine := scanner.Text()
		if len(currLine) == 0 {
			continue
		}
		copyFromFileRegex := regexp.MustCompile(`(?i)\\i (.*.sql)`)
		if !copyFromFileRegex.MatchString(strings.TrimLeft(currLine, " ")) {
			// This file contains DDL, no need for merging.
			return
		}
		fileMergeQueue = append(fileMergeQueue, copyFromFileRegex.FindStringSubmatch(strings.TrimLeft(currLine, ""))[1])
	}
	if len(fileMergeQueue) == 0 {
		return
	}
	// Preserve contents of file in case something goes wrong.
	tempFilePath := filepath.Join(filepath.Dir(filePath), strings.ToLower(objType)+"_old.sql")
	tempFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		utils.ErrExit("Error while opening %s: %v", filePath, err)
	}
	if _, err = file.Seek(0, 0); err != nil {
		utils.ErrExit("Error while merging DDLs into file %s: %v", filePath, err)
	}
	if _, err := io.Copy(tempFile, file); err != nil {
		utils.ErrExit("Could not preserve original contents of file %s in temp file: %v", filePath, err)
	}
	if err = tempFile.Close(); err != nil {
		utils.ErrExit("Error while closing copy file %s: %v", tempFilePath, err)
	}

	// Merge the files' contents into intended file.
	file.Truncate(0)
	_, err = file.Seek(0, 0)
	if err != nil {
		rollbackTransaction(file, filePath, tempFilePath)
		utils.ErrExit("Error while merging DDLs into file %s: %v", filePath, err)
	}
	for _, mergeFilePath := range fileMergeQueue {
		mergeFile, err := os.Open(mergeFilePath)
		if err != nil {
			rollbackTransaction(file, filePath, tempFilePath)
			utils.ErrExit("Error while opening %s: %v", filePath, err)
		}
		defer mergeFile.Close()
		utils.PrintAndLog("Merging file %s", mergeFilePath)
		_, err = io.Copy(file, mergeFile)
		if err != nil {
			rollbackTransaction(file, filePath, tempFilePath)
			utils.ErrExit("Error while copying from file %s: %v", mergeFilePath, err)
		}
	}
	if err = file.Close(); err != nil {
		log.Warnf("Unable to close file %s after merging statements: %v", filePath, err)
	}
}
