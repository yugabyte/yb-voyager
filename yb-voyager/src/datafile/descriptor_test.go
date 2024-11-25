package datafile

import (
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestFileEntryAndDescriptorStructures(t *testing.T) {
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

	t.Run("Check FileEntry structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(FileEntry{}), reflect.TypeOf(expectedFileEntry), "FileEntry")
	})

	t.Run("Check Descriptor structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(Descriptor{}), reflect.TypeOf(expectedDescriptor), "Descriptor")
	})
}
