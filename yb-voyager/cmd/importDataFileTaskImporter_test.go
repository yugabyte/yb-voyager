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
	"os"
	"testing"

	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestBasicTaskImport(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(ldataDir)
	}
	if lexportDir != "" {
		defer os.RemoveAll(lexportDir)
	}
	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_basic (id INT PRIMARY KEY, val TEXT);`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_basic;`)

	// file import
	fileContents := `id,val
1, "hello"
2, "world"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_basic", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter)
	testutils.FatalIfError(t, err)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		assert.NoError(t, err)
	}

	workerPool.Wait()
	var rowCount int64
	err = tdb.QueryRow("SELECT count(*) FROM test_table_basic").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rowCount)
}

func TestImportAllBatchesAndResume(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(ldataDir)
	}
	if lexportDir != "" {
		defer os.RemoveAll(lexportDir)
	}
	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_all (id INT PRIMARY KEY, val TEXT);`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_all;`)

	// file import
	fileContents := `id,val
1, "hello"
2, "world"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_all", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		assert.NoError(t, err)
	}

	workerPool.Wait()
	var rowCount int64
	err = tdb.QueryRow("SELECT count(*) FROM test_table_all").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rowCount)

	// simulate restart
	progressReporter = NewImportDataProgressReporter(true)
	workerPool = pool.New().WithMaxGoroutines(2)
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter)
	testutils.FatalIfError(t, err)

	assert.Equal(t, true, taskImporter.AllBatchesSubmitted())
	// assert.Equal(t, true, taskImporter.AllBatchesImported())
}

func TestTaskImportResumable(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(ldataDir)
	}
	if lexportDir != "" {
		defer os.RemoveAll(lexportDir)
	}
	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_resume (id INT PRIMARY KEY, val TEXT);`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_resume;`)

	// file import
	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_resume", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter)
	testutils.FatalIfError(t, err)

	// submit 1 batch
	err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
	assert.NoError(t, err)

	// check that the first batch was imported
	workerPool.Wait()
	var rowCount int64
	err = tdb.QueryRow("SELECT count(*) FROM test_table_resume").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rowCount)

	// simulate restart
	progressReporter = NewImportDataProgressReporter(true)
	workerPool = pool.New().WithMaxGoroutines(2)
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter)
	testutils.FatalIfError(t, err)

	// submit second batch, not first batch again as it was already imported
	err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
	assert.NoError(t, err)

	assert.Equal(t, true, taskImporter.AllBatchesSubmitted())
	workerPool.Wait()
	err = tdb.QueryRow("SELECT count(*) FROM test_table_resume").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), rowCount)
}

func TestTaskImportResumableNoPK(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(ldataDir)
	}
	if lexportDir != "" {
		defer os.RemoveAll(lexportDir)
	}
	setupYugabyteTestDb(t)
	defer testYugabyteDBTarget.Finalize()
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table_resume_no_pk (id INT, val TEXT);`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_resume_no_pk;`)

	// file import
	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table_resume_no_pk", 1)
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter)
	testutils.FatalIfError(t, err)

	// submit 1 batch
	err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
	assert.NoError(t, err)

	// check that the first batch was imported
	workerPool.Wait()
	var rowCount int64
	err = tdb.QueryRow("SELECT count(*) FROM test_table_resume_no_pk").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rowCount)

	// simulate restart
	progressReporter = NewImportDataProgressReporter(true)
	workerPool = pool.New().WithMaxGoroutines(2)
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter)
	testutils.FatalIfError(t, err)

	// submit second batch, not first batch again as it was already imported
	err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
	assert.NoError(t, err)

	assert.Equal(t, true, taskImporter.AllBatchesSubmitted())
	workerPool.Wait()
	err = tdb.QueryRow("SELECT count(*) FROM test_table_resume_no_pk").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), rowCount)
}
