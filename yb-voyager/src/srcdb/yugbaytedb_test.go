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
package srcdb

import (
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
	"gotest.tools/assert"
)

func TestYugabyteGetAllTableNames(t *testing.T) {
	testYugabyteDBSource.TestContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
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
		`CREATE TABLE test_schema.non_pk1(
		id INT,
		name VARCHAR(255)
	);`)
	defer testYugabyteDBSource.TestContainer.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	sqlname.SourceDBType = "postgresql"
	testYugabyteDBSource.Source.Schema = "test_schema"

	// Test GetAllTableNames
	actualTables := testYugabyteDBSource.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("test_schema", "foo"),
		sqlname.NewSourceName("test_schema", "bar"),
		sqlname.NewSourceName("test_schema", "non_pk1"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")
	testutils.AssertEqualSourceNameSlices(t, expectedTables, actualTables)
}

func TestYugabyteGetTableToUniqueKeyColumnsMap(t *testing.T) {
	testYugabyteDBSource.TestContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.unique_table (
            id SERIAL PRIMARY KEY,
            email VARCHAR(255) UNIQUE,
            phone VARCHAR(20) UNIQUE,
            address VARCHAR(255) UNIQUE
        );`,
		`INSERT INTO test_schema.unique_table (email, phone, address) VALUES
            ('john@example.com', '1234567890', '123 Elm Street'),
            ('jane@example.com', '0987654321', '456 Oak Avenue');`,
		`CREATE TABLE test_schema.another_unique_table (
            user_id SERIAL PRIMARY KEY,
            username VARCHAR(50) UNIQUE,
            age INT
        );`,
		`CREATE UNIQUE INDEX idx_age ON test_schema.another_unique_table(age);`,
		`INSERT INTO test_schema.another_unique_table (username, age) VALUES
            ('user1', 30),
            ('user2', 40);`)
	defer testYugabyteDBSource.TestContainer.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	uniqueTablesList := []sqlname.NameTuple{
		{CurrentName: sqlname.NewObjectName("postgresql", "test_schema", "test_schema", "unique_table")},
		{CurrentName: sqlname.NewObjectName("postgresql", "test_schema", "test_schema", "another_unique_table")},
	}

	actualUniqKeys, err := testYugabyteDBSource.DB().GetTableToUniqueKeyColumnsMap(uniqueTablesList)
	if err != nil {
		t.Fatalf("Error retrieving unique keys: %v", err)
	}

	expectedUniqKeys := map[string][]string{
		"test_schema.unique_table":         {"email", "phone", "address"},
		"test_schema.another_unique_table": {"username", "age"},
	}

	// Compare the maps by iterating over each table and asserting the columns list
	for table, expectedColumns := range expectedUniqKeys {
		actualColumns, exists := actualUniqKeys[table]
		if !exists {
			t.Errorf("Expected table %s not found in uniqueKeys", table)
		}

		testutils.AssertEqualStringSlices(t, expectedColumns, actualColumns)
	}
}

func TestYugabyteGetNonPKTables(t *testing.T) {
	testYugabyteDBSource.TestContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
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
	defer testYugabyteDBSource.TestContainer.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	actualTables, err := testYugabyteDBSource.DB().GetNonPKTables()
	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

	expectedTables := []string{`test_schema."non_pk2"`, `test_schema."non_pk1"`} // func returns table.Qualified.Quoted
	testutils.AssertEqualStringSlices(t, expectedTables, actualTables)
}
