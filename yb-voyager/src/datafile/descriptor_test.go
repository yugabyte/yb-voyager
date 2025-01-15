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

package datafile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
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
		testutils.CompareStructs(t, reflect.TypeOf(FileEntry{}), reflect.TypeOf(expectedFileEntry), "FileEntry")
	})

	t.Run("Validate Descriptor Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(Descriptor{}), reflect.TypeOf(expectedDescriptor), "Descriptor")
	})
}

func TestDescriptorJson(t *testing.T) {
	// Set up the temporary export directory
	exportDir := filepath.Join(os.TempDir(), "descriptor_test")
	outputFilePath := filepath.Join(exportDir, DESCRIPTOR_PATH)

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
	if err := os.MkdirAll(filepath.Join(exportDir, "metainfo"), 0755); err != nil {
		t.Fatalf("Failed to create export directory: %v", err)
	}

	// Clean up the export directory
	defer func() {
		if err := os.RemoveAll(exportDir); err != nil {
			t.Fatalf("Failed to remove export directory: %v", err)
		}
	}()

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
	testutils.CompareJson(t, outputFilePath, expectedJSON, exportDir)
}
