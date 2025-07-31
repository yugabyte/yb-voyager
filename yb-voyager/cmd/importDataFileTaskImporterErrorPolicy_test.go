//go:build integration

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
	"testing"

	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func getErrorsParentDir(lexportDir string) string {
	return filepath.Join(lexportDir, "data")
}

func cleanupExportDirDataDir(ldataDir, lexportDir string) {
	if ldataDir != "" {
		os.RemoveAll(ldataDir)
	}
	if lexportDir != "" {
		os.RemoveAll(lexportDir)
	}
}

func getBatchErrorBaseFilePath(batchBaseFilePath string) string {
	return fmt.Sprintf("ingestion-error.%s", batchBaseFilePath)
}

func assertTableRowCount(t *testing.T, tableName string, expectedCount int64) {
	var rowCount int64
	err := tdb.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s", tableName)).Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, expectedCount, rowCount, fmt.Sprintf("Expected row count for table %s to be %d", tableName, expectedCount))
}

func assertTableIds(t *testing.T, tableName string, expectedIds []int64) {
	var ids []int64
	rows, err := tdb.Query(fmt.Sprintf("SELECT id FROM %s", tableName))
	defer rows.Close()
	assert.NoError(t, err)

	for rows.Next() {
		var id int64
		err = rows.Scan(&id)
		testutils.FatalIfError(t, err)
		ids = append(ids, id)
	}
	assert.ElementsMatch(t, expectedIds, ids, fmt.Sprintf("Expected IDs for table %s do not match", tableName))
}

func assertBatchErrored(t *testing.T, batch *Batch, expectedRecordCount int64, expectedFileName string) {
	assert.Equal(t, expectedRecordCount, batch.RecordCount, "Expected record count in errored batch to match")
	batchBaseFilePath := filepath.Base(batch.GetFilePath())
	assert.Equal(t, expectedFileName, batchBaseFilePath, "Expected batch file name to match")
}

func assertBatchErrorFileContents(t *testing.T, batch *Batch, lexportDir string, state *ImportDataState, task *ImportFileTask, rows string, expectedErrorSubstring string) {
	taskFolderPath := fmt.Sprintf("file::%s:%s", filepath.Base(task.FilePath), importdata.ComputePathHash(task.FilePath))
	tableFolderPath := fmt.Sprintf("table::%s", task.TableNameTup.ForMinOutput())
	batchErrorBaseFilePath := getBatchErrorBaseFilePath(filepath.Base(batch.GetFilePath()))
	batchErrorFilePath := filepath.Join(getErrorsParentDir(lexportDir), "errors", tableFolderPath, taskFolderPath, batchErrorBaseFilePath)
	errorFileContentsBytes, err := os.ReadFile(batchErrorFilePath)
	assert.NoError(t, err)
	errorFileContents := string(errorFileContentsBytes)

	assert.Containsf(t, errorFileContents, rows, "file does not contain expected row data")
	assert.Containsf(t, errorFileContents, expectedErrorSubstring, "file does not contain expected error message")
}

func TestBasicTaskImportStachAndContinueErrorPolicy(t *testing.T) {
	ldataDir, lexportDir, state, _, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)
	// t.Cleanup(func() { cleanupExportDirDataDir(ldataDir, lexportDir) })

	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_error (id INT PRIMARY KEY, val TEXT);`,
		`INSERT INTO test_table_error VALUES (3, 'three');`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_error;`)

	// file import
	// gets divided into two batches. (1, 2), (3,4)
	// second batch (with row id 3) should fail with error (PK violation)
	fileContents := `id,val
1, "hello"
2, "world"
3, "three"
4, "four"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_error", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, scErrorHandler)
	testutils.FatalIfError(t, err)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		assert.NoError(t, err)
	}
	workerPool.Wait()

	assertTableRowCount(t, "test_table_error", 3)
	assertTableIds(t, "test_table_error", []int64{1, 2, 3})

	erroredBatches, err := state.GetErroredBatches(task.FilePath, task.TableNameTup)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(erroredBatches), "Expected one errored batch")

	assertBatchErrored(t, erroredBatches[0], 2, "batch::0.4.2.20.E")
	assertBatchErrorFileContents(t, erroredBatches[0], lexportDir, state, task,
		`id,val
3, "three"
4, "four"`,
		`ERROR: duplicate key value violates unique constraint "test_table_error_pkey" (SQLSTATE 23505)`)
}

func TestTaskImportStachAndContinueErrorPolicy_NoErrors(t *testing.T) {
	ldataDir, lexportDir, state, _, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)
	t.Cleanup(func() { cleanupExportDirDataDir(ldataDir, lexportDir) })

	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_error (id INT PRIMARY KEY, val TEXT);`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_error;`)

	// file import
	// gets divided into two batches. (1, 2), (3,4)
	// no errors expected
	fileContents := `id,val
1, "hello"
2, "world"
3, "three"
4, "four"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_error", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, scErrorHandler)
	testutils.FatalIfError(t, err)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		assert.NoError(t, err)
	}
	workerPool.Wait()

	assertTableRowCount(t, "test_table_error", 4)
	assertTableIds(t, "test_table_error", []int64{1, 2, 3, 4})

	erroredBatches, err := state.GetErroredBatches(task.FilePath, task.TableNameTup)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(erroredBatches), "Expected 0 errored batch")
}

func TestTaskImportStachAndContinueErrorPolicy_SingleBatchWithError(t *testing.T) {
	ldataDir, lexportDir, state, _, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)
	t.Cleanup(func() { cleanupExportDirDataDir(ldataDir, lexportDir) })

	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_error (id INT PRIMARY KEY, val TEXT);`,
		`INSERT INTO test_table_error VALUES (2, 'two');`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_error;`)

	// file import
	// gets divided into one batches. (1, 2)
	// the single batch will have an error.
	fileContents := `id,val
1, "hello"
2, "world"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_error", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, scErrorHandler)
	testutils.FatalIfError(t, err)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		assert.NoError(t, err)
	}
	workerPool.Wait()

	assertTableRowCount(t, "test_table_error", 1)
	assertTableIds(t, "test_table_error", []int64{2})

	erroredBatches, err := state.GetErroredBatches(task.FilePath, task.TableNameTup)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(erroredBatches), "Expected one errored batch")

	assertBatchErrored(t, erroredBatches[0], 2, "batch::0.2.2.28.E")
	assertBatchErrorFileContents(t, erroredBatches[0], lexportDir, state, task,
		`id,val
1, "hello"
2, "world"`,
		`ERROR: duplicate key value violates unique constraint "test_table_error_pkey" (SQLSTATE 23505)`)
}

func TestTaskImportStachAndContinueErrorPolicy_SingleBatch_OnPkConflictIgnore(t *testing.T) {
	ldataDir, lexportDir, state, _, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)
	t.Cleanup(func() { cleanupExportDirDataDir(ldataDir, lexportDir) })

	setupYugabyteTestDb(t)
	tconf.OnPrimaryKeyConflictAction = constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE
	t.Cleanup(func() {
		tconf.OnPrimaryKeyConflictAction = constants.PRIMARY_KEY_CONFLICT_ACTION_ERROR
	})

	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_unique_error (id INT PRIMARY KEY, email TEXT UNIQUE, val TEXT);`,
		`INSERT INTO test_table_unique_error VALUES (1000, 'abc@def.com', 'one');`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_unique_error;`)

	// file import
	// gets divided into one batch
	// the single batch will have a unique key violation on email (abc@def.com)
	fileContents := `id,val,email
1,"abc@def.com","hello"
2,"ghi@jkl.com","world"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_unique_error", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, scErrorHandler)
	testutils.FatalIfError(t, err)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		assert.NoError(t, err)
	}
	workerPool.Wait()

	assertTableRowCount(t, "test_table_unique_error", 1)
	assertTableIds(t, "test_table_unique_error", []int64{1000})

	erroredBatches, err := state.GetErroredBatches(task.FilePath, task.TableNameTup)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(erroredBatches), "Expected one errored batch")

	assertBatchErrored(t, erroredBatches[0], 2, "batch::0.2.2.60.E")
	assertBatchErrorFileContents(t, erroredBatches[0], lexportDir, state, task,
		fileContents,
		`ERROR: duplicate key value violates unique constraint "test_table_unique_error_email_key" (SQLSTATE 23505)`,
	)

	assertBatchErrorFileContents(t, erroredBatches[0], lexportDir, state, task,
		fileContents,
		PARTIAL_BATCH_ERROR_NOTE,
	)
}

func TestTaskImportStachAndContinueErrorPolicy_MultipleBatchesWithDifferentErrors(t *testing.T) {
	COPY_MAX_RETRY_COUNT = 1 // Disable retry for COPY command to test error handling
	ldataDir, lexportDir, state, _, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)
	t.Cleanup(func() { cleanupExportDirDataDir(ldataDir, lexportDir) })

	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_error (id INT PRIMARY KEY, val TEXT, not_null_col TEXT NOT NULL, num_col INT, fixed_col char(2));`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_error;`)

	// file import
	// gets divided into five batches (2 per batch)
	// first batch has error (string into an int column)
	// third batch has an error (null into a not null column)
	// fifth batch has an error (more than 2 chars into a fixed char[2] column)
	// second and fourth batches should be imported successfully.
	fileContents := `id,val,not_null_col,num_col,fixed_col
1,"hello","abc",10,"ab"
2,"world","xyz","xyz","cd"
3,"foo","bar",20,"ef"
4,"baz","qux",30,"gh"
5,"quux",NULL,40,"ij"
6,"corge","grault",50,"mn"
7,"garply","waldo",60,"op"
8,"fred","plugh",70,"qr"
9,"xyzzy","thud",80,"st"
10,"wibble","wobble",90,"uvwxyz"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_error", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, scErrorHandler)
	testutils.FatalIfError(t, err)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		assert.NoError(t, err)
	}
	workerPool.Wait()

	assertTableRowCount(t, "test_table_error", 4)
	assertTableIds(t, "test_table_error", []int64{3, 4, 7, 8})

	erroredBatches, err := state.GetErroredBatches(task.FilePath, task.TableNameTup)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(erroredBatches), "Expected three errored batch")

	tgtYBVersion := testutils.GetYBVersionFromTestContainer(t, testYugabyteDBTarget.TestContainer)

	assertBatchErrored(t, erroredBatches[1], 2, "batch::1.2.2.89.E")
	errorMsg := `ERROR: invalid input syntax for integer: "xyz" (SQLSTATE 22P02)`
	if tgtYBVersion.ReleaseType() == ybversion.V2025_1_0_0.ReleaseType() && tgtYBVersion.GreaterThanOrEqual(ybversion.V2025_1_0_0) {
		errorMsg = `ERROR: invalid input syntax for type integer: "xyz" (SQLSTATE 22P02)`
	}
	assertBatchErrorFileContents(t, erroredBatches[1], lexportDir, state, task,
		`id,val,not_null_col,num_col,fixed_col
1,"hello","abc",10,"ab"
2,"world","xyz","xyz","cd"`,
		errorMsg)

	assertBatchErrored(t, erroredBatches[2], 2, "batch::3.6.2.49.E")
	errorMsg = `ERROR: null value in column "not_null_col" violates not-null constraint (SQLSTATE 23502)`
	if tgtYBVersion.ReleaseType() == ybversion.V2025_1_0_0.ReleaseType() && tgtYBVersion.GreaterThanOrEqual(ybversion.V2025_1_0_0) {
		errorMsg = `ERROR: null value in column "not_null_col" of relation "test_table_error" violates not-null constraint (SQLSTATE 23502)`
	}
	assertBatchErrorFileContents(t, erroredBatches[2], lexportDir, state, task,
		`id,val,not_null_col,num_col,fixed_col
5,"quux",NULL,40,"ij"
6,"corge","grault",50,"mn"`,
		errorMsg)

	assertBatchErrored(t, erroredBatches[0], 2, "batch::0.10.2.57.E")
	assertBatchErrorFileContents(t, erroredBatches[0], lexportDir, state, task,
		`id,val,not_null_col,num_col,fixed_col
9,"xyzzy","thud",80,"st"
10,"wibble","wobble",90,"uvwxyz"`,
		`ERROR: value too long for type character(2) (SQLSTATE 22001)`)
}

func TestTaskImportStachAndContinueErrorPolicy_TaskResumptionAfterBatchError(t *testing.T) {
	ldataDir, lexportDir, state, _, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)
	scErrorHandler, err := importdata.GetImportDataErrorHandler(importdata.StashAndContinueErrorPolicy, getErrorsParentDir(lexportDir))
	testutils.FatalIfError(t, err)
	t.Cleanup(func() { cleanupExportDirDataDir(ldataDir, lexportDir) })

	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_error (id INT PRIMARY KEY, val TEXT);`,
		`INSERT INTO test_table_error VALUES (2, 'two');`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_error;`)

	// file import
	// gets divided into two batches (1,2), (3,4)
	// first batch (1,2) should have an error.
	// after resumption, the second batch (3,4) should be processed.
	fileContents := `id,val
1,"hello"
2,"world"
3,"three"
4,"four"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_error", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, scErrorHandler)
	testutils.FatalIfError(t, err)

	// ingest first batch.
	err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
	assert.NoError(t, err)
	workerPool.Wait()

	assertTableRowCount(t, "test_table_error", 1)
	erroredBatches, err := state.GetErroredBatches(task.FilePath, task.TableNameTup)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(erroredBatches), "Expected one errored batch")

	assertBatchErrored(t, erroredBatches[0], 2, "batch::1.2.2.27.E")
	assertBatchErrorFileContents(t, erroredBatches[0], lexportDir, state, task,
		`id,val
1,"hello"
2,"world"`,
		`ERROR: duplicate key value violates unique constraint "test_table_error_pkey" (SQLSTATE 23505)`)

	// simulate resumption
	progressReporter = NewImportDataProgressReporter(true)
	workerPool = pool.New().WithMaxGoroutines(2)
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, scErrorHandler)
	testutils.FatalIfError(t, err)

	// ingest second batch. This should not retry the first errored-out batch.
	err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
	assert.NoError(t, err)
	workerPool.Wait()
	assertTableRowCount(t, "test_table_error", 3)
	erroredBatches, err = state.GetErroredBatches(task.FilePath, task.TableNameTup)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(erroredBatches), "Expected one errored batch after resumption")
}
