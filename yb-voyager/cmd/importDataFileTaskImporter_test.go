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
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

type TestDB struct {
	testcontainers.TestContainer
	tgtdb.TargetDB
}

var testYugabyteDBTarget *TestDB

func setupYugabyteTestDb(t *testing.T) {
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err := yugabytedbContainer.Start(context.Background())
	testutils.FatalIfError(t, err)
	host, port, err := yugabytedbContainer.GetHostPort()
	testutils.FatalIfError(t, err)
	testYugabyteDBTarget := &TestDB{
		TestContainer: yugabytedbContainer,
		TargetDB: tgtdb.NewTargetDB(&tgtdb.TargetConf{
			TargetDBType: "yugabytedb",
			DBVersion:    yugabytedbContainer.GetConfig().DBVersion,
			User:         yugabytedbContainer.GetConfig().User,
			Password:     yugabytedbContainer.GetConfig().Password,
			Schema:       yugabytedbContainer.GetConfig().Schema,
			DBName:       yugabytedbContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
		}),
	}
	testYugabyteDBTarget.TestContainer.ExecuteSqls(
		`CREATE TABLE test_table (id INT PRIMARY KEY, val TEXT);`,
	)

	tdb = testYugabyteDBTarget.TargetDB
	err = tdb.Init()
	testutils.FatalIfError(t, err)
	err = tdb.CreateVoyagerSchema()
	testutils.FatalIfError(t, err)
	err = tdb.InitConnPool()
	testutils.FatalIfError(t, err)
}

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
