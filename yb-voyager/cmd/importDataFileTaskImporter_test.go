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

	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestBasicTaskImport(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}
	setupYugabyteTestDb(t)

	// file import
	fileContents := `id,val
1, "hello"
2, "world"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table")
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter)

	for !taskImporter.AllBatchesSubmitted() {
		err := taskImporter.SubmitNextBatch()
		assert.NoError(t, err)
	}

	workerPool.Wait()
	var rowCount int64
	err = tdb.QueryRow("SELECT count(*) FROM test_table").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rowCount)
}

func TestTaskImportResumable(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}
	setupYugabyteTestDb(t)

	// file import
	fileContents := `id,val
1, "hello"
2, "world"
3, "foo"
4, "bar"`
	_, task, err := createFileAndTask(lexportDir, fileContents, ldataDir, "test_table")
	testutils.FatalIfError(t, err)

	progressReporter := NewImportDataProgressReporter(true)
	workerPool := pool.New().WithMaxGoroutines(2)
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter)
	testutils.FatalIfError(t, err)
	// for !taskImporter.AllBatchesSubmitted() {
	// 	err := taskImporter.SubmitNextBatch()
	// 	assert.NoError(t, err)
	// }

	// submit 1 batch
	err = taskImporter.SubmitNextBatch()
	assert.NoError(t, err)

	// simulate restart
	workerPool.Wait()
	// check that the first batch was imported
	var rowCount int64
	err = tdb.QueryRow("SELECT count(*) FROM test_table").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rowCount)

	progressReporter = NewImportDataProgressReporter(true)
	workerPool = pool.New().WithMaxGoroutines(2)
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter)

	// submit second batch, not first batch again as it was already imported
	err = taskImporter.SubmitNextBatch()
	assert.NoError(t, err)

	assert.Equal(t, true, taskImporter.AllBatchesSubmitted())
	workerPool.Wait()
	err = tdb.QueryRow("SELECT count(*) FROM test_table").Scan(&rowCount)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), rowCount)
}
