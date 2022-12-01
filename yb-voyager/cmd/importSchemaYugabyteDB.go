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
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
)

func importSchemaInternal(exportDir string, importObjectList []string,
	skipFn func(string, string) bool) {
	schemaDir := filepath.Join(exportDir, "schema")
	for _, importObjectType := range importObjectList {
		importObjectFilePath := utils.GetObjectFilePath(schemaDir, importObjectType)
		if !utils.FileOrFolderExists(importObjectFilePath) {
			continue
		}
		fmt.Printf("\nImporting %s DDLs from %q\n\n", importObjectType, importObjectFilePath)
		executeSqlFile(importObjectFilePath, importObjectType, skipFn)
	}
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
		if importObjectsInStraightOrder {
			// Import the objects in the same order as when listed by the user.
			for _, listedObject := range includeObjectList {
				if slices.Contains(importObjectOrderList, listedObject) {
					finalImportObjectList = append(finalImportObjectList, listedObject)
				}
			}
		} else {
			// Import the objects in the default order.
			for _, supportedObject := range importObjectOrderList {
				if slices.Contains(includeObjectList, supportedObject) {
					finalImportObjectList = append(finalImportObjectList, supportedObject)
				}
			}
		}
	} else {
		finalImportObjectList = utils.SetDifference(importObjectOrderList, excludeObjectList)
	}
	if sourceDBType == "postgresql" && !slices.Contains(finalImportObjectList, "SCHEMA") && !flagPostImportData { // Schema should be migrated by default.
		finalImportObjectList = append([]string{"SCHEMA"}, finalImportObjectList...)
	}
	if !flagPostImportData {
		finalImportObjectList = append(finalImportObjectList, []string{"UNIQUE INDEX"}...)
	}
	return finalImportObjectList
}
