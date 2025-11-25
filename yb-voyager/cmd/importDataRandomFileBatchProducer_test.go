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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
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

	verifyProducerInitialState(t, producer)

	// Wait for batch to become available (producer goroutine is producing in background)
	// Use a reasonable timeout (e.g., 5 seconds)
	available := waitForBatchAvailable(producer, 5*time.Second)
	require.True(t, available, "Batch should become available within timeout")

	verifyBatchAvailableState(t, producer)

	// Call NextBatch() and verify it returns a batch
	batch, err := producer.NextBatch()
	require.NoError(t, err, "NextBatch() should return a batch without error")
	require.NotNil(t, batch, "NextBatch() should return a non-nil batch")

	// Verify batch number matches expected value (should be 0 for last batch) (since there is only 1 batch)
	assert.Equal(t, int64(0), batch.Number, "Batch number should be 0")
	assert.Equal(t, int64(1), batch.RecordCount, "Batch should contain 1 record")

	verifyProducerFinalState(t, producer)
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

// waitForPartialBatchesProduced waits for batches to be produced within a specified range, with a timeout.
// Returns the number of batches produced.
func waitForPartialBatchesProduced(t *testing.T, producer *RandomBatchProducer, minBatches, maxBatches int, timeout time.Duration) int {
	t.Helper()

	var batchesProduced int
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		producer.mu.Lock()
		batchesProduced = len(producer.sequentiallyProducedBatches)
		producer.mu.Unlock()

		if batchesProduced >= minBatches && batchesProduced < maxBatches {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.GreaterOrEqual(t, batchesProduced, minBatches,
		"Should have at least %d batches produced", minBatches)
	require.Less(t, batchesProduced, maxBatches,
		"Should have less than %d batches produced (partial)", maxBatches)

	return batchesProduced
}

// consumeAllBatches waits for and collects all batches from the producer
func consumeAllBatches(t *testing.T, producer *RandomBatchProducer, timeout time.Duration) []*Batch {
	t.Helper()

	var batches []*Batch
	for !producer.Done() {
		available := waitForBatchAvailable(producer, timeout)
		if !available {
			if producer.Done() {
				break
			}
			require.Fail(t, "Batch should become available within timeout")
		}

		// Verify state consistency: If IsBatchAvailable() should be true, when Done() is false
		assert.True(t, producer.IsBatchAvailable(), "IsBatchAvailable() must be true when Done() is false")

		batch, err := producer.NextBatch()
		require.NoError(t, err)
		require.NotNil(t, batch)
		batches = append(batches, batch)
	}

	return batches
}

// verifyProducerInitialState verifies the producer starts in the correct state
func verifyProducerInitialState(t *testing.T, producer *RandomBatchProducer) {
	t.Helper()

	assert.False(t, producer.Done(),
		"Done() should be false initially")
	assert.False(t, producer.IsBatchAvailable(),
		"IsBatchAvailable() should be false initially")
}

// verifyProducerFinalState verifies the producer ends in the correct state
func verifyProducerFinalState(t *testing.T, producer *RandomBatchProducer) {
	t.Helper()

	assert.True(t, producer.Done(),
		"Done() should be true after consuming all batches")
	assert.False(t, producer.IsBatchAvailable(),
		"IsBatchAvailable() should be false when Done() is true")
}

// verifyBatchAvailableState verifies the producer state when a batch is available
func verifyBatchAvailableState(t *testing.T, producer *RandomBatchProducer) {
	t.Helper()

	assert.True(t, producer.IsBatchAvailable(),
		"IsBatchAvailable() should be true when batch is available")
	assert.False(t, producer.Done(),
		"Done() should be false when batch is available")
}

// verifyBatchesComplete verifies batches are unique, have expected count, and fall within range
// Returns map of batch numbers for further verification if needed
func verifyBatchesComplete(t *testing.T, batches []*Batch, expectedCount int, minInclusive, maxExclusive int64) map[int64]bool {
	t.Helper()

	// Verify count
	require.Equal(t, expectedCount, len(batches),
		"Expected %d batches but got %d", expectedCount, len(batches))

	// Verify uniqueness and range in one pass
	batchNumbers := make(map[int64]bool)
	for _, batch := range batches {
		// Uniqueness
		require.False(t, batchNumbers[batch.Number],
			"Batch number %d should be unique", batch.Number)
		batchNumbers[batch.Number] = true

		// Range
		require.GreaterOrEqual(t, batch.Number, minInclusive,
			"Batch number should be >= %d", minInclusive)
		require.Less(t, batch.Number, maxExclusive,
			"Batch number should be < %d", maxExclusive)
	}

	// Final uniqueness count check
	require.Equal(t, expectedCount, len(batchNumbers),
		"Should have %d unique batch numbers", expectedCount)

	return batchNumbers
}

// verifyBatchesWithPreviousConsumption verifies batches with previously consumed batches
// Updates consumedBatchNumbers map with newly consumed batches
// Verifies final total matches expectedTotal
func verifyBatchesWithPreviousConsumption(t *testing.T, batches []*Batch, consumedBatchNumbers map[int64]bool, expectedTotal int, minInclusive, maxExclusive int64) {
	t.Helper()

	// Calculate expected count for this run
	expectedCount := expectedTotal - len(consumedBatchNumbers)
	require.Equal(t, expectedCount, len(batches),
		"Expected %d new batches but got %d", expectedCount, len(batches))

	// Verify uniqueness (against previously consumed) and range in one pass
	for _, batch := range batches {
		// Uniqueness (including previously consumed)
		require.False(t, consumedBatchNumbers[batch.Number],
			"Batch number %d should be unique (or was already consumed)", batch.Number)
		consumedBatchNumbers[batch.Number] = true

		// Range
		require.GreaterOrEqual(t, batch.Number, minInclusive,
			"Batch number should be >= %d", minInclusive)
		require.Less(t, batch.Number, maxExclusive,
			"Batch number should be < %d", maxExclusive)
	}

	// Verify final total
	require.Equal(t, expectedTotal, len(consumedBatchNumbers),
		"Should have %d total unique batch numbers after resumption", expectedTotal)
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

	verifyProducerInitialState(t, producer)

	// Collect all batches
	batches := consumeAllBatches(t, producer, 5*time.Second)
	verifyBatchesComplete(t, batches, 5, 0, 5)

	verifyProducerFinalState(t, producer)
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

	verifyProducerInitialState(t, producer)

	// Wait for batch to become available (producer creates a batch with 0 records)
	available := waitForBatchAvailable(producer, 5*time.Second)
	require.True(t, available, "Batch should become available within timeout")

	verifyBatchAvailableState(t, producer)

	// Verify a batch is produced with 0 record count
	batch, err := producer.NextBatch()
	require.NoError(t, err, "NextBatch() should return a batch without error")
	require.NotNil(t, batch, "NextBatch() should return a non-nil batch")
	assert.Equal(t, int64(0), batch.RecordCount, "Batch should contain 0 records (only header, no data)")

	verifyProducerFinalState(t, producer)
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

	verifyProducerInitialState(t, randomProducer)

	// Wait for batch to become available
	available := waitForBatchAvailable(randomProducer, 5*time.Second)
	require.True(t, available, "Batch should become available within timeout")

	verifyBatchAvailableState(t, randomProducer)

	// Get batch from random producer
	randomBatch, err := randomProducer.NextBatch()
	require.NoError(t, err, "NextBatch() should return a batch without error")
	require.NotNil(t, randomBatch, "NextBatch() should return a non-nil batch")

	verifyProducerFinalState(t, randomProducer)

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
	totalExpectedBatches := len(sequentialBatches)

	// Create RandomBatchProducer
	randomProducer, err := NewRandomFileBatchProducer(randomTask, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer randomProducer.Close()

	verifyProducerInitialState(t, randomProducer)

	// Collect all batches from random producer
	randomBatches := consumeAllBatches(t, randomProducer, 5*time.Second)
	verifyBatchesComplete(t, randomBatches, totalExpectedBatches, 0, int64(totalExpectedBatches))

	// Build map for comparison with sequential batches
	randomBatchMap := make(map[int64]*Batch)
	for _, batch := range randomBatches {
		randomBatchMap[batch.Number] = batch
	}

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

	verifyProducerFinalState(t, randomProducer)
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

	verifyProducerInitialState(t, producer)

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

// TestNextBatchCalledWhenNoBatchesAvailableProducerFinished tests test case 3.2:
// Call NextBatch() after all batches consumed (producer finished)
func TestNextBatchCalledWhenNoBatchesAvailableProducerFinished(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create a file that produces multiple batches
	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create RandomBatchProducer
	producer, err := NewRandomFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer producer.Close()

	// Consume all batches
	_ = consumeAllBatches(t, producer, 5*time.Second)

	// Verify producer is done after consuming all batches
	verifyProducerFinalState(t, producer)

	// Call NextBatch() after all batches consumed - should return error
	batch, err := producer.NextBatch()
	require.Error(t, err, "NextBatch() should return error when no batches available after producer finished")
	assert.Contains(t, err.Error(), "no batches available", "Error message should indicate no batches available")
	assert.Nil(t, batch, "NextBatch() should return nil batch when error occurs")
}

// ================================ Error Policy Tests ================================

// TestRandomBatchProducer_AbortHandler tests that abort error policy works with random batch producer
// Note: With abort policy, errors from the sequential producer cause utils.ErrExit to be called,
// which exits the process.
func TestRandomBatchProducer_AbortHandler(t *testing.T) {
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	abortErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.AbortErrorPolicy, getErrorsParentDir(lexportDir))
	require.NoError(t, err)

	fileContents := `id,val
1, "hello"
2, "world"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Swap in the mock valueConverter
	origValueConverter := valueConverter
	valueConverter = &mockValueConverterForTest{}
	t.Cleanup(func() { valueConverter = origValueConverter })

	// Expect error before Creating RandomBatchProducer
	var errExitCalled bool
	utils.MonkeyPatchUtilsErrExit(func(formatString string, args ...interface{}) {
		errExitCalled = true
	})
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
	})
	producer, err := NewRandomFileBatchProducer(task, state, abortErrorHandler, progressReporter)
	require.NoError(t, err)
	defer producer.Close()

	// With abort policy, when the sequential producer encounters a conversion error,
	// it returns an error from NextBatch(), which causes startProducingBatches() to return an error,
	// which then calls utils.ErrExit and exits the process.
	// Wait for ErrExit to be called with a timeout
	deadline := time.Now().Add(5 * time.Second)
	for !errExitCalled && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, errExitCalled, "utils.ErrExit should be called within timeout")
}

// TestRandomBatchProducer_StashAndContinue tests that stash-and-continue error policy works with random batch producer
func TestRandomBatchProducer_StashAndContinue(t *testing.T) {
	// Set max batch size in bytes to a small value to trigger the row-too-large error
	maxBatchSizeBytes := int64(20) // deliberately small to trigger error
	ldataDir, lexportDir, state, _, progressReporter, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	require.NoError(t, err)

	// The second row will be too large for the batch size
	fileContents := `id,val
1, "hello"
2, "this row is too long and should trigger an error because it exceeds the max batch size"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create RandomBatchProducer
	producer, err := NewRandomFileBatchProducer(task, state, scErrorHandler, progressReporter)
	require.NoError(t, err)
	defer producer.Close()

	// Wait for batch to become available
	available := waitForBatchAvailable(producer, 5*time.Second)
	require.True(t, available, "Batch should become available within timeout")

	// Get the batch - should contain only the first row (second row is skipped due to error)
	batch, err := producer.NextBatch()
	require.NoError(t, err, "NextBatch() should not return error with stash-and-continue policy")
	require.NotNil(t, batch, "Batch should not be nil")
	assert.Equal(t, int64(1), batch.RecordCount, "Batch should contain 1 record (second row skipped due to error)")

	// Verify error file contains the error for the second row
	assertProcessingErrorBatchFileContains(t, lexportDir, task,
		batch.Number, 1, 91,
		"larger than max batch size",
		"ROW: 2, \"this row is too long and should trigger an error because it exceeds the max batch size\"")

	// Verify producer eventually completes
	_ = waitForProducerDone(producer, 5*time.Second)
	assert.True(t, producer.Done(), "Done() should be true after producer finishes")
}

// ================================ Resumption Tests ================================

// delayedSequentialFileBatchProducer wraps SequentialFileBatchProducer and adds a delay in NextBatch()
// to control timing in tests.
type delayedSequentialFileBatchProducer struct {
	*SequentialFileBatchProducer
	sleepDuration time.Duration
}

func (d *delayedSequentialFileBatchProducer) NextBatch() (*Batch, error) {
	if d.sleepDuration > 0 {
		time.Sleep(d.sleepDuration)
	}
	return d.SequentialFileBatchProducer.NextBatch()
}

func TestRandomBatchProducer_Resumption_PartialBatchesProduced_NoneConsumed(t *testing.T) {
	// Create a file with 10 batches (20 rows, 2 rows per batch)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create file with 20 data rows (10 batches of 2 rows each)
	fileContents := "id,val\n"
	for i := 1; i <= 20; i++ {
		fileContents += fmt.Sprintf("%d,value%d\n", i, i)
	}

	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create sequential producer
	sequentialProducer, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	// Wrap it with a delayed version that sleeps 100ms per batch
	// This ensures we have time to observe partial batches
	delayedSequential := &delayedSequentialFileBatchProducer{
		SequentialFileBatchProducer: sequentialProducer,
		sleepDuration:               500 * time.Millisecond,
	}

	// Create random producer using the delayed sequential producer
	producer := newRandomFileBatchProducer(delayedSequential, task)

	// Wait for some batches to be produced (but not all)
	// We want to ensure we have partial batches (more than 0, less than 10)
	// Don't consume any batches yet - we want to test resumption with batches in memory
	_ = waitForPartialBatchesProduced(t, producer, 3, 7, 5*time.Second)

	// Note: The batches produced are saved to state(disk) by SequentialFileBatchProducer.
	// When we close, the batches in memory (sequentiallyProducedBatches) are lost,
	// but the batches already saved to state(disk) will be recovered on resumption.

	// Close the producer to simulate interruption
	producer.Close()

	// Create a new producer (simulating resumption)
	// This should recover the batches that were already produced
	sequentialProducer2, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	producer2 := newRandomFileBatchProducer(sequentialProducer2, task)
	defer producer2.Close()

	// Collect all batches from the resumed producer
	allBatches := consumeAllBatches(t, producer2, 5*time.Second)
	verifyBatchesComplete(t, allBatches, 10, 0, 10)

	verifyProducerFinalState(t, producer2)
}

func TestRandomBatchProducer_Resumption_PartialBatchesProduced_PartialConsumed(t *testing.T) {
	// Create a file with 10 batches (20 rows, 2 rows per batch)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create file with 20 data rows (10 batches of 2 rows each)
	fileContents := "id,val\n"
	for i := 1; i <= 20; i++ {
		fileContents += fmt.Sprintf("%d,value%d\n", i, i)
	}

	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create sequential producer
	sequentialProducer, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	// Wrap it with a delayed version that sleeps 100ms per batch
	// This ensures we have time to observe partial batches
	delayedSequential := &delayedSequentialFileBatchProducer{
		SequentialFileBatchProducer: sequentialProducer,
		sleepDuration:               500 * time.Millisecond,
	}

	// Create random producer using the delayed sequential producer
	producer := newRandomFileBatchProducer(delayedSequential, task)

	// Wait for some batches to be produced (but not all)
	// We want to ensure we have partial batches (more than 0, less than 10)
	// Don't consume any batches yet - we want to test resumption with batches in memory
	_ = waitForPartialBatchesProduced(t, producer, 3, 7, 5*time.Second)

	// Note: The batches produced are saved to state(disk) by SequentialFileBatchProducer.
	// When we close, the batches in memory (sequentiallyProducedBatches) are lost,
	// but the batches already saved to state(disk) will be recovered on resumption.

	// Close the producer to simulate interruption
	producer.Close()

	// mark some (at random) of the produced batches as consumed
	consumedBatchNumbers := make(map[int64]bool)
	for i := range len(producer.sequentiallyProducedBatches) {
		if i%2 == 0 {
			continue
		}
		batch := producer.sequentiallyProducedBatches[i]
		consumedBatchNumbers[batch.Number] = true
		err = batch.MarkInProgress()
		require.NoError(t, err)
		err = batch.MarkDone()
		require.NoError(t, err)
	}

	// Create a new producer (simulating resumption)
	// This should recover the batches that were already produced
	sequentialProducer2, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	producer2 := newRandomFileBatchProducer(sequentialProducer2, task)
	defer producer2.Close()

	// Collect all batches from the resumed producer
	allBatches := consumeAllBatches(t, producer2, 5*time.Second)
	verifyBatchesWithPreviousConsumption(t, allBatches, consumedBatchNumbers, 10, 0, 10)

	verifyProducerFinalState(t, producer2)
}

func TestRandomBatchProducer_Resumption_PartialBatchesProduced_AllConsumed(t *testing.T) {
	// Create a file with 10 batches (20 rows, 2 rows per batch)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create file with 20 data rows (10 batches of 2 rows each)
	fileContents := "id,val\n"
	for i := 1; i <= 20; i++ {
		fileContents += fmt.Sprintf("%d,value%d\n", i, i)
	}

	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create sequential producer
	sequentialProducer, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	// Wrap it with a delayed version that sleeps 100ms per batch
	// This ensures we have time to observe partial batches
	delayedSequential := &delayedSequentialFileBatchProducer{
		SequentialFileBatchProducer: sequentialProducer,
		sleepDuration:               500 * time.Millisecond,
	}

	// Create random producer using the delayed sequential producer
	producer := newRandomFileBatchProducer(delayedSequential, task)

	// Wait for some batches to be produced (but not all)
	// We want to ensure we have partial batches (more than 0, less than 10)
	// Don't consume any batches yet - we want to test resumption with batches in memory
	_ = waitForPartialBatchesProduced(t, producer, 3, 7, 5*time.Second)

	// Note: The batches produced are saved to state(disk) by SequentialFileBatchProducer.
	// When we close, the batches in memory (sequentiallyProducedBatches) are lost,
	// but the batches already saved to state(disk) will be recovered on resumption.

	// Close the producer to simulate interruption
	producer.Close()

	// mark all the produced batches as consumed
	consumedBatchNumbers := make(map[int64]bool)
	for _, batch := range producer.sequentiallyProducedBatches {
		consumedBatchNumbers[batch.Number] = true
		err = batch.MarkInProgress()
		require.NoError(t, err)
		err = batch.MarkDone()
		require.NoError(t, err)
	}

	// Create a new producer (simulating resumption)
	// This should recover the batches that were already produced
	sequentialProducer2, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	producer2 := newRandomFileBatchProducer(sequentialProducer2, task)
	defer producer2.Close()

	// Collect all batches from the resumed producer
	allBatches := consumeAllBatches(t, producer2, 5*time.Second)
	verifyBatchesWithPreviousConsumption(t, allBatches, consumedBatchNumbers, 10, 0, 10)

	verifyProducerFinalState(t, producer2)
}

func TestRandomBatchProducer_Resumption_SingleBatchProduced_NotConsumed(t *testing.T) {
	// Create a file with 10 batches (20 rows, 2 rows per batch)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create file with 1 data rows (1 batches)
	fileContents := "id,val\n"
	for i := 1; i <= 1; i++ {
		fileContents += fmt.Sprintf("%d,value%d\n", i, i)
	}
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create sequential producer

	producer, err := NewRandomFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	// Wait for all batches to be produced
	// The sequential producer is done when it has finished producing all batches
	for !producer.sequentialFileBatchProducer.Done() {
		time.Sleep(50 * time.Millisecond)
	}

	// Verify 1 batch is produced
	producer.mu.Lock()
	batchesProduced := len(producer.sequentiallyProducedBatches)
	producer.mu.Unlock()
	require.Equal(t, 1, batchesProduced, "Should have 1 batch produced")

	// Verify producer is not done yet (batches are in memory but not consumed)
	assert.False(t, producer.Done(), "Producer should not be done yet (batches in memory)")

	// Note: The batches produced are saved to state(disk) by SequentialFileBatchProducer.
	// When we close, the batches in memory (sequentiallyProducedBatches) are lost,
	// but the batches already saved to state(disk) will be recovered on resumption.

	// Close the producer to simulate interruption
	producer.Close()

	// Create a new producer (simulating resumption)
	// This should recover the batche that were already produced

	producer2, err := NewRandomFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)
	defer producer2.Close()

	// Collect all batches from the resumed producer
	allBatches := consumeAllBatches(t, producer2, 5*time.Second)

	// Verify we got all 10 batches
	require.Equal(t, 1, len(allBatches), "Should have recovered all 10 batches")

	// Verify batch number is 0 (last batch)
	require.Equal(t, int64(0), allBatches[0].Number, "Batch number should be 1")

	verifyProducerFinalState(t, producer2)
}

func TestRandomBatchProducer_Resumption_AllBatchesProduced_NoneConsumed(t *testing.T) {
	// Create a file with 10 batches (20 rows, 2 rows per batch)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create file with 20 data rows (10 batches of 2 rows each)
	fileContents := "id,val\n"
	for i := 1; i <= 20; i++ {
		fileContents += fmt.Sprintf("%d,value%d\n", i, i)
	}

	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create sequential producer
	sequentialProducer, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	// Create random producer
	producer := newRandomFileBatchProducer(sequentialProducer, task)

	// Wait for all batches to be produced
	// The sequential producer is done when it has finished producing all batches
	for !producer.sequentialFileBatchProducer.Done() {
		time.Sleep(50 * time.Millisecond)
	}

	// Verify all batches are produced
	producer.mu.Lock()
	batchesProduced := len(producer.sequentiallyProducedBatches)
	producer.mu.Unlock()
	require.Equal(t, 10, batchesProduced, "Should have all 10 batches produced")

	// Verify producer is not done yet (batches are in memory but not consumed)
	assert.False(t, producer.Done(), "Producer should not be done yet (batches in memory)")

	// Note: The batches produced are saved to state(disk) by SequentialFileBatchProducer.
	// When we close, the batches in memory (sequentiallyProducedBatches) are lost,
	// but the batches already saved to state(disk) will be recovered on resumption.

	// Close the producer to simulate interruption
	producer.Close()

	// Create a new producer (simulating resumption)
	// This should recover all the batches that were already produced
	sequentialProducer2, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	producer2 := newRandomFileBatchProducer(sequentialProducer2, task)
	defer producer2.Close()

	// Collect all batches from the resumed producer
	allBatches := consumeAllBatches(t, producer2, 5*time.Second)
	verifyBatchesComplete(t, allBatches, 10, 0, 10)

	verifyProducerFinalState(t, producer2)
}

func TestRandomBatchProducer_Resumption_AllBatchesProduced_PartialConsumed(t *testing.T) {
	// Create a file with 10 batches (20 rows, 2 rows per batch)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create file with 20 data rows (10 batches of 2 rows each)
	fileContents := "id,val\n"
	for i := 1; i <= 20; i++ {
		fileContents += fmt.Sprintf("%d,value%d\n", i, i)
	}

	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create sequential producer
	sequentialProducer, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	// Create random producer
	producer := newRandomFileBatchProducer(sequentialProducer, task)

	// Wait for all batches to be produced
	// The sequential producer is done when it has finished producing all batches
	for !producer.sequentialFileBatchProducer.Done() {
		time.Sleep(50 * time.Millisecond)
	}

	// Verify all batches are produced
	producer.mu.Lock()
	batchesProduced := len(producer.sequentiallyProducedBatches)
	producer.mu.Unlock()
	require.Equal(t, 10, batchesProduced, "Should have all 10 batches produced")

	// Verify producer is not done yet (batches are in memory but not consumed)
	assert.False(t, producer.Done(), "Producer should not be done yet (batches in memory)")

	// Note: The batches produced are saved to state(disk) by SequentialFileBatchProducer.
	// When we close, the batches in memory (sequentiallyProducedBatches) are lost,
	// but the batches already saved to state(disk) will be recovered on resumption.

	// Close the producer to simulate interruption
	producer.Close()

	// mark 3 as consumed
	consumedBatchesNumbers := make(map[int64]bool)
	for i := 0; i < 3; i++ {
		batch := producer.sequentiallyProducedBatches[i]
		err = batch.MarkInProgress()
		require.NoError(t, err)
		err = batch.MarkDone()
		require.NoError(t, err)
		consumedBatchesNumbers[batch.Number] = true
	}

	// Create a new producer (simulating resumption)
	// This should recover all the batches that were already produced
	sequentialProducer2, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	producer2 := newRandomFileBatchProducer(sequentialProducer2, task)
	defer producer2.Close()

	// Collect all batches from the resumed producer
	allBatches := consumeAllBatches(t, producer2, 5*time.Second)
	verifyBatchesWithPreviousConsumption(t, allBatches, consumedBatchesNumbers, 10, 0, 10)

	verifyProducerFinalState(t, producer2)
}

func TestRandomBatchProducer_Resumption_AllBatchesProduced_AllConsumed(t *testing.T) {
	// Create a file with 10 batches (20 rows, 2 rows per batch)
	ldataDir, lexportDir, state, errorHandler, progressReporter, err := setupExportDirAndImportDependencies(2, 1024)
	require.NoError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// Create file with 20 data rows (10 batches of 2 rows each)
	fileContents := "id,val\n"
	for i := 1; i <= 20; i++ {
		fileContents += fmt.Sprintf("%d,value%d\n", i, i)
	}

	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table", 1)
	require.NoError(t, err)

	// Create sequential producer
	sequentialProducer, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)

	// Create random producer
	producer := newRandomFileBatchProducer(sequentialProducer, task)

	// Wait for all batches to be produced
	// The sequential producer is done when it has finished producing all batches
	for !producer.sequentialFileBatchProducer.Done() {
		time.Sleep(50 * time.Millisecond)
	}

	// Verify all batches are produced
	producer.mu.Lock()
	batchesProduced := len(producer.sequentiallyProducedBatches)
	producer.mu.Unlock()
	require.Equal(t, 10, batchesProduced, "Should have all 10 batches produced")

	// Verify producer is not done yet (batches are in memory but not consumed)
	assert.False(t, producer.Done(), "Producer should not be done yet (batches in memory)")

	// Note: The batches produced are saved to state(disk) by SequentialFileBatchProducer.
	// When we close, the batches in memory (sequentiallyProducedBatches) are lost,
	// but the batches already saved to state(disk) will be recovered on resumption.

	// Close the producer to simulate interruption
	producer.Close()

	// mark ALL the produced batches as consumed
	consumedBatchesNumbers := make(map[int64]bool)
	for _, batch := range producer.sequentiallyProducedBatches {
		consumedBatchesNumbers[batch.Number] = true
		err = batch.MarkInProgress()
		require.NoError(t, err)
		err = batch.MarkDone()
		require.NoError(t, err)
	}

	// Create a new producer (simulating resumption)
	// This should result in an error because all batches were already produced and consumed
	var errExitCalled bool
	utils.MonkeyPatchUtilsErrExit(func(formatString string, args ...interface{}) {
		errExitCalled = true
	})
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
	})
	sequentialProducer2, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	require.NoError(t, err)
	producer2 := newRandomFileBatchProducer(sequentialProducer2, task)
	defer producer2.Close()

	deadline := time.Now().Add(5 * time.Second)
	for !errExitCalled && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, errExitCalled, "utils.ErrExit should be called within timeout")
}
