package namereg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		SourceName: newTableName(ORACLE, "SAKILA", "SAKILA", "TABLE1"),
		TargetName: newTableName(YUGABYTEDB, "public", "public", "table1"),
	}
	ntup.SetMode(IMPORT_TO_TARGET_MODE)
	assert.Equal(ntup.CurrentName, ntup.TargetName)
	assert.Equal(ntup.ForUserQuery(), `public."table1"`)
	schemaName, tableName := ntup.ForCatalogQuery()
	assert.Equal(schemaName, `public`)
	assert.Equal(tableName, `table1`)

	ntup.SetMode(IMPORT_TO_SOURCE_REPLICA_MODE)
	assert.Equal(ntup.CurrentName, ntup.SourceName)
	assert.Equal(ntup.ForUserQuery(), `SAKILA."TABLE1"`)
	schemaName, tableName = ntup.ForCatalogQuery()
	assert.Equal(schemaName, `SAKILA`)
	assert.Equal(tableName, `TABLE1`)

	ntup.SetMode(EXPORT_FROM_SOURCE_MODE)
	assert.Equal(ntup.CurrentName, ntup.SourceName)

	ntup.SetMode(EXPORT_FROM_TARGET_MODE)
	assert.Equal(ntup.CurrentName, ntup.TargetName)

	ntup.SetMode(UNSPECIFIED_MODE)
	assert.Nil(ntup.CurrentName)
}

func TestNameTupleMatchesPattern(t *testing.T) {
	assert := assert.New(t)

	ntup := &NameTuple{
		SourceName: newTableName(ORACLE, "SAKILA", "SAKILA", "TABLE1"),
		TargetName: newTableName(YUGABYTEDB, "public", "sakila", "table1"),
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

var oracleToYBNameRegistry = &NameRegistry{
	SourceDBType:              ORACLE,
	Mode:                      IMPORT_TO_TARGET_MODE,
	SourceDBSchemaNames:       []string{"SAKILA"},
	YBSchemaNames:             []string{"public"},
	DefaultSourceDBSchemaName: "SAKILA",
	DefaultYBSchemaName:       "public",
	SourceDBTableNames: map[string][]string{
		"SAKILA": {`TABLE1`, `TABLE2`, `Table2`, `MixedCaps`, `lower_caps`},
	},
	YBTableNames: map[string][]string{
		"public": {"table1", "table2", `Table2`, "mixedcaps", "lower_caps"},
	},
}

func buildNameTuple(reg *NameRegistry, sourceSchema, sourceTable, targetSchema, targetTable string) *NameTuple {
	ntup := &NameTuple{
		SourceName: newTableName(reg.SourceDBType, sourceSchema, sourceSchema, sourceTable),
		TargetName: newTableName(YUGABYTEDB, targetSchema, targetSchema, targetTable),
	}
	ntup.SetMode(reg.Mode)
	return ntup
}

func TestNameRegistrySuccessfulLookup(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	reg := oracleToYBNameRegistry
	table1 := buildNameTuple(reg, "SAKILA", "TABLE1", "public", "table1")
	table2 := buildNameTuple(reg, "SAKILA", "TABLE2", "public", "table2")
	mixedCaps := buildNameTuple(reg, "SAKILA", "MixedCaps", "public", "mixedcaps")
	lowerCaps := buildNameTuple(reg, "SAKILA", "lower_caps", "public", "lower_caps")

	var testCases = []struct {
		tableNames []string
		expected   *NameTuple
	}{
		{[]string{
			// YB side variants:
			`table1`, `"table1"`, `public.table1`, `public."table1"`, `public."TABLE1"`, `public.TABLE1`,
			// Oracle side variants:
			`TABLE1`, `"TABLE1"`, `SAKILA.TABLE1`, `SAKILA."TABLE1"`, `SAKILA."table1"`, `SAKILA.table1`,
		}, table1},
		{[]string{"table2", "TABLE2"}, table2},
		{[]string{
			// YB side variants:
			"MixedCaps", `"MixedCaps"`, `public.MixedCaps`, `public."MixedCaps"`, `public."MIXEDCAPS"`, `public.MIXEDCAPS`,
			// Oracle side variants:
			"MIXEDCAPS", `"MIXEDCAPS"`, `SAKILA.MIXEDCAPS`, `SAKILA."MIXEDCAPS"`, `SAKILA."mixedcaps"`, `SAKILA.mixedcaps`,
		}, mixedCaps},
		{[]string{
			// YB side variants:
			"lower_caps", `"lower_caps"`, `public.lower_caps`, `public."lower_caps"`, `public."LOWER_CAPS"`, `public.LOWER_CAPS`,
			// Oracle side variants:
			"LOWER_CAPS", `"LOWER_CAPS"`, `SAKILA.LOWER_CAPS`, `SAKILA."LOWER_CAPS"`, `SAKILA."lower_caps"`, `SAKILA.lower_caps`,
		}, lowerCaps},
	}

	for _, tc := range testCases {
		for _, tableName := range tc.tableNames {
			ntup, err := reg.LookupTableName(tableName)
			require.Nil(err)
			assert.Equal(tc.expected, ntup, "tableName: %s", tableName)
		}
	}
}
