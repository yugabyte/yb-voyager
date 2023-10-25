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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

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
	FileFormat                 string              `json:"FileFormat"`
	Delimiter                  string              `json:"Delimiter"`
	HasHeader                  bool                `json:"HasHeader"`
	ExportDir                  string              `json:"-"`
	QuoteChar                  byte                `json:"QuoteChar,omitempty"`
	EscapeChar                 byte                `json:"EscapeChar,omitempty"`
	NullString                 string              `json:"NullString,omitempty"`
	DataFileList               []*FileEntry        `json:"FileList"`
	TableNameToExportedColumns map[string][]string `json:"TableNameToExportedColumns"`
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
		isAbs := path.IsAbs(fileEntry.FilePath) ||
			strings.HasPrefix(fileEntry.FilePath, "s3://") || // AWS.
			strings.HasPrefix(fileEntry.FilePath, "gs://") || // GCP.
			strings.HasPrefix(fileEntry.FilePath, "https://") // Azure.
		if !isAbs {
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

func (dfd *Descriptor) GetDataFileEntryByTableName(tableName string) *FileEntry {
	for _, fileEntry := range dfd.DataFileList {
		if fileEntry.TableName == tableName {
			return fileEntry
		}
	}
	return nil
}
