package sqlname

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameTupleEquals(t *testing.T) {
	assert := assert.New(t)
	o1 := NewObjectName(POSTGRESQL, "public", "public", "table1")
	o2 := NewObjectName(POSTGRESQL, "public", "public", "table1")
	nameTuple1 := NameTuple{CurrentName: o1, SourceName: o1, TargetName: o1}
	nameTuple2 := NameTuple{CurrentName: o2, SourceName: o2, TargetName: o2}
	assert.True(nameTuple1.Equals(nameTuple2))

	o3 := NewObjectName(POSTGRESQL, "public", "public", "table2")
	nameTuple3 := NameTuple{CurrentName: o3, SourceName: o3, TargetName: o3}
	assert.False(nameTuple1.Equals(nameTuple3))
}

func TestPGDefaultSchemaCaseInsensitiveTableName(t *testing.T) {
	assert := assert.New(t)

	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a table name belonging to default schema.
	tableName := NewObjectName(POSTGRESQL, "public", "public", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "public",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `public."table1"`,
			Unquoted:  "public.table1",
			MinQuoted: "public.table1",
		},
		Unqualified: identifier{
			Quoted:    `"table1"`,
			Unquoted:  "table1",
			MinQuoted: "table1",
		},
		MinQualified: identifier{
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
	tableName := NewObjectName(POSTGRESQL, "public", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "schema1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `schema1."table1"`,
			Unquoted:  "schema1.table1",
			MinQuoted: "schema1.table1",
		},
		Unqualified: identifier{
			Quoted:    `"table1"`,
			Unquoted:  "table1",
			MinQuoted: "table1",
		},
		MinQualified: identifier{
			Quoted:    `schema1."table1"`,
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
	tableName := NewObjectName(POSTGRESQL, "public", "public", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "public",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `public."Table1"`,
			Unquoted:  `public.Table1`,
			MinQuoted: `public."Table1"`,
		},
		Unqualified: identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: identifier{
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
	tableName := NewObjectName(POSTGRESQL, "public", "schema1", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "schema1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `schema1."Table1"`,
			Unquoted:  `schema1.Table1`,
			MinQuoted: `schema1."Table1"`,
		},
		Unqualified: identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: identifier{
			Quoted:    `schema1."Table1"`,
			Unquoted:  `schema1.Table1`,
			MinQuoted: `schema1."Table1"`,
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
	tableName := NewObjectName(ORACLE, "SAKILA", "SAKILA", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "SAKILA",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `SAKILA."TABLE1"`,
			Unquoted:  `SAKILA.TABLE1`,
			MinQuoted: `SAKILA.TABLE1`,
		},
		Unqualified: identifier{
			Quoted:    `"TABLE1"`,
			Unquoted:  `TABLE1`,
			MinQuoted: `TABLE1`,
		},
		MinQualified: identifier{
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
	tableName := NewObjectName(ORACLE, "SAKILA", "SCHEMA1", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "SCHEMA1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `SCHEMA1."TABLE1"`,
			Unquoted:  `SCHEMA1.TABLE1`,
			MinQuoted: `SCHEMA1.TABLE1`,
		},
		Unqualified: identifier{
			Quoted:    `"TABLE1"`,
			Unquoted:  `TABLE1`,
			MinQuoted: `TABLE1`,
		},
		MinQualified: identifier{
			Quoted:    `SCHEMA1."TABLE1"`,
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
	tableName := NewObjectName(ORACLE, "SAKILA", "SAKILA", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "SAKILA",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `SAKILA."Table1"`,
			Unquoted:  `SAKILA.Table1`,
			MinQuoted: `SAKILA."Table1"`,
		},
		Unqualified: identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: identifier{
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
	tableName := NewObjectName(ORACLE, "SAKILA", "SCHEMA1", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "SCHEMA1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `SCHEMA1."Table1"`,
			Unquoted:  `SCHEMA1.Table1`,
			MinQuoted: `SCHEMA1."Table1"`,
		},
		Unqualified: identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: identifier{
			Quoted:    `SCHEMA1."Table1"`,
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
	tableName := NewObjectName(MYSQL, "sakila", "sakila", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "sakila",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `sakila."table1"`,
			Unquoted:  `sakila.table1`,
			MinQuoted: `sakila."table1"`,
		},
		Unqualified: identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
		MinQualified: identifier{
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
	tableName := NewObjectName(MYSQL, "sakila", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "schema1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `schema1."table1"`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `schema1."table1"`,
		},
		Unqualified: identifier{
			Quoted:    `"table1"`,
			Unquoted:  `table1`,
			MinQuoted: `"table1"`,
		},
		MinQualified: identifier{
			Quoted:    `schema1."table1"`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `schema1."table1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

func TestMySQLDefaultSchemaCaseSensitiveMixedCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a case-sensitive name with mixed cases.
	tableName := NewObjectName(MYSQL, "sakila", "sakila", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "sakila",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `sakila."Table1"`,
			Unquoted:  `sakila.Table1`,
			MinQuoted: `sakila."Table1"`,
		},
		Unqualified: identifier{
			Quoted:    `"Table1"`,
			Unquoted:  `Table1`,
			MinQuoted: `"Table1"`,
		},
		MinQualified: identifier{
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
	tableName := NewObjectName(MYSQL, "sakila", "schema1", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
		SchemaName:        "schema1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `schema1."TABLE1"`,
			Unquoted:  `schema1.TABLE1`,
			MinQuoted: `schema1."TABLE1"`,
		},
		Unqualified: identifier{
			Quoted:    `"TABLE1"`,
			Unquoted:  `TABLE1`,
			MinQuoted: `"TABLE1"`,
		},
		MinQualified: identifier{
			Quoted:    `schema1."TABLE1"`,
			Unquoted:  `schema1.TABLE1`,
			MinQuoted: `schema1."TABLE1"`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}
