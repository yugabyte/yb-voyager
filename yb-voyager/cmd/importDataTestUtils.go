//go:build unit || integration || integration_voyager_command

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
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

type dummyTDB struct {
	maxSizeBytes int64
	tgtdb.TargetYugabyteDB
}

func (d *dummyTDB) MaxBatchSizeInBytes() int64 {
	return d.maxSizeBytes
}

func setupExportDirAndImportDependencies(batchSizeRows int64, batchSizeBytes int64) (string, string, *ImportDataState, importdata.ImportDataErrorHandler, *ImportDataProgressReporter, error) {
	lexportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	if err != nil {
		return "", "", nil, nil, nil, err
	}

	ldataDir, err := os.MkdirTemp("/tmp", "data-dir-*")
	if err != nil {
		return "", "", nil, nil, nil, err
	}

	metaDB = CreateMigrationProjectIfNotExists(constants.POSTGRESQL, lexportDir)
	tdb = &dummyTDB{maxSizeBytes: batchSizeBytes}
	valueConverter = &dbzm.SnapshotPhaseNoOpValueConverter{}
	dataStore = datastore.NewDataStore(ldataDir)

	batchSizeInNumRows = batchSizeRows

	state := NewImportDataState(lexportDir)
	TableNameToSchema = utils.NewStructMap[sqlname.NameTuple, map[string]map[string]string]()
	importerRole = TARGET_DB_IMPORTER_ROLE

	errorHandler, err := importdata.GetImportDataErrorHandler(importdata.AbortErrorPolicy, filepath.Join(lexportDir, "data"), importerRole)

	if err != nil {
		return "", "", nil, nil, nil, err
	}
	progressReporter := NewImportDataProgressReporter(true)

	return ldataDir, lexportDir, state, errorHandler, progressReporter, nil
}

func createFileAndTask(lexportDir string, fileContents string, ldataDir string, tableName string, id int) (string, *ImportFileTask, error) {
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
		ID:           id,
		FilePath:     tempFile,
		TableNameTup: tableNameTup,
		RowCount:     1,
	}
	return tempFile, task, nil
}
