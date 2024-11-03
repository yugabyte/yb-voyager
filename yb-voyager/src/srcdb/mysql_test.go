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
	"gotest.tools/assert"
)

func TestMysqlGetAllTableNames(t *testing.T) {
	sqlname.SourceDBType = "mysql"

	// Test GetAllTableNames
	actualTables := mysqlTestDB.Source.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("DMS", "foo"),
		sqlname.NewSourceName("DMS", "bar"),
		sqlname.NewSourceName("DMS", "table1"),
		sqlname.NewSourceName("DMS", "table2"),
		sqlname.NewSourceName("DMS", "unique_table"),
		sqlname.NewSourceName("DMS", "non_pk1"),
		sqlname.NewSourceName("DMS", "non_pk2"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")

	// Sort and compare tables
	sortSourceNames(expectedTables)
	sortSourceNames(actualTables)
	for i, expectedTable := range expectedTables {
		assert.Equal(t, expectedTable.Qualified.MinQuoted, actualTables[i].Qualified.MinQuoted, "Expected table names to match")
	}
}

func TestMySQLGetNonPKTables(t *testing.T) {
	actualTables, err := mysqlTestDB.Source.DB().GetNonPKTables()
	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

	expectedTables := []string{"DMS.non_pk1", "DMS.non_pk2"}

	err = assertEqualStringSlices(t, expectedTables, actualTables)
	assert.NilError(t, err)
}
