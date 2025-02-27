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
	"testing"

	"github.com/samber/lo"
	"gotest.tools/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
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

func setupPostgreDBAndExportDependencies(t *testing.T) string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &testcontainers.ContainerConfig{
		DBName: "table_list_tests",
	}

	postgresContainer := testcontainers.NewTestContainer("postgresql", config)
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
			Schema:    "public|p1",
			DBName:    postgresContainer.GetConfig().DBName,
			Host:      host,
			Port:      port,
			SSLMode:   "disable",
		},
	}
	testExportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	if err != nil {
		t.Errorf("error initialising name reg for the source: %v", err)
	}

	sqlname.SourceDBType = testPostgresSource.DBType

	CreateMigrationProjectIfNotExists(constants.POSTGRESQL, testExportDir)
	//2 partitions table - 1 pure on public schema, 2nd parent on public and leafs on p1
	//2 normal table with FK
	schemaSqls := []string{
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
	testPostgresSource.ExecuteSqls(schemaSqls...)
	return testExportDir
}

func getNameTuple(s string) sqlname.NameTuple {
	defaultSchema, _ := getDefaultSourceSchemaName()
	return sqlname.NameTuple{
		SourceName:  sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
		CurrentName: sqlname.NewObjectNameWithQualifiedName(testPostgresSource.DBType, defaultSchema, s),
	}
}

func fetchTableListAndAssertResult(t *testing.T, expectedPartitionsToRootMap map[string]string, expectedTableList []sqlname.NameTuple) {
	partitionsToRootMap, tableList, err := fetchTablesNamesFromSourceAndFilterTableList()
	if err != nil {
		t.Errorf("error fetching the table list from DB: %v", err)
	}
	for expectedLeaf, expectedRoot := range expectedPartitionsToRootMap {
		actualRoot, ok := partitionsToRootMap[expectedLeaf]
		assert.Equal(t, ok, true)
		assert.Equal(t, actualRoot, expectedRoot)
	}
	fmt.Printf("actual - %v", tableList)
	fmt.Printf("expected - %v", expectedTableList)
	assert.Equal(t, len(tableList), len(expectedTableList))
	assert.Equal(t, len(lo.Keys(partitionsToRootMap)), len(lo.Keys(expectedPartitionsToRootMap)))
	diff := sqlname.SetDifferenceNameTuples(expectedTableList, tableList)
	assert.Equal(t, len(diff), 0)
}

func TestTableListInFreshRunOfExportDataBasic(t *testing.T) {

	testExportDir := setupPostgreDBAndExportDependencies(t)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls([]string{
		`DROP SCHEMA public CASCADE;`,
		`DROP SCHEMA p1 CASCADE;`,
	}...)

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
	fetchTableListAndAssertResult(t, expectedPartitionsToRootMap, expectedTableList)

}
func TestTableListInFreshRunOfExportDataFilterViaFlags(t *testing.T) {

	testExportDir := setupPostgreDBAndExportDependencies(t)

	err := testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()
	defer testPostgresSource.ExecuteSqls([]string{
		`DROP SCHEMA public CASCADE;`,
		`DROP SCHEMA p1 CASCADE;`,
	}...)

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

	fetchTableListAndAssertResult(t, expectedPartitionsToRootMap1, expectedTableList1)

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

	fetchTableListAndAssertResult(t, expectedPartitionsToRootMap2, expectedTableList2)

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
	fetchTableListAndAssertResult(t, expectedPartitionsToRootMap3, expectedTableList3)
}
