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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/exportdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var exportSnapshotStatusFile *jsonfile.JsonFile[exportdata.ExportSnapshotStatus]

func initializeExportTableMetadata(tableList []sqlname.NameTuple) {
	getRenamedTableTupleFn := func(table sqlname.NameTuple) (sqlname.NameTuple, bool) {
		return exportdata.GetRenamedTableTuple(metaDB, &source, table)
	}
	tablesProgressMetadata, exportSnapshotStatusFile = exportdata.InitializeExportTableMetadata(
		exportDir, tableList, getRenamedTableTupleFn)
}

func exportDataStatus(ctx context.Context, tablesProgressMetadata map[string]*utils.TableProgressMetadata, quitChan, exportSuccessChan chan bool, disablePb bool) {
	exportdata.ExportDataStatus(ctx, tablesProgressMetadata, quitChan, exportSuccessChan, disablePb,
		source.DB(), source.DBType, exporterRole, controlPlane, migrationUUID, exportSnapshotStatusFile)
}

func isDataLine(line string, sourceDBType string, insideCopyStmt *bool) bool {
	return exportdata.IsDataLine(line, sourceDBType, insideCopyStmt)
}
