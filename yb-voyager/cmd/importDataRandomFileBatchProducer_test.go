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
