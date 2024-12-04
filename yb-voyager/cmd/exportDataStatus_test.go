package cmd

import (
	"os"
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func TestExportSnapshotStatusStructs(t *testing.T) {
	// Define the expected structure for TableExportStatus
	expectedTableExportStatus := struct {
		TableName                string `json:"table_name"`
		FileName                 string `json:"file_name"`
		Status                   string `json:"status"`
		ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
	}{}

	// Define the expected structure for ExportSnapshotStatus
	expectedExportSnapshotStatus := struct {
		Tables map[string]*TableExportStatus `json:"tables"`
	}{}

	t.Run("Validate TableExportStatus Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(TableExportStatus{}), reflect.TypeOf(expectedTableExportStatus), "TableExportStatus")
	})

	t.Run("Validate ExportSnapshotStatus Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ExportSnapshotStatus{}), reflect.TypeOf(expectedExportSnapshotStatus), "ExportSnapshotStatus")
	})
}

func TestExportSnapshotStatusJson(t *testing.T) {
	// Create a table list of type []sqlname.NameTuple
	o1 := sqlname.NewObjectName(POSTGRESQL, "public", "public", "table1")
	o2 := sqlname.NewObjectName(POSTGRESQL, "public", "schema1", "table2")
	nameTuple1 := sqlname.NameTuple{CurrentName: o1, SourceName: o1, TargetName: o1}
	nameTuple2 := sqlname.NameTuple{CurrentName: o2, SourceName: o2, TargetName: o2}
	tableList := []sqlname.NameTuple{
		nameTuple1,
		nameTuple2,
	}

	exportDir = os.TempDir() + "/export_snapshot_status_test"

	// Make export directory
	err := os.MkdirAll(exportDir+"/metainfo", 0755)
	if err != nil {
		t.Fatalf("failed to create export directory: %v", err)
	}

	// Call initializeExportTableMetadata to create the export_snapshot_status.json file
	initializeExportTableMetadata(tableList)

	expectedExportSnapshotStatusJSON := `{
  "tables": {
    "public.\"table1\"": {
      "table_name": "public.\"table1\"",
      "file_name": "",
      "status": "NOT-STARTED",
      "exported_row_count_snapshot": 0
    },
    "schema1.\"table2\"": {
      "table_name": "schema1.\"table2\"",
      "file_name": "",
      "status": "NOT-STARTED",
      "exported_row_count_snapshot": 0
    }
  }
}`

	// Compare the JSON representation of the sample ExportSnapshotStatus instance
	utils.CompareJson(t, exportDir+"/metainfo/export_snapshot_status.json", expectedExportSnapshotStatusJSON, exportDir)

	// Clean up the export directory
	err = os.RemoveAll(exportDir)
}
