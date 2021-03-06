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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	"github.com/jackc/pgx/v4"
)

func YugabyteDBImportSchema(target *tgtdb.Target, exportDir string) {
	//this list also has defined the order to create object type in target YugabyteDB
	importObjectOrderList := utils.GetSchemaObjectList(sourceDBType)
	for _, importObjectType := range importObjectOrderList {
		var importObjectDirPath, importObjectFilePath string

		if importObjectType != "INDEX" {
			importObjectDirPath = filepath.Join(exportDir, "schema", strings.ToLower(importObjectType)+"s")
			importObjectFilePath = filepath.Join(importObjectDirPath, strings.ToLower(importObjectType)+".sql")
		} else {
			if target.ImportIndexesAfterData {
				continue
			}
			importObjectDirPath = filepath.Join(exportDir, "schema", "tables")
			importObjectFilePath = filepath.Join(importObjectDirPath, "INDEXES_table.sql")
		}

		if !utils.FileOrFolderExists(importObjectFilePath) {
			continue
		}

		fmt.Printf("importing %10s %5s", importObjectType, "")
		go utils.Wait("done\n", "")

		log.Infof("Importing %q", importObjectFilePath)

		conn, err := pgx.Connect(context.Background(), target.GetConnectionUri())
		if err != nil {
			utils.WaitChannel <- 1
			<-utils.WaitChannel
			utils.ErrExit("Failed to connect to the target DB: %s", err)
		}

		// target-db-schema is not public and source is either Oracle/MySQL
		if sourceDBType != POSTGRESQL {
			setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", target.Schema)
			log.Infof("Running query %q on the target DB", setSchemaQuery)
			_, err := conn.Exec(context.Background(), setSchemaQuery)
			if err != nil {
				utils.ErrExit("Failed to run %q on target DB: %s", setSchemaQuery, err)
			}

			setClientEncQuery := IMPORT_SESSION_SETTERS[0]
			log.Infof("Running query %q on the target DB", setClientEncQuery)
			_, err = conn.Exec(context.Background(), setClientEncQuery)
			if err != nil {
				utils.ErrExit("Failed to run %q on target DB: %s", setClientEncQuery, err)
			}
		}

		reCreateSchema := regexp.MustCompile(`(?i)CREATE SCHEMA public`)
		sqlStrArray := createSqlStrArray(importObjectFilePath, importObjectType)
		errOccured := 0
		for _, sqlStr := range sqlStrArray {
			log.Infof("Execute STATEMENT:\n%s", sqlStr[1])
			_, err := conn.Exec(context.Background(), sqlStr[0])
			if err != nil {
				log.Errorf("Previous SQL statement failed with error: %s", err)
				if strings.Contains(err.Error(), "already exists") {
					if !target.IgnoreIfExists && !reCreateSchema.MatchString(sqlStr[1]) {
						fmt.Printf("\b \n    %s\n", err.Error())
						fmt.Printf("    STATEMENT: %s\n", sqlStr[1])
						if !target.ContinueOnError {
							os.Exit(1)
						}
					}
				} else {
					errOccured = 1
					fmt.Printf("\b \n    %s\n", err.Error())
					fmt.Printf("    STATEMENT: %s\n", sqlStr[1])
					if !target.ContinueOnError { //default case
						fmt.Println(err)
						os.Exit(1)
					}
				}
				log.Infof("Continuing despite error: IgnoreIfExists(%v), ContinueOnError(%v)",
					target.IgnoreIfExists, target.ContinueOnError)
			}
		}

		utils.WaitChannel <- errOccured
		<-utils.WaitChannel

		conn.Close(context.Background())
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
