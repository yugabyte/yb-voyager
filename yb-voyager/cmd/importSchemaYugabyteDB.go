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
package cmd

import (
	"context"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
)

var defferedSqlStmts []sqlInfo
var failedSqlStmts []string

func importSchemaInternal(exportDir string, importObjectList []string,
	skipFn func(string, string) bool) {
	schemaDir := filepath.Join(exportDir, "schema")
	for _, importObjectType := range importObjectList {
		importObjectFilePath := utils.GetObjectFilePath(schemaDir, importObjectType)
		if !utils.FileOrFolderExists(importObjectFilePath) {
			continue
		}
		executeSqlFile(importObjectFilePath, importObjectType, skipFn)
	}

}

/*
Try re-executing each DDL from deffered list.
If fails, silently avoid the error.
Else remove from defferedSQLStmts list
At the end, add the unsuccessful ones to a failedSqlStmts list and report to the user
*/
func importDefferedStatements() {
	if len(defferedSqlStmts) == 0 {
		return
	}
	log.Infof("Number of statements in defferedSQLStmts list: %d\n", len(defferedSqlStmts))

	utils.PrintAndLog("\nExecuting the remaining SQL statements...\n\n")
	maxIterations := len(defferedSqlStmts)
	conn := newTargetConn()
	defer func() { conn.Close(context.Background()) }()

	var err error
	// max loop iterations to remove all errors
	for i := 1; i <= maxIterations && len(defferedSqlStmts) > 0; i++ {
		for j := 0; j < len(defferedSqlStmts); {
			_, err = conn.Exec(context.Background(), defferedSqlStmts[j].formattedStmt)
			if err == nil {
				utils.PrintAndLog("%s\n", utils.GetSqlStmtToPrint(defferedSqlStmts[j].stmt))
				// removing successfully executed SQL
				defferedSqlStmts = append(defferedSqlStmts[:j], defferedSqlStmts[j+1:]...)
				break // no increment in j
			} else {
				log.Infof("failed retry of deffered stmt: %s\n%v", utils.GetSqlStmtToPrint(defferedSqlStmts[j].stmt), err)
				// fails to execute in final attempt
				if i == maxIterations {
					errString := "/*\n" + err.Error() + "\n*/\n"
					failedSqlStmts = append(failedSqlStmts, errString+defferedSqlStmts[j].formattedStmt)
				}
				conn.Close(context.Background())
				conn = newTargetConn()
				j++
			}
		}
	}
}

func applySchemaObjectFilterFlags(importObjectOrderList []string) []string {
	var finalImportObjectList []string
	excludeObjectList := utils.CsvStringToSlice(tconf.ExcludeImportObjects)
	for i, item := range excludeObjectList {
		excludeObjectList[i] = strings.ToUpper(item)
	}
	if tconf.ImportObjects != "" {
		includeObjectList := utils.CsvStringToSlice(tconf.ImportObjects)
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
	if sourceDBType == "postgresql" && !slices.Contains(finalImportObjectList, "SCHEMA") && !bool(flagPostImportData) { // Schema should be migrated by default.
		finalImportObjectList = append([]string{"SCHEMA"}, finalImportObjectList...)
	}
	if !flagPostImportData {
		finalImportObjectList = append(finalImportObjectList, []string{"UNIQUE INDEX"}...)
	}
	return finalImportObjectList
}
