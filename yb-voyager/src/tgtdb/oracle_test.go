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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestOracleCheckTableHasPrimaryKey(t *testing.T) {
	// For now tables are created in Oracle through sql script at initialisation
	// testOracleTarget.ExecuteSqls(
	// 	`CREATE TABLE FOO (ID NUMBER PRIMARY KEY, NAME VARCHAR2(100));`,
	// 	`CREATE TABLE BAR (ID NUMBER, NAME VARCHAR2(100));`,
	// 	`CREATE TABLE UNIQUE_TABLE (ID NUMBER PRIMARY KEY, NAME VARCHAR2(100) UNIQUE);`,
	// 	`CREATE TABLE UNIQUE_TABLE (ID NUMBER UNIQUE, NAME VARCHAR2(100));`)

	tables := []sqlname.NameTuple{
		{CurrentName: sqlname.NewObjectName(ORACLE, "YBVOYAGER", "YBVOYAGER", "FOO")},
		{CurrentName: sqlname.NewObjectName(ORACLE, "YBVOYAGER", "YBVOYAGER", "TABLE1")},
		{CurrentName: sqlname.NewObjectName(ORACLE, "YBVOYAGER", "YBVOYAGER", "NON_PK1")},
	}

	expectedListOfTablesWithPK := []sqlname.NameTuple{
		{CurrentName: sqlname.NewObjectName(ORACLE, "YBVOYAGER", "YBVOYAGER", "FOO")},
		{CurrentName: sqlname.NewObjectName(ORACLE, "YBVOYAGER", "YBVOYAGER", "TABLE1")},
	}

	for _, table := range tables {
		hasPrimaryKey := testOracleTarget.CheckTableHasPrimaryKey(&table)
		if testutils.SliceContainsTuple(expectedListOfTablesWithPK, table) {
			assert.Equal(t, true, hasPrimaryKey, fmt.Sprintf("Table '%s' should have a primary key", table.CurrentName))
		} else {
			assert.Equal(t, false, hasPrimaryKey, fmt.Sprintf("Table '%s' should not have a primary key", table.CurrentName))
		}
	}

}
