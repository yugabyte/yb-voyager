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
package sqlname

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

func TestNameTupleEquals(t *testing.T) {
	assert := assert.New(t)
	o1 := NewObjectName(constants.POSTGRESQL, "public", "public", "table1")
	o2 := NewObjectName(constants.POSTGRESQL, "public", "public", "table1")
	nameTuple1 := NameTuple{CurrentName: o1, SourceName: o1, TargetName: o1}
	nameTuple2 := NameTuple{CurrentName: o2, SourceName: o2, TargetName: o2}
	assert.True(nameTuple1.Equals(nameTuple2))

	o3 := NewObjectName(constants.POSTGRESQL, "public", "public", "table2")
	nameTuple3 := NameTuple{CurrentName: o3, SourceName: o3, TargetName: o3}
	assert.False(nameTuple1.Equals(nameTuple3))
}

func TestPGDefaultSchemaCaseInsensitiveTableName(t *testing.T) {
	assert := assert.New(t)

	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a table name belonging to default schema.
	tableName := NewObjectName(constants.POSTGRESQL, "public", "public", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"public"`,
			Unquoted:  `public`,
			MinQuoted: `public`,
		},
		FromDefaultSchema: true,
		Qualified: Identifier{
			Quoted:    `"public"."table1"`,
			Unquoted:  "public.table1",
			MinQuoted: "public.table1",
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  "table1",
			MinQuoted: "table1",
		},
		MinQualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  "table1",
			MinQuoted: "table1",
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestPGNonDefaultSchemaCaseInsensitiveTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a table name belonging to a non-default schema.
	tableName := NewObjectName(constants.POSTGRESQL, "public", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"schema1"`,
			Unquoted:  `schema1`,
			MinQuoted: `schema1`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"schema1"."table1"`,
			Unquoted:  "schema1.table1",
			MinQuoted: "schema1.table1",
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  "table1",
			MinQuoted: "table1",
		},
		MinQualified: Identifier{
			Quoted:    `"schema1"."table1"`,
			Unquoted:  "schema1.table1",
			MinQuoted: "schema1.table1",
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

func TestPGDefaultSchemaCaseSensitiveTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a case-sensitive name with mixed cases.
	tableName := NewObjectName(constants.POSTGRESQL, "public", "public", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"public"`,
			Unquoted:  `public`,
			MinQuoted: `public`,
		},
		FromDefaultSchema: true,
		Qualified: Identifier{
			Quoted:    `"public"."Table1"`,
			Unquoted:  `public.Table1`,
			MinQuoted: `public."Table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestPGNonDefaultSchemaCaseSensitiveTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a case-sensitive name with mixed cases.
	tableName := NewObjectName(constants.POSTGRESQL, "public", "schema1", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"schema1"`,
			Unquoted:  `schema1`,
			MinQuoted: `schema1`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"schema1"."Table1"`,
			Unquoted:  `schema1.Table1`,
			MinQuoted: `schema1."Table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"schema1"."Table1"`,
			Unquoted:  `schema1.Table1`,
			MinQuoted: `schema1."Table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

func TestPGNonDefaultSchemaTableNameWithSpecialChars(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a case-sensitive name with mixed cases.
	tableName := NewObjectName(constants.POSTGRESQL, "public", "schema1", "table$1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"schema1"`,
			Unquoted:  `schema1`,
			MinQuoted: `schema1`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"schema1"."table$1"`,
			Unquoted:  `schema1.table$1`,
			MinQuoted: `schema1."table$1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table$1"`,
			Unquoted:  `table$1`,
			MinQuoted: `"table$1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"schema1"."table$1"`,
			Unquoted:  `schema1.table$1`,
			MinQuoted: `schema1."table$1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

//=====================================================

func TestOracleDefaultSchemaCaseInsensitiveTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with Oracle and default schema "SAKILA" and
	// a table name belonging to default schema.
	tableName := NewObjectName(constants.ORACLE, "SAKILA", "SAKILA", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"SAKILA"`,
			Unquoted:  `SAKILA`,
			MinQuoted: `SAKILA`,
		},
		FromDefaultSchema: true,
		Qualified: Identifier{
			Quoted:    `"SAKILA"."TABLE1"`,
			Unquoted:  `SAKILA.TABLE1`,
			MinQuoted: `SAKILA.TABLE1`,
		},
		Unqualified: Identifier{
			Quoted:    `"TABLE1"`,
			Unquoted:  `TABLE1`,
			MinQuoted: `TABLE1`,
		},
		MinQualified: Identifier{
			Quoted:    `"TABLE1"`,
			Unquoted:  `TABLE1`,
			MinQuoted: `TABLE1`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestOracleNonDefaultSchemaCaseInsensitiveTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with Oracle and default schema "SAKILA" and
	// a table name belonging to a non-default schema.
	tableName := NewObjectName(constants.ORACLE, "SAKILA", "SCHEMA1", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"SCHEMA1"`,
			Unquoted:  `SCHEMA1`,
			MinQuoted: `SCHEMA1`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"SCHEMA1"."TABLE1"`,
			Unquoted:  `SCHEMA1.TABLE1`,
			MinQuoted: `SCHEMA1.TABLE1`,
		},
		Unqualified: Identifier{
			Quoted:    `"TABLE1"`,
			Unquoted:  `TABLE1`,
			MinQuoted: `TABLE1`,
		},
		MinQualified: Identifier{
			Quoted:    `"SCHEMA1"."TABLE1"`,
			Unquoted:  `SCHEMA1.TABLE1`,
			MinQuoted: `SCHEMA1.TABLE1`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

func TestOracleDefaultSchemaCaseSensitiveTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with Oracle and default schema "SAKILA" and
	// a case-sensitive name with mixed cases.
	tableName := NewObjectName(constants.ORACLE, "SAKILA", "SAKILA", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"SAKILA"`,
			Unquoted:  `SAKILA`,
			MinQuoted: `SAKILA`,
		},
		FromDefaultSchema: true,
		Qualified: Identifier{
			Quoted:    `"SAKILA"."Table1"`,
			Unquoted:  `SAKILA.Table1`,
			MinQuoted: `SAKILA."Table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestOracleNonDefaultSchemaCaseSensitiveTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with Oracle and default schema "SAKILA" and
	// a case-sensitive name with mixed cases.
	tableName := NewObjectName(constants.ORACLE, "SAKILA", "SCHEMA1", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"SCHEMA1"`,
			Unquoted:  `SCHEMA1`,
			MinQuoted: `SCHEMA1`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"SCHEMA1"."Table1"`,
			Unquoted:  `SCHEMA1.Table1`,
			MinQuoted: `SCHEMA1."Table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"SCHEMA1"."Table1"`,
			Unquoted:  `SCHEMA1.Table1`,
			MinQuoted: `SCHEMA1."Table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

//=====================================================

func TestMySQLDefaultSchemaCaseSensitiveLowerCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a table name belonging to default schema.
	tableName := NewObjectName(constants.MYSQL, "sakila", "sakila", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"sakila"`,
			Unquoted:  `sakila`,
			MinQuoted: `"sakila"`,
		},
		FromDefaultSchema: true,
		Qualified: Identifier{
			Quoted:    `"sakila"."table1"`,
			Unquoted:  `sakila.table1`,
			MinQuoted: `"sakila"."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestMySQLNonDefaultSchemaCaseSensitiveLowerCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a table name belonging to a non-default schema.
	tableName := NewObjectName(constants.MYSQL, "sakila", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"schema1"`,
			Unquoted:  `schema1`,
			MinQuoted: `"schema1"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"schema1"."table1"`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `"schema1"."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"schema1"."table1"`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `"schema1"."table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

func TestMySQLDefaultSchemaCaseSensitiveMixedCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a case-sensitive name with mixed cases.
	tableName := NewObjectName(constants.MYSQL, "sakila", "sakila", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"sakila"`,
			Unquoted:  `sakila`,
			MinQuoted: `"sakila"`,
		},
		FromDefaultSchema: true,
		Qualified: Identifier{
			Quoted:    `"sakila"."Table1"`,
			Unquoted:  `sakila.Table1`,
			MinQuoted: `"sakila"."Table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestMySQLNonDefaultSchemaCaseSensitiveUpperCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a case-sensitive name with all upper case letters.
	tableName := NewObjectName(constants.MYSQL, "sakila", "schema1", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"schema1"`,
			Unquoted:  `schema1`,
			MinQuoted: `"schema1"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"schema1"."TABLE1"`,
			Unquoted:  `schema1.TABLE1`,
			MinQuoted: `"schema1"."TABLE1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"TABLE1"`,
			Unquoted:  `TABLE1`,
			MinQuoted: `"TABLE1"`,
		},
		MinQualified: Identifier{
			Quoted:    `"schema1"."TABLE1"`,
			Unquoted:  `schema1.TABLE1`,
			MinQuoted: `"schema1"."TABLE1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

func TestCaseSensitiveSchemaName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a case-sensitive schema name.
	tableName := NewObjectName(constants.POSTGRESQL, "public", "Schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"Schema1"`,
			Unquoted:  `Schema1`,
			MinQuoted: `"Schema1"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"Schema1"."table1"`,
			Unquoted:  `Schema1.table1`,
			MinQuoted: `"Schema1".table1`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `table1`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

	tableName = NewObjectName(constants.POSTGRESQL, "public", "SCHEMA1", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"SCHEMA1"`,
			Unquoted:  `SCHEMA1`,
			MinQuoted: `"SCHEMA1"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"SCHEMA1"."table1"`,
			Unquoted:  `SCHEMA1.table1`,
			MinQuoted: `"SCHEMA1".table1`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `table1`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

	tableName = NewObjectName(constants.POSTGRESQL, "public", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"schema1"`,
			Unquoted:  `schema1`,
			MinQuoted: `schema1`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"schema1"."table1"`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `schema1.table1`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `table1`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

	//oracle cases for schema name
	tableName = NewObjectName(constants.ORACLE, "SAKILA", "Schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"Schema1"`,
			Unquoted:  `Schema1`,
			MinQuoted: `"Schema1"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"Schema1"."table1"`,
			Unquoted:  `Schema1.table1`,
			MinQuoted: `"Schema1"."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

	tableName = NewObjectName(constants.ORACLE, "SAKILA", "SCHEMA1", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"SCHEMA1"`,
			Unquoted:  `SCHEMA1`,
			MinQuoted: `SCHEMA1`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"SCHEMA1"."table1"`,
			Unquoted:  `SCHEMA1.table1`,
			MinQuoted: `SCHEMA1."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

	tableName = NewObjectName(constants.ORACLE, "SAKILA", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"schema1"`,
			Unquoted:  `schema1`,
			MinQuoted: `"schema1"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"schema1"."table1"`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `"schema1"."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

	//mysql cases for db name as schema name
	tableName = NewObjectName(constants.MYSQL, "sakila", "sakila", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"sakila"`,
			Unquoted:  `sakila`,
			MinQuoted: `"sakila"`,
		},
		FromDefaultSchema: true,
		Qualified: Identifier{
			Quoted:    `"sakila"."table1"`,
			Unquoted:  `sakila.table1`,
			MinQuoted: `"sakila"."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Unqualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)

	tableName = NewObjectName(constants.MYSQL, "sakila", "Sakila", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"Sakila"`,
			Unquoted:  `Sakila`,
			MinQuoted: `"Sakila"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"Sakila"."table1"`,
			Unquoted:  `Sakila.table1`,
			MinQuoted: `"Sakila"."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

	tableName = NewObjectName(constants.MYSQL, "sakila", "SAKILA", "table1")
	assert.NotNil(tableName)
	expectedTableName = &ObjectName{
		SchemaName: Identifier{
			Quoted:    `"SAKILA"`,
			Unquoted:  `SAKILA`,
			MinQuoted: `"SAKILA"`,
		},
		FromDefaultSchema: false,
		Qualified: Identifier{
			Quoted:    `"SAKILA"."table1"`,
			Unquoted:  `SAKILA.table1`,
			MinQuoted: `"SAKILA"."table1"`,
		},
		Unqualified: Identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
	}
	expectedTableName.MinQualified = expectedTableName.Qualified
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)

}

func TestMatchesPattern(t *testing.T) {
	assert := assert.New(t)

	type patternTest struct {
		pattern     string
		table1Match bool
		table2Match bool
	}

	testCases := []struct {
		name       string
		schemaName string
		tableName1 string
		tableName2 string
		patterns   []patternTest
	}{
		{
			name:       "lowercase schema and table",
			schemaName: "public",
			tableName1: "table1",
			tableName2: "table2",
			patterns: []patternTest{
				{pattern: `"public"."table1"`, table1Match: true, table2Match: false},
				{pattern: `"public".table1`, table1Match: true, table2Match: false},
				{pattern: `public."table1"`, table1Match: true, table2Match: false},
				{pattern: `public.table1`, table1Match: true, table2Match: false},
				{pattern: `public.table2`, table1Match: false, table2Match: true},
				{pattern: `public.table*`, table1Match: true, table2Match: true},
				//case sensitive pattern's shouldn't match with lower case name
				{pattern: `public."Table1"`, table1Match: false, table2Match: false},
				{pattern: `public.Table1`, table1Match: true, table2Match: false},
				{pattern: `public."Table2"`, table1Match: false, table2Match: false},
				{pattern: `public.Table2`, table1Match: false, table2Match: true},
				{pattern: `public."Table*"`, table1Match: false, table2Match: false},
				{pattern: `"Public"."Table1"`, table1Match: false, table2Match: false},

			},
		},
		{
			name:       "case sensitive schema",
			schemaName: "Public",
			tableName1: "table1",
			tableName2: "table2",
			patterns: []patternTest{
				{pattern: `"Public"."table1"`, table1Match: true, table2Match: false},
				{pattern: `"Public".table1`, table1Match: true, table2Match: false},
				{pattern: `Public."table1"`, table1Match: true, table2Match: false},
				{pattern: `Public.table1`, table1Match: true, table2Match: false},
				{pattern: `Public.table2`, table1Match: false, table2Match: true},
				{pattern: `Public.table*`, table1Match: true, table2Match: true},
				//lower case pattern's shouldn't match with case sensitive schema name
				{pattern: `"public".table1`, table1Match: false, table2Match: false},
				{pattern: `public."table1"`, table1Match: true, table2Match: false},
				{pattern: `public.table1`, table1Match: true, table2Match: false},
				{pattern: `public.table2`, table1Match: false, table2Match: true},
				{pattern: `public.table*`, table1Match: true, table2Match: true},
			},
		},
		{
			name:       "case sensitive table",
			schemaName: "public",
			tableName1: "Table1",
			tableName2: "Table2",
			patterns: []patternTest{
				{pattern: `"public"."Table1"`, table1Match: true, table2Match: false},
				{pattern: `"public".Table1`, table1Match: true, table2Match: false},
				{pattern: `public."Table1"`, table1Match: true, table2Match: false},
				{pattern: `public.Table1`, table1Match: true, table2Match: false},
				{pattern: `public.Table2`, table1Match: false, table2Match: true},
				{pattern: `public.Table*`, table1Match: true, table2Match: true},
				//lower case pattern's shouldn't match with case sensitive table name
				{pattern: `"public".table1`, table1Match: true, table2Match: false},
				{pattern: `public."table1"`, table1Match: false, table2Match: false},
				{pattern: `public.table1`, table1Match: true, table2Match: false},
				{pattern: `public.table2`, table1Match: false, table2Match: true},
				{pattern: `public.table*`, table1Match: true, table2Match: true},
				//case sensitive schema shouldn't match with lower case schema name
				{pattern: `"Public"."Table1"`, table1Match: false, table2Match: false},
				{pattern: `"Public".Table1`, table1Match: false, table2Match: false},
				{pattern: `Public."Table1"`, table1Match: true, table2Match: false},
				{pattern: `Public.Table1`, table1Match: true, table2Match: false},
				{pattern: `Public.Table2`, table1Match: false, table2Match: true},
				{pattern: `Public.Table*`, table1Match: true, table2Match: true},
			},
		},
		{
			name:       "case sensitive schema and table",
			schemaName: "Public",
			tableName1: "Table1",
			tableName2: "Table2",
			patterns: []patternTest{
				{pattern: `"Public"."Table1"`, table1Match: true, table2Match: false},
				{pattern: `"Public".Table1`, table1Match: true, table2Match: false},
				{pattern: `Public."Table1"`, table1Match: true, table2Match: false},
				{pattern: `Public.Table1`, table1Match: true, table2Match: false},
				{pattern: `Public.Table2`, table1Match: false, table2Match: true},
				{pattern: `Public.Table*`, table1Match: true, table2Match: true},
				//lower case pattern's shouldn't match with case sensitive schema name
				{pattern: `"public".Table1`, table1Match: false, table2Match: false},
				{pattern: `public."Table1"`, table1Match: true, table2Match: false},
				{pattern: `public.Table1`, table1Match: true, table2Match: false},
				{pattern: `public.Table2`, table1Match: false, table2Match: true},
				{pattern: `public.Table*`, table1Match: true, table2Match: true},
				//case sensitive table shouldn't match with lower case table name
				{pattern: `"public"."table1"`, table1Match: false, table2Match: false},
				{pattern: `"public".table1`, table1Match: false, table2Match: false},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			table1 := NewObjectName(constants.POSTGRESQL, tc.schemaName, tc.schemaName, tc.tableName1)
			table2 := NewObjectName(constants.POSTGRESQL, tc.schemaName, tc.schemaName, tc.tableName2)

			for _, pt := range tc.patterns {
				matches, err := table1.MatchesPattern(pt.pattern)
				assert.Equal(pt.table1Match, matches, "table1 should match pattern %q for case %s", pt.pattern, tc.name)
				assert.NoError(err, "table1 pattern %q should not error", pt.pattern)

				matches, err = table2.MatchesPattern(pt.pattern)
				assert.Equal(pt.table2Match, matches, "table2 should match pattern %q for case %s", pt.pattern, tc.name)
				assert.NoError(err, "table2 pattern %q should not error", pt.pattern)
			}
		})
	}
}
