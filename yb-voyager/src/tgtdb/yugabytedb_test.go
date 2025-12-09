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
package tgtdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/versions"
)

func TestCreateVoyagerSchemaYB(t *testing.T) {
	db, err := sql.Open("pgx", testYugabyteDBTarget.GetConnectionString())
	assert.NoError(t, err)
	defer db.Close()

	// Wait for the database to be ready
	err = testutils.WaitForDBToBeReady(db)
	assert.NoError(t, err)

	// Initialize the TargetYugabyteDB instance
	yb := &TargetYugabyteDB{
		db: db,
	}

	// Call CreateVoyagerSchema
	err = yb.CreateVoyagerSchema()
	assert.NoError(t, err, "CreateVoyagerSchema failed")

	expectedTables := map[string]map[string]testutils.ColumnPropertiesPG{
		BATCH_METADATA_TABLE_NAME: {
			"migration_uuid": {Type: "uuid", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"data_file_name": {Type: "text", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"batch_number":   {Type: "integer", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"schema_name":    {Type: "text", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"table_name":     {Type: "text", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"rows_imported":  {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
		},
		EVENT_CHANNELS_METADATA_TABLE_NAME: {
			"migration_uuid":   {Type: "uuid", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"channel_no":       {Type: "integer", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"last_applied_vsn": {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"num_inserts":      {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"num_deletes":      {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"num_updates":      {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
		},
		EVENTS_PER_TABLE_METADATA_TABLE_NAME: {
			"migration_uuid": {Type: "uuid", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"table_name":     {Type: "text", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"channel_no":     {Type: "integer", IsNullable: "NO", Default: sql.NullString{Valid: false}, IsPrimary: true},
			"total_events":   {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"num_inserts":    {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"num_deletes":    {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
			"num_updates":    {Type: "bigint", IsNullable: "YES", Default: sql.NullString{Valid: false}, IsPrimary: false},
		},
	}

	// Validate the schema and tables
	t.Run("Check all the expected tables and no extra tables", func(t *testing.T) {
		testutils.CheckTableExistencePG(t, db, BATCH_METADATA_TABLE_SCHEMA, expectedTables)
	})

	// Validate columns for each table
	for tableName, expectedColumns := range expectedTables {
		t.Run(fmt.Sprintf("Check columns for %s table", tableName), func(t *testing.T) {
			table := strings.Split(tableName, ".")[1]
			testutils.CheckTableStructurePG(t, db, BATCH_METADATA_TABLE_SCHEMA, table, expectedColumns)
		})
	}
}

func TestYugabyteGetPrimaryKeyColumns(t *testing.T) {
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.foo (
			id INT,
			category TEXT,
			name TEXT,
			PRIMARY KEY (id, category)
		);`,
		`CREATE TABLE test_schema.bar (
			id INT PRIMARY KEY,
			name TEXT
		);`,
		`CREATE TABLE test_schema.baz (
			id INT,
			name TEXT
		);`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	tests := []struct {
		table          sqlname.NameTuple
		expectedPKCols []string
	}{
		{
			table:          testutils.CreateNameTupleWithTargetName("test_schema.foo", "public", POSTGRESQL),
			expectedPKCols: []string{"id", "category"},
		},
		{
			table:          testutils.CreateNameTupleWithTargetName("test_schema.bar", "public", POSTGRESQL),
			expectedPKCols: []string{"id"},
		},
		{
			table:          testutils.CreateNameTupleWithTargetName("test_schema.baz", "public", POSTGRESQL),
			expectedPKCols: nil,
		},
	}

	for _, tt := range tests {
		pkCols, err := testYugabyteDBTarget.GetPrimaryKeyColumns(tt.table)
		assert.NoError(t, err)
		testutils.AssertEqualStringSlices(t, tt.expectedPKCols, pkCols)
	}
}

func TestYugabyteGetNonEmptyTables(t *testing.T) {
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema`,
		`CREATE TABLE test_schema.foo (
			id INT PRIMARY KEY,
			name VARCHAR
		);`,
		`INSERT into test_schema.foo values (1, 'abc'), (2, 'xyz');`,
		`CREATE TABLE test_schema.bar (
			id INT PRIMARY KEY,
			name VARCHAR
		);`,
		`INSERT into test_schema.bar values (1, 'abc'), (2, 'xyz');`,
		`CREATE TABLE test_schema.unique_table (
			id SERIAL PRIMARY KEY,
			email VARCHAR(100),
			phone VARCHAR(100),
			address VARCHAR(255),
			UNIQUE (email, phone)
		);`,
		`CREATE TABLE test_schema.table1 (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100)
		);`,
		`CREATE TABLE test_schema.table2 (
			id SERIAL PRIMARY KEY,
			email VARCHAR(100)
		);`,
		`CREATE TABLE test_schema.non_pk1(
			id INT,
			name VARCHAR(255)
		);`,
		`CREATE TABLE test_schema.non_pk2(
			id INT,
			name VARCHAR(255)
		);`)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	tables := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.foo", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.bar", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.unique_table", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.table1", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.table2", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.non_pk1", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.non_pk2", "public", YUGABYTEDB),
	}

	expectedTables := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.foo", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.bar", "public", YUGABYTEDB),
	}

	actualTables := testYugabyteDBTarget.GetNonEmptyTables(tables)
	log.Infof("non empty tables: %+v\n", actualTables)
	testutils.AssertEqualNameTuplesSlice(t, expectedTables, actualTables)
}

func TestYugabyteGetPrimaryKeyConstraintNames(t *testing.T) {
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.foo (
            id INT PRIMARY KEY,
            name TEXT
        );`,
		`CREATE TABLE test_schema.bar (
            id INT,
            name TEXT,
            CONSTRAINT bar_primary_key PRIMARY KEY (id)
        );`,
		`CREATE TABLE test_schema.baz (
            id INT,
            name TEXT
        );`,
		`CREATE TABLE test_schema."CASE_sensitive" (
            id INT,
            name TEXT,
            CONSTRAINT "CASE_sensitive_pkey" PRIMARY KEY (id)
        );`,
		// Partitioned table examples
		// 1. Normal Partitioning
		`CREATE TABLE sales_region (id int, amount int, branch text, region text, PRIMARY KEY(id, region)) PARTITION BY LIST (region);`,
		`CREATE TABLE London PARTITION OF sales_region FOR VALUES IN ('London');`,
		`CREATE TABLE Sydney PARTITION OF sales_region FOR VALUES IN ('Sydney');`,
		`CREATE TABLE Boston PARTITION OF sales_region FOR VALUES IN ('Boston');`,

		// 2. Partioning with case sensitivity
		`CREATE TABLE test_schema."EmP" (
            emp_id   INT,
            emp_name TEXT,
            dep_code INT,
            PRIMARY KEY (emp_id)
        ) PARTITION BY HASH (emp_id);`,
		`CREATE TABLE test_schema."EmP_0" PARTITION OF test_schema."EmP" FOR VALUES WITH (MODULUS 3, REMAINDER 0);`,
		`CREATE TABLE test_schema."EmP_1" PARTITION OF test_schema."EmP" FOR VALUES WITH (MODULUS 3, REMAINDER 1);`,
		`CREATE TABLE test_schema."EmP_2" PARTITION OF test_schema."EmP" FOR VALUES WITH (MODULUS 3, REMAINDER 2);`,

		// 3. Multi level partitioning in public schema
		`CREATE TABLE customers (id INTEGER, statuses TEXT, arr NUMERIC, PRIMARY KEY(id, statuses, arr)) PARTITION BY LIST(statuses);`,

		`CREATE TABLE cust_active PARTITION OF customers FOR VALUES IN ('ACTIVE', 'RECURRING','REACTIVATED') PARTITION BY RANGE(arr);`,
		`CREATE TABLE cust_other  PARTITION OF customers DEFAULT;`,

		`CREATE TABLE cust_arr_small PARTITION OF cust_active FOR VALUES FROM (MINVALUE) TO (101) PARTITION BY HASH(id);`,
		`CREATE TABLE cust_part11 PARTITION OF cust_arr_small FOR VALUES WITH (modulus 2, remainder 0);`,
		`CREATE TABLE cust_part12 PARTITION OF cust_arr_small FOR VALUES WITH (modulus 2, remainder 1);`,

		`CREATE TABLE cust_arr_large PARTITION OF cust_active FOR VALUES FROM (101) TO (MAXVALUE) PARTITION BY HASH(id);`,
		`CREATE TABLE cust_part21 PARTITION OF cust_arr_large FOR VALUES WITH (modulus 2, remainder 0);`,
		`CREATE TABLE cust_part22 PARTITION OF cust_arr_large FOR VALUES WITH (modulus 2, remainder 1);`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA public CASCADE;`)

	tests := []struct {
		table           sqlname.NameTuple
		expectedPKNames []string
	}{
		{
			table:           testutils.CreateNameTupleWithTargetName("test_schema.foo", "public", POSTGRESQL),
			expectedPKNames: []string{"foo_pkey"},
		},
		{
			table:           testutils.CreateNameTupleWithTargetName("test_schema.bar", "public", POSTGRESQL),
			expectedPKNames: []string{"bar_primary_key"},
		},
		{
			table:           testutils.CreateNameTupleWithTargetName("test_schema.baz", "public", POSTGRESQL),
			expectedPKNames: nil,
		},
		{
			table:           testutils.CreateNameTupleWithTargetName("test_schema.\"CASE_sensitive\"", "public", POSTGRESQL),
			expectedPKNames: []string{"CASE_sensitive_pkey"},
		},
		{
			table:           testutils.CreateNameTupleWithTargetName("public.sales_region", "public", POSTGRESQL),
			expectedPKNames: []string{"sales_region_pkey", "london_pkey", "sydney_pkey", "boston_pkey"},
		},
		{
			table:           testutils.CreateNameTupleWithTargetName("test_schema.\"EmP\"", "public", POSTGRESQL),
			expectedPKNames: []string{"EmP_pkey", "EmP_0_pkey", "EmP_1_pkey", "EmP_2_pkey"},
		},
		{
			table: testutils.CreateNameTupleWithTargetName("public.customers", "public", POSTGRESQL),
			expectedPKNames: []string{"customers_pkey", "cust_active_pkey", "cust_other_pkey", "cust_arr_small_pkey",
				"cust_part11_pkey", "cust_part12_pkey", "cust_arr_large_pkey", "cust_part21_pkey", "cust_part22_pkey"},
		},
	}

	for _, tt := range tests {
		pkNames, err := testYugabyteDBTarget.GetPrimaryKeyConstraintNames(tt.table)
		assert.NoError(t, err)
		testutils.AssertEqualStringSlices(t, tt.expectedPKNames, pkNames)
	}
}

// this test is to ensure the query being used for fetching pg_stat_statements from target is working for voyager supported yb versions
func TestPGStatStatementsQuery(t *testing.T) {
	versionsList := versions.GetVoyagerSupportedYBVersions()

	// Test each supported yb version
	for _, version := range versionsList {
		t.Run(fmt.Sprintf("Version_%s", version), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			var testDB *TestDB
			var shouldCleanup bool
			if version == testYugabyteDBTarget.GetConfig().DBVersion {
				// Use shared container - don't destroy it
				testDB = testYugabyteDBTarget
				shouldCleanup = false
			} else {
				// Create new container for different version
				config := &testcontainers.ContainerConfig{
					DBType:    testcontainers.YUGABYTEDB,
					DBVersion: version,
				}
				testDB = createTestDBTarget(ctx, config)
				shouldCleanup = true
			}
			if shouldCleanup {
				defer destroyTestDBTarget(ctx, testDB)
			}

			// Enable pg_stat_statements extension
			testDB.ExecuteSqls(`CREATE EXTENSION IF NOT EXISTS pg_stat_statements;`)

			// Execute test queries to generate statistics
			testQueries := []string{
				"SELECT 1",
				"SELECT 2 + 3",
				"SELECT current_database()",
			}
			testDB.ExecuteSqls(testQueries...)

			ybTargetImpl, ok := testDB.TargetDB.(*TargetYugabyteDB)
			assert.True(t, ok, "Failed to cast TargetDB to TargetYugabyteDB for version %s", version)

			conn, err := pgx.Connect(ctx, testDB.GetConnectionString())
			assert.NoError(t, err, "Failed to get pgx connection for version %s", version)
			defer conn.Close(ctx)

			// Test the pg_stat_statements query
			query, err := ybTargetImpl.getPgStatStatementsQuery(conn)
			assert.NoError(t, err, "Failed to get pg_stat_statements query for version %s", version)

			rows, err := conn.Query(ctx, query)
			assert.NoError(t, err, "Failed to execute PG_STAT_STATEMENTS_QUERY for version %s", version)
			defer rows.Close()

			// Verify that we get some results
			var hasResults bool
			for rows.Next() {
				hasResults = true
				var queryid int64
				var query string
				var calls int64
				var rowCount int64
				var totalExecTime float64
				var meanExecTime float64
				var minExecTime float64
				var maxExecTime float64
				var stddevExecTime float64

				err := rows.Scan(&queryid, &query, &calls, &rowCount, &totalExecTime, &meanExecTime, &minExecTime, &maxExecTime, &stddevExecTime)
				assert.NoError(t, err, "Failed to scan pg_stat_statements row for query %s for yb version %s", query, version)
			}

			assert.True(t, hasResults, "Expected to find at least one query in pg_stat_statements for yb version %s", version)
			assert.NoError(t, rows.Err(), "Error occurred while iterating over rows for yb version %s", version)
		})
	}
}

func TestCollectPgStatStatements_BasicSelectQueries(t *testing.T) {
	// Test data: queries and their expected execution counts across nodes
	testQueries := map[string]struct {
		text          string
		parameterized string
		execCounts    []int // executions per node [node0, node1, node2]
	}{
		"simple_avg": {
			text:          `SELECT AVG(salary) FROM test_pgss.employees`,
			parameterized: `SELECT AVG(salary) FROM test_pgss.employees`,
			execCounts:    []int{1, 0, 0}, // only on node 0
		},
		"simple_count": {
			text:          `SELECT COUNT(*) FROM test_pgss.employees`,
			parameterized: `SELECT COUNT(*) FROM test_pgss.employees`,
			execCounts:    []int{0, 1, 0}, // only on node 1
		},
		"repeated_query": {
			text:          `SELECT 'test_merge' as marker, COUNT(*) FROM test_pgss.employees`,
			parameterized: `SELECT $1 as marker, COUNT(*) FROM test_pgss.employees`,
			execCounts:    []int{2, 3, 1}, // distributed across all nodes
		},
	}

	runPgStatStatementsTest(t, testQueries)
}

func TestCollectPgStatStatements_InsertUpdateDeleteQueries(t *testing.T) {
	// Test data: queries and their expected execution counts across nodes
	testQueries := map[string]struct {
		text          string
		parameterized string
		execCounts    []int // executions per node [node0, node1, node2]
	}{
		"insert": {
			text:          `INSERT INTO test_pgss.employees VALUES (4, 'David', 65000)`,
			parameterized: `INSERT INTO test_pgss.employees VALUES ($1, $2, $3)`,
			execCounts:    []int{1, 0, 0}, // only on node 0
		},
		"update": {
			text:          `UPDATE test_pgss.employees SET salary = 80000 WHERE id = 1`,
			parameterized: `UPDATE test_pgss.employees SET salary = $1 WHERE id = $2`,
			execCounts:    []int{1, 1, 0}, // on nodes 0 and 1
		},
		"delete": {
			text:          `DELETE FROM test_pgss.employees WHERE id = 4`,
			parameterized: `DELETE FROM test_pgss.employees WHERE id = $1`,
			execCounts:    []int{0, 0, 1}, // only on node 2
		},
	}

	runPgStatStatementsTest(t, testQueries)
}

// Helper function to run pg_stat_statements tests with different query sets
func runPgStatStatementsTest(t *testing.T, testQueries map[string]struct {
	text          string
	parameterized string
	execCounts    []int // executions per node [node0, node1, node2]
}) {
	// Setup test environment
	testYugabyteDBTargetCluster.ExecuteSqls(
		`CREATE SCHEMA IF NOT EXISTS test_pgss;`,
		`CREATE TABLE IF NOT EXISTS test_pgss.employees (
			id INT PRIMARY KEY, name TEXT, salary INT
		);`,
		`INSERT INTO test_pgss.employees VALUES
			(1, 'Alice', 75000), (2, 'Bob', 55000), (3, 'Charlie', 60000);`,
		`CREATE EXTENSION IF NOT EXISTS pg_stat_statements;`,
		`SELECT pg_stat_statements_reset();`,
	)
	defer testYugabyteDBTargetCluster.ExecuteSqls(`DROP SCHEMA test_pgss CASCADE;`)

	// Execute queries on different nodes
	for queryName, query := range testQueries {
		for nodeIdx, execCount := range query.execCounts {
			if execCount == 0 {
				continue
			}

			conn, err := testYugabyteDBTargetCluster.GetNodeConnection(nodeIdx)
			require.NoError(t, err, "Failed to connect to node %d for %s", nodeIdx, queryName)

			for i := 0; i < execCount; i++ {
				_, err = conn.Exec(query.text)
				require.NoError(t, err, "Failed to execute %s on node %d, iteration %d", queryName, nodeIdx, i+1)
			}

			conn.Close()
		}
	}

	// Collect PGSS
	_, tconfs, err := testYugabyteDBTargetCluster.GetYBServers() // calls overridden GetYBServers() method
	require.NoError(t, err)

	actualStatements, err := testYugabyteDBTargetCluster.collectPgStatStatements(tconfs)
	assert.NoError(t, err, "CollectPgStatStatements should not error")
	assert.NotNil(t, actualStatements, "Should return statements")

	// Validate results: check that our test queries have correct call counts
	foundQueries := make(map[string]bool)
	for _, actualPgss := range actualStatements {
		// verify that an fetched query doesn't have calls = 0 by any chance
		assert.Greater(t, actualPgss.Calls, int64(0),
			"Query %s should have positive calls", actualPgss.Query)

		for queryName, expectedPgss := range testQueries {
			if actualPgss.Query != expectedPgss.parameterized {
				continue
			}

			foundQueries[queryName] = true
			totalExpectedCalls := lo.Sum(expectedPgss.execCounts)

			assert.Equal(t, actualPgss.Calls, int64(totalExpectedCalls),
				"Query %s should have %d total calls, got %d", queryName, totalExpectedCalls, actualPgss.Calls)
			assert.Greater(t, actualPgss.TotalExecTime, float64(0),
				"Query %s should have positive total exec time", queryName)
			assert.Greater(t, actualPgss.MeanExecTime, float64(0),
				"Query %s should have positive mean exec time", queryName)
		}
	}

	// Ensure all test queries were found in results
	for queryName := range testQueries {
		assert.True(t, foundQueries[queryName], "Expected query %s not found in pg_stat_statements results", queryName)
	}
}

func TestYugabyteGetIdentityColumnNamesForTables(t *testing.T) {
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.always_table (
			id1 INT GENERATED ALWAYS AS IDENTITY,
			id2 INT GENERATED ALWAYS AS IDENTITY,
			data TEXT
		);`,
		`CREATE TABLE test_schema.bydefault_table (
			id1 INT GENERATED BY DEFAULT AS IDENTITY,
			id2 INT GENERATED BY DEFAULT AS IDENTITY,
			data TEXT
		);`,
		`CREATE TABLE test_schema.mixed_table (
			always_col INT GENERATED ALWAYS AS IDENTITY,
			bydefault_col INT GENERATED BY DEFAULT AS IDENTITY,
			regular_col INT,
			data TEXT
		);`,
		`CREATE TABLE test_schema.no_identity_table (
			id INT PRIMARY KEY,
			data TEXT
		);`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	tableTuplesList := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.always_table", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.bydefault_table", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.mixed_table", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.no_identity_table", "public", YUGABYTEDB),
	}

	// Expected results for ALWAYS identity columns
	expectedAlways := map[string][]string{
		"always_table": {"id1", "id2"},
		"mixed_table":  {"always_col"},
	}

	// Expected results for BY DEFAULT identity columns
	expectedByDefault := map[string][]string{
		"bydefault_table": {"id1", "id2"},
		"mixed_table":     {"bydefault_col"},
	}

	// Test 1: Get ALWAYS identity columns
	tableColsStructMap, err := testYugabyteDBTarget.GetIdentityColumnNamesForTables(tableTuplesList, constants.IDENTITY_GENERATION_ALWAYS)
	assert.NoError(t, err)

	// Verify ALWAYS results
	for _, tableTuple := range tableTuplesList {
		_, tableName := tableTuple.ForCatalogQuery()
		expectedCols, shouldExist := expectedAlways[tableName]
		actualCols, exists := tableColsStructMap.Get(tableTuple)

		if shouldExist {
			assert.True(t, exists, "Expected ALWAYS identity columns for table %s", tableName)
			testutils.AssertEqualStringSlices(t, expectedCols, actualCols)
		} else {
			assert.False(t, exists, "Expected no ALWAYS identity columns for table %s", tableName)
		}
	}

	// Test 2: Get BY DEFAULT identity columns
	tableColsStructMapByDefault, err := testYugabyteDBTarget.GetIdentityColumnNamesForTables(tableTuplesList, constants.IDENTITY_GENERATION_BY_DEFAULT)
	assert.NoError(t, err)

	// Verify BY DEFAULT results
	for _, tableTuple := range tableTuplesList {
		_, tableName := tableTuple.ForCatalogQuery()
		expectedCols, shouldExist := expectedByDefault[tableName]
		actualCols, exists := tableColsStructMapByDefault.Get(tableTuple)

		if shouldExist {
			assert.True(t, exists, "Expected BY DEFAULT identity columns for table %s", tableName)
			testutils.AssertEqualStringSlices(t, expectedCols, actualCols)
		} else {
			assert.False(t, exists, "Expected no BY DEFAULT identity columns for table %s", tableName)
		}
	}
}

func TestYugabyteGetIdentityColumnNamesForTables_CaseSensitiveAndTablePartitions(t *testing.T) {
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		// Case-sensitive table with case-sensitive identity columns
		`CREATE TABLE test_schema."MixedCaseTable" (
			"Id1" INT GENERATED ALWAYS AS IDENTITY,
			"ID2" INT GENERATED BY DEFAULT AS IDENTITY,
			"MixedCaseCol" TEXT,
			data TEXT
		);`,
		// Partitioned table with identity columns
		`CREATE TABLE test_schema.partitioned_table (
			"Id" INT GENERATED ALWAYS AS IDENTITY,
			"PartitionKey" INT GENERATED BY DEFAULT AS IDENTITY,
			region TEXT,
			data TEXT,
			PRIMARY KEY ("Id", region)
		) PARTITION BY LIST (region);`,
		`CREATE TABLE test_schema.partition_east PARTITION OF test_schema.partitioned_table FOR VALUES IN ('East');`,
		`CREATE TABLE test_schema.partition_west PARTITION OF test_schema.partitioned_table FOR VALUES IN ('West');`,
		// Regular table without identity columns for comparison
		`CREATE TABLE test_schema.regular_table (
			id INT PRIMARY KEY,
			data TEXT
		);`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	tableTuplesList := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.\"MixedCaseTable\"", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.partitioned_table", "public", YUGABYTEDB),
		testutils.CreateNameTupleWithTargetName("test_schema.regular_table", "public", YUGABYTEDB),
	}

	// Expected results for ALWAYS identity columns
	expectedAlways := map[string][]string{
		"MixedCaseTable":    {"Id1"},
		"partitioned_table": {"Id"},
	}

	// Expected results for BY DEFAULT identity columns
	expectedByDefault := map[string][]string{
		"MixedCaseTable":    {"ID2"},
		"partitioned_table": {"PartitionKey"},
	}

	// Test 1: Get ALWAYS identity columns
	tableColsStructMap, err := testYugabyteDBTarget.GetIdentityColumnNamesForTables(tableTuplesList, constants.IDENTITY_GENERATION_ALWAYS)
	assert.NoError(t, err)

	// Verify ALWAYS results
	for _, tableTuple := range tableTuplesList {
		_, tableName := tableTuple.ForCatalogQuery()
		expectedCols, shouldExist := expectedAlways[tableName]
		actualCols, exists := tableColsStructMap.Get(tableTuple)

		if shouldExist {
			assert.True(t, exists, "Expected ALWAYS identity columns for table %s", tableName)
			testutils.AssertEqualStringSlices(t, expectedCols, actualCols)
		} else {
			assert.False(t, exists, "Expected no ALWAYS identity columns for table %s", tableName)
		}
	}

	// Test 2: Get BY DEFAULT identity columns
	tableColsStructMapByDefault, err := testYugabyteDBTarget.GetIdentityColumnNamesForTables(tableTuplesList, constants.IDENTITY_GENERATION_BY_DEFAULT)
	assert.NoError(t, err)

	// Verify BY DEFAULT results
	for _, tableTuple := range tableTuplesList {
		_, tableName := tableTuple.ForCatalogQuery()
		expectedCols, shouldExist := expectedByDefault[tableName]
		actualCols, exists := tableColsStructMapByDefault.Get(tableTuple)

		if shouldExist {
			assert.True(t, exists, "Expected BY DEFAULT identity columns for table %s", tableName)
			testutils.AssertEqualStringSlices(t, expectedCols, actualCols)
		} else {
			assert.False(t, exists, "Expected no BY DEFAULT identity columns for table %s", tableName)
		}
	}
}

func TestYugabyteIdentityColumnsDisableEnableCycle(t *testing.T) {
	// Initialize connection pool used by the functions to be tested
	yb := testYugabyteDBTarget.TargetDB.(*TargetYugabyteDB)
	err := yb.InitConnPool()
	require.NoError(t, err)

	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.table1 (
			id INT GENERATED ALWAYS AS IDENTITY,
			data TEXT
		);`,
		`CREATE TABLE test_schema.table2 (
			id1 INT GENERATED ALWAYS AS IDENTITY,
			id2 INT GENERATED ALWAYS AS IDENTITY,
			data TEXT
		);`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	db, dbErr := testYugabyteDBTarget.GetConnection()
	require.NoError(t, dbErr)

	// Test complete disable/enable cycle: ALWAYS -> BY DEFAULT -> ALWAYS
	tableColumnsMap := createTableToColumnsStructMap("test_schema", map[string][]string{
		"table1": {"id"},
		"table2": {"id1", "id2"},
	})

	// Step 1: Verify initial ALWAYS state
	table1Types, err := getIdentityColumnTypes(db, "test_schema", "table1", []string{"id"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1Types["id"])

	table2Types, err := getIdentityColumnTypes(db, "test_schema", "table2", []string{"id1", "id2"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2Types["id1"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2Types["id2"])

	// Step 2: Disable identity columns (ALWAYS -> BY DEFAULT)
	err = yb.DisableGeneratedAlwaysAsIdentityColumns(tableColumnsMap)
	assert.NoError(t, err)

	// Verify columns are now BY DEFAULT
	table1TypesDisabled, err := getIdentityColumnTypes(db, "test_schema", "table1", []string{"id"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table1TypesDisabled["id"])

	table2TypesDisabled, err := getIdentityColumnTypes(db, "test_schema", "table2", []string{"id1", "id2"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2TypesDisabled["id1"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2TypesDisabled["id2"])

	// Step 3: Enable identity columns (BY DEFAULT -> ALWAYS)
	err = yb.EnableGeneratedAlwaysAsIdentityColumns(tableColumnsMap)
	assert.NoError(t, err)

	// Verify columns are back to ALWAYS
	table1TypesFinal, err := getIdentityColumnTypes(db, "test_schema", "table1", []string{"id"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1TypesFinal["id"])

	table2TypesFinal, err := getIdentityColumnTypes(db, "test_schema", "table2", []string{"id1", "id2"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2TypesFinal["id1"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2TypesFinal["id2"])
}

func TestYugabyteAlterActionMultipleColumns_IdentityAlways(t *testing.T) {
	// Initialize connection pool used by the functions to be tested
	yb := testYugabyteDBTarget.TargetDB.(*TargetYugabyteDB)
	err := yb.InitConnPool()
	require.NoError(t, err)

	// Create tables with BY DEFAULT identity columns mixed with regular columns
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_alter_always;`,
		// Table with multiple identity columns - only some will be altered
		`CREATE TABLE test_alter_always.table1 (
			id INT,
			id1 INT GENERATED BY DEFAULT AS IDENTITY,
			id2 INT GENERATED BY DEFAULT AS IDENTITY,
			id3 INT GENERATED BY DEFAULT AS IDENTITY,
			data TEXT
		);`,
		`CREATE TABLE test_alter_always.table2 (
			col_int INT GENERATED BY DEFAULT AS IDENTITY,
			col_bigint BIGINT GENERATED BY DEFAULT AS IDENTITY,
			col_smallint SMALLINT GENERATED BY DEFAULT AS IDENTITY,
			name TEXT
		);`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_alter_always CASCADE;`)

	db, dbErr := testYugabyteDBTarget.GetConnection()
	require.NoError(t, dbErr)

	// Verify initial BY DEFAULT state for all identity columns
	table1Types, err := getIdentityColumnTypes(db, "test_alter_always", "table1", []string{"id1", "id2", "id3"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table1Types["id1"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table1Types["id2"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table1Types["id3"])

	table2Types, err := getIdentityColumnTypes(db, "test_alter_always", "table2", []string{"col_int", "col_bigint", "col_smallint"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2Types["col_int"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2Types["col_bigint"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2Types["col_smallint"])

	// Action: Enable ALWAYS (BY DEFAULT -> ALWAYS) using alterColumns directly
	// Note: Only altering table1's id1 and id2 columns, leaving id3 unchanged
	tableColumnsMap := createTableToColumnsStructMap("test_alter_always", map[string][]string{
		"table1": {"id1", "id2"},
		"table2": {"col_int", "col_bigint", "col_smallint"},
	})

	err = yb.alterColumns(tableColumnsMap, constants.PG_SET_GENERATED_ALWAYS)
	assert.NoError(t, err)

	// Verify: table1's id1 and id2 columns changed to ALWAYS
	table1TypesAlways, err := getIdentityColumnTypes(db, "test_alter_always", "table1", []string{"id1", "id2", "id3"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1TypesAlways["id1"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1TypesAlways["id2"])
	// Verify: table1's id3 column remains BY DEFAULT (unchanged)
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table1TypesAlways["id3"], "id3 should remain BY DEFAULT")

	// Verify: table2's identity columns changed to ALWAYS
	table2TypesAlways, err := getIdentityColumnTypes(db, "test_alter_always", "table2", []string{"col_int", "col_bigint", "col_smallint"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2TypesAlways["col_int"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2TypesAlways["col_bigint"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2TypesAlways["col_smallint"])
}

func TestYugabyteAlterActionMultipleColumns_IdentityDefault(t *testing.T) {
	// Initialize connection pool used by the functions to be tested
	yb := testYugabyteDBTarget.TargetDB.(*TargetYugabyteDB)
	err := yb.InitConnPool()
	require.NoError(t, err)

	// Create tables with ALWAYS identity columns mixed with regular columns
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_alter_default;`,
		// Table with multiple identity columns - only some will be altered
		`CREATE TABLE test_alter_default.table1 (
			id INT,
			id1 INT GENERATED ALWAYS AS IDENTITY,
			id2 INT GENERATED ALWAYS AS IDENTITY,
			id3 INT GENERATED ALWAYS AS IDENTITY,
			data TEXT
		);`,
		`CREATE TABLE test_alter_default.table2 (
			col_int INT GENERATED ALWAYS AS IDENTITY,
			col_bigint BIGINT GENERATED ALWAYS AS IDENTITY,
			col_smallint SMALLINT GENERATED ALWAYS AS IDENTITY,
			name TEXT
		);`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_alter_default CASCADE;`)

	db, dbErr := testYugabyteDBTarget.GetConnection()
	require.NoError(t, dbErr)

	// Verify initial ALWAYS state for all identity columns
	table1Types, err := getIdentityColumnTypes(db, "test_alter_default", "table1", []string{"id1", "id2", "id3"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1Types["id1"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1Types["id2"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1Types["id3"])

	table2Types, err := getIdentityColumnTypes(db, "test_alter_default", "table2", []string{"col_int", "col_bigint", "col_smallint"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2Types["col_int"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2Types["col_bigint"])
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table2Types["col_smallint"])

	// Action: Disable to BY DEFAULT (ALWAYS -> BY DEFAULT) using alterColumns directly
	// Note: Only altering table1's id1 and id2 columns, leaving id3 unchanged
	tableColumnsMap := createTableToColumnsStructMap("test_alter_default", map[string][]string{
		"table1": {"id1", "id2"},
		"table2": {"col_int", "col_bigint", "col_smallint"},
	})

	err = yb.alterColumns(tableColumnsMap, constants.PG_SET_GENERATED_BY_DEFAULT)
	assert.NoError(t, err)

	// Verify: table1's id1 and id2 columns changed to BY DEFAULT
	table1TypesDefault, err := getIdentityColumnTypes(db, "test_alter_default", "table1", []string{"id1", "id2", "id3"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table1TypesDefault["id1"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table1TypesDefault["id2"])
	// Verify: table1's id3 column remains ALWAYS (unchanged)
	assert.Equal(t, constants.IDENTITY_GENERATION_ALWAYS, table1TypesDefault["id3"], "id3 should remain ALWAYS")

	// Verify: table2's identity columns changed to BY DEFAULT
	table2TypesDefault, err := getIdentityColumnTypes(db, "test_alter_default", "table2", []string{"col_int", "col_bigint", "col_smallint"})
	require.NoError(t, err)
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2TypesDefault["col_int"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2TypesDefault["col_bigint"])
	assert.Equal(t, constants.IDENTITY_GENERATION_BY_DEFAULT, table2TypesDefault["col_smallint"])
}

func TestGetTablesHavingExpressionIndexes(t *testing.T) {
	testYugabyteDBTarget.ExecuteSqls(
		`CREATE SCHEMA test_expression_indexes;`,
		`CREATE TABLE test_expression_indexes.table1 (
			id INT,
			data TEXT,
			val text,
			val2 text
		);`,
		`CREATE TABLE test_expression_indexes.table2 (
			id INT,
			data TEXT,
			val text,
			val2 text
		);`,
		`CREATE TABLE test_expression_indexes.table3 (
			id INT,
			data TEXT,
			val text,
			val2 text
		);`,
		`CREATE TABLE test_expression_indexes.table4 (
			id INT,
			data TEXT,
			val text,
			val2 text
		);`,
		`CREATE TABLE test_expression_indexes.table5 (
			id INT,
			data TEXT,
			val text,
			val2 text
		);`,
		`CREATE TABLE test_expression_indexes."Table6" (
			id INT,
			"Data" TEXT,
			val text,
			val2 text
		);`,
		`CREATE TABLE test_expression_indexes."Table7" (
			id INT,
			"Data" TEXT,
			val text,
			"Val2" text
		);`,
		`CREATE TABLE test_expression_indexes."Table8" (
			id INT,
			"Data" TEXT,
			val text,
			"Val2" text
		);`,
		`CREATE UNIQUE INDEX idx_expression_indexes ON test_expression_indexes.table1 ((val || val2));`,
		`CREATE UNIQUE INDEX idx_expression_indexes_2 ON test_expression_indexes.table2 (lower(val));`,
		`CREATE UNIQUE INDEX idx_expression_indexes_3 ON test_expression_indexes.table3 ((val || val2), data);`,
		`CREATE INDEX idx_expression_indexes_4 ON test_expression_indexes.table4 (lower(val || val2), id, data);`, //normal index
		`CREATE UNIQUE INDEX idx_expression_indexes_5 ON test_expression_indexes.table5 (data);`,                  //normal unique index
		`CREATE UNIQUE INDEX idx_expression_indexes_6 ON test_expression_indexes."Table6" (lower("Data"));`,
		`CREATE UNIQUE INDEX idx_expression_indexes_7 ON test_expression_indexes."Table7" (lower("Data" || "Val2"));`,
		`CREATE UNIQUE INDEX idx_expression_indexes_8 ON test_expression_indexes."Table8" ("Data");`,
		`CREATE INDEX idx_expression_indexes_9 ON test_expression_indexes."Table8" (("Data" || "Val2"));`, //normal index

		//partitions cases
		//only two partitions have expression indexes
		`CREATE TABLE test_expression_indexes.table_partitioned (
			id INT,
			data TEXT,
			val text,
			val2 text
		) PARTITION BY LIST (data);
		CREATE TABLE test_expression_indexes.table_partitioned_l PARTITION OF test_expression_indexes.table_partitioned FOR VALUES IN ('London');
		CREATE TABLE test_expression_indexes.table_partitioned_s PARTITION OF test_expression_indexes.table_partitioned FOR VALUES IN ('Sydney');
		CREATE TABLE test_expression_indexes.table_partitioned_b PARTITION OF test_expression_indexes.table_partitioned FOR VALUES IN ('Boston');

		CREATE UNIQUE INDEX idx_expression_indexes_10 ON test_expression_indexes.table_partitioned_l (lower(val || val2));
		CREATE UNIQUE INDEX idx_expression_indexes_12 ON test_expression_indexes.table_partitioned_b (lower(val || val2));`,

		//one of the leaf child in multi level partitioning has expression indexes
		`CREATE TABLE test_expression_indexes.table_partitioned1 (
			id INT,
			data TEXT,
			val text,
			val2 text
		) PARTITION BY LIST (data);
		CREATE TABLE test_expression_indexes.table_partitioned_l1 PARTITION OF test_expression_indexes.table_partitioned1 FOR VALUES IN ('London') PARTITION BY LIST (val);
		CREATE TABLE test_expression_indexes.table_partitioned_s1 PARTITION OF test_expression_indexes.table_partitioned1 FOR VALUES IN ('Sydney');
		CREATE TABLE test_expression_indexes.table_partitioned_b1 PARTITION OF test_expression_indexes.table_partitioned1 FOR VALUES IN ('Boston');

		CREATE TABLE test_expression_indexes.table_partitioned_l1_part1 PARTITION OF test_expression_indexes.table_partitioned_l1 FOR VALUES IN ('ABC data');
		CREATE TABLE test_expression_indexes.table_partitioned_l1_part2 PARTITION OF test_expression_indexes.table_partitioned_l1 FOR VALUES IN ('XYZ data');

		
		CREATE UNIQUE INDEX idx_expression_indexes_13 ON test_expression_indexes.table_partitioned_l1_part1 (lower(val || val2));`,

		//only two partitions have expression indexes
		`CREATE TABLE test_expression_indexes.table_partitioned2 (
			id INT,
			data TEXT,
			val text,
			val2 text
		) PARTITION BY LIST (data);
		CREATE TABLE test_expression_indexes.table_partitioned_l2 PARTITION OF test_expression_indexes.table_partitioned2 FOR VALUES IN ('London');
		CREATE TABLE test_expression_indexes.table_partitioned_s2 PARTITION OF test_expression_indexes.table_partitioned2 FOR VALUES IN ('Sydney');
		CREATE TABLE test_expression_indexes.table_partitioned_b2 PARTITION OF test_expression_indexes.table_partitioned2 FOR VALUES IN ('Boston');

		CREATE UNIQUE INDEX idx_expression_indexes_14 ON test_expression_indexes.table_partitioned_l2 (lower(val || val2));
		CREATE UNIQUE INDEX idx_expression_indexes_15 ON test_expression_indexes.table_partitioned_b2 (lower(val || val2));`,

		//none of the partitions have expression indexes
		`CREATE TABLE test_expression_indexes.table_partitioned3 (
			id INT,
			data TEXT,
			val text,
			val2 text
		) PARTITION BY LIST (data);
		CREATE TABLE test_expression_indexes.table_partitioned_l3 PARTITION OF test_expression_indexes.table_partitioned3 FOR VALUES IN ('London');
		CREATE TABLE test_expression_indexes.table_partitioned_s3 PARTITION OF test_expression_indexes.table_partitioned3 FOR VALUES IN ('Sydney');
		CREATE TABLE test_expression_indexes.table_partitioned_b3 PARTITION OF test_expression_indexes.table_partitioned3 FOR VALUES IN ('Boston');`,

		//root table has the expression index
		`CREATE TABLE test_expression_indexes.table_partitioned4 (
			id INT,
			data TEXT,
			val text,
			val2 text
		) PARTITION BY LIST (data);
		CREATE TABLE test_expression_indexes.table_partitioned_l4 PARTITION OF test_expression_indexes.table_partitioned4 FOR VALUES IN ('London');
		CREATE TABLE test_expression_indexes.table_partitioned_s4 PARTITION OF test_expression_indexes.table_partitioned4 FOR VALUES IN ('Sydney');
		CREATE TABLE test_expression_indexes.table_partitioned_b4 PARTITION OF test_expression_indexes.table_partitioned4 FOR VALUES IN ('Boston');

		CREATE UNIQUE INDEX idx_expression_indexes_16 ON test_expression_indexes.table_partitioned4 (data, upper(val || val2));`,

		//cross schema partitions and
		`CREATE SCHEMA test_expression_indexes_cross;
		CREATE TABLE test_expression_indexes_cross.table_partitioned5 (
			id INT,
			data TEXT,
			val text,
			val2 text
		) PARTITION BY LIST (data);
		CREATE TABLE test_expression_indexes.table_partitioned_l5 PARTITION OF test_expression_indexes_cross.table_partitioned5 FOR VALUES IN ('London');
		CREATE TABLE test_expression_indexes.table_partitioned_s5 PARTITION OF test_expression_indexes_cross.table_partitioned5 FOR VALUES IN ('Sydney');
		CREATE TABLE test_expression_indexes.table_partitioned_b5 PARTITION OF test_expression_indexes_cross.table_partitioned5 FOR VALUES IN ('Boston');

		CREATE UNIQUE INDEX idx_expression_indexes_17 ON test_expression_indexes.table_partitioned_l5 (data, upper(val || val2));`,
	)
	defer testYugabyteDBTarget.ExecuteSqls(`DROP SCHEMA test_expression_indexes CASCADE;
	DROP SCHEMA test_expression_indexes_cross CASCADE;`)

	table1 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table1", "public", YUGABYTEDB)
	table2 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table2", "public", YUGABYTEDB)
	table3 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table3", "public", YUGABYTEDB)
	table4 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table4", "public", YUGABYTEDB)
	table5 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table5", "public", YUGABYTEDB)
	table6 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.\"Table6\"", "public", YUGABYTEDB)
	table7 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.\"Table7\"", "public", YUGABYTEDB)
	table8 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.\"Table8\"", "public", YUGABYTEDB)
	partitionedTable := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned", "public", YUGABYTEDB)
	partitionedTable1 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned1", "public", YUGABYTEDB)
	partitionedTable2 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned2", "public", YUGABYTEDB)
	partitionedTable3 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned3", "public", YUGABYTEDB)
	partitionedTable4 := testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned4", "public", YUGABYTEDB)
	partitionedTable5 := testutils.CreateNameTupleWithTargetName("test_expression_indexes_cross.table_partitioned5", "public", YUGABYTEDB)

	tableTuplesList := []sqlname.NameTuple{
		table1,
		table2,
		table3,
		table4,
		table5,
		table6,
		table7,
		table8,
		partitionedTable,
		partitionedTable1,
		partitionedTable2,
		partitionedTable3,
		partitionedTable4,
		partitionedTable5,
	}

	yb, ok := testYugabyteDBTarget.TargetDB.(*TargetYugabyteDB)
	require.True(t, ok)

	tableCatalogNamesHavingExpressionIndexes, err := yb.GetTablesHavingExpressionUniqueIndexes(tableTuplesList)
	require.NoError(t, err)
	assert.Equal(t, 14, len(tableCatalogNamesHavingExpressionIndexes))
	expectedTableTuplesHavingExpressionIndexes := []string{
		table1.AsQualifiedCatalogName(),
		table2.AsQualifiedCatalogName(),
		table3.AsQualifiedCatalogName(),
		table6.AsQualifiedCatalogName(),
		table7.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l1_part1", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l2", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b2", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l4", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b4", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_s4", "public", YUGABYTEDB).AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l5", "public", YUGABYTEDB).AsQualifiedCatalogName(),
	}
	assert.ElementsMatch(t, expectedTableTuplesHavingExpressionIndexes, tableCatalogNamesHavingExpressionIndexes)

	leafTableToRootTableMap, err := yb.GetPartitionTableToRootTableMap(tableTuplesList)
	require.NoError(t, err)
	expectedLeafTableToRootTableMap := map[string]string{
		table1.AsQualifiedCatalogName():           table1.AsQualifiedCatalogName(),
		table2.AsQualifiedCatalogName():           table2.AsQualifiedCatalogName(),
		table3.AsQualifiedCatalogName():           table3.AsQualifiedCatalogName(),
		table4.AsQualifiedCatalogName():           table4.AsQualifiedCatalogName(),
		table5.AsQualifiedCatalogName():           table5.AsQualifiedCatalogName(),
		table6.AsQualifiedCatalogName():           table6.AsQualifiedCatalogName(),
		table7.AsQualifiedCatalogName():           table7.AsQualifiedCatalogName(),
		table8.AsQualifiedCatalogName():           table8.AsQualifiedCatalogName(),
		partitionedTable.AsQualifiedCatalogName(): partitionedTable.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_s", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable.AsQualifiedCatalogName(),
		partitionedTable1.AsQualifiedCatalogName(): partitionedTable1.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l1", "public", YUGABYTEDB).AsQualifiedCatalogName():       partitionedTable1.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l1_part1", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable1.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l1_part2", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable1.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_s1", "public", YUGABYTEDB).AsQualifiedCatalogName():       partitionedTable1.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b1", "public", YUGABYTEDB).AsQualifiedCatalogName():       partitionedTable1.AsQualifiedCatalogName(),
		partitionedTable2.AsQualifiedCatalogName(): partitionedTable2.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l2", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable2.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b2", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable2.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_s2", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable2.AsQualifiedCatalogName(),
		partitionedTable3.AsQualifiedCatalogName(): partitionedTable3.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l3", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable3.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_s3", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable3.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b3", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable3.AsQualifiedCatalogName(),
		partitionedTable4.AsQualifiedCatalogName(): partitionedTable4.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l4", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable4.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b4", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable4.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_s4", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable4.AsQualifiedCatalogName(),
		partitionedTable5.AsQualifiedCatalogName(): partitionedTable5.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_l5", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable5.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_s5", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable5.AsQualifiedCatalogName(),
		testutils.CreateNameTupleWithTargetName("test_expression_indexes.table_partitioned_b5", "public", YUGABYTEDB).AsQualifiedCatalogName(): partitionedTable5.AsQualifiedCatalogName(),
	}
	assert.Equal(t, len(lo.Keys(expectedLeafTableToRootTableMap)), len(lo.Keys(leafTableToRootTableMap)))
	for expectedKey, expectedValue := range expectedLeafTableToRootTableMap {
		returnedValue, ok := leafTableToRootTableMap[expectedKey]
		assert.True(t, ok)
		assert.Equal(t, expectedValue, returnedValue)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// Helper function to get identity column types from pg_catalog
func getIdentityColumnTypes(db *sql.DB, schemaName, tableName string, columnNames []string) (map[string]string, error) {
	quotedColumns := make([]string, len(columnNames))
	for i, col := range columnNames {
		quotedColumns[i] = fmt.Sprintf("'%s'", col)
	}

	query := fmt.Sprintf(`
		SELECT a.attname as column_name,
			   CASE
				   WHEN a.attidentity = 'a' THEN 'ALWAYS'
				   WHEN a.attidentity = 'd' THEN 'BY DEFAULT'
				   ELSE 'NONE'
			   END as identity_type
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON a.attrelid = c.oid
		JOIN pg_catalog.pg_namespace n ON c.relnamespace = n.oid
		WHERE n.nspname = '%s'
		  AND c.relname = '%s'
		  AND a.attname IN (%s)
		ORDER BY a.attname;
	`, schemaName, tableName, strings.Join(quotedColumns, ","))

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var colName, identityType string
		err := rows.Scan(&colName, &identityType)
		if err != nil {
			return nil, err
		}
		result[colName] = identityType
	}
	return result, nil
}

// Helper function to create StructMap[NameTuple, []string] mapping table NameTuples to their identity column lists
func createTableToColumnsStructMap(schemaName string, tables map[string][]string) *utils.StructMap[sqlname.NameTuple, []string] {
	tableColumnsMap := utils.NewStructMap[sqlname.NameTuple, []string]()
	for tableName, columns := range tables {
		qualifiedName := schemaName + "." + tableName
		tableNameTup := testutils.CreateNameTupleWithTargetName(qualifiedName, "public", YUGABYTEDB)
		tableColumnsMap.Put(tableNameTup, columns)
	}
	return tableColumnsMap
}
