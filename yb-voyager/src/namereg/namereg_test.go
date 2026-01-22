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
package namereg

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

var oracleToYBNameRegistry = &NameRegistry{
	SourceDBType: constants.ORACLE,
	params: NameRegistryParams{
		Role: TARGET_DB_IMPORTER_ROLE,
	},
	SourceDBSchemaNames:       []string{"SAKILA"},
	YBSchemaNames:             []string{"public"},
	DefaultSourceDBSchemaName: "SAKILA",
	DefaultYBSchemaName:       "public",
	//DefaultSourceReplicaDBSchemaName: "SAKILA_FF", // Will be set using SetDefaultSourceReplicaDBSchemaName().
	SourceDBTableNames: map[string][]string{
		"SAKILA": {`TABLE1`, `TABLE2`, `Table2`, `MixedCaps`, `MixedCaps1`, `MixedCAPS1`, `lower_caps`, `TABLE3`},
	},
	YBTableNames: map[string][]string{
		"public": {"table1", "table2", `Table2`, `mixedcaps`, `MixedCaps1`, `MixedCAPS1`, "lower_caps", `table123`},
	},
}

func buildNameTuple(reg *NameRegistry, sourceSchema, sourceTable, targetSchema, targetTable string) sqlname.NameTuple {
	var sourceName *sqlname.ObjectName
	var targetName *sqlname.ObjectName
	if sourceSchema != "" && sourceTable != "" {
		sourceName = sqlname.NewObjectName(reg.SourceDBType, sourceSchema, sourceSchema, sourceTable)
	}
	if targetSchema != "" && targetTable != "" {
		targetName = sqlname.NewObjectName(constants.YUGABYTEDB, targetSchema, targetSchema, targetTable)
	}
	return NewNameTuple(reg.params.Role, sourceName, targetName)
}

func TestNameTuple(t *testing.T) {
	assert := assert.New(t)
	sourceName := sqlname.NewObjectName(constants.ORACLE, "SAKILA", "SAKILA", "TABLE1")
	targetName := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", "table1")

	ntup := NewNameTuple(TARGET_DB_IMPORTER_ROLE, sourceName, targetName)

	assert.Equal(ntup.CurrentName, ntup.TargetName)
	assert.Equal(ntup.ForUserQuery(), `"public"."table1"`)
	schemaName, tableName := ntup.ForCatalogQuery()
	assert.Equal(schemaName, `public`)
	assert.Equal(tableName, `table1`)

	ntup = NewNameTuple(SOURCE_REPLICA_DB_IMPORTER_ROLE, sourceName, targetName)

	assert.Equal(ntup.CurrentName, ntup.SourceName)
	assert.Equal(ntup.ForUserQuery(), `"SAKILA"."TABLE1"`)
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
	sourceName := sqlname.NewObjectName(constants.ORACLE, "SAKILA", "SAKILA", "TABLE1")
	targetName := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "sakila", "table1")
	ntup := NewNameTuple(TARGET_DB_IMPORTER_ROLE, sourceName, targetName)

	testCases := []struct {
		pattern string
		match   bool
	}{
		{"table1", true}, // effectively: <defaultSchema>.table1 i.e. public.table1
		{"table2", false},
		{"table", false},
		{"TABLE1", true},
		{"TABLE2", false},
		{"TABLE", false},
		{"TABLE*", true},
		{"table*", true},
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

func TestNameTupleMatchesPatternMySQL(t *testing.T) {
	assert := assert.New(t)
	sourceName := sqlname.NewObjectName(constants.MYSQL, "test", "test", "Table1")
	targetName := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "test", "table1")
	ntup := NewNameTuple(TARGET_DB_IMPORTER_ROLE, sourceName, targetName)
	testCases := []struct {
		pattern string
		match   bool
	}{
		{"table1", true},
		{"table2", false},
		{"table", false},
		{`"Table1"`, true},
		{"Table1", true},
		{"TABLE2", false},
		{"TABLE", false},
		{"TABLE*", true},
		{`"Table*"`, true},
		{"Table*", true},
		{"TEST.TABLE1", true},
		{"test.TABLE2", false},
		{"test.TABLE", false},
		{"test.TABLE*", true}, // Case-sensitive, so "SAKILA.TABLE*" does not match "test.Table1"
		{"test.table1", true}, // Case-sensitive, so "sakila.table1" does not match "test.Table1"
		{"test.table2", false},
		{"test.table", false},
		{"test.table*", true}, // Case-sensitive, so "sakila.table*" does not match "test.Table1"
	}

	for _, tc := range testCases {
		match, err := ntup.MatchesPattern(tc.pattern)
		assert.Nil(err)
		assert.Equal(tc.match, match, "pattern: %s, expected: %b, got: %b", tc.pattern, tc.match, match)
	}
}
func TestNameMatchesPattern(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	reg := &NameRegistry{
		SourceDBType: constants.ORACLE,
		params: NameRegistryParams{
			Role: SOURCE_DB_EXPORTER_ROLE,
		},
		SourceDBSchemaNames:       []string{"TEST_SCHEMA"},
		DefaultSourceDBSchemaName: "TEST_SCHEMA",
		SourceDBTableNames: map[string][]string{
			"TEST_SCHEMA": {
				"C", "C1", "C2", "Case_Sensitive_Columns", "EMPLOYEES", "FOO", "Mixed_Case_Table_Name_Test",
				"RESERVED_COLUMN", "SESSION_LOG", "SESSION_LOG1", "SESSION_LOG2", "SESSION_LOG3", "SESSION_LOG4",
				"TEST_TIMEZONE", "TRUNC_TEST", "check", "group",
			},
		},
	}
	// Prepare a list of all NamedTuples.
	ntups := make([]sqlname.NameTuple, 0)
	for _, tableName := range reg.SourceDBTableNames["TEST_SCHEMA"] {
		ntup, err := reg.LookupTableName(tableName)
		require.Nil(err)
		ntups = append(ntups, ntup)
	}
	// Write a table-driven test to test the MatchesPattern() method using following patterns:
	// session_log,session_log?,"group","check",test*,"*Case*",c*
	var testCases = []struct {
		pattern  string
		expected []string
	}{
		{"session_log", []string{"TEST_SCHEMA.SESSION_LOG"}},
		{"session_log?", []string{"TEST_SCHEMA.SESSION_LOG1", "TEST_SCHEMA.SESSION_LOG2", "TEST_SCHEMA.SESSION_LOG3", "TEST_SCHEMA.SESSION_LOG4"}},
		{"group", []string{"TEST_SCHEMA.group"}},
		{"check", []string{"TEST_SCHEMA.check"}},
		{"test*", []string{"TEST_SCHEMA.TEST_TIMEZONE"}},
		{`"*Case*"`, []string{"TEST_SCHEMA.Case_Sensitive_Columns", "TEST_SCHEMA.Mixed_Case_Table_Name_Test"}},
		{"c*", []string{"TEST_SCHEMA.C", "TEST_SCHEMA.C1", "TEST_SCHEMA.C2", "TEST_SCHEMA.Case_Sensitive_Columns", "TEST_SCHEMA.check"}},
	}
	for _, tc := range testCases {
		for _, ntup := range ntups {
			match, err := ntup.MatchesPattern(tc.pattern)
			require.Nil(err)
			tableName := ntup.AsQualifiedCatalogName()
			if match {
				assert.Contains(tc.expected, tableName, "pattern: %s, tableName: %s", tc.pattern, tableName)
			} else {
				assert.NotContains(tc.expected, tableName, "pattern: %s, tableName: %s", tc.pattern, tableName)
			}
		}
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
		expected   sqlname.NameTuple
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
	_, err := reg.LookupTableName("table3")
	require.NotNil(err)
	assert.ErrorAs(err, &errNameNotFound)
	assert.Equal(&ErrNameNotFound{ObjectType: "table", Name: "table3"}, errNameNotFound)

	// Missing schema name.
	_, err = reg.LookupTableName("schema1.table1")
	require.NotNil(err)
	assert.ErrorAs(err, &errNameNotFound)
	assert.Equal(&ErrNameNotFound{ObjectType: "schema", Name: "schema1"}, errNameNotFound)
	assert.Contains(err.Error(), "schema1.table1")

	// Missing schema and table name.
	_, err = reg.LookupTableName("schema1.table3")
	require.NotNil(err)
	assert.ErrorAs(err, &errNameNotFound)
	assert.Equal(&ErrNameNotFound{ObjectType: "schema", Name: "schema1"}, errNameNotFound)
	assert.Contains(err.Error(), "schema1.table3")

	// Multiple matches.
	_, err = reg.LookupTableName("mixedCaps1")
	require.NotNil(err)
	assert.ErrorAs(err, &errMultipleMatchingNames)
	assert.Equal(&ErrMultipleMatchingNames{ObjectType: "table", Names: []string{"MixedCaps1", "MixedCAPS1"}},
		errMultipleMatchingNames)

	// No default schema.
	reg.DefaultSourceDBSchemaName = ""
	_, err = reg.LookupTableName("table1")
	require.NotNil(err)

	assert.Contains(err.Error(), "either both or none of the default schema")
	reg.DefaultYBSchemaName = ""
	_, err = reg.LookupTableName("table1")
	require.NotNil(err)
	assert.Contains(err.Error(), "no default schema name")
	reg.DefaultSourceDBSchemaName = "SAKILA"
	reg.DefaultYBSchemaName = "public"

	//source table not exist in target
	_, err = reg.LookupTableName("SAKILA.TABLE3")
	require.NotNil(err)
	assert.Contains(err.Error(), "lookup target table name")

	var tuple sqlname.NameTuple
	sourceObj := sqlname.NewObjectNameWithQualifiedName(constants.ORACLE, "SAKILA", "\"SAKILA\".\"TABLE3\"")
	reg.params.Role = SOURCE_DB_EXPORTER_ROLE
	expectedTuple := sqlname.NameTuple{
		SourceName:  sourceObj,
		CurrentName: sourceObj,
	}
	tuple, err = reg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole("SAKILA.TABLE3")
	require.Nil(err)
	assert.Equal(expectedTuple, tuple, "tableName: SAKILA.TABLE3")

	//target table not exist in source
	_, err = reg.LookupTableName("public.table123")
	require.NotNil(err)
	assert.Contains(err.Error(), "lookup source table name")
	reg.params.Role = TARGET_DB_IMPORTER_ROLE
	targetObj := sqlname.NewObjectNameWithQualifiedName(constants.YUGABYTEDB, "public", "\"public\".\"table123\"")
	expectedTuple = sqlname.NameTuple{
		TargetName:  targetObj,
		CurrentName: targetObj,
	}
	tuple, err = reg.LookupTableNameAndIgnoreIfSourceNotFound("public.table123")
	require.Nil(err)
	assert.Equal(expectedTuple, tuple, "tableName: public.table123")

}

func TestDifferentSchemaInSameDBAsSourceReplica1(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	regCopy := *oracleToYBNameRegistry // Copy the registry.
	reg := &regCopy
	reg.params.Role = SOURCE_REPLICA_DB_IMPORTER_ROLE

	// Set the default source replica schema name.
	reg.setDefaultSourceReplicaDBSchemaName("SAKILA_FF")
	reg.DefaultSourceDBSchemaName = "SAKILA"

	table1 := buildNameTuple(reg, "SAKILA_FF", "TABLE1", "public", "table1")
	table2 := buildNameTuple(reg, "SAKILA_FF", "TABLE2", "public", "table2")
	mixedCaps := buildNameTuple(reg, "SAKILA_FF", "MixedCaps", "public", "mixedcaps")
	lowerCaps := buildNameTuple(reg, "SAKILA_FF", "lower_caps", "public", "lower_caps")

	var testCases = []struct {
		tableNames []string
		expected   sqlname.NameTuple
	}{
		{[]string{
			// YB side variants:
			`table1`, `"table1"`, `public.table1`, `public."table1"`, `public."TABLE1"`, `public.TABLE1`,
			// Oracle source-replica side variants:
			`TABLE1`, `"TABLE1"`, `SAKILA_FF.TABLE1`, `SAKILA_FF."TABLE1"`, `SAKILA_FF."table1"`, `SAKILA_FF.table1`,
			// oracle source side :
			`SAKILA.TABLE1`, `SAKILA."TABLE1"`, `SAKILA."table1"`, `SAKILA.table1`,
		}, table1},
		{[]string{"table2", "TABLE2"}, table2},
		{[]string{
			// YB side variants:
			"MixedCaps", `"MixedCaps"`, `public.MixedCaps`, `public."MixedCaps"`, `public."MIXEDCAPS"`, `public.MIXEDCAPS`,
			// Oracle source-replica side variants:
			"MIXEDCAPS", `"MIXEDCAPS"`, `SAKILA_FF.MIXEDCAPS`, `SAKILA_FF."MIXEDCAPS"`, `SAKILA_FF."mixedcaps"`, `SAKILA_FF.mixedcaps`,
			// oracle source side :
			`SAKILA.MIXEDCAPS`, `SAKILA."MIXEDCAPS"`, `SAKILA."mixedcaps"`, `SAKILA.mixedcaps`,
		}, mixedCaps},
		{[]string{
			// YB side variants:
			"lower_caps", `"lower_caps"`, `public.lower_caps`, `public."lower_caps"`, `public."LOWER_CAPS"`, `public.LOWER_CAPS`,
			// Oracle source-replica side variants:
			"LOWER_CAPS", `"LOWER_CAPS"`, `SAKILA_FF.LOWER_CAPS`, `SAKILA_FF."LOWER_CAPS"`, `SAKILA_FF."lower_caps"`, `SAKILA_FF.lower_caps`,
			// oracle source side:
			`SAKILA.LOWER_CAPS`, `SAKILA."LOWER_CAPS"`, `SAKILA."lower_caps"`, `SAKILA.lower_caps`,
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

	_, err = reg.LookupTableName("SAKILA_FF.table1")
	require.NotNil(err)
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
	dbType        string
	tableNames    map[string][]string // schemaName -> tableNames
	sequenceNames map[string][]string // schemaName -> sequenceNames
	schemaNames   []string
}

func (db *dummySourceDB) GetAllTableNamesRaw(schemaName string) ([]string, error) {
	tableNames, ok := db.tableNames[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %q not found", schemaName)
	}
	return tableNames, nil
}

func (db *dummySourceDB) GetAllSequencesRaw(schemaName string) ([]string, error) {
	sequenceNames, ok := db.sequenceNames[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %q not found", schemaName)
	}
	return sequenceNames, nil
}

func (db *dummySourceDB) GetAllSchemaNamesIdentifiers() ([]sqlname.Identifier, error) {
	return sqlname.ParseIdentifiersFromStrings(db.dbType, db.schemaNames), nil
}

type dummyTargetDB struct {
	tableNames    map[string][]string // schemaName -> tableNames
	sequenceNames map[string][]string // schemaName -> sequenceNames
}

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

func (db *dummyTargetDB) GetAllSequencesRaw(schemaName string) ([]string, error) {
	sequenceNames, ok := db.sequenceNames[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %q not found", schemaName)
	}
	return sequenceNames, nil
}

func TestNameRegistryWithDummyDBs(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create a dummy source DB.
	dummySdb := &dummySourceDB{
		tableNames: map[string][]string{
			"SAKILA": {"TABLE1", "TABLE2", "MixedCaps", "lower_caps"},
		},
		sequenceNames: map[string][]string{
			"SAKILA": {"SEQ1", "SEQ2"},
		},
		dbType:      constants.ORACLE,
		schemaNames: []string{"SAKILA"},
	}

	// Create a dummy target DB.
	dummyTdb := &dummyTargetDB{
		tableNames: map[string][]string{
			"ybsakila": {"table1", "table2", "mixedcaps", "lower_caps"},
		},
		sequenceNames: map[string][]string{
			"ybsakila": {"seq1", "seq2"},
		},
	}

	sourceTableNamesMap := make(map[string][]string)
	for k, v := range dummySdb.tableNames {
		sourceTableNamesMap[k] = append(sourceTableNamesMap[k], v...)
	}
	sourceSequenceNamesMap := make(map[string][]string)
	for k, v := range dummySdb.sequenceNames {
		sourceSequenceNamesMap[k] = append(sourceSequenceNamesMap[k], v...)
	}

	targetTableNamesMap := make(map[string][]string)
	for k, v := range dummyTdb.tableNames {
		targetTableNamesMap[k] = append(targetTableNamesMap[k], v...)
	}
	targetSequenceNamesMap := make(map[string][]string)
	for k, v := range dummyTdb.sequenceNames {
		targetSequenceNamesMap[k] = append(targetSequenceNamesMap[k], v...)
	}

	// Create a NameRegistry using the dummy DBs.
	currentMode := SOURCE_DB_EXPORTER_ROLE
	newNameRegistry := func(tSchema string) *NameRegistry {
		params := NameRegistryParams{
			FilePath:       "",
			Role:           currentMode,
			SourceDBType:   constants.ORACLE,
			SourceDBSchema: []string{"SAKILA"},
			SourceDBName:   "ORCLPDB1",
			TargetDBSchema: []string{"ybsakila"},
			SDB:            dummySdb,
			YBDB:           dummyTdb,
		}
		reg := NewNameRegistry(params)
		reg.params.FilePath = "dummy_name_registry.json"
		return reg
	}
	reg := newNameRegistry("")

	// Delete the dummy_name_registry.json file if it exists.
	_ = os.Remove(reg.params.FilePath)

	err := reg.Init()
	require.Nil(err)
	assert.Equal(constants.ORACLE, reg.SourceDBType)
	assert.Equal("SAKILA", reg.DefaultSourceDBSchemaName)
	assert.Equal(sourceTableNamesMap, reg.SourceDBTableNames)
	assert.Equal(sourceSequenceNamesMap, reg.SourceDBSequenceNames)
	table1 := buildNameTuple(reg, "SAKILA", "TABLE1", "", "")
	ntup, err := reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	seq1 := buildNameTuple(reg, "SAKILA", "SEQ1", "", "")
	stup, err := reg.LookupTableName("SEQ1")
	require.Nil(err)
	assert.Equal(seq1, stup)

	// When `export data` restarts, the registry should be reloaded from the file.
	reg = newNameRegistry("")
	err = reg.Init()
	require.Nil(err)
	assert.Equal(constants.ORACLE, reg.SourceDBType)
	assert.Equal("SAKILA", reg.DefaultSourceDBSchemaName)
	assert.Equal(sourceTableNamesMap, reg.SourceDBTableNames)
	assert.Equal(sourceSequenceNamesMap, reg.SourceDBSequenceNames)
	ntup, err = reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	assert.Equal(`"SAKILA"."TABLE1"`, table1.ForUserQuery())

	// Change the mode to IMPORT_TO_TARGET_MODE.
	currentMode = TARGET_DB_IMPORTER_ROLE
	reg = newNameRegistry("ybsakila")
	err = reg.Init()
	require.Nil(err)
	assert.Equal(reg.YBSchemaNames, []string{"ybsakila"})
	assert.Equal(reg.DefaultYBSchemaName, "ybsakila")
	assert.Equal(targetTableNamesMap, reg.YBTableNames)
	assert.Equal(targetSequenceNamesMap, reg.YBSequenceNames)
	table1 = buildNameTuple(reg, "SAKILA", "TABLE1", "ybsakila", "table1")
	ntup, err = reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	ntup, err = reg.LookupTableName("ybsakila.table1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	assert.Equal(`"ybsakila"."table1"`, table1.ForUserQuery())

	// When `import data` restarts, the registry should be reloaded from the file.
	reg = newNameRegistry("ybsakila")
	err = reg.Init()
	require.Nil(err)
	assert.Equal(reg.YBSchemaNames, []string{"ybsakila"})
	assert.Equal(reg.DefaultYBSchemaName, "ybsakila")
	assert.Equal(targetTableNamesMap, reg.YBTableNames)

	// Change the mode to IMPORT_TO_SOURCE_REPLICA_MODE.
	currentMode = SOURCE_REPLICA_DB_IMPORTER_ROLE
	reg = newNameRegistry("SAKILA_FF")
	err = reg.Init()
	require.Nil(err)
	assert.Equal(reg.DefaultSourceReplicaDBSchemaName, "SAKILA_FF")
	assert.Equal(reg.SourceDBTableNames["SAKILA_FF"], sourceTableNamesMap["SAKILA"])
	table1 = buildNameTuple(reg, "SAKILA_FF", "TABLE1", "ybsakila", "table1")
	ntup, err = reg.LookupTableName("TABLE1")
	require.Nil(err)
	assert.Equal(table1, ntup)
	assert.Equal(`"SAKILA_FF"."TABLE1"`, table1.ForUserQuery())
}

// Unit tests for breaking changes in NameRegistry.

func TestNameRegistryStructs(t *testing.T) {

	tests := []struct {
		name         string
		actualType   reflect.Type
		expectedType interface{}
	}{
		{
			name:       "Validate NameRegistryParams Struct Definition",
			actualType: reflect.TypeOf(NameRegistryParams{}),
			expectedType: struct {
				FilePath       string
				Role           string
				SourceDBType   string
				SourceDBSchema string
				SourceDBName   string
				SDB            SourceDBInterface
				TargetDBSchema string
				YBDB           YBDBInterface
			}{},
		},
		{
			name:       "Validate NameRegistry Struct Definition",
			actualType: reflect.TypeOf(NameRegistry{}),
			expectedType: struct {
				SourceDBType                     string
				SourceDBSchemaNames              []string
				SourceDBSchemaIdentifiers        []sqlname.Identifier `json:"-"`
				DefaultSourceDBSchemaName        string
				SourceDBTableNames               map[string][]string
				YBSchemaNames                    []string
				DefaultYBSchemaName              string
				YBTableNames                     map[string][]string
				DefaultSourceReplicaDBSchemaName string
				params                           NameRegistryParams
				SourceDBSequenceNames            map[string][]string
				YBSequenceNames                  map[string][]string
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CompareStructs(t, tt.actualType, reflect.TypeOf(tt.expectedType), tt.name)
		})
	}
}

func TestNameRegistryJson(t *testing.T) {
	exportDir := filepath.Join(os.TempDir(), "namereg")
	outputFilePath := filepath.Join(exportDir, "test_dummy_name_registry.json")

	// Create a sample NameRegistry instance
	reg := &NameRegistry{
		SourceDBType: constants.ORACLE,
		params: NameRegistryParams{
			FilePath:       outputFilePath,
			Role:           TARGET_DB_IMPORTER_ROLE,
			SourceDBType:   constants.ORACLE,
			SourceDBSchema: []string{"SAKILA"},
			SourceDBName:   "ORCLPDB1",
			TargetDBSchema: []string{"ybsakila"},
		},
		SourceDBSchemaNames:       []string{"SAKILA"},
		DefaultSourceDBSchemaName: "SAKILA",
		SourceDBTableNames: map[string][]string{
			"SAKILA": {"TABLE1", "TABLE2", "MixedCaps", "lower_caps"},
		},
		YBSchemaNames:       []string{"ybsakila"},
		DefaultYBSchemaName: "ybsakila",
		YBTableNames: map[string][]string{
			"ybsakila": {"table1", "table2", "mixedcaps", "lower_caps"},
		},
		DefaultSourceReplicaDBSchemaName: "SAKILA_FF",
	}

	// Ensure the export directory exists
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		t.Fatalf("Failed to create export directory: %v", err)
	}

	// Clean up the export directory
	defer func() {
		if err := os.RemoveAll(exportDir); err != nil {
			t.Fatalf("Failed to remove export directory: %v", err)
		}
	}()

	// Marshal the NameRegistry instance to JSON
	err := reg.save()
	if err != nil {
		t.Fatalf("Failed to save NameRegistry to JSON: %v", err)
	}

	// TODO: Use a single string instead of a slice of strings for the expected JSON
	expectedJSON := strings.Join([]string{
		"{",
		`  "SourceDBType": "oracle",`,
		`  "SourceDBSchemaNames": [`,
		`    "SAKILA"`,
		"  ],",
		`  "DefaultSourceDBSchemaName": "SAKILA",`,
		`  "SourceDBTableNames": {`,
		`    "SAKILA": [`,
		`      "TABLE1",`,
		`      "TABLE2",`,
		`      "MixedCaps",`,
		`      "lower_caps"`,
		"    ]",
		"  },",
		`  "YBSchemaNames": [`,
		`    "ybsakila"`,
		"  ],",
		`  "DefaultYBSchemaName": "ybsakila",`,
		`  "YBTableNames": {`,
		`    "ybsakila": [`,
		`      "table1",`,
		`      "table2",`,
		`      "mixedcaps",`,
		`      "lower_caps"`,
		"    ]",
		"  },",
		`  "DefaultSourceReplicaDBSchemaName": "SAKILA_FF",`,
		`  "SourceDBSequenceNames": null,`,
		`  "YBSequenceNames": null`,
		"}",
	}, "\n")

	// Read the JSON file and compare it with the expected JSON
	testutils.CompareJson(t, outputFilePath, expectedJSON, exportDir)
}
