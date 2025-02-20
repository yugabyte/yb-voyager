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

func TestMysqlGetAllTableNames(t *testing.T) {
	testMySQLSource.ExecuteSqls(
		`CREATE DATABASE test;`,
		`CREATE TABLE test.foo (
		id INT PRIMARY KEY,
		name VARCHAR(255)
	);`,
		`CREATE TABLE test.bar (
		id INT PRIMARY KEY,
		name VARCHAR(255)
	);`,
		`CREATE TABLE test.non_pk1(
		id INT,
		name VARCHAR(255)
	);`)
	defer testMySQLSource.ExecuteSqls(`DROP DATABASE test;`)

	sqlname.SourceDBType = "mysql"
	testMySQLSource.Source.DBName = "test" // used in query of GetAllTableNames()

	// Test GetAllTableNames
	_ = testMySQLSource.DB().Connect()
	actualTables := testMySQLSource.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("test", "foo"),
		sqlname.NewSourceName("test", "bar"),
		sqlname.NewSourceName("test", "non_pk1"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")

	testutils.AssertEqualSourceNameSlices(t, expectedTables, actualTables)
}

// TODO: Seems like a Bug somwhere, because now mysql.GetAllNonPkTables() as it is returning all the tables created in this test
// func TestMySQLGetNonPKTables(t *testing.T) {
// 	testMySQLSource.ExecuteSqls(
// 		`CREATE DATABASE test;`,
// 		`CREATE TABLE test.table1 (
// 		id INT AUTO_INCREMENT PRIMARY KEY,
// 		name VARCHAR(100)
// 	);`,
// 		`CREATE TABLE test.table2 (
// 		id INT AUTO_INCREMENT PRIMARY KEY,
// 		email VARCHAR(100)
// 	);`,
// 		`CREATE TABLE test.non_pk1(
// 		id INT,
// 		name VARCHAR(255)
// 	);`,
// 		`CREATE TABLE test.non_pk2(
// 		id INT,
// 		name VARCHAR(255)
// 	);`)
// 	defer testMySQLSource.ExecuteSqls(`DROP DATABASE test;`)

// 	testMySQLSource.Source.DBName = "test"
// 	actualTables, err := testMySQLSource.DB().GetNonPKTables()
// 	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

// 	expectedTables := []string{"test.non_pk1", "test.non_pk2"}

// 	testutils.AssertEqualStringSlices(t, expectedTables, actualTables)
// }
