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
	actualTables := testMySQLSource.DB().GetAllTableNames()
	expectedTables := []*sqlname.SourceName{
		sqlname.NewSourceName("dms", "foo"),
		sqlname.NewSourceName("dms", "bar"),
		sqlname.NewSourceName("dms", "table1"),
		sqlname.NewSourceName("dms", "table2"),
		sqlname.NewSourceName("dms", "unique_table"),
		sqlname.NewSourceName("dms", "non_pk1"),
		sqlname.NewSourceName("dms", "non_pk2"),
	}
	assert.Equal(t, len(expectedTables), len(actualTables), "Expected number of tables to match")

	assertEqualSourceNameSlices(t, expectedTables, actualTables)
}

func TestMySQLGetNonPKTables(t *testing.T) {
	actualTables, err := testMySQLSource.Source.DB().GetNonPKTables()
	assert.NilError(t, err, "Expected nil but non nil error: %v", err)

	expectedTables := []string{"dms.non_pk1", "dms.non_pk2"}

	assertEqualStringSlices(t, expectedTables, actualTables)
}
