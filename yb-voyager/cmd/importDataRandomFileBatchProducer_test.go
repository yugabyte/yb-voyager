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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForBatchAvailable waits for a batch to become available in the producer, with a timeout.
// Returns true if batch becomes available, false if timeout is reached.
func waitForBatchAvailable(producer *RandomBatchProducer, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if producer.IsBatchAvailable() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestSingleBatchProductionAndConsumption tests test case 1.1:
// Create producer with a file that produces exactly 1 batch
func TestSingleBatchProductionAndConsumption(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create a file that produces exactly 1 batch (header + 1 row)
	fileContents := `id,val
1, "hello"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create RandomBatchProducer
	producer, err := NewRandomFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer producer.Close()

	// Verify initial state: Done() should be false, IsBatchAvailable() should be false
	assert.False(t, producer.Done(), "Done() should be false initially")
	assert.False(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be false initially")

	// Wait for batch to become available (producer goroutine is producing in background)
	// Use a reasonable timeout (e.g., 5 seconds)
	available := waitForBatchAvailable(producer, 5*time.Second)
	require.True(t, available, "Batch should become available within timeout")

	// Verify state when batch is available: IsBatchAvailable() should be true, Done() should be false
	assert.True(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be true when batch is available")
	assert.False(t, producer.Done(), "Done() should be false when batch is available")

	// Call NextBatch() and verify it returns a batch
	batch, err := producer.NextBatch()
	require.NoError(t, err, "NextBatch() should return a batch without error")
	require.NotNil(t, batch, "NextBatch() should return a non-nil batch")

	// Verify batch number matches expected value (should be 0 for last batch) (since there is only 1 batch)
	assert.Equal(t, int64(0), batch.Number, "Batch number should be 0")
	assert.Equal(t, int64(1), batch.RecordCount, "Batch should contain 1 record")

	// After consuming the batch, Done() should be true, IsBatchAvailable() should be false
	assert.True(t, producer.Done(), "Done() should be true after consuming the batch")
	assert.False(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be false after consuming the batch")
}

// waitForProducerDone waits for the producer to finish producing all batches, with a timeout.
// Returns true if producer is done, false if timeout is reached.
func waitForProducerDone(producer *RandomBatchProducer, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if producer.Done() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestMultipleBatchesProductionAndConsumption tests test case 1.2:
// Create producer with a file that produces N batches (e.g., 5, 10, 20)
func TestMultipleBatchesProductionAndConsumption(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create a file that produces 5 batches (batch size is 2 rows, so 10 rows = 5 batches)
	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"
5, "baz"
6, "qux"
7, "quux"
8, "corge"
9, "grault"
10, "waldo"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create RandomBatchProducer
	producer, err := NewRandomFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer producer.Close()

	// Verify initial state: Done() should be false, IsBatchAvailable() should be false
	assert.False(t, producer.Done(), "Done() should be false initially")
	assert.False(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be false initially")

	// Collect all batches
	var batches []*Batch
	batchNumbers := make(map[int64]bool)
	totalExpectedBatches := 5

	// Consume all batches
	for !producer.Done() {
		// Wait for batch to become available
		available := waitForBatchAvailable(producer, 5*time.Second)
		if !available {
			// If no batch available and producer is done, break
			if producer.Done() {
				break
			}
			// Otherwise timeout occurred, which shouldn't happen
			require.Fail(t, "Batch should become available within timeout")
		}

		// Verify state consistency: If IsBatchAvailable() is true, Done() must be false
		assert.False(t, producer.Done(), "State consistency: Done() must be false when IsBatchAvailable() is true")

		// Consume the batch
		batch, err := producer.NextBatch()
		require.NoError(t, err, "NextBatch() should return a batch without error")
		require.NotNil(t, batch, "NextBatch() should return a non-nil batch")

		batches = append(batches, batch)

		// Verify batch is unique (no duplicate batch numbers)
		require.False(t, batchNumbers[batch.Number], "Batch number %d should be unique", batch.Number)
		batchNumbers[batch.Number] = true
	}

	// Verify all N batches are eventually consumed
	assert.Equal(t, totalExpectedBatches, len(batches), "Total batches consumed should equal %d", totalExpectedBatches)
	assert.Equal(t, totalExpectedBatches, len(batchNumbers), "Total unique batch numbers should equal %d", totalExpectedBatches)

	// Verify Done() returns true only after all batches consumed
	assert.True(t, producer.Done(), "Done() should be true after all batches consumed")

	// Verify IsBatchAvailable() returns false when Done() is true
	assert.False(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be false when Done() is true")
}

// TestFileWithOnlyHeader tests test case 1.4:
// Create producer with a file containing only header (CSV format)
func TestFileWithOnlyHeader(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create a file with only header (no data rows)
	fileContents := `id,val`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create RandomBatchProducer
	producer, err := NewRandomFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer producer.Close()

	// Verify initial state: Done() should be false, IsBatchAvailable() should be false
	assert.False(t, producer.Done(), "Done() should be false initially")
	assert.False(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be false initially")

	// Wait for batch to become available (producer creates a batch with 0 records)
	available := waitForBatchAvailable(producer, 5*time.Second)
	require.True(t, available, "Batch should become available within timeout")

	// Verify state when batch is available: IsBatchAvailable() should be true, Done() should be false
	assert.True(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be true when batch is available")
	assert.False(t, producer.Done(), "Done() should be false when batch is available")

	// Verify a batch is produced with 0 record count
	batch, err := producer.NextBatch()
	require.NoError(t, err, "NextBatch() should return a batch without error")
	require.NotNil(t, batch, "NextBatch() should return a non-nil batch")
	assert.Equal(t, int64(0), batch.RecordCount, "Batch should contain 0 records (only header, no data)")

	// After consuming the batch, Done() should be true, IsBatchAvailable() should be false
	assert.True(t, producer.Done(), "Done() should be true after consuming the batch")
	assert.False(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be false after consuming the batch")
}

// assertBatchesMatch verifies that two batches are identical by checking:
// - File contents match
// - All metadata matches (batch number, record count, table name, schema name, offsets, byte count, interrupted flag)
// Note: BaseFilePath is not compared as it's expected to differ between sequential and random producers
func assertBatchesMatch(t *testing.T, batch1 *Batch, batch2 *Batch, msgAndArgs ...interface{}) {
	require.NotNil(t, batch1, "Batch1 should not be nil")
	require.NotNil(t, batch2, "Batch2 should not be nil")

	// Verify batch number matches
	assert.Equal(t, batch1.Number, batch2.Number, append(msgAndArgs, "Batch number should match")...)

	// Verify record count matches
	assert.Equal(t, batch1.RecordCount, batch2.RecordCount, append(msgAndArgs, "Record count should match")...)

	// Verify batch contents match
	batch1Contents, err := os.ReadFile(batch1.GetFilePath())
	require.NoError(t, err, append(msgAndArgs, "Should be able to read batch1 file")...)
	batch2Contents, err := os.ReadFile(batch2.GetFilePath())
	require.NoError(t, err, append(msgAndArgs, "Should be able to read batch2 file")...)
	assert.Equal(t, string(batch1Contents), string(batch2Contents), append(msgAndArgs, "Batch file contents should match")...)

	// Verify batch metadata matches
	// Note: BaseFilePath is not compared as it's expected to differ between sequential and random producers
	assert.Equal(t, batch1.TableNameTup, batch2.TableNameTup, append(msgAndArgs, "Table name should match")...)
	assert.Equal(t, batch1.SchemaName, batch2.SchemaName, append(msgAndArgs, "Schema name should match")...)
	assert.Equal(t, batch1.OffsetStart, batch2.OffsetStart, append(msgAndArgs, "Offset start should match")...)
	assert.Equal(t, batch1.OffsetEnd, batch2.OffsetEnd, append(msgAndArgs, "Offset end should match")...)
	assert.Equal(t, batch1.ByteCount, batch2.ByteCount, append(msgAndArgs, "Byte count should match")...)
	assert.Equal(t, batch1.Interrupted, batch2.Interrupted, append(msgAndArgs, "Interrupted flag should match")...)
}

// TestSingleBatchMatchVerification tests test case 2.1:
// Create producer with a file that produces exactly 1 batch and verify it matches sequential producer
func TestSingleBatchMatchVerification(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create a file that produces exactly 1 batch (header + 1 row)
	fileContents := `id,val
1, "hello"`

	// Create two separate tasks - one for sequential producer, one for random producer
	_, sequentialTask, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)
	_, randomTask, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 2)
	require.NoError(t, err)

	// Create SequentialFileBatchProducer to get the expected batch
	sequentialProducer, err := NewSequentialFileBatchProducer(sequentialTask, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer sequentialProducer.Close()

	sequentialBatch, err := sequentialProducer.NextBatch()
	require.NoError(t, err, "Sequential producer should produce a batch")
	require.NotNil(t, sequentialBatch, "Sequential batch should not be nil")
	require.True(t, sequentialProducer.Done(), "Sequential producer should be done after producing 1 batch")

	// Create RandomBatchProducer with separate task
	randomProducer, err := NewRandomFileBatchProducer(randomTask, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer randomProducer.Close()

	// Verify initial state: Done() should be false, IsBatchAvailable() should be false
	assert.False(t, randomProducer.Done(), "Done() should be false initially")
	assert.False(t, randomProducer.IsBatchAvailable(), "IsBatchAvailable() should be false initially")

	// Wait for batch to become available
	available := waitForBatchAvailable(randomProducer, 5*time.Second)
	require.True(t, available, "Batch should become available within timeout")

	// Verify state when batch is available: IsBatchAvailable() should be true, Done() should be false
	assert.True(t, randomProducer.IsBatchAvailable(), "IsBatchAvailable() should be true when batch is available")
	assert.False(t, randomProducer.Done(), "Done() should be false when batch is available")

	// Get batch from random producer
	randomBatch, err := randomProducer.NextBatch()
	require.NoError(t, err, "NextBatch() should return a batch without error")
	require.NotNil(t, randomBatch, "NextBatch() should return a non-nil batch")

	// After consuming the batch, Done() should be true, IsBatchAvailable() should be false
	assert.True(t, randomProducer.Done(), "Done() should be true after consuming the batch")
	assert.False(t, randomProducer.IsBatchAvailable(), "IsBatchAvailable() should be false after consuming the batch")

	// Verify batches match (contents and all metadata)
	assertBatchesMatch(t, sequentialBatch, randomBatch, "Sequential and random batches should match")

}

// TestMultipleBatchesMatchVerification tests test case 2.2:
// Create producer with a file that produces N batches and verify they match sequential producer
func TestMultipleBatchesMatchVerification(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create a file that produces 5 batches (batch size is 2 rows, so 10 rows = 5 batches)
	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"
5, "baz"
6, "qux"
7, "quux"
8, "corge"
9, "grault"
10, "waldo"`

	// Create two separate tasks - one for sequential producer, one for random producer
	_, sequentialTask, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)
	_, randomTask, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 2)
	require.NoError(t, err)

	// Create SequentialFileBatchProducer and collect all batches
	sequentialProducer, err := NewSequentialFileBatchProducer(sequentialTask, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer sequentialProducer.Close()

	var sequentialBatches []*Batch
	var sequentialBatchMap = make(map[int64]*Batch)
	for !sequentialProducer.Done() {
		batch, err := sequentialProducer.NextBatch()
		require.NoError(t, err, "Sequential producer should produce batches")
		require.NotNil(t, batch, "Sequential batch should not be nil")
		sequentialBatches = append(sequentialBatches, batch)
		sequentialBatchMap[batch.Number] = batch
	}
	require.True(t, sequentialProducer.Done(), "Sequential producer should be done after producing all batches")
	totalExpectedBatches := len(sequentialBatches)

	// Create RandomBatchProducer
	randomProducer, err := NewRandomFileBatchProducer(randomTask, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer randomProducer.Close()

	// Verify initial state: Done() should be false, IsBatchAvailable() should be false
	assert.False(t, randomProducer.Done(), "Done() should be false initially")
	assert.False(t, randomProducer.IsBatchAvailable(), "IsBatchAvailable() should be false initially")

	// Collect all batches from random producer
	var randomBatches []*Batch
	var randomBatchMap = make(map[int64]*Batch)
	batchNumbersSeen := make(map[int64]bool)

	// Consume all batches
	for !randomProducer.Done() {
		// Wait for batch to become available
		available := waitForBatchAvailable(randomProducer, 5*time.Second)
		if !available {
			// If no batch available and producer is done, break
			if randomProducer.Done() {
				break
			}
			// Otherwise timeout occurred, which shouldn't happen
			require.Fail(t, "Batch should become available within timeout")
		}

		// Verify state consistency: If IsBatchAvailable() is true, Done() must be false
		assert.False(t, randomProducer.Done(), "State consistency: Done() must be false when IsBatchAvailable() is true")

		// Consume the batch
		batch, err := randomProducer.NextBatch()
		require.NoError(t, err, "NextBatch() should return a batch without error")
		require.NotNil(t, batch, "NextBatch() should return a non-nil batch")

		randomBatches = append(randomBatches, batch)
		randomBatchMap[batch.Number] = batch

		// Verify batch is unique (no duplicate batch numbers)
		require.False(t, batchNumbersSeen[batch.Number], "Batch number %d should be unique", batch.Number)
		batchNumbersSeen[batch.Number] = true
	}

	// Verify batch count from random producer matches sequential producer
	assert.Equal(t, totalExpectedBatches, len(randomBatches), "Batch count from random producer should match sequential producer")
	assert.Equal(t, totalExpectedBatches, len(randomBatchMap), "Batch map size should match expected count")
	assert.Equal(t, totalExpectedBatches, len(batchNumbersSeen), "Unique batch numbers should match expected count")

	// Verify all batches from sequential producer are present in random producer (by batch number)
	for batchNum, sequentialBatch := range sequentialBatchMap {
		randomBatch, exists := randomBatchMap[batchNum]
		require.True(t, exists, "Batch number %d from sequential producer should be present in random producer", batchNum)
		// Verify batch contents and metadata match
		assertBatchesMatch(t, sequentialBatch, randomBatch, "Batches with number %d should match", batchNum)
	}

	// Verify all batch numbers from sequential producer appear in random producer
	// (This ensures no gaps - if sequential has batches 0,1,2,3,4 then random should have the same)
	for batchNum := range sequentialBatchMap {
		_, exists := randomBatchMap[batchNum]
		require.True(t, exists, "Batch number %d should be present in random producer", batchNum)
	}

	// Verify final state: Done() should be true, IsBatchAvailable() should be false
	assert.True(t, randomProducer.Done(), "Done() should be true after consuming all batches")
	assert.False(t, randomProducer.IsBatchAvailable(), "IsBatchAvailable() should be false when Done() is true")
}

// TestNextBatchCalledWhenNoBatchesAvailableProducerRunning tests test case 3.1:
// Call NextBatch() before any batches are produced (producer still running)
// Non-deterministic test case, it depends on the speed of the batch production, but test assertions
// are still valid for all scenarios.
func TestNextBatchCalledWhenNoBatchesAvailableProducerRunning(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(1000, 1000000)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Programmatically create a large file with many rows to slow down batch production
	// This increases the chance that NextBatch() will be called before any batches are available
	var fileContentsBuilder strings.Builder
	fileContentsBuilder.WriteString("id,val\n")
	// Create 100 rows to make batch production take longer
	for i := 1; i <= 100; i++ {
		fileContentsBuilder.WriteString(fmt.Sprintf("%d, \"value%d\"\n", i, i))
	}
	fileContents := fileContentsBuilder.String()

	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create RandomBatchProducer
	producer, err := NewRandomFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer producer.Close()

	// Verify initial state
	assert.False(t, producer.Done(), "Done() should be false initially")
	assert.False(t, producer.IsBatchAvailable(), "IsBatchAvailable() should be false initially")

	// Immediately call NextBatch() - this may return error if no batches available yet,
	// or may return a batch if batch production was very fast (race condition)
	batch, err := producer.NextBatch()

	// Verify behavior based on what happened
	if err != nil {
		// Error path: no batches were available yet
		require.Error(t, err, "NextBatch() should return error when no batches available")
		assert.Contains(t, err.Error(), "no batches available", "Error message should indicate no batches available")
		assert.Nil(t, batch, "NextBatch() should return nil batch when error occurs")

		// Verify producer continues running in background by waiting for a batch to become available
		available := waitForBatchAvailable(producer, 10*time.Second)
		require.True(t, available, "Producer should continue running and produce batches in background")
	} else {
		// Batch was returned (race condition - batch was produced very quickly)
		// This is also valid - verify the producer continues correctly
		require.NotNil(t, batch, "If no error, batch should not be nil")
		// Producer continues running, verify we can get more batches or it completes
		if !producer.Done() {
			// Wait for more batches or completion
			done := waitForProducerDone(producer, 10*time.Second)
			require.True(t, done, "Producer should eventually complete")
		}
	}
}
