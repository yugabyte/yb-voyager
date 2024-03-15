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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var oracleToYBNameRegistry = &NameRegistry{
	SourceDBType: ORACLE,
	params: NameRegistryParams{
		Role: TARGET_DB_IMPORTER_ROLE,
	},
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

func buildNameTuple(reg *NameRegistry, sourceSchema, sourceTable, targetSchema, targetTable string) *sqlname.NameTuple {
	var sourceName *sqlname.ObjectName
	var targetName *sqlname.ObjectName
	if sourceSchema != "" && sourceTable != "" {
		sourceName = sqlname.NewObjectName(reg.SourceDBType, sourceSchema, sourceSchema, sourceTable)
	}
	if targetSchema != "" && targetTable != "" {
		targetName = sqlname.NewObjectName(YUGABYTEDB, targetSchema, targetSchema, targetTable)
	}
	return NewNameTuple(reg.params.Role, sourceName, targetName)
}

func TestNameTuple(t *testing.T) {
	assert := assert.New(t)
	sourceName := sqlname.NewObjectName(ORACLE, "SAKILA", "SAKILA", "TABLE1")
	targetName := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "table1")

	ntup := NewNameTuple(TARGET_DB_IMPORTER_ROLE, sourceName, targetName)

	assert.Equal(ntup.CurrentName, ntup.TargetName)
	assert.Equal(ntup.ForUserQuery(), `public."table1"`)
	schemaName, tableName := ntup.ForCatalogQuery()
	assert.Equal(schemaName, `public`)
	assert.Equal(tableName, `table1`)

	ntup = NewNameTuple(SOURCE_REPLICA_DB_IMPORTER_ROLE, sourceName, targetName)

	assert.Equal(ntup.CurrentName, ntup.SourceName)
	assert.Equal(ntup.ForUserQuery(), `SAKILA."TABLE1"`)
	schemaName, tableName = ntup.ForCatalogQuery()
	assert.Equal(schemaName, `SAKILA`)
	assert.Equal(tableName, `TABLE1`)

	ntup = NewNameTuple(SOURCE_DB_EXPORTER_ROLE, sourceName, targetName)
	assert.Equal(ntup.CurrentName, ntup.SourceName)

	ntup = NewNameTuple(TARGET_DB_EXPORTER_FF_ROLE, sourceName, targetName)
	assert.Equal(ntup.CurrentName, ntup.TargetName)
}

func TestNameTupleMatchesPattern(t *testing.T) {
	assert := assert.New(t)
	sourceName := sqlname.NewObjectName(ORACLE, "SAKILA", "SAKILA", "TABLE1")
	targetName := sqlname.NewObjectName(YUGABYTEDB, "public", "sakila", "table1")
	ntup := NewNameTuple(TARGET_DB_IMPORTER_ROLE, sourceName, targetName)

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
		expected   *sqlname.NameTuple
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
	reg.params.Role = SOURCE_REPLICA_DB_IMPORTER_ROLE

	// Set the default source replica schema name.
	reg.setDefaultSourceReplicaDBSchemaName("SAKILA_FF")

	table1 := buildNameTuple(reg, "SAKILA_FF", "TABLE1", "public", "table1")
	table2 := buildNameTuple(reg, "SAKILA_FF", "TABLE2", "public", "table2")
	mixedCaps := buildNameTuple(reg, "SAKILA_FF", "MixedCaps", "public", "mixedcaps")
	lowerCaps := buildNameTuple(reg, "SAKILA_FF", "lower_caps", "public", "lower_caps")

	var testCases = []struct {
		tableNames []string
		expected   *sqlname.NameTuple
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

	reg.params.Role = SOURCE_REPLICA_DB_IMPORTER_ROLE
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
		reg := NewNameRegistry("", currentMode, sconf.DBType, sconf.Schema, sconf.DBName, tconf.Schema, dummySdb, dummyTdb)
		reg.params.FilePath = "dummy_name_registry.json"
		return reg
	}
	reg := newNameRegistry()

	// Delete the dummy_name_registry.json file if it exists.
	_ = os.Remove(reg.params.FilePath)

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
