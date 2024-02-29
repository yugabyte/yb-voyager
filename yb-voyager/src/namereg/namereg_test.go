package namereg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPGDefaultSchemaCaseInsensitiveTableName(t *testing.T) {
	assert := assert.New(t)

	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a table name belonging to default schema.
	tableName := newTableName(POSTGRESQL, "public", "public", "table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(POSTGRESQL, "public", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(POSTGRESQL, "public", "public", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(POSTGRESQL, "public", "schema1", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(ORACLE, "SAKILA", "SAKILA", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(ORACLE, "SAKILA", "SCHEMA1", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(ORACLE, "SAKILA", "SAKILA", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(ORACLE, "SAKILA", "SCHEMA1", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
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
	tableName := newTableName(MYSQL, "sakila", "sakila", "table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
		SchemaName:        "sakila",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `sakila.table1`,
			Unquoted:  `sakila.table1`,
			MinQuoted: `sakila.table1`,
		},
		Unqualified: identifier{
			Quoted:    `table1`,
			Unquoted:  `table1`,
			MinQuoted: `table1`,
		},
		MinQualified: identifier{
			Quoted:    `table1`,
			Unquoted:  `table1`,
			MinQuoted: `table1`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestMySQLNonDefaultSchemaCaseSensitiveLowerCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a table name belonging to a non-default schema.
	tableName := newTableName(MYSQL, "sakila", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
		SchemaName:        "schema1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `schema1.table1`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `schema1.table1`,
		},
		Unqualified: identifier{
			Quoted:    `table1`,
			Unquoted:  `table1`,
			MinQuoted: `table1`,
		},
		MinQualified: identifier{
			Quoted:    `schema1.table1`,
			Unquoted:  `schema1.table1`,
			MinQuoted: `schema1.table1`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

func TestMySQLDefaultSchemaCaseSensitiveMixedCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a case-sensitive name with mixed cases.
	tableName := newTableName(MYSQL, "sakila", "sakila", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
		SchemaName:        "sakila",
		FromDefaultSchema: true,
		Qualified: identifier{
			Quoted:    `sakila.Table1`,
			Unquoted:  `sakila.Table1`,
			MinQuoted: `sakila.Table1`,
		},
		Unqualified: identifier{
			Quoted:    `Table1`,
			Unquoted:  `Table1`,
			MinQuoted: `Table1`,
		},
		MinQualified: identifier{
			Quoted:    `Table1`,
			Unquoted:  `Table1`,
			MinQuoted: `Table1`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Unqualified)
}

func TestMySQLNonDefaultSchemaCaseSensitiveUpperCaseTableName(t *testing.T) {
	assert := assert.New(t)
	// Test NewTableName() with MySQL and default schema "sakila" and
	// a case-sensitive name with all upper case letters.
	tableName := newTableName(MYSQL, "sakila", "schema1", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &TableName{
		SchemaName:        "schema1",
		FromDefaultSchema: false,
		Qualified: identifier{
			Quoted:    `schema1.TABLE1`,
			Unquoted:  `schema1.TABLE1`,
			MinQuoted: `schema1.TABLE1`,
		},
		Unqualified: identifier{
			Quoted:    `TABLE1`,
			Unquoted:  `TABLE1`,
			MinQuoted: `TABLE1`,
		},
		MinQualified: identifier{
			Quoted:    `schema1.TABLE1`,
			Unquoted:  `schema1.TABLE1`,
			MinQuoted: `schema1.TABLE1`,
		},
	}
	assert.Equal(expectedTableName, tableName)
	assert.Equal(tableName.MinQualified, tableName.Qualified)
}

//=====================================================

func TestNameTuple(t *testing.T) {
	assert := assert.New(t)

	ntup := &NameTuple{
		SourceTableName: newTableName(ORACLE, "SAKILA", "SAKILA", "TABLE1"),
		TargetTableName: newTableName(YUGABYTEDB, "public", "public", "table1"),
	}
	ntup.SetMode(IMPORT_TO_TARGET_MODE)
	assert.Equal(ntup.CurrentTableName, ntup.TargetTableName)
	assert.Equal(ntup.ForUserQuery(), `public."table1"`)
	schemaName, tableName := ntup.ForCatalogQuery()
	assert.Equal(schemaName, `public`)
	assert.Equal(tableName, `table1`)

	ntup.SetMode(IMPORT_TO_SOURCE_REPLICA_MODE)
	assert.Equal(ntup.CurrentTableName, ntup.SourceTableName)
	assert.Equal(ntup.ForUserQuery(), `SAKILA."TABLE1"`)
	schemaName, tableName = ntup.ForCatalogQuery()
	assert.Equal(schemaName, `SAKILA`)
	assert.Equal(tableName, `TABLE1`)

	ntup.SetMode(EXPORT_FROM_SOURCE_MODE)
	assert.Equal(ntup.CurrentTableName, ntup.SourceTableName)

	ntup.SetMode(EXPORT_FROM_TARGET_MODE)
	assert.Equal(ntup.CurrentTableName, ntup.TargetTableName)

	ntup.SetMode(UNSPECIFIED_MODE)
	assert.Nil(ntup.CurrentTableName)
}

func TestNameTupleMatchesPattern(t *testing.T) {
	assert := assert.New(t)

	ntup := &NameTuple{
		SourceTableName: newTableName(ORACLE, "SAKILA", "SAKILA", "TABLE1"),
		TargetTableName: newTableName(YUGABYTEDB, "public", "sakila", "table1"),
	}
	ntup.SetMode(IMPORT_TO_TARGET_MODE)

	testCases := []struct {
		pattern string
		match   bool
	}{
		{"table1", false}, // effectively: <defaultSchema>.table1 i.e. public.table1
		{"table2", false},
		{"table", false},
		{"TABLE1", true},
		{"TABLE2", false},
		{"TABLE", false},
		{"TABLE*", true},
		{"table*", false},
		{"SAKILA.TABLE1", true},
		{"SAKILA.TABLE2", false},
		{"SAKILA.TABLE", false},
		{"SAKILA.TABLE*", true},
		{"SAKILA.table*", true}, // Schema name comparison is case insensitive. Matches with target name.
		{"sakila.table1", true},
		{"sakila.table2", false},
		{"sakila.table", false},
		{"sakila.table*", true},
	}

	for _, tc := range testCases {
		match, err := ntup.MatchesPattern(tc.pattern)
		assert.Nil(err)
		assert.Equal(tc.match, match, "pattern: %s, expected: %b, got: %b", tc.pattern, tc.match, match)
	}
}

//=====================================================

func TestNameRegistry(t *testing.T) {
	assert := assert.New(t)

	reg := &NameRegistry{
		SourceDBType:              POSTGRESQL,
		SourceDBSchemaNames:       []string{"public", "schema1", "schema2"},
		TargetDBSchemaNames:       []string{"public", "schema1", "schema2"},
		DefaultSourceDBSchemaName: "public",
		DefaultTargetDBSchemaName: "public",
		SourceDBTableNames: map[string][]string{
			"public":  {"table1", "table2"},
			"schema1": {"table1", "table2"},
			"schema2": {"table1", "table2"},
		},
		TargetDBTableNames: map[string][]string{
			"public":  {"table1", "table2"},
			"schema1": {"table1", "table2"},
			"schema2": {"table1", "table2"},
		},
	}

	tableName, err := reg.LookupTableName("table1")
	assert.Nil(err)
	assert.NotNil(tableName)
	expectedTableName := &NameTuple{
		SourceTableName: &TableName{
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
		},
	}
	expectedTableName.TargetTableName = expectedTableName.SourceTableName // Source and Target are the same for PG source.
	assert.Equal(expectedTableName, tableName)
}
