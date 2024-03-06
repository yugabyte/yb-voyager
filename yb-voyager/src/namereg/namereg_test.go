package namereg

import (
	"fmt"
	"os"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

func TestPGDefaultSchemaCaseInsensitiveTableName(t *testing.T) {
	assert := assert.New(t)

	// Test NewTableName() with PostgreSQL and default schema "public" and
	// a table name belonging to default schema.
	tableName := newObjectName(POSTGRESQL, "public", "public", "table1")
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
	tableName := newObjectName(POSTGRESQL, "public", "schema1", "table1")
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
	tableName := newObjectName(POSTGRESQL, "public", "public", "Table1")
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
	tableName := newObjectName(POSTGRESQL, "public", "schema1", "Table1")
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
	tableName := newObjectName(ORACLE, "SAKILA", "SAKILA", "TABLE1")
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
	tableName := newObjectName(ORACLE, "SAKILA", "SCHEMA1", "TABLE1")
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
	tableName := newObjectName(ORACLE, "SAKILA", "SAKILA", "Table1")
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
	tableName := newObjectName(ORACLE, "SAKILA", "SCHEMA1", "Table1")
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
	tableName := newObjectName(MYSQL, "sakila", "sakila", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
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
	tableName := newObjectName(MYSQL, "sakila", "schema1", "table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
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
	tableName := newObjectName(MYSQL, "sakila", "sakila", "Table1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
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
	tableName := newObjectName(MYSQL, "sakila", "schema1", "TABLE1")
	assert.NotNil(tableName)
	expectedTableName := &ObjectName{
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
		SourceName: newObjectName(ORACLE, "SAKILA", "SAKILA", "TABLE1"),
		TargetName: newObjectName(YUGABYTEDB, "public", "public", "table1"),
	}
	ntup.SetMode(TARGET_DB_IMPORTER_ROLE)
	assert.Equal(ntup.CurrentName, ntup.TargetName)
	assert.Equal(ntup.ForUserQuery(), `public."table1"`)
	schemaName, tableName := ntup.ForCatalogQuery()
	assert.Equal(schemaName, `public`)
	assert.Equal(tableName, `table1`)

	ntup.SetMode(SOURCE_REPLICA_DB_IMPORTER_ROLE)
	assert.Equal(ntup.CurrentName, ntup.SourceName)
	assert.Equal(ntup.ForUserQuery(), `SAKILA."TABLE1"`)
	schemaName, tableName = ntup.ForCatalogQuery()
	assert.Equal(schemaName, `SAKILA`)
	assert.Equal(tableName, `TABLE1`)

	ntup.SetMode(SOURCE_DB_EXPORTER_ROLE)
	assert.Equal(ntup.CurrentName, ntup.SourceName)

	ntup.SetMode(TARGET_DB_EXPORTER_FF_ROLE)
	assert.Equal(ntup.CurrentName, ntup.TargetName)
}

func TestNameTupleMatchesPattern(t *testing.T) {
	assert := assert.New(t)

	ntup := &NameTuple{
		SourceName: newObjectName(ORACLE, "SAKILA", "SAKILA", "TABLE1"),
		TargetName: newObjectName(YUGABYTEDB, "public", "sakila", "table1"),
	}
	ntup.SetMode(TARGET_DB_IMPORTER_ROLE)

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
	role:                      TARGET_DB_IMPORTER_ROLE,
	SourceDBSchemaNames:       []string{"SAKILA"},
	YBSchemaNames:             []string{"public"},
	DefaultSourceDBSchemaName: "SAKILA",
	DefaultYBSchemaName:       "public",
	//DefaultSourceReplicaDBSchemaName: "SAKILA_FF", // Will be set using SetDefaultSourceReplicaDBSchemaName().
	SourceDBTableNames: map[string][]string{
		"SAKILA": {`TABLE1`, `TABLE2`, `Table2`, `MixedCaps`, `MixedCaps1`, `MixedCAPS1`, `lower_caps`},
	},
	YBTableNames: map[string][]string{
		"public": {"table1", "table2", `Table2`, `mixedcaps`, `MixedCaps1`, `MixedCAPS1`, "lower_caps"},
	},
}

func buildNameTuple(reg *NameRegistry, sourceSchema, sourceTable, targetSchema, targetTable string) *NameTuple {
	ntup := &NameTuple{}

	if sourceSchema != "" && sourceTable != "" {
		ntup.SourceName = newObjectName(reg.SourceDBType, sourceSchema, sourceSchema, sourceTable)
	}
	if targetSchema != "" && targetTable != "" {
		ntup.TargetName = newObjectName(YUGABYTEDB, targetSchema, targetSchema, targetTable)
	}
	ntup.SetMode(reg.role)
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

func TestNameRegistryFailedLookup(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	errMultipleMatchingNames := &ErrMultipleMatchingNames{}
	errNameNotFound := &ErrNameNotFound{}

	// Missing table name.
	reg := oracleToYBNameRegistry
	ntup, err := reg.LookupTableName("table3")
	require.NotNil(err)
	assert.Nil(ntup)
	assert.ErrorAs(err, &errNameNotFound)
	assert.Equal(&ErrNameNotFound{ObjectType: "table", Name: "table3"}, errNameNotFound)

	// Missing schema name.
	ntup, err = reg.LookupTableName("schema1.table1")
	require.NotNil(err)
	assert.Nil(ntup)
	assert.ErrorAs(err, &errNameNotFound)
	assert.Equal(&ErrNameNotFound{ObjectType: "schema", Name: "schema1"}, errNameNotFound)
	assert.Contains(err.Error(), "schema1.table1")

	// Missing schema and table name.
	ntup, err = reg.LookupTableName("schema1.table3")
	require.NotNil(err)
	assert.Nil(ntup)
	assert.ErrorAs(err, &errNameNotFound)
	assert.Equal(&ErrNameNotFound{ObjectType: "schema", Name: "schema1"}, errNameNotFound)
	assert.Contains(err.Error(), "schema1.table3")

	// Multiple matches.
	ntup, err = reg.LookupTableName("mixedCaps1")
	require.NotNil(err)
	assert.Nil(ntup)
	assert.ErrorAs(err, &errMultipleMatchingNames)
	assert.Equal(&ErrMultipleMatchingNames{ObjectType: "table", Names: []string{"MixedCaps1", "MixedCAPS1"}},
		errMultipleMatchingNames)

	// No default schema.
	reg.DefaultSourceDBSchemaName = ""
	ntup, err = reg.LookupTableName("table1")
	require.NotNil(err)

	assert.Nil(ntup)
	assert.Contains(err.Error(), "either both or none of the default schema")
	reg.DefaultYBSchemaName = ""
	ntup, err = reg.LookupTableName("table1")
	require.NotNil(err)
	assert.Nil(ntup)
	assert.Contains(err.Error(), "no default schema name")
	reg.DefaultSourceDBSchemaName = "SAKILA"
	reg.DefaultYBSchemaName = "public"
}

func TestDifferentSchemaInSameDBAsSourceReplica1(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	regCopy := *oracleToYBNameRegistry // Copy the registry.
	reg := &regCopy
	reg.role = SOURCE_REPLICA_DB_IMPORTER_ROLE

	// Set the default source replica schema name.
	reg.setDefaultSourceReplicaDBSchemaName("SAKILA_FF")

	table1 := buildNameTuple(reg, "SAKILA_FF", "TABLE1", "public", "table1")
	table2 := buildNameTuple(reg, "SAKILA_FF", "TABLE2", "public", "table2")
	mixedCaps := buildNameTuple(reg, "SAKILA_FF", "MixedCaps", "public", "mixedcaps")
	lowerCaps := buildNameTuple(reg, "SAKILA_FF", "lower_caps", "public", "lower_caps")

	var testCases = []struct {
		tableNames []string
		expected   *NameTuple
	}{
		{[]string{
			// YB side variants:
			`table1`, `"table1"`, `public.table1`, `public."table1"`, `public."TABLE1"`, `public.TABLE1`,
			// Oracle side variants:
			`TABLE1`, `"TABLE1"`, `SAKILA_FF.TABLE1`, `SAKILA_FF."TABLE1"`, `SAKILA_FF."table1"`, `SAKILA_FF.table1`,
		}, table1},
		{[]string{"table2", "TABLE2"}, table2},
		{[]string{
			// YB side variants:
			"MixedCaps", `"MixedCaps"`, `public.MixedCaps`, `public."MixedCaps"`, `public."MIXEDCAPS"`, `public.MIXEDCAPS`,
			// Oracle side variants:
			"MIXEDCAPS", `"MIXEDCAPS"`, `SAKILA_FF.MIXEDCAPS`, `SAKILA_FF."MIXEDCAPS"`, `SAKILA_FF."mixedcaps"`, `SAKILA_FF.mixedcaps`,
		}, mixedCaps},
		{[]string{
			// YB side variants:
			"lower_caps", `"lower_caps"`, `public.lower_caps`, `public."lower_caps"`, `public."LOWER_CAPS"`, `public.LOWER_CAPS`,
			// Oracle side variants:
			"LOWER_CAPS", `"LOWER_CAPS"`, `SAKILA_FF.LOWER_CAPS`, `SAKILA_FF."LOWER_CAPS"`, `SAKILA_FF."lower_caps"`, `SAKILA_FF.lower_caps`,
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

func TestDifferentSchemaInSameDBAsSourceReplica2(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	regCopy := *oracleToYBNameRegistry // Copy the registry.
	reg := &regCopy

	table1 := buildNameTuple(reg, "SAKILA", "TABLE1", "public", "table1")

	ntup, err := reg.LookupTableName("table1")
	require.Nil(err)
	assert.Equal(table1, ntup)

	ntup, err = reg.LookupTableName("SAKILA_FF.table1")
	require.NotNil(err)
	assert.Nil(ntup)
	errNameNotFound := &ErrNameNotFound{}
	assert.ErrorAs(err, &errNameNotFound)
	assert.Equal(&ErrNameNotFound{ObjectType: "schema", Name: "SAKILA_FF"}, errNameNotFound)

	reg.role = SOURCE_REPLICA_DB_IMPORTER_ROLE
	table1FF := buildNameTuple(reg, "SAKILA_FF", "TABLE1", "public", "table1")
	reg.setDefaultSourceReplicaDBSchemaName("SAKILA_FF")
	ntup, err = reg.LookupTableName("table1")
	require.Nil(err)
	assert.Equal(table1FF, ntup)
}

// TODO: Add similar tests for PG.
// TODO: Add similar tests for MySQL.

//=====================================================

type dummySourceDB struct {
	*srcdb.Oracle                     // Just to satisfy the interface.
	tableNames    map[string][]string // schemaName -> tableNames
}

var _ srcdb.SourceDB = &dummySourceDB{}

func (db *dummySourceDB) GetAllTableNamesRaw(schemaName string) ([]string, error) {
	tableNames, ok := db.tableNames[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %q not found", schemaName)
	}
	return tableNames, nil
}

type dummyTargetDB struct {
	*tgtdb.TargetYugabyteDB                     // Just to satisfy the interface.
	tableNames              map[string][]string // schemaName -> tableNames
}

var _ tgtdb.TargetDB = &dummyTargetDB{}

func (db *dummyTargetDB) GetAllSchemaNamesRaw() ([]string, error) {
	return lo.Keys(db.tableNames), nil
}

func (db *dummyTargetDB) GetAllTableNamesRaw(schemaName string) ([]string, error) {
	tableNames, ok := db.tableNames[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %q not found", schemaName)
	}
	return tableNames, nil
}

func TestNameRegistryWithDummyDBs(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sconf := &srcdb.Source{
		DBType: ORACLE,
		Schema: "SAKILA",
	}
	tconf := &tgtdb.TargetConf{
		Schema: "ybsakila",
	}
	// Create a dummy source DB.
	dummySdb := &dummySourceDB{
		Oracle: &srcdb.Oracle{},
		tableNames: map[string][]string{
			"SAKILA": {"TABLE1", "TABLE2", "MixedCaps", "lower_caps"},
		},
	}

	// Create a dummy target DB.
	dummyTdb := &dummyTargetDB{
		TargetYugabyteDB: &tgtdb.TargetYugabyteDB{},
		tableNames: map[string][]string{
			"ybsakila": {"table1", "table2", "mixedcaps", "lower_caps"},
		},
	}

	// Create a NameRegistry using the dummy DBs.
	currentMode := SOURCE_DB_EXPORTER_ROLE
	newNameRegistry := func() *NameRegistry {
		reg := NewNameRegistry("", currentMode, sconf, dummySdb, tconf, dummyTdb)
		reg.filePath = "dummy_name_registry.json"
		return reg
	}
	reg := newNameRegistry()

	// Delete the dummy_name_registry.json file if it exists.
	_ = os.Remove(reg.filePath)

	err := reg.Init()
	require.Nil(err)
	assert.Equal(sconf.DBType, reg.SourceDBType)
	assert.Equal("SAKILA", reg.DefaultSourceDBSchemaName)
	assert.Equal(dummySdb.tableNames, reg.SourceDBTableNames)
	table1 := buildNameTuple(reg, "SAKILA", "TABLE1", "", "")
	ntup, err := reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)

	// When `export data` restarts, the registry should be reloaded from the file.
	reg = newNameRegistry()
	err = reg.Init()
	require.Nil(err)
	assert.Equal(sconf.DBType, reg.SourceDBType)
	assert.Equal("SAKILA", reg.DefaultSourceDBSchemaName)
	assert.Equal(dummySdb.tableNames, reg.SourceDBTableNames)
	ntup, err = reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	assert.Equal(`SAKILA."TABLE1"`, table1.ForUserQuery())

	// Change the mode to IMPORT_TO_TARGET_MODE.
	currentMode = TARGET_DB_IMPORTER_ROLE
	reg = newNameRegistry()
	err = reg.Init()
	require.Nil(err)
	assert.Equal(reg.YBSchemaNames, []string{"ybsakila"})
	assert.Equal(reg.DefaultYBSchemaName, "ybsakila")
	assert.Equal(dummyTdb.tableNames, reg.YBTableNames)
	table1 = buildNameTuple(reg, "SAKILA", "TABLE1", "ybsakila", "table1")
	ntup, err = reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	ntup, err = reg.LookupTableName("ybsakila.table1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	assert.Equal(`ybsakila."table1"`, table1.ForUserQuery())

	// When `import data` restarts, the registry should be reloaded from the file.
	reg = newNameRegistry()
	err = reg.Init()
	require.Nil(err)
	assert.Equal(reg.YBSchemaNames, []string{"ybsakila"})
	assert.Equal(reg.DefaultYBSchemaName, "ybsakila")
	assert.Equal(dummyTdb.tableNames, reg.YBTableNames)

	// Change the mode to IMPORT_TO_SOURCE_REPLICA_MODE.
	currentMode = SOURCE_REPLICA_DB_IMPORTER_ROLE
	tconf.Schema = "SAKILA_FF"
	reg = newNameRegistry()
	err = reg.Init()
	require.Nil(err)
	assert.Equal(reg.DefaultSourceReplicaDBSchemaName, "SAKILA_FF")
	assert.Equal(reg.SourceDBTableNames["SAKILA_FF"], dummySdb.tableNames["SAKILA"])
	table1 = buildNameTuple(reg, "SAKILA_FF", "TABLE1", "ybsakila", "table1")
	ntup, err = reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	assert.Equal(`SAKILA_FF."TABLE1"`, table1.ForUserQuery())
}
