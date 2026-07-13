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

func TestPostgresGetPrimaryKeyColumns(t *testing.T) {
	testPostgresTarget.ExecuteSqls(
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
	}

	for _, tt := range tests {
		pkCols, err := testPostgresTarget.GetPrimaryKeyColumns(tt.table)
		assert.NoError(t, err)
		testutils.AssertEqualStringSlices(t, tt.expectedPKCols, pkCols)
	}
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
		`CREATE TABLE test_schema.unique_table (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE,
			phone VARCHAR(20) UNIQUE,
			address VARCHAR(255) UNIQUE
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
		// table with only a primary key and no unique index/constraint -> should not appear in the map
		`CREATE TABLE test_schema.pk_only_table (
			id INT PRIMARY KEY,
			name TEXT
		);`,
	)
	defer testPostgresTarget.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	tablesList := []sqlname.NameTuple{
		testutils.CreateNameTupleWithTargetName("test_schema.unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.another_unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.composite_unique_table", "public", POSTGRESQL),
		testutils.CreateNameTupleWithTargetName("test_schema.pk_only_table", "public", POSTGRESQL),
	}

	actualIndexes, err := testPostgresTarget.GetTableToUniqueIndexesMap(tablesList)
	require.NoError(t, err)

	expectedIndexesByTable := utils.NewStructMap[sqlname.NameTuple, [][]string]()
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.unique_table", "public", POSTGRESQL), [][]string{
		{"email"},
		{"phone"},
		{"address"},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.another_unique_table", "public", POSTGRESQL), [][]string{
		{"username"},
		{"age"},
	})
	expectedIndexesByTable.Put(testutils.CreateNameTupleWithTargetName("test_schema.composite_unique_table", "public", POSTGRESQL), [][]string{
		{"first_name", "last_name"},
		{"phone"},
	})

	assert.Equal(t, len(expectedIndexesByTable.Keys()), len(actualIndexes.Keys()), "Expected number of tables to match")

	expectedIndexesByTable.IterKV(func(table sqlname.NameTuple, expectedIndexes [][]string) (bool, error) {
		actualIndexesForTable, exists := actualIndexes.Get(table)
		if !exists {
			t.Errorf("Expected table %s not found in unique indexes map", table)
			return true, nil
		}
		assertEqualUniqueIndexes(t, expectedIndexes, actualIndexesForTable)
		return true, nil
	})
}
