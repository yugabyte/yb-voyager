//go:build unit || integration || cdc_benchmark

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
package importdata

import (
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
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

// testImporterRole is the importer role every component test in this package runs as.
const testImporterRole = constants.TARGET_DB_IMPORTER_ROLE

// Test fixture shared by the component tests in this package. It stands in for the
// cmd package-level variables the components used to read directly; every test's
// setupExportDirAndImportDependencies call resets it.
var (
	testStateCfg    ImportDataStateConfig
	testProducerCfg SequentialFileBatchProducerConfig
	testImporterCfg FileTaskImporterConfig
)

// testImporter stands in for the cmd package-level variables the streaming and
// cdc-partition-key tests used to set directly (importerRole, tdb, cdcPartitionKey,
// ...). Like those globals it lives for the whole test binary; tests set what they
// need and restore it in cleanups, as before.
var testImporter = NewImporter(Config{})

// setTestCdcPartitionKeyOverrides sets the raw --cdc-partition-key-overrides value
// together with its parsed form (in production cmd parses the flag once and passes
// both in Config).
func setTestCdcPartitionKeyOverrides(raw string, parsed map[string]CdcPartitionKeyOverride) {
	testImporter.cfg.CdcPartitionKeyOverrides = raw
	testImporter.cfg.CdcPartitionKeyOverridesParsed = parsed
}

func testDataFileDescriptor(lexportDir string) *datafile.Descriptor {
	return &datafile.Descriptor{
		FileFormat: "csv",
		Delimiter:  ",",
		HasHeader:  true,
		ExportDir:  lexportDir,
		QuoteChar:  '"',
		EscapeChar: '\\',
		NullString: "NULL",
	}
}

// initTestMetaDB creates the export-dir skeleton and metadata DB the way cmd's
// CreateMigrationProjectIfNotExists + initMetaDB do for these tests (minus the
// schema object dirs and the anonymizer, which no component here reads).
func initTestMetaDB(exportDir string) (*metadb.MetaDB, error) {
	for _, subdir := range []string{
		"schema", "data", "reports",
		"assessment", "assessment/metadata", "assessment/dbs", "assessment/metadata/schema", "assessment/reports",
		"metainfo", "metainfo/data", "metainfo/conf", "metainfo/ssl",
		"temp", "temp/ora2pg_temp_dir", "temp/schema",
	} {
		if err := os.MkdirAll(filepath.Join(exportDir, subdir), 0755); err != nil {
			return nil, err
		}
	}
	if err := metadb.CreateAndInitMetaDBIfRequired(exportDir); err != nil {
		return nil, err
	}
	m, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return nil, err
	}
	if err := m.InitMigrationStatusRecord(""); err != nil {
		return nil, err
	}
	if err := m.InitImportDataStatusRecord(); err != nil {
		return nil, err
	}
	if err := m.InitImportDataFileStatusRecord(); err != nil {
		return nil, err
	}
	return m, nil
}

func setupExportDirAndImportDependencies(batchSizeRows int64, batchSizeBytes int64) (string, string, *ImportDataState, ImportDataErrorHandler, *ImportDataProgressReporter, error) {
	lexportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	if err != nil {
		return "", "", nil, nil, nil, err
	}

	ldataDir, err := os.MkdirTemp("/tmp", "data-dir-*")
	if err != nil {
		return "", "", nil, nil, nil, err
	}

	_, err = initTestMetaDB(lexportDir)
	if err != nil {
		return "", "", nil, nil, nil, err
	}
	tdb := &dummyTDB{maxSizeBytes: batchSizeBytes}
	dfd := testDataFileDescriptor(lexportDir)
	tableToColumnNames := utils.NewStructMap[sqlname.NameTuple, []string]()

	testStateCfg = ImportDataStateConfig{
		ExportDir:          lexportDir,
		ImporterRole:       testImporterRole,
		Tdb:                tdb,
		DataFileDescriptor: dfd,
	}
	testProducerCfg = SequentialFileBatchProducerConfig{
		ImporterRole:       testImporterRole,
		Tdb:                tdb,
		DataFileDescriptor: dfd,
		DataStore:          datastore.NewDataStore(ldataDir),
		BatchSizeInNumRows: batchSizeRows,
		ValueConverter:     &dbzm.SnapshotPhaseNoOpValueConverter{},
		TableToColumnNames: tableToColumnNames,
	}
	testImporterCfg = FileTaskImporterConfig{
		ImporterRole:       testImporterRole,
		Tdb:                tdb,
		DataFileDescriptor: dfd,
		TableToColumnNames: tableToColumnNames,
		TableNameToSchema:  utils.NewStructMap[sqlname.NameTuple, map[string]map[string]string](),
	}

	state := NewImportDataState(testStateCfg)

	errorHandler, err := GetImportDataErrorHandler(AbortErrorPolicy, filepath.Join(lexportDir, "data"), testImporterRole)

	if err != nil {
		return "", "", nil, nil, nil, err
	}
	progressReporter := NewImportDataProgressReporter(true)

	return ldataDir, lexportDir, state, errorHandler, progressReporter, nil
}

func createFileAndTask(lexportDir string, fileContents string, ldataDir string, tableName string, id int) (string, *ImportFileTask, error) {
	dfd := testDataFileDescriptor(lexportDir)
	testStateCfg.DataFileDescriptor = dfd
	testProducerCfg.DataFileDescriptor = dfd
	testImporterCfg.DataFileDescriptor = dfd
	tempFile, err := testutils.CreateTempFile(ldataDir, fileContents, dfd.FileFormat)
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
