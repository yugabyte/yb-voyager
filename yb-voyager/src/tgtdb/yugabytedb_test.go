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
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
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
			table:          sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "foo")},
			expectedPKCols: []string{"id", "category"},
		},
		{
			table:          sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "bar")},
			expectedPKCols: []string{"id"},
		},
		{
			table:          sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "baz")},
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
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "foo")},
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "bar")},
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "unique_table")},
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "table1")},
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "table2")},
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "non_pk1")},
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "non_pk2")},
	}

	expectedTables := []sqlname.NameTuple{
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "foo")},
		{CurrentName: sqlname.NewObjectName(YUGABYTEDB, "test_schema", "test_schema", "bar")},
	}

	actualTables := testYugabyteDBTarget.GetNonEmptyTables(tables)
	log.Infof("non empty tables: %+v\n", actualTables)
	testutils.AssertEqualNameTuplesSlice(t, expectedTables, actualTables)
}

func TestGetPrimaryKeyConstraintNames(t *testing.T) {
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
			table:           sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "foo")},
			expectedPKNames: []string{"foo_pkey"},
		},
		{
			table:           sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "bar")},
			expectedPKNames: []string{"bar_primary_key"},
		},
		{
			table:           sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "baz")},
			expectedPKNames: nil,
		},
		{
			table:           sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "CASE_sensitive")},
			expectedPKNames: []string{"CASE_sensitive_pkey"},
		},
		{
			table:           sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "public", "public", "sales_region")},
			expectedPKNames: []string{"sales_region_pkey", "london_pkey", "sydney_pkey", "boston_pkey"},
		},
		{
			table:           sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "test_schema", "test_schema", "EmP")},
			expectedPKNames: []string{"EmP_pkey", "EmP_0_pkey", "EmP_1_pkey", "EmP_2_pkey"},
		},
		{
			table: sqlname.NameTuple{CurrentName: sqlname.NewObjectName(POSTGRESQL, "public", "public", "customers")},
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
	versions := testutils.GetYBVersions()

	// Test each supported yb version
	for _, version := range versions {
		t.Run(fmt.Sprintf("Version_%s", version), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			config := &testcontainers.ContainerConfig{
				DBType:    testcontainers.YUGABYTEDB,
				DBVersion: version,
			}
			testDB := createTestDBTarget(ctx, config)
			defer destroyTestDBTarget(ctx, testDB)

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
