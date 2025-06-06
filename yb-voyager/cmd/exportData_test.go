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
	"os"
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"gotest.tools/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

type TestDB struct {
	testcontainers.TestContainer
	*srcdb.Source
}

var (
	testPostgresSource   *TestDB
	testYugabyteDBSource *TestDB
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil, nil)
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

	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil, nil)
	err = yugabytedbContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start yugabytedb container: %v", err)
	}
	host, port, err = yugabytedbContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testYugabyteDBSource = &TestDB{
		TestContainer: yugabytedbContainer,
		Source: &srcdb.Source{
			DBType:    "yugabytedb",
			DBVersion: yugabytedbContainer.GetConfig().DBVersion,
			User:      yugabytedbContainer.GetConfig().User,
			Password:  yugabytedbContainer.GetConfig().Password,
			Schema:    yugabytedbContainer.GetConfig().Schema,
			DBName:    yugabytedbContainer.GetConfig().DBName,
			Host:      host,
			Port:      port,
			SSLMode:   "disable",
		},
	}

	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)
	exitCode := m.Run()

	// cleaning up all the running containers
	testcontainers.TerminateAllContainers()

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

func setupPostgresDBSourceAndYugabyteDBTargetWithExportDependencies(t *testing.T, sqls []string, schemas string) string {
	testExportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	sqlname.SourceDBType = POSTGRESQL

	CreateMigrationProjectIfNotExists(constants.POSTGRESQL, testExportDir)
	testPostgresSource.Schema = schemas
	testPostgresSource.ExecuteSqls(sqls...)
	testYugabyteDBSource.Schema = schemas
	testYugabyteDBSource.ExecuteSqls(sqls...)

	targetConf := tgtdb.TargetConf{
		DBVersion:    testYugabyteDBSource.TestContainer.GetConfig().DBVersion,
		User:         testYugabyteDBSource.TestContainer.GetConfig().User,
		Password:     testYugabyteDBSource.TestContainer.GetConfig().Password,
		Schema:       testYugabyteDBSource.TestContainer.GetConfig().Schema,
		DBName:       testYugabyteDBSource.TestContainer.GetConfig().DBName,
		Host:         testYugabyteDBSource.Source.Host,
		Port:         testYugabyteDBSource.Source.Port,
		TargetDBType: YUGABYTEDB,
		SSLMode:      "disable",
	}
	testYugabyteDBTarget = &TestTargetDB{
		Tconf:         targetConf,
		TestContainer: testYugabyteDBSource.TestContainer,
		TargetDB:      tgtdb.NewTargetDB(&targetConf),
	}
	return testExportDir
}

func getNameTuple(s string) sqlname.NameTuple {
	defaultSchema, _ := getDefaultSourceSchemaName()
	return sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
	}
}

func getNameTupleWithTargetName(s string) sqlname.NameTuple {
	defaultSchema, _ := getDefaultSourceSchemaName()
	return sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
		TargetName:  sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
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
	missingTables, extraTables, err := guardrailsAroundFirstRunAndCurrentRunTableList(firstRunTableWithLeafParititons, currentRunTableListWithLeafPartitions)
	assert.ErrorContains(t, err, `Changing the table list during live-migration is not allowed.`)
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

	//Run the function assert the error exit scenario
	startClean = utils.BoolStr(withStartClean)
	_, _, err := getInitialTableList()
	assert.ErrorContains(t, err, expectedUnknownErrorMsg)
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
	pgSchemasTest1    = "public|p1"
	pgSchemaSqlsTest2 = []string{
		`CREATE SCHEMA test1;`,
		`CREATE SCHEMA test2;`,
		`CREATE TABLE test1.customers (id INTEGER, statuses TEXT, arr NUMERIC, PRIMARY KEY(id, statuses, arr)) PARTITION BY LIST(statuses);`,
		`CREATE TABLE test1.cust_active PARTITION OF test1.customers FOR VALUES IN ('ACTIVE', 'RECURRING','REACTIVATED') PARTITION BY RANGE(arr);`,
		`CREATE TABLE test2.cust_other  PARTITION OF test1.customers DEFAULT;`,
		`CREATE TABLE test2.cust_arr_small PARTITION OF test1.cust_active FOR VALUES FROM (MINVALUE) TO (101) PARTITION BY HASH(id);`,
		`CREATE TABLE test1.cust_part11 PARTITION OF test2.cust_arr_small FOR VALUES WITH (modulus 2, remainder 0);`,
		`CREATE TABLE test1.cust_part12 PARTITION OF test2.cust_arr_small FOR VALUES WITH (modulus 2, remainder 1);`,
		`CREATE TABLE test2.cust_arr_large PARTITION OF test1.cust_active FOR VALUES FROM (101) TO (MAXVALUE) PARTITION BY HASH(id);`,
		`CREATE TABLE test1.cust_part21 PARTITION OF test2.cust_arr_large FOR VALUES WITH (modulus 2, remainder 0);`,
		`CREATE TABLE test1.cust_part22 PARTITION OF test2.cust_arr_large FOR VALUES WITH (modulus 2, remainder 1);`,
		`CREATE TABLE test1."Foo" (id int, val text);`,
		`CREATE TABLE test1."Foo1" (id int, val text);`,
		`CREATE TABLE test1."Foo2" (id int, val text);`,
	}
	pgSchemasTest2   = "test1|test2"
	cleanUpSqlsTest2 = []string{
		`DROP SCHEMA test1 cascade;`,
		`DROP SCHEMA test2 cascade;`,
	}
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
	utils.DoNotPrompt = true

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

	testCasesWithDifferentTableListFlagValuesForBasicSchema(t, expectedTableList, expectedPartitionsToRootMap)

}

func testCasesWithDifferentTableListFlagValuesForBasicSchema(t *testing.T, firstRunTableList []sqlname.NameTuple, firstRunPartitionsToRootMap map[string]string) {
	//case1: getInitialTableList for subsequent run with  start-clean false and basic with same table-list flags so no guardrails
	source.TableList = "test_partitions_sequences,datatypes1,foreign_test,public.sales_region"
	source.ExcludeTableList = "p1.london,p1.sydney"
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

	//Case when user fixes the table-list flags to have initial list, it should return the same list and partitions map
	//--table-list test_partitions_sequences,datatypes1,foreign_test,public.sales_region --exclude-table-list p1.london,p1.sydney
	source.TableList = "test_partitions_sequences,datatypes1,foreign_test,p1.boston"
	source.ExcludeTableList = ""
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

}

func TestTableListInSubsequentRunOfExportDatWithTableListFlagsPGWithMultiLevelPartitionsAndWildCards(t *testing.T) {
	testExportDir := setupPostgreDBAndExportDependencies(t, pgSchemaSqlsTest2, pgSchemasTest2)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqlsTest2...)
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
	expectedPartitionsToRootMapWithoutFlags := map[string]string{
		`test1.cust_part22`: `test1.customers`,
		`test1.cust_part21`: `test1.customers`,
		`test1.cust_part12`: `test1.customers`,
		`test1.cust_part11`: `test1.customers`,
		`test2.cust_other`:  `test1.customers`,
	}

	expectedTableListWithoutFlags := []sqlname.NameTuple{
		getNameTuple(`test1.cust_part22`),
		getNameTuple(`test1.cust_part21`),
		getNameTuple(`test1.cust_part12`),
		getNameTuple(`test1.cust_part11`),
		getNameTuple(`test2.cust_other`),
		getNameTuple(`test1.customers`),
		getNameTuple(`test1."Foo"`),
		getNameTuple(`test1."Foo1"`),
		getNameTuple(`test1."Foo2"`),
	}

	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMapWithoutFlags, expectedTableListWithoutFlags)

	//case table-list
	//--table-list test1.customers,test1.Foo* --exclude-table-list test2.cust_arr*
	source.TableList = "test1.customers,test1.Foo*"
	source.ExcludeTableList = "test2.cust_arr*"

	//Fetch table list and partitions to root mapping in the first run
	expectedPartitionsToRootMap := map[string]string{
		`test2.cust_other`: `test1.customers`,
	}

	expectedTableList := []sqlname.NameTuple{
		getNameTuple(`test2.cust_other`),
		getNameTuple(`test1.customers`),
		getNameTuple(`test1."Foo"`),
		getNameTuple(`test1."Foo1"`),
		getNameTuple(`test1."Foo2"`),
	}

	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap, expectedTableList)

	expectedTableListWithOnlyRootTable := []sqlname.NameTuple{}

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

	testCasesWithDifferentTableListFlagValuesWithMultiLevelPartitionsAndWildCards(t, expectedTableList, expectedPartitionsToRootMap)

}

func testCasesWithDifferentTableListFlagValuesWithMultiLevelPartitionsAndWildCards(t *testing.T, firstRunTableList []sqlname.NameTuple, firstRunPartitionsToRootMap map[string]string) {
	//case1: getInitialTableList for subsequent run with  start-clean false and basic with same table-list flags so no guardrails
	source.TableList = "test1.customers,test1.Foo*"
	source.ExcludeTableList = "test2.cust_arr*"
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case2: getInitialTableList for subsequent run with  start-clean false and basic with no table-list flags so reporting extra tables found
	source.TableList = ""
	source.ExcludeTableList = ""
	utils.DoNotPrompt = true

	rootTables := []sqlname.NameTuple{
		getNameTuple(`test1.customers`),
	}

	expectedExtraTables0 := []sqlname.NameTuple{
		getNameTuple(`test1.cust_part22`),
		getNameTuple(`test1.cust_part21`),
		getNameTuple(`test1.cust_part12`),
		getNameTuple(`test1.cust_part11`),
	}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, nil, expectedExtraTables0, firstRunTableList, rootTables)

	//case3: getInitialTableList for subsequent run with  start-clean false and basic with table-list flag
	//--table-list
	source.ExcludeTableList = "test1.cust_other,test1.Fooabc,test1.Foo1,test1.Foo2"
	expectedUnknownErrorMsg := `Unknown table names in the exclude list: [test1.cust_other test1.Fooabc]`
	testUnknownTableCaseForTableListFlags(t, false, expectedUnknownErrorMsg)

	//case4: getInitialTableList for subsequent run with  start-clean false and basic with table-list flag with  same list so no guardrails
	//--table-list
	source.ExcludeTableList = ""
	source.TableList = "test2.cust_other,test1.Foo,test1.Foo1,test1.Foo2"
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case5: getInitialTableList for subsequent run with  start-clean false and basic with table-list flag with  same list but with wildcard characters so no guardrails
	//--table-list test2.cust_o*,test1.Foo*
	source.TableList = "test2.cust_o*,test1.Foo*"
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

	//case6: getInitialTableList for subsequent run with  start-clean false and basic with only exclude-table-list flag for guardrails
	startClean = false
	//--exclude-table-list test1.Foo?,test2.cust_arr_small
	source.TableList = ""
	source.ExcludeTableList = "test1.Foo?,test2.cust_arr_small"
	utils.DoNotPrompt = true

	expectedMissingTables2 := []sqlname.NameTuple{
		getNameTuple(`test1."Foo1"`),
		getNameTuple(`test1."Foo2"`),
	}

	expectedExtraTables2 := []sqlname.NameTuple{
		getNameTuple(`test1.cust_part22`),
		getNameTuple(`test1.cust_part21`),
	}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables2, expectedExtraTables2, firstRunTableList, rootTables)

	//case7: getInitialTableList for subsequent run with  start-clean false and basic with table-list and exclude-table-list flags for guardrails
	//--table-list test1.cust_part1?,test2.cust_other,test1."Foo2" --exclude-table-list test1.cust_part2?
	source.TableList = `test1.cust_part1?,test2.cust_other,test1."Foo2"`
	source.ExcludeTableList = `test1.cust_part2?`
	utils.DoNotPrompt = true

	expectedMissingTables3 := []sqlname.NameTuple{
		getNameTuple(`test1."Foo"`),
		getNameTuple(`test1."Foo1"`),
	}

	expectedExtraTables3 := []sqlname.NameTuple{
		getNameTuple(`test1.cust_part12`),
		getNameTuple(`test1.cust_part11`),
	}

	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables3, expectedExtraTables3, firstRunTableList, rootTables)

	//case8: getInitialTableList for subsequent run with  start-clean false and basic with different set of table-list and exclude-table-list flags but same list overall so no guardrails
	//--table-list test1.customers,test1.Foo,test1.Foo? --exclude-table-list test2.cust_arr_small,test2.cust_arr_large
	source.TableList = "test1.customers,test1.Foo,test1.Foo?"
	source.ExcludeTableList = "test2.cust_arr_small,test2.cust_arr_large"
	utils.DoNotPrompt = true
	assertInitialTableListOnSubsequentRun(t, false, firstRunTableList, firstRunPartitionsToRootMap)

}

func TestTableListInFreshRunOfExportDataBasicYB(t *testing.T) {

	testExportDir := setupPostgresDBSourceAndYugabyteDBTargetWithExportDependencies(t, pgSchemaSqls, pgSchemasTest1)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqls...)
	err = testYugabyteDBTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to yugabyte database: %w", err)
	}
	err = InitNameRegistry(testExportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, testYugabyteDBTarget.TargetDB, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}
	testYugabyteDBTarget.Finalize()
	err = testYugabyteDBSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testYugabyteDBSource.DB().Disconnect()
	defer testYugabyteDBSource.ExecuteSqls(cleanUpSqls...)
	if testExportDir != "" {
		defer os.RemoveAll(testExportDir)
	}

	err = InitNameRegistry(testExportDir, TARGET_DB_EXPORTER_FB_ROLE, testYugabyteDBSource.Source, testYugabyteDBSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	metaDB = initMetaDB(testExportDir)
	//Running the command level functions
	source = *testYugabyteDBSource.Source

	expectedPartitionsToRootMap := map[string]string{
		"public.test_partitions_sequences_l": "public.test_partitions_sequences",
		"public.test_partitions_sequences_s": "public.test_partitions_sequences",
		"public.test_partitions_sequences_b": "public.test_partitions_sequences",
		"p1.london":                          "public.sales_region",
		"p1.sydney":                          "public.sales_region",
		"p1.boston":                          "public.sales_region",
	}
	expectedTableList := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTupleWithTargetName("public.sales_region"),
		getNameTuple("p1.London"),
		getNameTuple("p1.Sydney"),
		getNameTuple("p1.Boston"),
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}
	assertTableListFilteringInTheFirstRun(t, expectedPartitionsToRootMap, expectedTableList)

	//Create msr with required details for subsequent run

	expectedTableListWithOnlyRootTable := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTupleWithTargetName("public.sales_region"),
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}
	err = metaDB.UpdateMigrationStatusRecord(func(msr *metadb.MigrationStatusRecord) {
		msr.SourceDBConf = testPostgresSource.Source
		msr.TargetDBConf = &testYugabyteDBTarget.Tconf
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

	assertInitialTableListOnSubsequentRun(t, false, expectedTableList, expectedPartitionsToRootMap)

	//Updating the metadb with less tables to do some guardrails testing
	expectedNewTableListWithLessTables := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTuple("p1.London"),
		getNameTuple("p1.Sydney"),
		getNameTuple("p1.Boston"),
		getNameTupleWithTargetName("public.sales_region"),
	}
	err = metaDB.UpdateMigrationStatusRecord(func(msr *metadb.MigrationStatusRecord) {
		msr.TableListExportedFromSource = []string{
			getNameTupleWithTargetName("public.test_partitions_sequences").ForOutput(),
			getNameTupleWithTargetName("public.sales_region").ForOutput(),
		}
		msr.TargetExportedTableListWithLeafPartitions = lo.Map(expectedNewTableListWithLessTables, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
		msr.TargetRenameTablesMap = expectedPartitionsToRootMap
	})
	if err != nil {
		t.Fatalf("error updating msr: %v", err)
	}

	//case1 with exclude-table-list for the excluded tables, no guardrails
	source.ExcludeTableList = "datatypes1,foreign_test"
	assertInitialTableListOnSubsequentRun(t, false, expectedNewTableListWithLessTables, expectedPartitionsToRootMap)

	//case2 with table list for included tables, no guardrails
	source.TableList = "test_partitions_sequences,sales_Region"
	source.ExcludeTableList = ""
	assertInitialTableListOnSubsequentRun(t, false, expectedNewTableListWithLessTables, expectedPartitionsToRootMap)

	//case3 with  table-list and exclude-table-list flags with  some different set of tables so guardrails case for extra and missing both
	source.TableList = "test_partitions_sequences,sales_region,datatypes1"
	source.ExcludeTableList = "p1.boston"
	extraTables1 := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.datatypes1"),
	}
	missingTables1 := []sqlname.NameTuple{
		getNameTuple("p1.Boston"),
	}
	rootTables := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.sales_region"),
		getNameTupleWithTargetName("public.test_partitions_sequences"),
	}
	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, missingTables1, extraTables1, expectedNewTableListWithLessTables, rootTables)

	//case4 with  table-list and exclude-table-list flags with  some different set of tables so guardrails case for extra only
	source.TableList = "test_partitions_sequences,p1.london,p1.sydney,p1.boston,datatypes1,foreign_test"
	source.ExcludeTableList = ""
	extraTables2 := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}
	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, nil, extraTables2, expectedNewTableListWithLessTables, rootTables)

	//case5 with  table-list and exclude-table-list flags with  some different set of tables so guardrails case for missing only
	source.ExcludeTableList = "test_partitions_sequences,datatypes1,foreign_test"
	missingTables3 := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
	}
	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, missingTables3, nil, expectedNewTableListWithLessTables, rootTables)
}

func TestTableListInFreshRunOfExportDataForTablesExtraInSource(t *testing.T) {

	testExportDir := setupPostgresDBSourceAndYugabyteDBTargetWithExportDependencies(t, pgSchemaSqls, pgSchemasTest1)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	testPostgresSource.ExecuteSqls(`CREATE TABLE test_source_extra(id int, val text);`)
	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqls...)
	defer testPostgresSource.ExecuteSqls(`DROP TABLE test_source_extra;`)

	metaDB = initMetaDB(testExportDir)
	//Running the command level functions
	source = *testPostgresSource.Source
	source.ExcludeTableList = "test_source_extra"

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

	//Create msr with required details for subsequent run

	expectedTableListWithOnlyRootTable := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences"),
		getNameTuple("public.sales_region"),
		getNameTuple("public.datatypes1"),
		getNameTuple("public.foreign_test"),
	}
	err = metaDB.UpdateMigrationStatusRecord(func(msr *metadb.MigrationStatusRecord) {
		msr.SourceDBConf = testPostgresSource.Source
		msr.TargetDBConf = &testYugabyteDBTarget.Tconf
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

	//Start import-data
	err = testYugabyteDBTarget.Init()
	testutils.FatalIfError(t, err)
	err = InitNameRegistry(testExportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, testYugabyteDBTarget.TargetDB, false)
	testutils.FatalIfError(t, err)

	testYugabyteDBTarget.ExecuteSqls(cleanUpSqls...)

	testYugabyteDBTarget.Finalize()
	if testExportDir != "" {
		defer os.RemoveAll(testExportDir)
	}

	expectedTableList = []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTupleWithTargetName("public.sales_region"),
		getNameTuple("p1.London"),
		getNameTuple("p1.Sydney"),
		getNameTuple("p1.Boston"),
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}
	//export-data
	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	testutils.FatalIfError(t, err)

	assertInitialTableListOnSubsequentRun(t, false, expectedTableList, expectedPartitionsToRootMap)

	source.TableList = "test_partitions_sequences,test_source_extra"
	source.ExcludeTableList = ""
	//Guardrails test case for missing and extra tables

	rootTables := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTupleWithTargetName("public.sales_region"),
	}
	expectedMissingTables := []sqlname.NameTuple{
		getNameTuple("p1.London"),
		getNameTuple("p1.Sydney"),
		getNameTuple("p1.Boston"),
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}

	ntuple := getNameTuple("public.test_source_extra")
	ntuple.TargetName = nil
	expectedExtraTables := []sqlname.NameTuple{
		ntuple,
	}
	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables, expectedExtraTables, expectedTableList, rootTables)

}

func TestTableListInFreshRunOfExportDataForTablesExtraInTarget(t *testing.T) {

	testExportDir := setupPostgresDBSourceAndYugabyteDBTargetWithExportDependencies(t, pgSchemaSqls, pgSchemasTest1)

	err := testPostgresSource.DB().Connect()
	testutils.FatalIfError(t, err)

	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	testutils.FatalIfError(t, err)

	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls(cleanUpSqls...)
	metaDB = initMetaDB(testExportDir)
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

	//Create msr with required details for subsequent run

	expectedTableListWithOnlyRootTable := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTupleWithTargetName("public.sales_region"),
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}
	err = metaDB.UpdateMigrationStatusRecord(func(msr *metadb.MigrationStatusRecord) {
		msr.SourceDBConf = testPostgresSource.Source
		msr.TargetDBConf = &testYugabyteDBTarget.Tconf
		msr.TableListExportedFromSource = lo.Map(expectedTableListWithOnlyRootTable, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
		msr.SourceExportedTableListWithLeafPartitions = lo.Map(expectedTableList, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
		msr.SourceRenameTablesMap = expectedPartitionsToRootMap
	})
	testutils.FatalIfError(t, err)

	//Start import-data
	err = testYugabyteDBTarget.Init()
	testutils.FatalIfError(t, err)

	testYugabyteDBSource.ExecuteSqls(`CREATE TABLE test_target_extra(id int, val text);`)
	err = InitNameRegistry(testExportDir, TARGET_DB_IMPORTER_ROLE, nil, nil, &testYugabyteDBTarget.Tconf, testYugabyteDBTarget.TargetDB, false)
	testutils.FatalIfError(t, err)

	testYugabyteDBTarget.Finalize()
	err = testYugabyteDBSource.DB().Connect()
	testutils.FatalIfError(t, err)

	defer testYugabyteDBSource.DB().Disconnect()
	defer testYugabyteDBSource.ExecuteSqls(cleanUpSqls...)
	defer testYugabyteDBSource.ExecuteSqls(`DROP TABLE test_target_extra;`)
	if testExportDir != "" {
		defer os.RemoveAll(testExportDir)
	}

	expectedTableList = []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTupleWithTargetName("public.sales_region"),
		getNameTuple("p1.London"),
		getNameTuple("p1.Sydney"),
		getNameTuple("p1.Boston"),
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}
	//export-data
	err = InitNameRegistry(testExportDir, SOURCE_DB_EXPORTER_ROLE, testPostgresSource.Source, testPostgresSource.DB(), nil, nil, false)
	testutils.FatalIfError(t, err)

	assertInitialTableListOnSubsequentRun(t, false, expectedTableList, expectedPartitionsToRootMap)

	//export-data-from-target
	source.TableList = "test_partitions_sequences,sales_region,datatypes1,foreign_test"
	err = InitNameRegistry(testExportDir, TARGET_DB_EXPORTER_FB_ROLE, testYugabyteDBSource.Source, testYugabyteDBSource.DB(), nil, nil, false)
	testutils.FatalIfError(t, err)

	assertInitialTableListOnSubsequentRun(t, false, expectedTableList, expectedPartitionsToRootMap)

	source.TableList = "sales_region,test_target_extra"
	//Guardrails test case for missing and extra tables

	rootTables := []sqlname.NameTuple{
		getNameTupleWithTargetName("public.test_partitions_sequences"),
		getNameTupleWithTargetName("public.sales_region"),
	}
	expectedMissingTables := []sqlname.NameTuple{
		getNameTuple("public.test_partitions_sequences_l"),
		getNameTuple("public.test_partitions_sequences_s"),
		getNameTuple("public.test_partitions_sequences_b"),
		getNameTupleWithTargetName("public.datatypes1"),
		getNameTupleWithTargetName("public.foreign_test"),
	}

	ntuple := getNameTupleWithTargetName("public.test_target_extra")
	ntuple.SourceName = nil
	expectedExtraTables := []sqlname.NameTuple{
		ntuple,
	}
	assertGuardrailsChecksForMissingAndExtraTablesInSubsequentRun(t, expectedMissingTables, expectedExtraTables, expectedTableList, rootTables)

}
