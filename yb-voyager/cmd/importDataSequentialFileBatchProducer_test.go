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
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// mockValueConverterForTest implements SnapshotPhaseValueConverter and always returns an error for ConvertRow.
type mockValueConverterForTest struct{}

func (m *mockValueConverterForTest) ConvertRow(tableNameTup sqlname.NameTuple, columnNames []string, columnValues []string) error {
	return fmt.Errorf("mock conversion error")
}
func (m *mockValueConverterForTest) GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) {
	return nil, nil
}

// Helper mock for row-specific error
// Returns a conversion error for a specific row number (rowToError)
type mockRowErrorValueConverter struct {
	rowToError int
}

func (m *mockRowErrorValueConverter) ConvertRow(tableNameTup sqlname.NameTuple, columnNames []string, columnValues []string) error {
	if m.rowToError > 0 && len(columnValues) > 0 {
		// Extract row number from first column value (assuming format: N, ...)
		if len(columnValues[0]) > 0 && columnValues[0][0] >= '1' && columnValues[0][0] <= '9' {
			if int(columnValues[0][0]-'0') == m.rowToError {
				return fmt.Errorf("mock conversion error")
			}
		}
	}
	return nil
}
func (m *mockRowErrorValueConverter) GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) {
	return nil, nil
}

// In cmd package, we have 3 TestMain functions, one for each build tag(unit, integration, integration_voyager_command).
func TestMain(m *testing.M) {
	// set logging level to WARN
	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	os.Exit(exitCode)
}

func TestBasicFileBatchProducer(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	assert.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	fileContents := `id,val
1, "hello"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)
	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
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
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
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
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
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

	batch1ExpectedContents := "id,val\n1, \"hello\"\n2, \"world\""
	assert.Equal(t, int64(2), batches[0].RecordCount)
	batchContents, err := os.ReadFile(batches[0].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, batch1ExpectedContents, string(batchContents))

	batch2ExpectedContents := "id,val\n3, \"foo\"\n4, \"bar\""
	assert.Equal(t, int64(2), batches[1].RecordCount)
	batchContents, err = os.ReadFile(batches[1].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, batch2ExpectedContents, string(batchContents))
}

func TestFileBatchProducerBasedOnSizeThreshold(t *testing.T) {
	// max batch size in size is 25 bytes
	maxBatchSizeBytes := int64(25)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
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
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)

	assert.False(t, batchproducer.Done())

	var batches []*Batch
	for !batchproducer.Done() {
		batch, err := batchproducer.NextBatch()
		assert.NoError(t, err)
		assert.NotNil(t, batch)
		batches = append(batches, batch)
	}

	// 4 batches should be produced
	// while calculating for the batches, the header is also considered
	assert.Equal(t, 4, len(batches))

	batch1ExpectedContents := "id,val\n1, \"abcde\""
	assert.Equal(t, int64(1), batches[0].RecordCount)
	assert.LessOrEqual(t, batches[0].ByteCount, maxBatchSizeBytes)
	batchContents, err := os.ReadFile(batches[0].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, batch1ExpectedContents, string(batchContents))

	batch2ExpectedContents := "id,val\n2, \"ghijk\""
	assert.Equal(t, int64(1), batches[1].RecordCount)
	assert.LessOrEqual(t, batches[1].ByteCount, maxBatchSizeBytes)
	batchContents, err = os.ReadFile(batches[1].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, batch2ExpectedContents, string(batchContents))

	batch3ExpectedContents := "id,val\n3, \"mnopq\""
	assert.Equal(t, int64(1), batches[2].RecordCount)
	assert.LessOrEqual(t, batches[2].ByteCount, maxBatchSizeBytes)
	batchContents, err = os.ReadFile(batches[2].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, batch3ExpectedContents, string(batchContents))

	batch4ExpectedContents := "id,val\n4, \"stuvw\""
	assert.Equal(t, int64(1), batches[3].RecordCount)
	assert.LessOrEqual(t, batches[3].ByteCount, maxBatchSizeBytes)
	batchContents, err = os.ReadFile(batches[3].GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, batch4ExpectedContents, string(batchContents))
}

func TestFileBatchProducerThrowsErrorWhenSingleRowGreaterThanMaxBatchSize(t *testing.T) {
	// max batch size in size is 25 bytes
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(1000, 25)
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
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
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

func TestFileBatchProducerResumable(t *testing.T) {
	// max batch size in rows is 2
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
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
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// generate one batch
	batch1, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1)
	assert.Equal(t, int64(2), batch1.RecordCount)
	assert.Greater(t, batch1.CumByteOffsetEnd, int64(0),
		"Batch 1 should have positive cumByteOffset")

	// simulate a crash and recover
	batchproducer, err = NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// state should have recovered that one batch
	assert.Equal(t, 1, len(batchproducer.pendingBatches))
	assert.True(t, cmp.Equal(batch1, batchproducer.pendingBatches[0]))
	assert.Equal(t, batch1.CumByteOffsetEnd, batchproducer.lastBatchCumByteOffsetEnd)
	assert.Equal(t, batch1.CumByteOffsetEnd, batchproducer.cumByteOffsetEnd)

	// verify that it picks up from pendingBatches
	// instead of procing a new batch.
	batch1Recovered, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1Recovered)
	assert.True(t, cmp.Equal(batch1, batch1Recovered))
	assert.Equal(t, 0, len(batchproducer.pendingBatches))
	assert.False(t, batchproducer.Done())

	// get final batch via byte-seek
	batch2, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch2)
	assert.Equal(t, int64(2), batch2.RecordCount)
	assert.True(t, batchproducer.Done())

	assert.Greater(t, batch2.CumByteOffsetEnd, batch1.CumByteOffsetEnd,
		"Batch 2 cumByteOffset should be greater than batch 1")
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch2.CumByteOffsetEnd,
		"Last batch cumByteOffset should equal file size")
}

func TestFileBatchProducerResumeAfterAllBatchesProduced(t *testing.T) {
	// max batch size in rows is 2
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
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
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// generate all batches
	batches := []*Batch{}
	for !batchproducer.Done() {
		batch, err := batchproducer.NextBatch()
		assert.NoError(t, err)
		assert.NotNil(t, batch)
		batches = append(batches, batch)
	}

	// simulate a crash and recover
	batchproducer, err = NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// state should have recovered two batches
	assert.Equal(t, 2, len(batchproducer.pendingBatches))

	// verify that it picks up from pendingBatches
	// instead of procing a new batch.
	recoveredBatches := []*Batch{}
	for !batchproducer.Done() {
		batch, err := batchproducer.NextBatch()
		assert.NoError(t, err)
		assert.NotNil(t, batch)
		recoveredBatches = append(recoveredBatches, batch)
	}
	assert.Equal(t, len(batches), len(recoveredBatches))
	assert.ElementsMatch(t, batches, recoveredBatches)
}

func getErrorsParentDir(lexportDir string) string {
	return filepath.Join(lexportDir, "data")
}

func TestFileBatchProducer_StashAndContinue_ConversionError(t *testing.T) {
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	fileContents := `id,val
1,hello
2,world`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	// Swap in the mock valueConverter
	origValueConverter := valueConverter
	valueConverter = &mockValueConverterForTest{}
	t.Cleanup(func() { valueConverter = origValueConverter })

	batchproducer, err := NewSequentialFileBatchProducer(task, state, true, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := batchproducer.NextBatch()
	// Should not return an error,
	assert.NoError(t, err)
	assert.Equal(t, int64(0), batch.RecordCount)
	// all errors are stashed, so the last batch that is created should have 0 records
	assert.Equal(t, int64(0), (batch.Number))
	assert.Equal(t, true, batchproducer.Done())

	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 2, 15,
		"ERROR: transforming line number=1",
		"mock conversion error",
		"ROW: 1,hello",
		"ERROR: transforming line number=2",
		"ROW: 2,world",
	)
}

func TestFileBatchProducer_AbortHandler_ConversionError(t *testing.T) {
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	abortErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.AbortErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	fileContents := `id,val
1,hello
2,world`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	// Swap in the mock valueConverter
	origValueConverter := valueConverter
	valueConverter = &mockValueConverterForTest{}
	t.Cleanup(func() { valueConverter = origValueConverter })

	batchproducer, err := NewSequentialFileBatchProducer(task, state, true, abortErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := batchproducer.NextBatch()
	// Should return an error due to abort policy
	assert.Error(t, err)
	assert.Nil(t, batch)
	assert.Contains(t, err.Error(), "mock conversion error")
}

func TestFileBatchProducer_StashAndContinue_RowTooLargeError(t *testing.T) {
	// Set max batch size in bytes to a small value to trigger the row-too-large error
	maxBatchSizeBytes := int64(20) // deliberately small to trigger error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The second row will be too large for the batch size
	fileContents := `id,val
1, "hello"
2, "this row is too long and should trigger an error because it exceeds the max batch size"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := batchproducer.NextBatch()
	// Should not return an error, but the batch should only contain the first row
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount) // second row should be skipped due to error

	// assertProcessingErrorFileContains(t, lexportDir, task,
	// 	"larger than max batch size",
	// 	"ROW: 2, \"this row is too long and should trigger an error because it exceeds the max batch size\"")

	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 1, 91,
		"larger than max batch size",
		"ROW: 2, \"this row is too long and should trigger an error because it exceeds the max batch size\"")
}

func TestFileBatchProducer_StashAndContinue_RowTooLargeError_FirstRow(t *testing.T) {
	// Set max batch size in bytes to a small value to trigger the row-too-large error
	maxBatchSizeBytes := int64(20) // deliberately small to trigger error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The first row will be too large for the batch size
	fileContents := `id,val
1, "this row is too long and should trigger an error because it exceeds the max batch size"
2, "hello"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := batchproducer.NextBatch()
	// Should not return an error, but the batch should only contain the second row
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount) // first row should be skipped due to error

	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 1, 92,
		"larger than max batch size",
		"ROW: 1, \"this row is too long and should trigger an error because it exceeds the max batch size\"")
}

func TestFileBatchProducer_StashAndContinue_RowTooLargeError_FirstFiveRows(t *testing.T) {
	// Set max batch size in bytes to a small value to trigger the row-too-large error
	maxBatchSizeBytes := int64(20) // deliberately small to trigger error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The first 5 rows will be too large for the batch size
	fileContents := `id,val
1, "this row is too long and should trigger an error because it exceeds the max batch size"
2, "this row is too long and should trigger an error because it exceeds the max batch size"
3, "this row is too long and should trigger an error because it exceeds the max batch size"
4, "this row is too long and should trigger an error because it exceeds the max batch size"
5, "this row is too long and should trigger an error because it exceeds the max batch size"
6, "hello"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := batchproducer.NextBatch()
	// Should not return an error, but the batch should only contain the last 1 rows
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount) // first 5 rows should be skipped due to error

	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 5, 460, // 5 rows * 92 bytes each = 460 bytes
		"larger than max batch size",
		"ROW: 1, \"this row is too long and should trigger an error because it exceeds the max batch size\"",
		"ROW: 2, \"this row is too long and should trigger an error because it exceeds the max batch size\"",
		"ROW: 3, \"this row is too long and should trigger an error because it exceeds the max batch size\"",
		"ROW: 4, \"this row is too long and should trigger an error because it exceeds the max batch size\"",
		"ROW: 5, \"this row is too long and should trigger an error because it exceeds the max batch size\"")
}

func TestFileBatchProducer_StashAndContinue_RowTooLargeError_LastBatch(t *testing.T) {
	// Set max batch size in bytes to a small value to trigger the row-too-large error
	maxBatchSizeBytes := int64(20) // deliberately small to trigger error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// First batch: small rows that fit within 20 bytes
	// Second batch: large rows that exceed the batch size
	fileContents := `id,val
1,a
2,b
3,c
4,d
5,e
6, "this row is too long and should trigger an error because it exceeds the max batch size"
7, "this row is too long and should trigger an error because it exceeds the max batch size"
8, "this row is too long and should trigger an error because it exceeds the max batch size"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	// First batch should contain the first 3 small rows
	batch, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(3), batch.RecordCount) // first 3 rows should be processed successfully
	assertNoProcessingErrorBatchFileExists(t, lexportDir, task, batch.Number)

	// Second batch will contain all remaining rows 4,5, excluding the large rows.
	batch, err = batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(2), batch.RecordCount)
	assert.Equal(t, true, batchproducer.Done())

	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 3, 275, // 3 rows * 92 bytes each = 276 bytes - 1 byte for no new-line at the end of the file
		"larger than max batch size",
		"ROW: 6, \"this row is too long and should trigger an error because it exceeds the max batch size\"",
		"ROW: 7, \"this row is too long and should trigger an error because it exceeds the max batch size\"",
		"ROW: 8, \"this row is too long and should trigger an error because it exceeds the max batch size\"")

}

func TestFileBatchProducer_StashAndContinue_RowTooLargeError_processingErrorFileRewrittenOnResumption(t *testing.T) {
	// Set max batch size in bytes to a small value to trigger the row-too-large error
	maxBatchSizeBytes := int64(20) // deliberately small to trigger error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The second row will be too large for the batch size
	fileContents := `id,val
1, "hello"
2, "this row is too long and should trigger an error because it exceeds the max batch size"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := batchproducer.NextBatch()
	// Should not return an error, but the batch should only contain the first row
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount) // second row should be skipped due to error

	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 1, 91,
		"larger than max batch size",
		"ROW: 2, \"this row is too long and should trigger an error because it exceeds the max batch size\"")

	// simulate crash and recover
	// delete the batch file only and not the processing error batch file.
	err = os.Remove(batch.GetFilePath())
	assert.NoError(t, err)

	batchproducer, err = NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// regenerate the batch.
	batch, err = batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(1), batch.RecordCount)

	//processing error file should be rewritten without any issue.
	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 1, 91,
		"larger than max batch size",
		"ROW: 2, \"this row is too long and should trigger an error because it exceeds the max batch size\"")
}

func TestFileBatchProducer_StashAndContinue_RowTooLargeErrorDoesNotCountTowardsBatchSize(t *testing.T) {
	// Set max batch size in bytes to a value that would only allow two small rows, but the second row will be too large and should not count towards the batch size
	maxBatchSizeBytes := int64(25) // enough for header + one small row + one more if no error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The second row will be too large for the batch size, but the third row is small enough to fit if the second is skipped
	fileContents := `id,val
1, "ok"
2, "this row is way too long to fit in the batch and should be skipped"
3, "ok2"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	// First batch: should contain row 1 and row 3 (row 2 is too large and skipped, its size does not count)
	batch, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(2), batch.RecordCount)
	assert.Equal(t, int64(0), batch.Number) // last batch

	// Error file should contain the error for row 2
	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 1, 72,
		"larger than max batch size",
		"ROW: 2, \"this row is way too long to fit in the batch and should be skipped\"",
	)
}

func TestFileBatchProducer_StashAndContinue_ConversionErrorDoesNotCountTowardsBatchSize(t *testing.T) {
	// Set max batch size in bytes to a value that would only allow two small rows, but the second row will error (conversion error) and should not count towards the batch size
	maxBatchSizeBytes := int64(25) // enough for header + one small row + one more if no error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The second row will error (conversion error), but is large enough that if it counted, the third row would not fit
	fileContents := `id,val
1,ok
2,errorrow
3,ok2`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	// Use a mock valueConverter that errors on row 2
	origValueConverter := valueConverter
	valueConverter = &mockRowErrorValueConverter{rowToError: 2}
	t.Cleanup(func() { valueConverter = origValueConverter })

	batchproducer, err := NewSequentialFileBatchProducer(task, state, true, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	// First batch: should contain row 1 and row 3 (row 2 is errored and skipped, its size does not count)
	batch, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	assert.Equal(t, int64(2), batch.RecordCount)

	// Error file should contain the error for row 2
	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 1, 11,
		"ERROR: transforming line number=2",
		"mock conversion error",
		"ROW: 2,errorrow",
	)
}

func TestFileBatchProducer_StashAndContinue_Resumption(t *testing.T) {
	// Set max batch size in bytes to a small value to trigger the row-too-large error
	maxBatchSizeBytes := int64(20) // deliberately small to trigger error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The second row will be too large for the batch size
	fileContents := `id,val
1, "hello"
2, "this row is too long and should trigger an error because it exceeds the max batch size"
3, "another"
4, "this row is also too long and should trigger an error because it exceeds the max batch size"
5, "final"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	// first batch will contain the first row only
	batch1, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1)
	assert.Equal(t, int64(1), batch1.RecordCount)

	// error file should have the error for 2nd row.
	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch1.Number, 1, 92,
		"larger than max batch size",
		"ROW: 2, \"this row is too long and should trigger an error because it exceeds the max batch size\"")

	// simulate a crash and recover
	batchproducer, err = NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())
	// get the batch1 again, because they would still be pending.
	assert.Equal(t, 1, len(batchproducer.pendingBatches))
	batch1Recovered, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1Recovered)
	assert.Equal(t, batch1, batch1Recovered)

	// second batch will contain the third row only, and stash the 4th row
	batch2, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch2)
	assert.Equal(t, int64(1), batch2.RecordCount)

	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch2.Number, 1, 97,
		"larger than max batch size",
		"ROW: 4, \"this row is also too long and should trigger an error because it exceeds the max batch size\"")

	// third batch should be produced, which will contain the row 5
	batch3, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch3)
	assert.Equal(t, int64(1), batch3.RecordCount)

	// batch producer should be done
	assert.True(t, batchproducer.Done())
}

func TestFileBatchProducer_StashAndContinue_MultipleTasksSameTable(t *testing.T) {
	// Set max batch size in bytes to a value that would only allow two small rows, but the second row will error (conversion error) and should not count towards the batch size
	maxBatchSizeBytes := int64(25) // enough for header + one small row + one more if no error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	testutils.FatalIfError(t, err)
	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)

	// The second row will error (conversion error), but is large enough that if it counted, the third row would not fit
	fileContents1 := `id,val
1,ok
2,errorrow
3,ok2`
	_, task1, err := createFileAndTask(lexportDir, fileContents1, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	fileContents2 := `id,val
1,ok
2,errorrow
3,ok2`
	_, task2, err := createFileAndTask(lexportDir, fileContents2, ldataDir, "test_table", 2)
	assert.NoError(t, err)

	// Use a mock valueConverter that errors on row 2
	origValueConverter := valueConverter
	valueConverter = &mockRowErrorValueConverter{rowToError: 2}
	t.Cleanup(func() { valueConverter = origValueConverter })

	batchproducer1, err := NewSequentialFileBatchProducer(task1, state, true, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	// First batch: should contain row 1 and row 3 (row 2 is errored and skipped, its size does not count)
	batch1, err := batchproducer1.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1)
	assert.Equal(t, int64(2), batch1.RecordCount)

	batchproducer2, err := NewSequentialFileBatchProducer(task2, state, true, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	// First batch: should contain row 1 and row 3 (row 2 is errored and skipped, its size does not count)
	batch2, err := batchproducer2.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch2)
	assert.Equal(t, int64(2), batch2.RecordCount)

	// Error file should contain the error for row 2
	assertProcessingErrorBatchFileContains(t, lexportDir, task1,
		batch1.Number, 1, 11,
		"ERROR: transforming line number=2",
		"mock conversion error",
		"ROW: 2,errorrow",
	)

	assertProcessingErrorBatchFileContains(t, lexportDir, task2,
		batch2.Number, 1, 11,
		"ERROR: transforming line number=2",
		"mock conversion error",
		"ROW: 2,errorrow",
	)
}

// assertProcessingErrorFileContains asserts that the error log file for a given task and table contains all the expected substrings.
func assertProcessingErrorBatchFileContains(t *testing.T, lexportDir string, task *ImportFileTask,
	batchNumber int64, expectedRowCount int64, expectedByteCount int64,
	expectedSubstrings ...string) {
	taskFolderPath := fmt.Sprintf("file::%s:%s", filepath.Base(task.FilePath), importdata.ComputePathHash(task.FilePath))
	tableFolderPath := fmt.Sprintf("table::%s", task.TableNameTup.ForKey())
	errorFileName := fmt.Sprintf("processing-errors.%d.%d.%d.log", batchNumber, expectedRowCount, expectedByteCount)
	errorsFilePath := filepath.Join(getErrorsParentDir(lexportDir), "errors", tableFolderPath, taskFolderPath, errorFileName)
	assert.FileExists(t, errorsFilePath)
	errorFileContentsBytes, err := os.ReadFile(errorsFilePath)
	assert.NoError(t, err)
	errorFileContents := string(errorFileContentsBytes)
	for _, substr := range expectedSubstrings {
		assert.Contains(t, errorFileContents, substr)
	}
}

// ==================== parseBatchFileName tests ====================

func TestParseBatchFileName_6Fields(t *testing.T) {
	batchNum, offsetEnd, recordCount, byteCount, cumByteOffsetEnd, state, err := parseBatchFileName("batch::2.10000.5000.215572.320000.C")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batchNum)
	assert.Equal(t, int64(10000), offsetEnd)
	assert.Equal(t, int64(5000), recordCount)
	assert.Equal(t, int64(215572), byteCount)
	assert.Equal(t, int64(320000), cumByteOffsetEnd)
	assert.Equal(t, "C", state)
}

func TestParseBatchFileName_LastBatch(t *testing.T) {
	batchNum, offsetEnd, recordCount, byteCount, cumByteOffsetEnd, state, err := parseBatchFileName("batch::0.50000.3000.64000.4012345678.D")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), batchNum) // LAST_SPLIT_NUM
	assert.Equal(t, int64(50000), offsetEnd)
	assert.Equal(t, int64(3000), recordCount)
	assert.Equal(t, int64(64000), byteCount)
	assert.Equal(t, int64(4012345678), cumByteOffsetEnd)
	assert.Equal(t, "D", state)
}

func TestParseBatchFileName_InvalidFieldCount(t *testing.T) {
	// Too few fields
	_, _, _, _, _, _, err := parseBatchFileName("batch::1.2.3.D")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 6 fields")

	// Old 5-field format is no longer supported
	_, _, _, _, _, _, err = parseBatchFileName("batch::1.5000.5000.107786.D")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 6 fields")
}

func TestParseBatchFileName_InvalidState(t *testing.T) {
	_, _, _, _, _, _, err := parseBatchFileName("batch::1.5000.5000.107786.320000.X")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")
}

func TestParseBatchFileName_AllStates(t *testing.T) {
	for _, expectedState := range []string{"C", "P", "D", "E"} {
		fileName := fmt.Sprintf("batch::1.100.100.5000.10000.%s", expectedState)
		_, _, _, _, _, state, err := parseBatchFileName(fileName)
		assert.NoError(t, err)
		assert.Equal(t, expectedState, state)
	}
}

// ==================== CumByteOffset tracking tests ====================

func TestBatchCumByteOffsetTracking(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	batchproducer, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)

	var batches []*Batch
	for !batchproducer.Done() {
		batch, err := batchproducer.NextBatch()
		assert.NoError(t, err)
		batches = append(batches, batch)
	}

	assert.Equal(t, 2, len(batches))
	// CumByteOffset should be positive for all batches
	for _, batch := range batches {
		assert.Greater(t, batch.CumByteOffsetEnd, int64(0), "CumByteOffset should be positive")
	}
	// CumByteOffset should be monotonically increasing
	assert.Greater(t, batches[1].CumByteOffsetEnd, batches[0].CumByteOffsetEnd,
		"CumByteOffset should increase across batches")

	// Last batch's CumByteOffset should equal total file size
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batches[len(batches)-1].CumByteOffsetEnd,
		"Last batch CumByteOffset should equal file size")

	// First batch's CumByteOffset should equal header bytes + batch 1 data bytes
	headerBytes := int64(len("id,val\n"))
	batch1DataBytes := int64(len("1, \"hello\"\n") + len("2, \"world\"\n"))
	assert.Equal(t, headerBytes+batch1DataBytes, batches[0].CumByteOffsetEnd,
		"First batch CumByteOffset should equal header + batch1 data bytes")
}

func TestByteSeekResumption(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	// First run: produce one batch (rows 1-2)
	bp1, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	batch1, err := bp1.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch1.RecordCount)
	assert.Greater(t, batch1.CumByteOffsetEnd, int64(0))
	bp1.Close()

	// Resume: should use byte-seek (cumByteOffset > 0)
	bp2, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.Equal(t, batch1.CumByteOffsetEnd, bp2.lastBatchCumByteOffsetEnd,
		"Recovered lastBatchCumByteOffsetEnd should match batch1's CumByteOffset")

	// Get pending batch 1 first
	recoveredBatch, err := bp2.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, batch1.Number, recoveredBatch.Number)

	// Now produce the second batch (rows 3-4) via byte-seek
	batch2, err := bp2.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch2.RecordCount)
	assert.True(t, bp2.Done())
	bp2.Close()

	// Verify data correctness: batch 2 should contain rows 3-4
	batchContents, err := os.ReadFile(batch2.GetFilePath())
	assert.NoError(t, err)
	expected := "id,val\n3, \"foo\"\n4, \"bar\""
	assert.Equal(t, expected, string(batchContents))

	// Last batch's CumByteOffset should equal total file size
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch2.CumByteOffsetEnd,
		"Last batch CumByteOffset should equal file size")
}

func TestByteSeekResumption_NoHeader(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	rows := []string{"1\thello", "2\tworld", "3\tfoo", "4\tbar"}
	dataFileDescriptor = &datafile.Descriptor{
		FileFormat: "text",
		Delimiter:  "\t",
		HasHeader:  false,
		ExportDir:  lexportDir,
	}
	fileContent := strings.Join(rows, "\n") + "\n"
	tempFile, err := testutils.CreateTempFile(ldataDir, fileContent, "text")
	assert.NoError(t, err)

	sourceName := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", "test_no_header")
	tableNameTup := sqlname.NameTuple{SourceName: sourceName, CurrentName: sourceName}
	task := &ImportFileTask{
		ID:           1,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     int64(len(rows)),
	}

	// First run: produce one batch (rows 1-2)
	bp1, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	batch1, err := bp1.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch1.RecordCount)
	assert.Greater(t, batch1.CumByteOffsetEnd, int64(0))
	bp1.Close()

	// Resume: should use byte-seek
	bp2, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)

	// Get pending batch 1
	_, err = bp2.NextBatch()
	assert.NoError(t, err)

	// Produce second batch via byte-seek
	batch2, err := bp2.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch2.RecordCount)
	assert.True(t, bp2.Done())
	bp2.Close()

	// Verify content
	batchContents, err := os.ReadFile(batch2.GetFilePath())
	assert.NoError(t, err)
	expected := strings.Join(rows[2:], "\n")
	assert.Equal(t, expected, string(batchContents))

	// Last batch's CumByteOffset should equal total file size
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch2.CumByteOffsetEnd,
		"Last batch CumByteOffset should equal file size (no header)")
}

// Verifies that when a row is read but carried forward to the next batch,
// the current batch's cumByteOffset excludes that row's bytes.
func TestCumByteOffset_CarriedForwardLine(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"
5, "baz"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	bp, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)

	batch1, err := bp.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch1.RecordCount)

	batch2, err := bp.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch2.RecordCount)

	batch3, err := bp.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), batch3.RecordCount)
	assert.True(t, bp.Done())

	// Batch 1 cumByteOffset should point right after row 2 (not after row 3)
	headerLen := int64(len("id,val\n"))
	row1Len := int64(len("1, \"hello\"\n"))
	row2Len := int64(len("2, \"world\"\n"))
	assert.Equal(t, headerLen+row1Len+row2Len, batch1.CumByteOffsetEnd,
		"Batch 1 cumByteOffset should exclude carried-forward row 3")

	// Batch 2 cumByteOffset should point right after row 4
	row3Len := int64(len("3, \"foo\"\n"))
	row4Len := int64(len("4, \"bar\"\n"))
	assert.Equal(t, batch1.CumByteOffsetEnd+row3Len+row4Len, batch2.CumByteOffsetEnd,
		"Batch 2 cumByteOffset should be batch1 + row3 + row4")

	// Last batch cumByteOffset should equal file size
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch3.CumByteOffsetEnd,
		"Last batch cumByteOffset should equal file size")
}

// Simulates 3 resume cycles (produce 2 → kill → resume → produce 2 → kill → resume → finish)
// and verifies cumByteOffset is monotonic, correct across cycles, and batch data is intact after seek.
func TestMultipleResumeCycles(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(1, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	rows := []string{"a\tb", "c\td", "e\tf", "g\th", "i\tj", "k\tl"}
	dataFileDescriptor = &datafile.Descriptor{
		FileFormat: "text",
		Delimiter:  "\t",
		HasHeader:  false,
		ExportDir:  lexportDir,
	}
	fileContent := strings.Join(rows, "\n") + "\n"
	tempFile, err := testutils.CreateTempFile(ldataDir, fileContent, "text")
	assert.NoError(t, err)

	sourceName := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", "test_multi_resume")
	tableNameTup := sqlname.NameTuple{SourceName: sourceName, CurrentName: sourceName}
	task := &ImportFileTask{
		ID:           1,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     int64(len(rows)),
	}

	// Cycle 1: produce 2 batches
	bp1, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	b1, err := bp1.NextBatch()
	assert.NoError(t, err)
	b2, err := bp1.NextBatch()
	assert.NoError(t, err)
	assert.False(t, bp1.Done())
	bp1.Close()

	// Cycle 2: resume, get 2 pending + produce 2 new
	bp2, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.Equal(t, b2.CumByteOffsetEnd, bp2.lastBatchCumByteOffsetEnd)

	// Drain pending
	_, err = bp2.NextBatch()
	assert.NoError(t, err)
	_, err = bp2.NextBatch()
	assert.NoError(t, err)

	// Produce 2 new via byte-seek
	b3, err := bp2.NextBatch()
	assert.NoError(t, err)
	b4, err := bp2.NextBatch()
	assert.NoError(t, err)
	assert.False(t, bp2.Done())
	bp2.Close()

	// Cycle 3: resume again, get 4 pending + produce remaining 2
	bp3, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.Equal(t, b4.CumByteOffsetEnd, bp3.lastBatchCumByteOffsetEnd)

	// Drain 4 pending
	for i := 0; i < 4; i++ {
		_, err = bp3.NextBatch()
		assert.NoError(t, err)
	}

	// Produce remaining 2 via byte-seek
	b5, err := bp3.NextBatch()
	assert.NoError(t, err)
	b6, err := bp3.NextBatch()
	assert.NoError(t, err)
	assert.True(t, bp3.Done())
	bp3.Close()

	// CumByteOffset should be monotonically increasing across all 6 batches
	allBatches := []*Batch{b1, b2, b3, b4, b5, b6}
	for i := 1; i < len(allBatches); i++ {
		assert.Greater(t, allBatches[i].CumByteOffsetEnd, allBatches[i-1].CumByteOffsetEnd,
			"CumByteOffset should increase: batch %d vs %d", i, i+1)
	}

	// Last batch should equal file size
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), b6.CumByteOffsetEnd,
		"Last batch cumByteOffset should equal file size")

	// Verify data correctness of byte-seeked batches
	b3Contents, err := os.ReadFile(b3.GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, rows[2], string(b3Contents))

	b5Contents, err := os.ReadFile(b5.GetFilePath())
	assert.NoError(t, err)
	assert.Equal(t, rows[4], string(b5Contents))
}

// Verifies cumByteOffset equals file size for a single-row file that produces one batch.
func TestCumByteOffset_SingleRowFile(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(100, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	fileContents := `id,val
1, "only row"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	bp, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := bp.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), batch.RecordCount)
	assert.Equal(t, int64(0), batch.Number) // last batch = LAST_SPLIT_NUM
	assert.True(t, bp.Done())

	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch.CumByteOffsetEnd,
		"Single-row file: cumByteOffset should equal file size")
	assert.Greater(t, batch.CumByteOffsetEnd, int64(0))
}

// Verifies cumByteOffset still advances past a stashed (too-large) row's bytes,
// so cumByteOffset equals file size even when a row is error-stashed mid-batch.
func TestCumByteOffset_WithStashedErrors(t *testing.T) {
	maxBatchSizeBytes := int64(200)
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	assert.NoError(t, err)

	longVal := strings.Repeat("x", 220) // 220+ bytes, exceeds MaxBatchSizeInBytes of 200
	fileContents := fmt.Sprintf("id,val\n1, \"ok\"\n2, \"%s\"\n3, \"ok2\"", longVal)
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	bp, err := NewSequentialFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := bp.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch.RecordCount) // rows 1 and 3 (row 2 stashed)
	assert.True(t, bp.Done())

	// cumByteOffset must equal total file size, including the stashed row's bytes
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch.CumByteOffsetEnd,
		"cumByteOffset should include stashed row bytes and equal file size")
}

// Produces 4 of 8 batches, resumes via byte-seek, produces remaining 4,
// and verifies each seeked batch contains the correct row data.
func TestByteSeekResumption_ManyBatches(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(1, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	dataFileDescriptor = &datafile.Descriptor{
		FileFormat: "text",
		Delimiter:  "\t",
		HasHeader:  false,
		ExportDir:  lexportDir,
	}
	rows := []string{"r1\tc1", "r2\tc2", "r3\tc3", "r4\tc4", "r5\tc5", "r6\tc6", "r7\tc7", "r8\tc8"}
	fileContent := strings.Join(rows, "\n") + "\n"
	tempFile, err := testutils.CreateTempFile(ldataDir, fileContent, "text")
	assert.NoError(t, err)

	sourceName := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", "test_many_batches")
	tableNameTup := sqlname.NameTuple{SourceName: sourceName, CurrentName: sourceName}
	task := &ImportFileTask{
		ID:           1,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     8,
	}

	// First run: produce 4 batches
	bp1, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	firstRunBatches := make([]*Batch, 0)
	for i := 0; i < 4; i++ {
		b, err := bp1.NextBatch()
		assert.NoError(t, err)
		firstRunBatches = append(firstRunBatches, b)
	}
	bp1.Close()

	// Resume: byte-seek to midpoint
	bp2, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.Equal(t, firstRunBatches[3].CumByteOffsetEnd, bp2.lastBatchCumByteOffsetEnd)

	// Drain 4 pending
	for i := 0; i < 4; i++ {
		_, err = bp2.NextBatch()
		assert.NoError(t, err)
	}

	// Produce remaining 4 via byte-seek
	seekedBatches := make([]*Batch, 0)
	for !bp2.Done() {
		b, err := bp2.NextBatch()
		assert.NoError(t, err)
		seekedBatches = append(seekedBatches, b)
	}
	bp2.Close()
	assert.Equal(t, 4, len(seekedBatches))

	// Verify each seeked batch has correct content
	for i, b := range seekedBatches {
		contents, err := os.ReadFile(b.GetFilePath())
		assert.NoError(t, err)
		assert.Equal(t, rows[4+i], string(contents),
			"Batch %d after seek should contain row %d", i+5, i+5)
	}

	// Last batch cumByteOffset = file size
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), seekedBatches[len(seekedBatches)-1].CumByteOffsetEnd)
}

// Verifies byte-seek resumption with SQL (ora2pg) format. When opened at a byte offset
// mid-file, the SqlDataFile must assume it is inside a COPY block (via openedAtOffset > 0)
// so that data rows are parsed correctly without waiting for a COPY preamble.
func TestByteSeekResumption_SqlFormat(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	// ora2pg-style SQL: COPY preamble, data rows, end-of-copy marker
	sqlContent := `COPY "public"."test_table" ("id", "val") FROM STDIN;
1	hello
2	world
3	foo
4	bar
\.
`
	dataRows := []string{"1\thello", "2\tworld", "3\tfoo", "4\tbar"}

	dataFileDescriptor = &datafile.Descriptor{
		FileFormat: "sql",
		HasHeader:  false,
		ExportDir:  lexportDir,
	}
	tempFile, err := testutils.CreateTempFile(ldataDir, sqlContent, "sql")
	assert.NoError(t, err)

	sourceName := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", "test_sql_seek")
	tableNameTup := sqlname.NameTuple{SourceName: sourceName, CurrentName: sourceName}
	task := &ImportFileTask{
		ID:           1,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     4,
	}

	// First run: produce one batch (rows 1-2)
	bp1, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	batch1, err := bp1.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch1.RecordCount)
	assert.Greater(t, batch1.CumByteOffsetEnd, int64(0))
	bp1.Close()

	// Resume: byte-seek past the COPY preamble + first 2 data rows
	bp2, err := NewSequentialFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.Equal(t, batch1.CumByteOffsetEnd, bp2.lastBatchCumByteOffsetEnd)

	// Drain pending batch 1
	_, err = bp2.NextBatch()
	assert.NoError(t, err)

	// Produce batch 2 via byte-seek — SqlDataFile must know it's inside COPY
	batch2, err := bp2.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch2.RecordCount)
	assert.True(t, bp2.Done())
	bp2.Close()

	// Verify batch 2 contains rows 3-4 (correct parsing after seek)
	b2Contents, err := os.ReadFile(batch2.GetFilePath())
	assert.NoError(t, err)
	expected := strings.Join(dataRows[2:], "\n")
	assert.Equal(t, expected, string(b2Contents))

	// Last batch cumByteOffset should equal file size
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch2.CumByteOffsetEnd,
		"Last batch cumByteOffset should equal file size for SQL format")
}

// Verifies cumByteOffset equals file size when a row is stashed due to a
// transformation/conversion error (as opposed to a too-large row error).
func TestCumByteOffset_WithConversionErrors(t *testing.T) {
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, 1024)
	assert.NoError(t, err)
	defer os.RemoveAll(ldataDir)
	defer os.RemoveAll(lexportDir)

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	assert.NoError(t, err)

	fileContents := `id,val
1,ok
2,errorrow
3,ok2`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	assert.NoError(t, err)

	origValueConverter := valueConverter
	valueConverter = &mockRowErrorValueConverter{rowToError: 2}
	t.Cleanup(func() { valueConverter = origValueConverter })

	bp, err := NewSequentialFileBatchProducer(task, state, true, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	batch, err := bp.NextBatch()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), batch.RecordCount) // rows 1 and 3 (row 2 conversion-errored)
	assert.True(t, bp.Done())

	// cumByteOffset must equal total file size, including the errored row's bytes
	fileInfo, err := os.Stat(task.FilePath)
	assert.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), batch.CumByteOffsetEnd,
		"cumByteOffset should include conversion-errored row bytes and equal file size")
}

func assertNoProcessingErrorBatchFileExists(t *testing.T, lexportDir string, task *ImportFileTask, batchNumber int64) {
	taskFolderPath := fmt.Sprintf("file::%s:%s", filepath.Base(task.FilePath), importdata.ComputePathHash(task.FilePath))
	tableFolderPath := fmt.Sprintf("table::%s", task.TableNameTup.ForKey())
	errorsDir := filepath.Join(getErrorsParentDir(lexportDir), "errors", tableFolderPath, taskFolderPath)

	// Check if the errors directory exists
	if _, err := os.Stat(errorsDir); os.IsNotExist(err) {
		// Directory doesn't exist, so no error files exist
		return
	}

	// Look for any processing error files for this batch number
	entries, err := os.ReadDir(errorsDir)
	assert.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if this is a processing error file for the specified batch number
		if strings.HasPrefix(entry.Name(), fmt.Sprintf("processing-errors.%d.", batchNumber)) {
			t.Errorf("Expected no processing error file for batch %d, but found: %s", batchNumber, entry.Name())
		}
	}
}
