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
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"gotest.tools/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

type TestDB struct {
	testcontainers.TestContainer
	*srcdb.Source
}

var (
	testPostgresSource *TestDB
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}
	host, port, err := postgresContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testPostgresSource = &TestDB{
		TestContainer: postgresContainer,
		Source: &srcdb.Source{
			DBType:    "postgresql",
			DBVersion: postgresContainer.GetConfig().DBVersion,
			User:      postgresContainer.GetConfig().User,
			Password:  postgresContainer.GetConfig().Password,
			Schema:    postgresContainer.GetConfig().Schema,
			DBName:    postgresContainer.GetConfig().DBName,
			Host:      host,
			Port:      port,
			SSLMode:   "disable",
		},
	}
	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)
	exitCode := m.Run()

	testPostgresSource.Terminate(ctx)

	os.Exit(exitCode)
}
func setupPostgreDBAndExportDependencies(t *testing.T, sqls []string, schemas string) string {

	testExportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	sqlname.SourceDBType = testPostgresSource.DBType

	CreateMigrationProjectIfNotExists(constants.POSTGRESQL, testExportDir)
	testPostgresSource.Schema = schemas
	testPostgresSource.ExecuteSqls(sqls...)
	return testExportDir
}

func getNameTuple(s string) sqlname.NameTuple {
	defaultSchema, _ := getDefaultSourceSchemaName()
	return sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
	}
}

func assertTableListFilteringInTheFirstRun(t *testing.T, expectedPartitionsToRootMap map[string]string, expectedTableList []sqlname.NameTuple) {
	partitionsToRootMap, tableList, err := fetchTablesNamesFromSourceAndFilterTableList()
	if err != nil {
		t.Errorf("error fetching the table list from DB: %v", err)
	}
	for expectedLeaf, expectedRoot := range expectedPartitionsToRootMap {
		actualRoot, ok := partitionsToRootMap[expectedLeaf]
		assert.Equal(t, ok, true)
		assert.Equal(t, actualRoot, expectedRoot)
	}
	assert.Equal(t, len(tableList), len(expectedTableList))
	assert.Equal(t, len(lo.Keys(partitionsToRootMap)), len(lo.Keys(expectedPartitionsToRootMap)))
	diff := sqlname.SetDifferenceNameTuples(expectedTableList, tableList)
	assert.Equal(t, len(diff), 0)
}

func assertInitialTableListOnSubsequentRun(t *testing.T, withStartClean bool, expectedTableList []sqlname.NameTuple, expectedPartitionsToRootMap map[string]string) {
	startClean = utils.BoolStr(withStartClean)
	retrievedPartitionsToRootTableMap, retrievedTableListFromFirstRun, err := getInitialTableList()
	if err != nil {
		t.Fatalf("retrieving first run table list from msr: %v", err)
	}
	for expectedLeaf, expectedRoot := range expectedPartitionsToRootMap {
		actualRoot, ok := retrievedPartitionsToRootTableMap[expectedLeaf]
		assert.Equal(t, ok, true)
		assert.Equal(t, actualRoot, expectedRoot)
	}

	assert.Equal(t, len(expectedTableList), len(retrievedTableListFromFirstRun))
	diff := sqlname.SetDifferenceNameTuples(expectedTableList, retrievedTableListFromFirstRun)
	assert.Equal(t, len(diff), 0)

	assert.Equal(t, len(lo.Keys(expectedPartitionsToRootMap)), len(lo.Keys(retrievedPartitionsToRootTableMap)))
}

func assertDetectAndReportNewLeafAddedOnSource(t *testing.T, rootTables []sqlname.NameTuple, expectedRootToNewLeafTablesMap map[string][]string) {
	nameRegistryList, err := getRegisteredNameRegList()
	if err != nil {
		t.Fatalf("error getting registered list: %v", err)
	}

	rootToNewLeafTablesMap, err := detectAndReportNewLeafPartitionsOnPartitionedTables(rootTables, nameRegistryList)
	if err != nil {
		t.Fatalf("error detecting new leafs on root tables: %v", err)
	}
	assert.Equal(t, len(lo.Keys(rootToNewLeafTablesMap)), len(lo.Keys(expectedRootToNewLeafTablesMap)))
	for root, expectedLeafs := range expectedRootToNewLeafTablesMap {
		actualLeafs, ok := rootToNewLeafTablesMap[root]
		assert.Equal(t, ok, true)
		assert.Equal(t, len(expectedLeafs), len(actualLeafs))
		for _, l := range expectedLeafs {
			assert.Equal(t, slices.Contains(actualLeafs, l), true)
		}
	}
}

// Asserting guardrails checks for missing and extra tables
func assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t *testing.T, expectedMissingTables []sqlname.NameTuple, expectedExtraTables []sqlname.NameTuple, firstRunTableList []sqlname.NameTuple, rootTables []sqlname.NameTuple) {
	nameRegistryList, err := getRegisteredNameRegList()
	if err != nil {
		t.Fatalf("error getting registered list: %v", err)
	}
	firstRunTableWithLeafParititons, currentRunTableListWithLeafPartitions, err := applyTableListFlagsOnCurrentAndRemoveRootsFromBothLists(nameRegistryList, source.TableList, source.ExcludeTableList, nil, rootTables, firstRunTableList)
	if err != nil {
		t.Fatalf("error getting first run and current run list: %v", err)
	}
	//Reporting the guardrail msgs only on leaf tables to be consistent so filtering the root table from both the list
	missingTables, extraTables := guardrailsAroundFirstRunAndCurrentRunTableList(firstRunTableWithLeafParititons, currentRunTableListWithLeafPartitions)
	assert.Equal(t, len(missingTables), len(expectedMissingTables))
	assert.Equal(t, len(extraTables), len(expectedExtraTables))

	for _, l := range expectedMissingTables {
		assert.Equal(t, lo.ContainsBy(missingTables, func(t sqlname.NameTuple) bool {
			return t.Equals(l)
		}), true)
	}
	for _, l := range expectedExtraTables {
		assert.Equal(t, lo.ContainsBy(extraTables, func(t sqlname.NameTuple) bool {
			return t.Equals(l)
		}), true)
	}

}

// Tests the unknown table case by over ridding the utils.ErrExit function to assert the error msg
func testUnknownTableCaseForTableListFlags(t *testing.T, withStartClean bool, expectedUnknownErrorMsg string) {
	previousFunction := utils.ErrExit
	//changing the error exit function to test the unknown table scenario
	utils.ErrExit = func(formatString string, args ...interface{}) {
		errMSg := fmt.Sprintf(formatString, args...)
		assert.Equal(t, strings.Contains(errMSg, expectedUnknownErrorMsg), true)
	}

	//Run the function assert the error exit scenario
	startClean = utils.BoolStr(withStartClean)
	getInitialTableList()

	utils.ErrExit = previousFunction
}

var (
	cleanUpSqls = []string{
		`DROP TABLE public.test_partitions_sequences ;`,
		`DROP SCHEMA p1 CASCADE;`,
		`DROP TABLE public.sales_region;`,
		`DROP table datatypes1 CASCADE;`,
		`DROP TYPE week;`,
		`drop table foreign_test ;`,
	}
	//2 partitions table - 1 pure on public schema, 2nd parent on public and leafs on p1
	//2 normal table with FK
	pgSchemaSqls = []string{
		`CREATE TABLE public.test_partitions_sequences (id serial, amount int, branch text, region text, PRIMARY KEY(id, region)) PARTITION BY LIST (region);`,
		`CREATE TABLE public.test_partitions_sequences_l PARTITION OF public.test_partitions_sequences FOR VALUES IN ('London');`,
		`CREATE TABLE public.test_partitions_sequences_s PARTITION OF public.test_partitions_sequences FOR VALUES IN ('Sydney');`,
		`CREATE TABLE public.test_partitions_sequences_b PARTITION OF public.test_partitions_sequences FOR VALUES IN ('Boston');`,
		`CREATE SCHEMA p1;`,
		`CREATE TABLE public.sales_region (id int, amount int, branch text, region text, PRIMARY KEY(id, region)) PARTITION BY LIST (region);`,
		`CREATE TABLE p1.London PARTITION OF public.sales_region FOR VALUES IN ('London');`,
		`CREATE TABLE p1.Sydney PARTITION OF public.sales_region FOR VALUES IN ('Sydney');`,
		`CREATE TABLE p1.Boston PARTITION OF public.sales_region FOR VALUES IN ('Boston');`,
		`CREATE TYPE week AS ENUM ('Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun');`,
		`create table datatypes1(id serial primary key, bool_type boolean,char_type1 CHAR (1),varchar_type VARCHAR(100),byte_type bytea, enum_type week);`,
		`CREATE TABLE foreign_test ( ID int NOT NULL, ONumber int NOT NULL, PID int, PRIMARY KEY (ID), FOREIGN KEY (PID) REFERENCES datatypes1(ID));`,
	}
	pgSchemasTest1 = "public|p1"
)

func TestTableListInFreshRunOfExportDataBasicPG(t *testing.T) {

	testExportDir := setupPostgreDBAndExportDependencies(t, pgSchemaSqls, pgSchemasTest1)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqls...)
	if testExportDir != "" {
		defer os.RemoveAll(testExportDir)
	}

	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}
	//Running the command level functions
	source = *testPostgresSource.Source

	expectedPartitionsToRootMap := map[string]string{
		"public.test_partitions_sequences_l": "public.test_partitions_sequences",
		"public.test_partitions_sequences_s": "public.test_partitions_sequences",
		"public.test_partitions_sequences_b": "public.test_partitions_sequences",
		"p1.london":                          "public.sales_region",
		"p1.sydney":                          "public.sales_region",
		"p1.boston":                          "public.sales_region",
	}
	expectedTableList := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTuple("public.sales_region"),
		getNameTuple("p1.London"),
		getNameTuple("p1.Sydney"),
		getNameTuple("p1.Boston"),
		getNameTuple("public.datatypes1"),
		getNameTuple("public.foreign_test"),
	}
	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap, expectedTableList)

}
func TestTableListInFreshRunOfExportDataFilterViaFlagsPG(t *testing.T) {

	testExportDir := setupPostgreDBAndExportDependencies(t, pgSchemaSqls, pgSchemasTest1)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqls...)
	if testExportDir != "" {
		defer os.RemoveAll(testExportDir)
	}

	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	//Running the command level functions
	source = *testPostgresSource.Source

	//case1: TableList
	//--table-list test_partitions_sequences,datatypes1
	source.TableList = "test_partitions_sequences,datatypes1"

	expectedPartitionsToRootMap1 := map[string]string{
		"public.test_partitions_sequences_l": "public.test_partitions_sequences",
		"public.test_partitions_sequences_s": "public.test_partitions_sequences",
		"public.test_partitions_sequences_b": "public.test_partitions_sequences",
	}

	expectedTableList1 := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTuple("public.datatypes1"),
	}

	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap1, expectedTableList1)

	//case2: TableList And exclude table with filtering some leaf partitions
	//--table-list test_partitions_sequences,datatypes1,foreign_test,public.sales_region --exclude-table-list p1.london,p1.sydney
	source.TableList = "test_partitions_sequences,datatypes1,foreign_test,public.sales_region"
	source.ExcludeTableList = "p1.london,p1.sydney"

	expectedPartitionsToRootMap2 := map[string]string{
		"public.test_partitions_sequences_l": "public.test_partitions_sequences",
		"public.test_partitions_sequences_s": "public.test_partitions_sequences",
		"public.test_partitions_sequences_b": "public.test_partitions_sequences",
		"p1.boston":                          "public.sales_region",
	}

	expectedTableList2 := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTuple("public.datatypes1"),
		getNameTuple("p1.Boston"),
		getNameTuple("public.sales_region"),
		getNameTuple("public.foreign_test"),
	}

	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap2, expectedTableList2)

	//case3: exclude tableList
	// --exclude-table-list public.test_partitions_sequences,foreign_test,p1.sydney
	source.ExcludeTableList = "public.test_partitions_sequences,foreign_test,p1.sydney"

	expectedPartitionsToRootMap3 := map[string]string{
		"p1.london": "public.sales_region",
		"p1.boston": "public.sales_region",
	}

	expectedTableList3 := []sqlname.NameTuple{
		getNameTuple("public.sales_region"),
		getNameTuple("p1.London"),
		getNameTuple("p1.Boston"),
		getNameTuple("public.datatypes1"),
	}
	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap3, expectedTableList3)
}

func TestTableListInSubsequentRunOfExportDataBasicPG(t *testing.T) {
	testExportDir := setupPostgreDBAndExportDependencies(t, pgSchemaSqls, pgSchemasTest1)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqls...)
	if testExportDir != "" {
		defer os.RemoveAll(testExportDir)
	}

	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	metaDB = initMetaDB(testExportDir)

	//Running the command level functions
	source = *testPostgresSource.Source

	//Fetch table list and partitions to root mapping in the first run
	expectedPartitionsToRootMap := map[string]string{
		"public.test_partitions_sequences_l": "public.test_partitions_sequences",
		"public.test_partitions_sequences_s": "public.test_partitions_sequences",
		"public.test_partitions_sequences_b": "public.test_partitions_sequences",
		"p1.london":                          "public.sales_region",
		"p1.sydney":                          "public.sales_region",
		"p1.boston":                          "public.sales_region",
	}
	expectedTableList := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTuple("public.sales_region"),
		getNameTuple("p1.London"),
		getNameTuple("p1.Sydney"),
		getNameTuple("p1.Boston"),
		getNameTuple("public.datatypes1"),
		getNameTuple("public.foreign_test"),
	}
	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap, expectedTableList)

	expectedTableListWithOnlyRootTable := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.sales_region"),
		getNameTuple("public.datatypes1"),
		getNameTuple("public.foreign_test"),
	}

	//Create msr with required details for subsequent run
	err = metaDB.UpdateMigrationStatusRecord(func(msr *metadb.MigrationStatusRecord) {
		msr.SourceDBConf = testPostgresSource.Source
		msr.TableListExportedFromSource = lo.Map(expectedTableListWithOnlyRootTable, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
		msr.SourceExportedTableListWithLeafPartitions = lo.Map(expectedTableList, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
		msr.SourceRenameTablesMap = expectedPartitionsToRootMap
	})
	if err != nil {
		t.Fatalf("error updating msr: %v", err)
	}

	// getInitialTableList for subsequent run with  start-clean false and basic without table-list flags so no guardrails

	assertInitialTableListOnSubsequentRun(t, false, expectedTableList, expectedPartitionsToRootMap)

	//Case: DDL Changes on the Source and then get initial table on re-run
	//Adding new table on the source and asserting that its not coming up in the list in resumption case
	ddlChanges := []string{
		`CREATE TABLE public.test_ddl_change(id int, val text);`,                                                                 // Adding normal table
		`CREATE TABLE public.test_partitions_sequences_p PARTITION OF public.test_partitions_sequences FOR VALUES IN ('Paris');`, // Adding a new leaf on the test_partititons_sequences table (included in table-list)
	}

	testPostgresSource.ExecuteSqls(ddlChanges...)

	// getInitialTableList for subsequent run with  start-clean false and basic without table-list flags so no guardrails
	startClean = false

	//validate detectAndReportNewLeafPartitionsForRootTables  for the new leaf table added above

	rootTables := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
	}

	assertDetectAndReportNewLeafAddedOnSource(t, rootTables, map[string][]string{
		rootTables[0].AsQualifiedCatalogName(): []string{
			getNameTuple("public.test_partitions_sequences_p").AsQualifiedCatalogName(),
		},
	})

	assertInitialTableListOnSubsequentRun(t, false, expectedTableList, expectedPartitionsToRootMap)

	//Case Unknown table name case for the new leaf or any dummy table name
	// getInitialTableList for subsequent run with  start-clean false and basic with table-list flag with unknown table so no guardrails
	utils.DoNotPrompt = true
	source.TableList = "test_partitions_sequences_p,fake_table_name"
	expectedUnknownErrorMsg := `Unknown table names in the include list: [test_partitions_sequences_p fake_table_name]`
	testUnknownTableCaseForTableListFlags(t, false, expectedUnknownErrorMsg)

	//Case --start-clean true
	// getInitialTableList for subsequent run with  start-clean false and basic without table-list flags so no guardrails
	source.TableList = ""

	//Cleaning up nameregistry in start-clean true case as it is required to get new table list
	nameregFile := filepath.Join(testExportDir, "metainfo", "name_registry.json")
	err = os.Remove(nameregFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove name registry file: %s", err)
	}

	//re-initialize name reg for the updated source state
	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	newExpectedTableList := expectedTableList
	newExpectedTableList = append(newExpectedTableList, []sqlname.NameTuple{
		getNameTuple("public.test_ddl_change"),
		getNameTuple("public.test_partitions_sequences_p"),
	}...)

	newExpectedParititonsToRootTable := expectedPartitionsToRootMap
	newExpectedParititonsToRootTable["public.test_partitions_sequences_p"] = "public.test_partitions_sequences"

	assertInitialTableListOnSubsequentRun(t, true, newExpectedTableList, newExpectedParititonsToRootTable)

	testPostgresSource.ExecuteSqls([]string{
		`DROP TABLE public.test_ddl_change;`,
		`DROP TABLE public.test_partitions_sequences_p;`,
	}...)

}

func TestTableListInSubsequentRunOfExportDatWithTableListFlagsPG(t *testing.T) {
	testExportDir := setupPostgreDBAndExportDependencies(t, pgSchemaSqls, pgSchemasTest1)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqls...)
	if testExportDir != "" {
		defer os.RemoveAll(testExportDir)
	}

	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	metaDB = initMetaDB(testExportDir)

	//Running the command level functions
	source = *testPostgresSource.Source

	//case table-list
	//--table-list test_partitions_sequences,datatypes1,foreign_test,public.sales_region --exclude-table-list p1.london,p1.sydney
	source.TableList = "test_partitions_sequences,datatypes1,foreign_test,public.sales_region"
	source.ExcludeTableList = "p1.london,p1.sydney"

	//Fetch table list and partitions to root mapping in the first run
	expectedPartitionsToRootMap := map[string]string{
		"public.test_partitions_sequences_l": "public.test_partitions_sequences",
		"public.test_partitions_sequences_s": "public.test_partitions_sequences",
		"public.test_partitions_sequences_b": "public.test_partitions_sequences",
		"p1.boston":                          "public.sales_region",
	}

	expectedTableList := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTuple("public.datatypes1"),
		getNameTuple("p1.Boston"),
		getNameTuple("public.sales_region"),
		getNameTuple("public.foreign_test"),
	}

	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap, expectedTableList)

	expectedTableListWithOnlyRootTable := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.sales_region"),
		getNameTuple("public.datatypes1"),
		getNameTuple("public.foreign_test"),
	}

	//Create msr with required details for subsequent run
	err = metaDB.UpdateMigrationStatusRecord(func(msr *metadb.MigrationStatusRecord) {
		msr.SourceDBConf = testPostgresSource.Source
		msr.TableListExportedFromSource = lo.Map(expectedTableListWithOnlyRootTable, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
		msr.SourceExportedTableListWithLeafPartitions = lo.Map(expectedTableList, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
		msr.SourceRenameTablesMap = expectedPartitionsToRootMap
	})
	if err != nil {
		t.Fatalf("error updating msr: %v", err)
	}

	testCasesWithDifferentTableListFlagValuesTest1(t, expectedTableList, expectedPartitionsToRootMap)

}

func testCasesWithDifferentTableListFlagValuesTest1(t *testing.T, firstRunTableList []sqlname.NameTuple, firstRunPartitionsToRootMap map[string]string) {
	//case1: getInitialTableList for subsequent run with  start-clean false and basic with same table-list flags so no guardrails
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case2: getInitialTableList for subsequent run with  start-clean false and basic with no table-list flags so reporting extra tables found 
	source.TableList = ""
	source.ExcludeTableList = ""
	utils.DoNotPrompt = true

	rootTables := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.sales_region"),
	}

	expectedExtraTables0 := []sqlname.NameTuple{
		getNameTuple("p1.london"),
		getNameTuple("p1.sydney"),
	}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, nil, expectedExtraTables0, firstRunTableList, rootTables)
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case3: getInitialTableList for subsequent run with  start-clean false and basic with table-list flag
	//--table-list test_partitions_sequences,sales_region
	source.TableList = "test_partitions_sequences,sales_region"

	expectedMissingTables := []sqlname.NameTuple{
		getNameTuple("public.datatypes1"),
		getNameTuple("public.foreign_test"),
	}

	expectedExtraTables := []sqlname.NameTuple{
		getNameTuple("p1.london"),
		getNameTuple("p1.sydney"),
	}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables, expectedExtraTables, firstRunTableList, rootTables)
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case4: getInitialTableList for subsequent run with  start-clean false and basic with only exclude-table-list flag
	startClean = false
	//--exclude-table-list sales_region,foreign_test
	source.TableList = ""
	source.ExcludeTableList = "sales_region,foreign_test"
	utils.DoNotPrompt = true

	expectedMissingTables2 := []sqlname.NameTuple{
		getNameTuple("p1.boston"),
		getNameTuple("public.foreign_test"),
	}

	expectedExtraTables2 := []sqlname.NameTuple{}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables2, expectedExtraTables2, firstRunTableList, rootTables)
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case5: getInitialTableList for subsequent run with  start-clean false and basic with table-list and exclude-table-list flags
	//--table-list test_partitions_sequences,foreign_test,p1.london --exclude-table-list sales_region
	source.TableList = "test_partitions_sequences,foreign_test,p1.london"
	source.ExcludeTableList = "sales_region"
	utils.DoNotPrompt = true

	expectedMissingTables3 := []sqlname.NameTuple{
		getNameTuple("p1.boston"),
		getNameTuple("public.datatypes1"),
	}

	expectedExtraTables3 := []sqlname.NameTuple{}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables3, expectedExtraTables3, firstRunTableList, rootTables)
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case6: getInitialTableList for subsequent run with  start-clean false and basic with different set of table-list and exclude-table-list flags
	//--table-list test_partitions_sequences,p1.boston,foreign_test,p1.london,datatypes1 --exclude-table-list datatypes1,test_partitions_sequences_s
	source.TableList = "test_partitions_sequences,p1.boston,foreign_test,p1.london,datatypes1"
	source.ExcludeTableList = "datatypes1,test_partitions_sequences_s"
	utils.DoNotPrompt = true

	expectedMissingTables4 := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.datatypes1"),
	}

	expectedExtraTables4 := []sqlname.NameTuple{
		getNameTuple("p1.london"),
	}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables4, expectedExtraTables4, firstRunTableList, rootTables)
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)
}
