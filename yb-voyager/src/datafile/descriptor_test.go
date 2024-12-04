package datafile

import (
	"os"
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestDescriptorStructs(t *testing.T) {
	// Define the expected structure for FileEntry
	expectedFileEntry := struct {
		FilePath  string `json:"FilePath"`
		TableName string `json:"TableName"`
		RowCount  int64  `json:"RowCount"`
		FileSize  int64  `json:"FileSize"`
	}{}

	// Define the expected structure for Descriptor
	expectedDescriptor := struct {
		FileFormat                 string              `json:"FileFormat"`
		Delimiter                  string              `json:"Delimiter"`
		HasHeader                  bool                `json:"HasHeader"`
		ExportDir                  string              `json:"-"`
		QuoteChar                  byte                `json:"QuoteChar,omitempty"`
		EscapeChar                 byte                `json:"EscapeChar,omitempty"`
		NullString                 string              `json:"NullString,omitempty"`
		DataFileList               []*FileEntry        `json:"FileList"`
		TableNameToExportedColumns map[string][]string `json:"TableNameToExportedColumns"`
	}{}

	t.Run("Validate FileEntry Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(FileEntry{}), reflect.TypeOf(expectedFileEntry), "FileEntry")
	})

	t.Run("Validate Descriptor Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(Descriptor{}), reflect.TypeOf(expectedDescriptor), "Descriptor")
	})
}

func TestDescriptorJson(t *testing.T) {
	// Set up the temporary export directory
	exportDir := os.TempDir() + "/descriptor_test"
	outputFilePath := exportDir + DESCRIPTOR_PATH

	// Create a sample Descriptor instance
	descriptor := Descriptor{
		FileFormat: "csv",
		Delimiter:  ",",
		HasHeader:  true,
		ExportDir:  exportDir,
		QuoteChar:  '"',
		EscapeChar: '\\',
		NullString: "NULL",
		DataFileList: []*FileEntry{
			{
				FilePath:  "file.csv", // Use relative path for testing absolute path handling.
				TableName: "public.my_table",
				RowCount:  100,
				FileSize:  2048,
			},
		},
		TableNameToExportedColumns: map[string][]string{
			"public.my_table": {"id", "name", "age"},
		},
	}

	// Ensure the export directory exists
	if err := os.MkdirAll(exportDir+"/metainfo", 0755); err != nil {
		t.Fatalf("Failed to create export directory: %v", err)
	}

	// Save the Descriptor to JSON
	descriptor.Save()

	expectedJSON := `{
	"FileFormat": "csv",
	"Delimiter": ",",
	"HasHeader": true,
	"QuoteChar": 34,
	"EscapeChar": 92,
	"NullString": "NULL",
	"FileList": [
		{
			"FilePath": "file.csv",
			"TableName": "public.my_table",
			"RowCount": 100,
			"FileSize": 2048
		}
	],
	"TableNameToExportedColumns": {
		"public.my_table": [
			"id",
			"name",
			"age"
		]
	}
}`

	// Compare the output JSON with the expected JSON
	utils.CompareJson(t, outputFilePath, expectedJSON, exportDir)
}
