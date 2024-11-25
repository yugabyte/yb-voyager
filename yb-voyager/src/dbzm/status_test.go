package dbzm

import (
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestExportStatusStructures(t *testing.T) {
	// Define the expected structure for TableExportStatus
	expectedTableExportStatus := struct {
		Sno                      int    `json:"sno"`
		DatabaseName             string `json:"database_name"`
		SchemaName               string `json:"schema_name"`
		TableName                string `json:"table_name"`
		FileName                 string `json:"file_name"`
		ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
	}{}

	// Define the expected structure for ExportStatus
	expectedExportStatus := struct {
		Mode      string              `json:"mode"`
		Tables    []TableExportStatus `json:"tables"`
		Sequences map[string]int64    `json:"sequences"`
	}{}

	t.Run("Check TableExportStatus structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(TableExportStatus{}), reflect.TypeOf(expectedTableExportStatus), "TableExportStatus")
	})

	t.Run("Check ExportStatus structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(ExportStatus{}), reflect.TypeOf(expectedExportStatus), "ExportStatus")
	})
}
