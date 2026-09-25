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
package tgtdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

type partitionedTablesWithoutOwnPKLister interface {
	GetPartitionedTablesWithoutOwnPrimaryKey(tables []sqlname.NameTuple) ([]sqlname.NameTuple, error)
}

func TestPostgresGetPartitionedTablesWithoutOwnPrimaryKey(t *testing.T) {
	testGetPartitionedTablesWithoutOwnPrimaryKey(t, testPostgresTarget, POSTGRESQL)
}

func TestYugabyteGetPartitionedTablesWithoutOwnPrimaryKey(t *testing.T) {
	testGetPartitionedTablesWithoutOwnPrimaryKey(t, testYugabyteDBTarget, YUGABYTEDB)
}

func testGetPartitionedTablesWithoutOwnPrimaryKey(t *testing.T, target *TestDB, dbType string) {
	target.ExecuteSqls(
		`CREATE SCHEMA pk_test;`,
		`CREATE SCHEMA pk_test_leaves;`,
		`CREATE TABLE pk_test.plain_with_pk (id INT PRIMARY KEY);`,
		`CREATE TABLE pk_test.plain_without_pk (id INT);`,
		`CREATE TABLE pk_test.root_with_pk (id INT, region TEXT, PRIMARY KEY (id, region)) PARTITION BY LIST (region);`,
		`CREATE TABLE pk_test.root_with_pk_r1 PARTITION OF pk_test.root_with_pk FOR VALUES IN ('r1');`,
		`CREATE TABLE pk_test.leaf_pk_only (id INT NOT NULL, region TEXT NOT NULL) PARTITION BY LIST (region);`,
		`CREATE TABLE pk_test.leaf_pk_only_r1 PARTITION OF pk_test.leaf_pk_only FOR VALUES IN ('r1');`,
		`CREATE TABLE pk_test_leaves.leaf_pk_only_r2 PARTITION OF pk_test.leaf_pk_only FOR VALUES IN ('r2');`,
		`ALTER TABLE pk_test.leaf_pk_only_r1 ADD PRIMARY KEY (id);`,
		`ALTER TABLE pk_test_leaves.leaf_pk_only_r2 ADD PRIMARY KEY (id);`,
		`CREATE TABLE pk_test."CaseRoot" (id INT NOT NULL, region TEXT NOT NULL) PARTITION BY LIST (region);`,
		`CREATE TABLE pk_test."CaseRoot_R1" PARTITION OF pk_test."CaseRoot" FOR VALUES IN ('r1');`,
		`ALTER TABLE pk_test."CaseRoot_R1" ADD PRIMARY KEY (id);`,
		`CREATE TABLE pk_test.multi_level (id INT NOT NULL, region TEXT NOT NULL, amt INT NOT NULL) PARTITION BY LIST (region);`,
		`CREATE TABLE pk_test.multi_level_a PARTITION OF pk_test.multi_level FOR VALUES IN ('a') PARTITION BY RANGE (amt);`,
		`CREATE TABLE pk_test.multi_level_a_low PARTITION OF pk_test.multi_level_a FOR VALUES FROM (MINVALUE) TO (100);`,
		`ALTER TABLE pk_test.multi_level_a_low ADD PRIMARY KEY (id);`,
	)
	defer target.ExecuteSqls(`DROP SCHEMA pk_test CASCADE;`, `DROP SCHEMA pk_test_leaves CASCADE;`)

	tuple := func(name string) sqlname.NameTuple {
		return testutils.CreateNameTupleWithTargetName(name, "public", dbType)
	}
	requested := []sqlname.NameTuple{
		tuple("pk_test.plain_with_pk"),
		tuple("pk_test.plain_without_pk"),
		tuple("pk_test.root_with_pk"),
		tuple("pk_test.leaf_pk_only"),
		tuple(`pk_test."CaseRoot"`),
		tuple("pk_test.multi_level"),
	}

	lister, ok := target.TargetDB.(partitionedTablesWithoutOwnPKLister)
	require.True(t, ok, "%s target must implement GetPartitionedTablesWithoutOwnPrimaryKey", dbType)

	result, err := lister.GetPartitionedTablesWithoutOwnPrimaryKey(requested)
	require.NoError(t, err)
	assert.ElementsMatch(t, []sqlname.NameTuple{
		tuple("pk_test.leaf_pk_only"),
		tuple(`pk_test."CaseRoot"`),
		tuple("pk_test.multi_level"),
	}, result)

	emptyResult, err := lister.GetPartitionedTablesWithoutOwnPrimaryKey(nil)
	require.NoError(t, err)
	assert.Empty(t, emptyResult)
}
