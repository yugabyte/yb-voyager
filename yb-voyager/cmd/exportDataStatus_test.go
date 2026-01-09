//go:build unit

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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestExportSnapshotStatusStructs(t *testing.T) {

	test := []struct {
		name         string
		actualType   reflect.Type
		expectedType interface{}
	}{
		{
			name:       "Validate TableExportStatus Struct Definition",
			actualType: reflect.TypeOf(TableExportStatus{}),
			expectedType: struct {
				TableName                string `json:"table_name"`
				FileName                 string `json:"file_name"`
				Status                   string `json:"status"`
				ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
			}{},
		},
		{
			name:       "Validate ExportSnapshotStatus Struct Definition",
			actualType: reflect.TypeOf(ExportSnapshotStatus{}),
			expectedType: struct {
				Tables map[string]*TableExportStatus `json:"tables"`
			}{},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CompareStructs(t, tt.actualType, reflect.TypeOf(tt.expectedType), tt.name)
		})
	}
}

func TestExportSnapshotStatusJson(t *testing.T) {
	// Create a table list of type []sqlname.NameTuple
	o1 := sqlname.NewObjectName(POSTGRESQL, "public", "public", "table1")
	o2 := sqlname.NewObjectName(POSTGRESQL, "public", "schema1", "table2")
	tableList := []sqlname.NameTuple{
		{CurrentName: o1, SourceName: o1, TargetName: o1},
		{CurrentName: o2, SourceName: o2, TargetName: o2},
	}

	exportDir = filepath.Join(os.TempDir(), "export_snapshot_status_test")
	// Make export directory
	err := os.MkdirAll(filepath.Join(exportDir, "metainfo"), 0755)
	if err != nil {
		t.Fatalf("failed to create export directory: %v", err)
	}

	// Clean up the export directory
	defer func() {
		err := os.RemoveAll(exportDir)
		if err != nil {
			t.Fatalf("failed to remove export directory: %v", err)
		}
	}()

	outputFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")

	// Call initializeExportTableMetadata to create the export_snapshot_status.json file
	initializeExportTableMetadata(tableList)

	expectedExportSnapshotStatusJSON := `{
  "tables": {
    "\"public\".\"table1\"": {
      "table_name": "\"public\".\"table1\"",
      "file_name": "",
      "status": "NOT-STARTED",
      "exported_row_count_snapshot": 0
    },
    "\"schema1\".\"table2\"": {
      "table_name": "\"schema1\".\"table2\"",
      "file_name": "",
      "status": "NOT-STARTED",
      "exported_row_count_snapshot": 0
    }
  }
}`

	// Compare the JSON representation of the sample ExportSnapshotStatus instance
	testutils.CompareJson(t, outputFilePath, expectedExportSnapshotStatusJSON, exportDir)
}
