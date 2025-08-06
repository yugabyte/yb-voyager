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
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestBasicTaskImport(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, err := setupExportDirAndImportDependencies(2, 1024)
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
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)
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
	ldataDir, lexportDir, state, errorHandler, err := setupExportDirAndImportDependencies(2, 1024)
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
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)

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
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)
	testutils.FatalIfError(t, err)

	assert.Equal(t, true, taskImporter.AllBatchesSubmitted())
	// assert.Equal(t, true, taskImporter.AllBatchesImported())
}

func TestTaskImportResumable(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, err := setupExportDirAndImportDependencies(2, 1024)
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
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)
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
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)
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
	ldataDir, lexportDir, state, errorHandler, err := setupExportDirAndImportDependencies(2, 1024)
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
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)
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
	taskImporter, err = NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)
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

func TestTaskImportErrorsOutWithAbortErrorPolicy(t *testing.T) {
	ldataDir, lexportDir, state, errorHandler, err := setupExportDirAndImportDependencies(2, 1024)
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
		`CREATE TABLE test_table_error (id INT PRIMARY KEY, val TEXT);`,
		`INSERT INTO test_table_error VALUES (3, 'three');`,
	)
	defer testYugabyteDBTarget.TestContainer.ExecuteSqls(`DROP TABLE test_table_error;`)

	// file import
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
	taskImporter, err := NewFileTaskImporter(task, state, workerPool, progressReporter, nil, false, errorHandler)
	testutils.FatalIfError(t, err)

	utils.MonkeyPatchUtilsErrExitWithPanic()
	t.Cleanup(utils.RestoreUtilsErrExit)
	assert.Panics(t, func() {
		for !taskImporter.AllBatchesSubmitted() {
			err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
			assert.NoError(t, err)
		}
		workerPool.Wait()
	})
}

// ------------------------------------------------------------------------------------------------
// Import Batch Retryable Error Detection Tests
// ------------------------------------------------------------------------------------------------

type testCase struct {
	name              string
	errorType         string
	expectedRetryable bool
	description       string
	dataGenerator     func() string
}

func createTestCases() []testCase {
	return []testCase{
		{
			name:              "numeric_field_overflow",
			errorType:         "22003",
			expectedRetryable: false,
			description:       "Should not retry numeric overflow errors",
			dataGenerator:     createNumericOverflowData,
		},
		{
			name:              "string_data_right_truncation",
			errorType:         "22001",
			expectedRetryable: false,
			description:       "Should not retry string truncation errors",
			dataGenerator:     createStringTruncationData,
		},
		{
			name:              "unique_constraint_violation",
			errorType:         "23505",
			expectedRetryable: false,
			description:       "Should not retry unique constraint violations",
			dataGenerator:     createUniqueConstraintViolationData,
		},
		{
			name:              "not_null_violation",
			errorType:         "23502",
			expectedRetryable: false,
			description:       "Should not retry not null violations",
			dataGenerator:     createNotNullViolationData,
		},
		{
			name:              "check_constraint_violation",
			errorType:         "23514",
			expectedRetryable: false,
			description:       "Should not retry check constraint violations",
			dataGenerator:     createCorruptedCopyData,
		},
		{
			name:              "valid_data",
			errorType:         "valid",
			expectedRetryable: true, // Should succeed, so no retry needed
			description:       "Should handle valid data successfully",
			dataGenerator:     createValidData,
		},
	}
}

func createTestTableWithConstraints(t *testing.T, container testcontainers.TestContainer) {
	createSchemaSQL := `CREATE SCHEMA IF NOT EXISTS test_schema;`

	createTableSQL := `
	CREATE TABLE test_schema.error_test_table (
		id SERIAL PRIMARY KEY,
		small_int_col SMALLINT,           -- For numeric overflow (22003)
		varchar_col VARCHAR(10),          -- For string truncation (22001)
		unique_col VARCHAR(50) UNIQUE,    -- For unique constraint (23505)
		not_null_col VARCHAR(50) NOT NULL, -- For not null violation (23502)
		check_col INTEGER CHECK (check_col > 0) -- For check constraint (23514)
	);`

	container.ExecuteSqls(createSchemaSQL, createTableSQL)
}

func createNumericOverflowData() string {
	// Data that will cause SMALLINT overflow (max 32767)
	return "101\t99999\ttest\tunique101\ttest\t100\n" // 99999 > 32767
}

func createStringTruncationData() string {
	// Data that will cause VARCHAR(10) truncation
	return "102\t100\tvery_long_string_that_exceeds_ten_characters\tunique102\ttest\t100\n"
}

func createUniqueConstraintViolationData() string {
	// Data with duplicate unique values
	return "103\t100\ttest\tduplicate_value\ttest\t100\n104\t200\ttest2\tduplicate_value\ttest2\t200\n"
}

func createNotNullViolationData() string {
	// Data with NULL in not null column
	return "105\t100\ttest\tunique105\t\ttest\t100\n" // Empty string for not_null_col
}

func createCorruptedCopyData() string {
	// Create data that will cause a check constraint violation (SQLSTATE 23514)
	return "106\t100\ttest\tunique106\ttest\t-100\n" // -100 violates CHECK (check_col > 0)
}

func createValidData() string {
	// Valid data that should import successfully
	return "999\t100\ttest\tunique999\ttest\t100\n"
}

func createBatchFromData(t *testing.T, data string, tableName sqlname.NameTuple) *Batch {
	// Create temporary batch file
	tempDir := t.TempDir()
	batchFilePath := filepath.Join(tempDir, "test_batch.sql")

	err := os.WriteFile(batchFilePath, []byte(data), 0644)
	require.NoError(t, err, "Failed to create batch file")

	return &Batch{
		Number:       1,
		TableNameTup: tableName,
		SchemaName:   tableName.CurrentName.SchemaName,
		FilePath:     batchFilePath,
		BaseFilePath: batchFilePath,
		OffsetStart:  0,
		OffsetEnd:    int64(len(data)),
		RecordCount:  1,
		ByteCount:    int64(len(data)),
		Interrupted:  false,
	}
}

func createTargetYugabyteDB(t *testing.T, container testcontainers.TestContainer) *tgtdb.TargetYugabyteDB {
	// Get container connection details
	host, port, err := container.GetHostPort()
	require.NoError(t, err, "Failed to get container host/port")

	config := container.GetConfig()

	// Create TargetConf
	tconf := &tgtdb.TargetConf{
		Host:         host,
		Port:         port,
		User:         config.User,
		Password:     config.Password,
		DBName:       config.DBName,
		Schema:       "public", // Set default schema
		TargetDBType: tgtdb.YUGABYTEDB,
	}

	// Create and initialize target database
	targetDB := tgtdb.NewTargetDB(tconf).(*tgtdb.TargetYugabyteDB)
	err = targetDB.Init()
	require.NoError(t, err, "Failed to initialize target database")

	// Initialize connection pool
	err = targetDB.InitConnPool()
	require.NoError(t, err, "Failed to initialize connection pool")

	return targetDB
}

func testErrorDetection(t *testing.T, targetDB *tgtdb.TargetYugabyteDB, tc testCase) {
	// Create batch with erroneous data
	tableName := sqlname.NameTuple{CurrentName: sqlname.NewObjectName(tgtdb.YUGABYTEDB, "test_schema", "test_schema", "error_test_table")}
	batch := createBatchFromData(t, tc.dataGenerator(), tableName)

	// Create import batch args
	args := &tgtdb.ImportBatchArgs{
		TableNameTup:       tableName,
		Columns:            []string{"id", "small_int_col", "varchar_col", "unique_col", "not_null_col", "check_col"},
		PrimaryKeyColumns:  []string{"id"},
		PKConflictAction:   "ERROR",
		FileFormat:         "csv",
		HasHeader:          false,
		Delimiter:          "\t",
		NullString:         "",
		RowsPerTransaction: 1000,
	}

	// Call ImportBatch method
	_, err, _ := targetDB.ImportBatch(batch, args, "", nil, false)

	// For valid data, we expect no error
	if tc.errorType == "valid" {
		assert.NoError(t, err, "Valid data should import successfully")
		return
	}

	// Check if error is retryable
	isRetryable := !targetDB.IsNonRetryableCopyError(err)

	// Debug output
	t.Logf("=== DEBUG: %s ===", tc.name)
	t.Logf("Expected retryable: %v", tc.expectedRetryable)
	t.Logf("Actual retryable: %v", isRetryable)
	t.Logf("Expected SQLSTATE: %s", tc.errorType)
	if err != nil {
		t.Logf("Actual error: %v", err)
		// Try to extract SQLSTATE code from error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Logf("SQLSTATE code: %s", pgErr.Code)
			t.Logf("SQLSTATE message: %s", pgErr.Message)
		} else {
			t.Logf("Error is not a pgconn.PgError, type: %T", err)
		}
	} else {
		t.Logf("No error returned (this should not happen for error test cases)")
	}
	t.Logf("IsNonRetryableCopyError result: %v", !isRetryable)
	t.Logf("==================")

	// Assert expected behavior
	assert.Equal(t, tc.expectedRetryable, isRetryable, tc.description)
}

func TestImportBatchErrorDetection_YugabyteDB(t *testing.T) {
	ctx := context.Background()

	// Setup YB container
	ybContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err := ybContainer.Start(ctx)
	require.NoError(t, err, "Failed to start YugabyteDB container")
	defer ybContainer.Terminate(ctx)

	// Create target database instance
	targetDB := createTargetYugabyteDB(t, ybContainer)

	// Create Voyager schema and metadata tables
	err = targetDB.CreateVoyagerSchema()
	require.NoError(t, err, "Failed to create Voyager schema")

	// Create test table
	createTestTableWithConstraints(t, ybContainer)

	// Run test cases
	testCases := createTestCases()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testErrorDetection(t, targetDB, tc)
		})
	}
}

func TestImportBatchErrorDetection_PostgreSQL(t *testing.T) {
	ctx := context.Background()

	// Setup PG container
	pgContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := pgContainer.Start(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	defer pgContainer.Terminate(ctx)

	// Create target database instance
	targetDB := createTargetPostgreSQL(t, pgContainer)

	// Create Voyager schema and metadata tables
	err = targetDB.CreateVoyagerSchema()
	require.NoError(t, err, "Failed to create Voyager schema")

	// Create test table
	createTestTableWithConstraints(t, pgContainer)

	// Run test cases
	testCases := createTestCases()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testErrorDetectionPG(t, targetDB, tc)
		})
	}
}

func createTargetPostgreSQL(t *testing.T, container testcontainers.TestContainer) *tgtdb.TargetPostgreSQL {
	// Get container connection details
	host, port, err := container.GetHostPort()
	require.NoError(t, err, "Failed to get container host/port")

	config := container.GetConfig()

	// Create TargetConf
	tconf := &tgtdb.TargetConf{
		Host:         host,
		Port:         port,
		User:         config.User,
		Password:     config.Password,
		DBName:       config.DBName,
		Schema:       "public", // Set default schema
		TargetDBType: tgtdb.POSTGRESQL,
	}

	// Create and initialize target database
	targetDB := tgtdb.NewTargetDB(tconf).(*tgtdb.TargetPostgreSQL)
	err = targetDB.Init()
	require.NoError(t, err, "Failed to initialize target database")

	// Initialize connection pool
	err = targetDB.InitConnPool()
	require.NoError(t, err, "Failed to initialize connection pool")

	return targetDB
}

func testErrorDetectionPG(t *testing.T, targetDB *tgtdb.TargetPostgreSQL, tc testCase) {
	// Create batch with erroneous data
	tableName := sqlname.NameTuple{CurrentName: sqlname.NewObjectName(tgtdb.POSTGRESQL, "test_schema", "test_schema", "error_test_table")}
	batch := createBatchFromData(t, tc.dataGenerator(), tableName)

	// Create import batch args
	args := &tgtdb.ImportBatchArgs{
		TableNameTup:       tableName,
		Columns:            []string{"id", "small_int_col", "varchar_col", "unique_col", "not_null_col", "check_col"},
		PrimaryKeyColumns:  []string{"id"},
		PKConflictAction:   "ERROR",
		FileFormat:         "csv",
		HasHeader:          false,
		Delimiter:          "\t",
		NullString:         "",
		RowsPerTransaction: 1000,
	}

	// Call ImportBatch method
	_, err, _ := targetDB.ImportBatch(batch, args, "", nil, false)

	// For valid data, we expect no error
	if tc.errorType == "valid" {
		assert.NoError(t, err, "Valid data should import successfully")
		return
	}

	// Check if error is retryable
	isRetryable := !targetDB.IsNonRetryableCopyError(err)

	// Debug output
	t.Logf("=== DEBUG: %s ===", tc.name)
	t.Logf("Expected retryable: %v", tc.expectedRetryable)
	t.Logf("Actual retryable: %v", isRetryable)
	t.Logf("Expected SQLSTATE: %s", tc.errorType)
	if err != nil {
		t.Logf("Actual error: %v", err)
		// Try to extract SQLSTATE code from error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Logf("SQLSTATE code: %s", pgErr.Code)
			t.Logf("SQLSTATE message: %s", pgErr.Message)
		} else {
			t.Logf("Error is not a pgconn.PgError, type: %T", err)
		}
	} else {
		t.Logf("No error returned (this should not happen for error test cases)")
	}
	t.Logf("IsNonRetryableCopyError result: %v", !isRetryable)
	t.Logf("==================")

	// Assert expected behavior
	assert.Equal(t, tc.expectedRetryable, isRetryable, tc.description)
}
