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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func YugabyteDBImportSchema(target *tgtdb.Target, exportDir string) {
	schemaDirPath := filepath.Join(exportDir, "schema")
	// this list also has defined the order to create object type in target YugabyteDB
	importObjectOrderList := utils.GetSchemaObjectList(sourceDBType)
	for _, importObjectType := range importObjectOrderList {
		if importObjectType == "INDEX" && target.ImportIndexesAfterData {
			continue
		}

		importObjectFilePath := utils.GetObjectFilePath(schemaDirPath, importObjectType)
		if !utils.FileOrFolderExists(importObjectFilePath) {
			log.Warnf("file %q doesn't exist, no import", importObjectFilePath)
			continue
		}

		fmt.Printf("importing %10s %5s", importObjectType, "")
		log.Infof("importing %q", importObjectFilePath)
		go utils.Wait("done\n", "")

		status := executeSqlFile(importObjectFilePath, importObjectType)
		if status == 1 {
			utils.ErrExit("Abort! error occured during %q import", importObjectType)
		}

		if importObjectType == "INDEX" && !target.ImportIndexesAfterData {
			partIdxFilePath := filepath.Join(schemaDirPath, "partitions/PARTITION_INDEXES_partition.sql")
			status = executeSqlFile(partIdxFilePath, importObjectType)
			if status == 1 {
				utils.ErrExit("Abort! error occured during %q import", importObjectType)
			}
		}

		utils.WaitChannel <- status
		<-utils.WaitChannel
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
