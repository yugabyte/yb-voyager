package datafile

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	DESCRIPTOR_PATH = "/metainfo/dataFileDescriptor.json"
)

type FileEntry struct {
	// The in-memory Descriptor MUST always have absolute paths.
	// If the on-disk JSON file has relative file-paths, they are converted to
	// absolute paths when the JSON file is loaded.
	FilePath string `json:"FilePath"`
	// Case sensitive table names and reserved words used as table names are quoted.
	// If the `TableName` doesn't have schema name, it is assumed to be the "public" schema.
	TableName string `json:"TableName"`
	// The number of rows in the file.
	// In case of `import data file` the number of rows in the file is not known upfront.
	// In that case, this field is set to -1.
	RowCount int64 `json:"RowCount"`
	FileSize int64 `json:"FileSize"`
}

type Descriptor struct {
	FileFormat   string       `json:"FileFormat"`
	Delimiter    string       `json:"Delimiter"`
	HasHeader    bool         `json:"HasHeader"`
	ExportDir    string       `json:"-"`
	QuoteChar    byte         `json:"QuoteChar,omitempty"`
	EscapeChar   byte         `json:"EscapeChar,omitempty"`
	DataFileList []*FileEntry `json:"FileList"`
}

func OpenDescriptor(exportDir string) *Descriptor {
	dfd := &Descriptor{
		ExportDir: exportDir,
	}

	filePath := exportDir + DESCRIPTOR_PATH
	log.Infof("loading DataFileDescriptor from %q", filePath)
	dfdJson, err := os.ReadFile(filePath)
	if err != nil {
		utils.ErrExit("load data descriptor file: %v", err)
	}

	err = json.Unmarshal(dfdJson, &dfd)
	if err != nil {
		utils.ErrExit("unmarshal dfd: %v", err)
	}

	// Prefix the export dir to the file paths, if the paths are not absolute.
	for _, fileEntry := range dfd.DataFileList {
		if !path.IsAbs(fileEntry.FilePath) {
			fileEntry.FilePath = path.Join(exportDir, "data", fileEntry.FilePath)
		}
	}
	log.Infof("Parsed DataFileDescriptor: %v", spew.Sdump(dfd))
	return dfd
}

func (dfd *Descriptor) Save() {
	filePath := dfd.ExportDir + DESCRIPTOR_PATH
	log.Infof("storing DataFileDescriptor at %q", filePath)

	bytes, err := json.MarshalIndent(dfd, "", "\t")
	if err != nil {
		utils.ErrExit("marshalling the dfd struct: %v", err)
	}

	err = os.WriteFile(filePath, bytes, 0644)
	if err != nil {
		fmt.Printf("%+v\n", dfd)
		utils.ErrExit("writing DataFileDescriptor: %v", err)
	}
}

func (dfd *Descriptor) GetFileEntry(filePath, tableName string) *FileEntry {
	for _, fileEntry := range dfd.DataFileList {
		if fileEntry.FilePath == filePath && fileEntry.TableName == tableName {
			return fileEntry
		}
	}
	return nil
}
