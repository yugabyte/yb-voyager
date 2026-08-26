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
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestCreateVoyagerSchemaPG(t *testing.T) {
	db, err := sql.Open("pgx", testPostgresTarget.GetConnectionString())
	assert.NoError(t, err)
	defer db.Close()

	// Wait for the database to be ready
	err = testutils.WaitForDBToBeReady(db)
	assert.NoError(t, err)

	// Initialize the TargetYugabyteDB instance
	pg := &TargetPostgreSQL{
		db: db,
	}

	// Call CreateVoyagerSchema
	err = pg.CreateVoyagerSchema()
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

func TestPostgresGetPrimaryKeyColumnsForTables(t *testing.T) {
	testPostgresTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		// composite primary key: column order must follow the PK definition.
		`CREATE TABLE test_schema.foo (
			id INT,
			category TEXT,
			name TEXT,
			PRIMARY KEY (id, category)
		);`,
		// single-column primary key.
		`CREATE TABLE test_schema.bar (
			id INT PRIMARY KEY,
			name TEXT
		);`,
		// no primary key: should be absent from the result map.
		`CREATE TABLE test_schema.baz (
			id INT,
			name TEXT
		);`,
		// composite primary key declared in non-attnum order: proves the declared key order
		// (category, id) is preserved rather than sorted by attnum.
		`CREATE TABLE test_schema.reversed_pk (
			id INT,
			category TEXT,
			PRIMARY KEY (category, id)
		);`,
		// covering primary key: INCLUDE columns must be excluded (indnkeyatts filter).
		`CREATE TABLE test_schema.covering_pk (
			id INT,
			name TEXT,
			PRIMARY KEY (id) INCLUDE (name)
		);`,
	)
	defer testPostgresTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

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
		{
			table:          testutils.CreateNameTupleWithTargetName("test_schema.reversed_pk", "public", POSTGRESQL),
			expectedPKCols: []string{"category", "id"},
		},
		{
			table:          testutils.CreateNameTupleWithTargetName("test_schema.covering_pk", "public", POSTGRESQL),
			expectedPKCols: []string{"id"},
		},
	}

	var tablesList []sqlname.NameTuple
	for _, tt := range tests {
		tablesList = append(tablesList, tt.table)
	}

	// Batched: fetch primary keys for all tables in a single call.
	result, err := testPostgresTarget.GetPrimaryKeyColumnsForTables(tablesList)
	require.NoError(t, err)

	for _, tt := range tests {
		pkCols, _ := result.Get(tt.table)
		testutils.AssertEqualStringSlices(t, tt.expectedPKCols, pkCols)
	}

	// Empty input returns an empty (non-nil) map without querying.
	emptyResult, err := testPostgresTarget.GetPrimaryKeyColumnsForTables(nil)
	require.NoError(t, err)
	require.NotNil(t, emptyResult)
	require.Equal(t, 0, len(emptyResult.Keys()))
}

func TestPostgresGetNonEmptyTables(t *testing.T) {
	testPostgresTarget.ExecuteSqls(
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
			UNIQUE (email, phone)  -- Unique constraint on combination of columns
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
	defer testPostgresTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	tables := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.foo", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.bar", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.table1", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.table2", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.non_pk1", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.non_pk2", "public", POSTGRESQL),
	}

	expectedTables := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.foo", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.bar", "public", POSTGRESQL),
	}

	actualTables := testPostgresTarget.GetNonEmptyTables(tables)
	testutils.AssertEqualNameTuplesSlice(t, expectedTables, actualTables)
}

func TestPostgresTargetGetTableToUniqueIndexesMap(t *testing.T) {
	testPostgresTarget.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE SCHEMA other_schema;`,
		`CREATE TABLE test_schema.unique_table (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE,
			phone VARCHAR(20) UNIQUE,
			address VARCHAR(255) UNIQUE
		);`,
		// Same table name in another schema: must not leak into results for test_schema.unique_table.
		`CREATE TABLE other_schema.unique_table (
			id SERIAL PRIMARY KEY,
			other_col VARCHAR(255) UNIQUE
		);`,
		`CREATE TABLE test_schema.another_unique_table (
			user_id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE,
			age INT
		);`,
		`CREATE UNIQUE INDEX idx_age ON test_schema.another_unique_table(age);`,
		`CREATE TABLE test_schema.composite_unique_table (
			id SERIAL PRIMARY KEY,
			first_name VARCHAR(100),
			last_name VARCHAR(100),
			phone VARCHAR(20) UNIQUE,
			CONSTRAINT unique_name UNIQUE (first_name, last_name)
		);`,
		// NULLS NOT DISTINCT via: CREATE UNIQUE INDEX, table CONSTRAINT, and column-level UNIQUE.
		`CREATE TABLE test_schema.nulls_not_distinct_table (
			id SERIAL PRIMARY KEY,
			token VARCHAR(100),
			code VARCHAR(100),
			sku VARCHAR(100),
			product_no INT UNIQUE NULLS NOT DISTINCT,
			CONSTRAINT unique_sku_nnd UNIQUE NULLS NOT DISTINCT (sku)
		);`,
		`CREATE UNIQUE INDEX idx_token_nnd ON test_schema.nulls_not_distinct_table(token) NULLS NOT DISTINCT;`,
		`CREATE UNIQUE INDEX idx_code_default ON test_schema.nulls_not_distinct_table(code);`,
		// table with only a primary key and no unique index/constraint -> should not appear in the map
		`CREATE TABLE test_schema.pk_only_table (
			id INT PRIMARY KEY,
			name TEXT
		);`,
		// partitioned table whose unique indexes are defined on the leaf partitions.
		// The merged result should be attributed to the root table.
		`CREATE TABLE test_schema.part_table (
			id INT,
			region TEXT,
			id1 INT,
			id2 INT,
			PRIMARY KEY (id, region)
		) PARTITION BY LIST (region);`,
		`CREATE TABLE test_schema.part_table_r1 PARTITION OF test_schema.part_table FOR VALUES IN ('r1');`,
		`CREATE TABLE test_schema.part_table_r2 PARTITION OF test_schema.part_table FOR VALUES IN ('r2');`,
		// r1's (id1, id2) index is NULLS NOT DISTINCT, r2's is the default NULLS DISTINCT.
		// When merged into the root, the stricter NULLS NOT DISTINCT should win.
		`CREATE UNIQUE INDEX idx_part_table_r1_id1_id2 ON test_schema.part_table_r1 (id1, id2) NULLS NOT DISTINCT;`,
		`CREATE UNIQUE INDEX idx_part_table_r2_id1_id2 ON test_schema.part_table_r2 (id1, id2);`,
		// case-sensitive table/column unique index
		`CREATE TABLE test_schema."CaseTable" (
			id SERIAL PRIMARY KEY,
			"Id2" INT,
			id3 INT
		);`,
		`CREATE UNIQUE INDEX idx_case_id2 ON test_schema."CaseTable" ("Id2");`,
		`CREATE UNIQUE INDEX idx_case_id3 ON test_schema."CaseTable" (id3);`,
		// partial unique index (WHERE clause) should still be discovered by column list
		`CREATE TABLE test_schema.partial_unique_table (
			id SERIAL PRIMARY KEY,
			check_id INT,
			most_recent BOOLEAN
		);`,
		`CREATE UNIQUE INDEX idx_partial_check_id ON test_schema.partial_unique_table (check_id) WHERE most_recent;`,
		// expression-only unique index has no plain columns, so the table should not appear
		`CREATE UNIQUE INDEX idx_partial_check_id_include ON test_schema.partial_unique_table ((check_id+id)) INCLUDE (most_recent);`, //this won't reflect in the query results as we don't really fetch indexes properly that have expressions in the key
		`CREATE TABLE test_schema.expression_unique_table (
			id SERIAL PRIMARY KEY,
			email TEXT
		);`,
		`CREATE UNIQUE INDEX idx_expr_email ON test_schema.expression_unique_table (lower(email));`,
		// mixed expression+column unique index should surface only the plain column
		`CREATE TABLE test_schema.mixed_expression_unique_table (
			id SERIAL PRIMARY KEY,
			email TEXT,
			code TEXT
		);`,
		`CREATE UNIQUE INDEX idx_mixed_expr ON test_schema.mixed_expression_unique_table (lower(email), code);`,
		`CREATE UNIQUE INDEX idx_including_unique ON test_schema.mixed_expression_unique_table (email) INCLUDE (code);`,
		`CREATE TABLE test_schema.unique_table_with_include (
			id SERIAL PRIMARY KEY,
			email TEXT,
			code TEXT,
			UNIQUE (code) INCLUDE (email)
		);`,
	)
	defer testPostgresTarget.ExecuteSqls(
		`DROP SCHEMA test_schema CASCADE;`,
		`DROP SCHEMA other_schema CASCADE;`,
	)

	tablesList := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.another_unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.composite_unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.nulls_not_distinct_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.pk_only_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.part_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.\"CaseTable\"", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.partial_unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.expression_unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.mixed_expression_unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.unique_table_with_include", "public", YUGABYTEDB),
	}

	actualIndexes, err := testPostgresTarget.GetTableToUniqueIndexesMap(tablesList)
	require.NoError(t, err)

	expectedIndexesByTable := utils.NewStructMap[sqlname.NameTuple, []UniqueIndex]()
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.unique_table", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"email"}},
		{Columns: []string{"phone"}},
		{Columns: []string{"address"}},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.another_unique_table", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"username"}},
		{Columns: []string{"age"}},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.composite_unique_table", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"first_name", "last_name"}},
		{Columns: []string{"phone"}},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.nulls_not_distinct_table", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"token"}, NullsNotDistinct: true},
		{Columns: []string{"code"}, NullsNotDistinct: false},
		{Columns: []string{"sku"}, NullsNotDistinct: true},
		{Columns: []string{"product_no"}, NullsNotDistinct: true},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.part_table", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"id1", "id2"}, NullsNotDistinct: true},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.\"CaseTable\"", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"Id2"}},
		{Columns: []string{"id3"}},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.partial_unique_table", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"check_id"}},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.mixed_expression_unique_table", "public", POSTGRESQL), []UniqueIndex{
		{Columns: []string{"code"}},
		{Columns: []string{"email"}},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.unique_table_with_include", "public", YUGABYTEDB), []UniqueIndex{
		{Columns: []string{"code"}},
	})

	assert.Equal(t, len(expectedIndexesByTable.Keys()), len(actualIndexes.Keys()), "Expected number of tables to match")

	expectedIndexesByTable.IterKV(func(table sqlname.NameTuple, expectedIndexes []UniqueIndex) (bool, error) {
		actualIndexesForTable, exists := actualIndexes.Get(table)
		if !exists {
			t.Errorf("Expected table %s not found in unique indexes map", table)
			return true, nil
		}
		assertEqualUniqueIndexes(t, expectedIndexes, actualIndexesForTable)
		return true, nil
	})

	// pk-only and expression-only unique-index tables must be absent from the map
	_, pkOnlyExists := actualIndexes.Get(testutils.CreateNameTupleWithTargetName("test_schema.pk_only_table", "public", POSTGRESQL))
	assert.False(t, pkOnlyExists, "pk_only_table should not appear in unique indexes map")
	_, exprOnlyExists := actualIndexes.Get(testutils.CreateNameTupleWithTargetName("test_schema.expression_unique_table", "public", POSTGRESQL))
	assert.False(t, exprOnlyExists, "expression_unique_table should not appear (expression-only unique index)")

	// subset tableList: only the requested table is returned, and other_schema does not leak
	subsetList := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.unique_table", "public", POSTGRESQL),
	}
	subsetIndexes, err := testPostgresTarget.GetTableToUniqueIndexesMap(subsetList)
	require.NoError(t, err)
	assert.Equal(t, 1, len(subsetIndexes.Keys()))
	subsetActual, exists := subsetIndexes.Get(subsetList[0])
	require.True(t, exists)
	assertEqualUniqueIndexes(t, []UniqueIndex{
		{Columns: []string{"email"}},
		{Columns: []string{"phone"}},
		{Columns: []string{"address"}},
	}, subsetActual)
	_, otherSchemaExists := subsetIndexes.Get(testutils.CreateNameTupleWithTargetName("other_schema.unique_table", "public", POSTGRESQL))
	assert.False(t, otherSchemaExists, "other_schema.unique_table must not leak into subset results")
}
