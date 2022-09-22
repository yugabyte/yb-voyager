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
	"golang.org/x/exp/slices"

	"github.com/jackc/pgx/v4"
)

func getImportObjectFilePath(importObjectType string) string {
	if importObjectType == "INDEX" {
		d := filepath.Join(exportDir, "schema", "tables")
		return filepath.Join(d, "INDEXES_table.sql")
	} else {
		d := filepath.Join(exportDir, "schema", strings.ToLower(importObjectType)+"s")
		return filepath.Join(d, strings.ToLower(importObjectType)+".sql")
	}
}

func YugabyteDBImportSchema(target *tgtdb.Target, exportDir string) {
	finalImportObjectList := getImportObjectList()
	if len(finalImportObjectList) == 0 {
		utils.ErrExit("No schema objects to import! Must import at least 1 of the supported schema object types: %v", utils.GetSchemaObjectList(sourceDBType))
	}

	var err error
	var conn *pgx.Conn

	for _, importObjectType := range finalImportObjectList {
		importObjectFilePath := getImportObjectFilePath(importObjectType)
		if !utils.FileOrFolderExists(importObjectFilePath) {
			continue
		}

		fmt.Printf("\nImporting %q\n\n", importObjectFilePath)
		log.Infof("Importing %q", importObjectFilePath)

		if conn == nil {
			conn, err = pgx.Connect(context.Background(), target.GetConnectionUri())
			if err != nil {
				utils.ErrExit("Failed to connect to the target DB: %s", err)
			}
		}
		// target-db-schema is not public and source is either Oracle/MySQL
		if sourceDBType != POSTGRESQL {
			setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", target.Schema)
			log.Infof("Running query %q on the target DB", setSchemaQuery)
			_, err := conn.Exec(context.Background(), setSchemaQuery)
			if err != nil {
				utils.ErrExit("Failed to run %q on target DB: %s", setSchemaQuery, err)
			}

			log.Infof("Running query %q on the target DB", SET_CLIENT_ENCODING_TO_UTF8)
			_, err = conn.Exec(context.Background(), SET_CLIENT_ENCODING_TO_UTF8)
			if err != nil {
				utils.ErrExit("Failed to run %q on target DB: %s", SET_CLIENT_ENCODING_TO_UTF8, err)
			}
		}

		reCreateSchema := regexp.MustCompile(`(?i)CREATE SCHEMA public`)
		sqlInfoArr := createSqlStrInfoArray(importObjectFilePath, importObjectType)
		for _, sqlInfo := range sqlInfoArr {
			if !(strings.HasPrefix(sqlInfo.stmt, "SET ") || strings.HasPrefix(sqlInfo.stmt, "SELECT ")) {
				if len(sqlInfo.stmt) < 80 {
					fmt.Printf("%s\n", sqlInfo.stmt)
				} else {
					fmt.Printf("%s ...\n", sqlInfo.stmt[:80])
				}
			}
			log.Infof("Execute STATEMENT:\n%s", sqlInfo.formattedStmtStr)
			_, err := conn.Exec(context.Background(), sqlInfo.stmt)
			if err != nil {
				log.Errorf("Previous SQL statement failed with error: %s", err)
				if strings.Contains(err.Error(), "already exists") {
					if !target.IgnoreIfExists && !reCreateSchema.MatchString(sqlInfo.formattedStmtStr) {
						fmt.Printf("\b \n    %s\n", err.Error())
						fmt.Printf("    STATEMENT: %s\n", sqlInfo.formattedStmtStr)
						if !target.ContinueOnError {
							os.Exit(1)
						}
					}
				} else if strings.Contains(err.Error(), "multiple primary keys") {
					if !target.IgnoreIfExists {
						fmt.Printf("\b \n	%s\n", err.Error())
						fmt.Printf("	STATEMENT: %s\n", sqlInfo.formattedStmtStr)
						if !target.ContinueOnError {
							os.Exit(1)
						}
					}
				} else {
					fmt.Printf("\b \n    %s\n", err.Error())
					fmt.Printf("    STATEMENT: %s\n", sqlInfo.formattedStmtStr)
					if !target.ContinueOnError { //default case
						fmt.Println(err)
						os.Exit(1)
					}
					conn.Close(context.Background())
					conn = nil
				}
				log.Infof("Continuing despite error: IgnoreIfExists(%v), ContinueOnError(%v)",
					target.IgnoreIfExists, target.ContinueOnError)
			}
		}
	}
	if conn != nil {
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

func getImportObjectList() []string {
	var finalImportObjectList []string
	excludeObjectList := utils.CsvStringToSlice(target.ExcludeImportObjects)
	for i, item := range excludeObjectList {
		excludeObjectList[i] = strings.ToUpper(item)
	}
	// This list also has defined the order to create object type in target YugabyteDB.
	importObjectOrderList := utils.GetSchemaObjectList(sourceDBType)
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
	return finalImportObjectList
}
