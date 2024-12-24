package dbzm

import (
	"reflect"
	"testing"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestExportStatusStructs(t *testing.T) {
	test := []struct {
		name         string
		actualType   reflect.Type
		expectedType interface{}
	}{
		{
			name:       "Validate TableExportStatus Struct Definition",
			actualType: reflect.TypeOf(TableExportStatus{}),
			expectedType: struct {
				Sno                      int    `json:"sno"`
				DatabaseName             string `json:"database_name"`
				SchemaName               string `json:"schema_name"`
				TableName                string `json:"table_name"`
				FileName                 string `json:"file_name"`
				ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
			}{},
		},
		{
			name:       "Validate ExportStatus Struct Definition",
			actualType: reflect.TypeOf(ExportStatus{}),
			expectedType: struct {
				Mode      string              `json:"mode"`
				Tables    []TableExportStatus `json:"tables"`
				Sequences map[string]int64    `json:"sequences"`
			}{},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CompareStructs(t, tt.actualType, reflect.TypeOf(tt.expectedType), tt.name)
		})
	}
}

// TODO: Implement this test
// The export status json file is created by debezium and currently we dont have infrastructure to test it.
// To test this we need to create a json file (using dbzm code) and read it back (here) and compare the values.
// func TestReadExportStatus(t *testing.T) {
//}
