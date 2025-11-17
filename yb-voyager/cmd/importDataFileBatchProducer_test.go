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

	batchproducer, err := NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// generate one batch
	batch1, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1)
	assert.Equal(t, int64(2), batch1.RecordCount)

	// simulate a crash and recover
	batchproducer, err = NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// state should have recovered that one batch
	assert.Equal(t, 1, len(batchproducer.pendingBatches))
	assert.True(t, cmp.Equal(batch1, batchproducer.pendingBatches[0]))

	// verify that it picks up from pendingBatches
	// instead of procing a new batch.
	batch1Recovered, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1Recovered)
	assert.True(t, cmp.Equal(batch1, batch1Recovered))
	assert.Equal(t, 0, len(batchproducer.pendingBatches))
	assert.False(t, batchproducer.Done())

	// get final batch
	batch2, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch2)
	assert.Equal(t, int64(2), batch2.RecordCount)
	assert.True(t, batchproducer.Done())
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

	batchproducer, err := NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
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
	batchproducer, err = NewFileBatchProducer(task, state, false, errorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, true, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, true, abortErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer, err = NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, true, scErrorHandler, progressReporter)
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

	batchproducer, err := NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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
	batchproducer, err = NewFileBatchProducer(task, state, false, scErrorHandler, progressReporter)
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

	batchproducer1, err := NewFileBatchProducer(task1, state, true, scErrorHandler, progressReporter)
	assert.NoError(t, err)

	// First batch: should contain row 1 and row 3 (row 2 is errored and skipped, its size does not count)
	batch1, err := batchproducer1.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1)
	assert.Equal(t, int64(2), batch1.RecordCount)

	batchproducer2, err := NewFileBatchProducer(task2, state, true, scErrorHandler, progressReporter)
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
