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
	"os"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

type dummyTDB struct {
	maxSizeBytes int64
	tgtdb.TargetYugabyteDB
}

func (d *dummyTDB) MaxBatchSizeInBytes() int64 {
	return d.maxSizeBytes
}

type TestTargetDB struct {
	testcontainers.TestContainer
	tgtdb.TargetDB
}

var testYugabyteDBTarget *TestTargetDB

func setupYugabyteTestDb(t *testing.T) {
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err := yugabytedbContainer.Start(context.Background())
	testutils.FatalIfError(t, err)
	host, port, err := yugabytedbContainer.GetHostPort()
	testutils.FatalIfError(t, err)
	testYugabyteDBTarget = &TestTargetDB{
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

	tdb = testYugabyteDBTarget.TargetDB
	err = tdb.Init()
	testutils.FatalIfError(t, err)
	err = tdb.CreateVoyagerSchema()
	testutils.FatalIfError(t, err)
	err = tdb.InitConnPool()
	testutils.FatalIfError(t, err)
}

func setupExportDirAndImportDependencies(batchSizeRows int64, batchSizeBytes int64) (string, string, *ImportDataState, error) {
	lexportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	if err != nil {
		return "", "", nil, err
	}

	ldataDir, err := os.MkdirTemp("/tmp", "data-dir-*")
	if err != nil {
		return "", "", nil, err
	}

	CreateMigrationProjectIfNotExists(constants.POSTGRESQL, lexportDir)
	tdb = &dummyTDB{maxSizeBytes: batchSizeBytes}
	valueConverter = &dbzm.NoOpValueConverter{}
	dataStore = datastore.NewDataStore(ldataDir)

	batchSizeInNumRows = batchSizeRows

	state := NewImportDataState(lexportDir)
	TableNameToSchema = utils.NewStructMap[sqlname.NameTuple, map[string]map[string]string]()
	return ldataDir, lexportDir, state, nil
}

func createFileAndTask(lexportDir string, fileContents string, ldataDir string, tableName string) (string, *ImportFileTask, error) {
	dataFileDescriptor = &datafile.Descriptor{
		FileFormat: "csv",
		Delimiter:  ",",
		HasHeader:  true,
		ExportDir:  lexportDir,
		QuoteChar:  '"',
		EscapeChar: '\\',
		NullString: "NULL",
	}
	tempFile, err := testutils.CreateTempFile(ldataDir, fileContents, dataFileDescriptor.FileFormat)
	if err != nil {
		return "", nil, err
	}

	sourceName := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", tableName)
	tableNameTup := sqlname.NameTuple{SourceName: sourceName, CurrentName: sourceName}
	task := &ImportFileTask{
		ID:           1,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     1,
	}
	return tempFile, task, nil
}
