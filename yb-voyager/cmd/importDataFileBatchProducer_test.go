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

	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// In cmd package, we have 3 TestMain functions, one for each build tag(unit, integration, integration_voyager_command).
func TestMain(m *testing.M) {
	// set logging level to WARN
	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	os.Exit(exitCode)
}

func TestBasicFileBatchProducer(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
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
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
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
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(1000, maxBatchSizeBytes)
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
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(1000, 25)
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

func TestFileBatchProducerResumable(t *testing.T) {
	// max batch size in rows is 2
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
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

	batchproducer, err := NewFileBatchProducer(task, state)
	assert.NoError(t, err)
	assert.False(t, batchproducer.Done())

	// generate one batch
	batch1, err := batchproducer.NextBatch()
	assert.NoError(t, err)
	assert.NotNil(t, batch1)
	assert.Equal(t, int64(2), batch1.RecordCount)

	// simulate a crash and recover
	batchproducer, err = NewFileBatchProducer(task, state)
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
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
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

	batchproducer, err := NewFileBatchProducer(task, state)
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
	batchproducer, err = NewFileBatchProducer(task, state)
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
