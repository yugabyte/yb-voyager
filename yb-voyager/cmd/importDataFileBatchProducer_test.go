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
package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type DummyTdb struct {
	tgtdb.TargetYugabyteDB
}

func (t *DummyTdb) MaxBatchSizeInBytes() int64 {
	return 1024
}

func createTempFile(dir string, fileContents string) (string, error) {
	// Create a temporary file
	file, err := os.CreateTemp(dir, "temp-*.txt")
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Write some text to the file
	_, err = file.WriteString(fileContents)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}

func TestBasicFileBatchProducer(t *testing.T) {
	fileContents := `id,val
1, "hello"`

	exportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	assert.NoError(t, err)
	defer os.RemoveAll(fmt.Sprintf("%s/", exportDir))
	dataDir, err := os.MkdirTemp("/tmp", "data-dir-*")
	assert.NoError(t, err)
	defer os.RemoveAll(fmt.Sprintf("%s/", dataDir))
	tempFile, err := createTempFile(dataDir, fileContents)
	assert.NoError(t, err)

	CreateMigrationProjectIfNotExists(constants.POSTGRESQL, exportDir)
	tdb = &DummyTdb{}
	valueConverter = &dbzm.NoOpValueConverter{}
	dataStore = datastore.NewDataStore(dataDir)
	batchSizeInNumRows = 2
	dataFileDescriptor = &datafile.Descriptor{
		FileFormat: "csv",
		Delimiter:  ",",
		HasHeader:  true,
		ExportDir:  exportDir,
		QuoteChar:  '"',
		EscapeChar: '\\',
		NullString: "NULL",
		// DataFileList: []*FileEntry{
		// 	{
		// 		FilePath:  "file.csv", // Use relative path for testing absolute path handling.
		// 		TableName: "public.my_table",
		// 		RowCount:  100,
		// 		FileSize:  2048,
		// 	},
		// },
		// TableNameToExportedColumns: map[string][]string{
		// 	"public.my_table": {"id", "name", "age"},
		// },
	}

	sourceName := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", "test_table")
	tableNameTup := sqlname.NameTuple{SourceName: sourceName, CurrentName: sourceName}
	task := &ImportFileTask{
		ID:           1,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     1,
	}

	state := NewImportDataState(exportDir)

	batchproducer, err := NewFileBatchProducer(task, state)
	assert.NoError(t, err)

	assert.False(t, batchproducer.Done())

	batch, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount)
}
