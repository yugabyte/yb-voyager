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

type dummyTDB struct {
	maxSizeBytes int64
	tgtdb.TargetYugabyteDB
}

func (d *dummyTDB) MaxBatchSizeInBytes() int64 {
	return d.maxSizeBytes
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

func setupDependenciesForTest(batchSizeRows int64, batchSizeBytes int64) (string, string, *ImportDataState, error) {
	lexportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	if err != nil {
		return "", "", nil, err
	}

	ldataDir, err := os.MkdirTemp("/tmp", "data-dir-*")
	if err != nil {
		return "", "", nil, err
	}

	CreateMigrationProjectIfNotExists(constants.POSTGRESQL, lexportDir)
	tdb = &dummyTDB{maxSizeBytes: batchSizeBytes}
	valueConverter = &dbzm.NoOpValueConverter{}
	dataStore = datastore.NewDataStore(ldataDir)

	batchSizeInNumRows = batchSizeRows

	state := NewImportDataState(lexportDir)
	return ldataDir, lexportDir, state, nil
}

func setupFileForTest(lexportDir string, fileContents string, dir string, tableName string) (string, *ImportFileTask, error) {
	dataFileDescriptor = &datafile.Descriptor{
		FileFormat: "csv",
		Delimiter:  ",",
		HasHeader:  true,
		ExportDir:  lexportDir,
		QuoteChar:  '"',
		EscapeChar: '\\',
		NullString: "NULL",
	}
	tempFile, err := createTempFile(dir, fileContents)
	if err != nil {
		return "", nil, err
	}

	sourceName := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", tableName)
	tableNameTup := sqlname.NameTuple{SourceName: sourceName, CurrentName: sourceName}
	task := &ImportFileTask{
		ID:           1,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     1,
	}
	return tempFile, task, nil
}

func TestBasicFileBatchProducer(t *testing.T) {
	ldataDir, lexportDir, state, err := setupDependenciesForTest(2, 1024)
	assert.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	fileContents := `id,val
1, "hello"`
	_, task, err := setupFileForTest(lexportDir, fileContents, ldataDir, "test_table")
	assert.NoError(t, err)

	batchproducer, err := NewFileBatchProducer(task, state)
	assert.NoError(t, err)

	assert.False(t, batchproducer.Done())

	batch, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount)
	assert.True(t, batchproducer.Done())
}

func TestFileBatchProducerBasedOnRowsThreshold(t *testing.T) {
	// max batch size in rows is 2
	ldataDir, lexportDir, state, err := setupDependenciesForTest(2, 1024)
	assert.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"`
	_, task, err := setupFileForTest(lexportDir, fileContents, ldataDir, "test_table")
	assert.NoError(t, err)

	batchproducer, err := NewFileBatchProducer(task, state)
	assert.NoError(t, err)

	assert.False(t, batchproducer.Done())

	var batches []*Batch
	for !batchproducer.Done() {
		batch, err := batchproducer.NextBatch()
		assert.NoError(t, err)
		assert.NotNil(t, batch)
		batches = append(batches, batch)
	}

	// 2 batches should be produced
	assert.Equal(t, 2, len(batches))
	// each of length 2
	assert.Equal(t, int64(2), batches[0].RecordCount)
	batchContents, err := os.ReadFile(batches[0].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, "id,val\n1, \"hello\"\n2, \"world\"", string(batchContents))

	assert.Equal(t, int64(2), batches[1].RecordCount)
	batchContents, err = os.ReadFile(batches[1].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, "id,val\n3, \"foo\"\n4, \"bar\"", string(batchContents))
}

func TestFileBatchProducerBasedOnSizeThreshold(t *testing.T) {
	// max batch size in size is 25 bytes
	ldataDir, lexportDir, state, err := setupDependenciesForTest(1000, 25)
	assert.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// each row exccept header is 10 bytes
	fileContents := `id,val
1, "abcde"
2, "ghijk"
3, "mnopq"
4, "stuvw"`
	_, task, err := setupFileForTest(lexportDir, fileContents, ldataDir, "test_table")
	assert.NoError(t, err)

	batchproducer, err := NewFileBatchProducer(task, state)
	assert.NoError(t, err)

	assert.False(t, batchproducer.Done())

	var batches []*Batch
	for !batchproducer.Done() {
		batch, err := batchproducer.NextBatch()
		assert.NoError(t, err)
		assert.NotNil(t, batch)
		batches = append(batches, batch)
	}

	// 3 batches should be produced
	// while calculating for the first batch, the header is also considered
	assert.Equal(t, 3, len(batches))
	// each of length 2
	assert.Equal(t, int64(1), batches[0].RecordCount)
	batchContents, err := os.ReadFile(batches[0].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, "id,val\n1, \"abcde\"", string(batchContents))

	assert.Equal(t, int64(2), batches[1].RecordCount)
	batchContents, err = os.ReadFile(batches[1].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, "id,val\n2, \"ghijk\"\n3, \"mnopq\"", string(batchContents))

	assert.Equal(t, int64(1), batches[2].RecordCount)
	batchContents, err = os.ReadFile(batches[2].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, "id,val\n4, \"stuvw\"", string(batchContents))
}

func TestFileBatchProducerThrowsErrorWhenSingleRowGreaterThanMaxBatchSize(t *testing.T) {
	// max batch size in size is 25 bytes
	ldataDir, lexportDir, state, err := setupDependenciesForTest(1000, 25)
	assert.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// 3rd row is greater than max batch size
	fileContents := `id,val
1, "abcdef"
2, "ghijk"
3, "mnopq1234567899876543"
4, "stuvw"`
	_, task, err := setupFileForTest(lexportDir, fileContents, ldataDir, "test_table")
	assert.NoError(t, err)

	batchproducer, err := NewFileBatchProducer(task, state)
	assert.NoError(t, err)

	assert.False(t, batchproducer.Done())

	// 1st batch is fine.
	batch, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount)

	// 2nd batch should throw error
	_, err = batchproducer.NextBatch()
	assert.ErrorContains(t, err, "larger than max batch size")
}
