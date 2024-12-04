package dbzm

import (
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestExportStatusStructs(t *testing.T) {
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

	t.Run("Validate TableExportStatus Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(TableExportStatus{}), reflect.TypeOf(expectedTableExportStatus), "TableExportStatus")
	})

	t.Run("Validate ExportStatus Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ExportStatus{}), reflect.TypeOf(expectedExportStatus), "ExportStatus")
	})
}

// TODO: Implement this test
// The export status json file is created by debezium and currently we dont have infrastructure to test it.
// To test this we need to create a json file (using dbzm code) and read it back (here) and compare the values.
// func TestReadExportStatus(t *testing.T) {
//}
