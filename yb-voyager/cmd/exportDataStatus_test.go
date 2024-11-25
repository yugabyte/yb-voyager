package cmd

import (
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestExportSnapshotStatusStructures(t *testing.T) {
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

	t.Run("Check TableExportStatus structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(TableExportStatus{}), reflect.TypeOf(expectedTableExportStatus), "TableExportStatus")
	})

	t.Run("Check ExportSnapshotStatus structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(ExportSnapshotStatus{}), reflect.TypeOf(expectedExportSnapshotStatus), "ExportSnapshotStatus")
	})
}
