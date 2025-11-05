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
