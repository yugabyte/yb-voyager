//go:build unit

// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemasnapshot

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

// TestNewProviderPostgres asserts that newProvider("postgresql", db) returns a
// provider whose DatabaseType() is "postgresql" and HasStableIdentity() is true.
func TestNewProviderPostgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	_ = mock // no queries expected; just constructing the provider

	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err, "newProvider must succeed for postgresql")
	assert.Equal(t, "postgresql", p.DatabaseType())
	assert.True(t, p.HasStableIdentity(), "PostgreSQL provider must report stable identity")
}

// TestNewProviderUnknownType asserts that newProvider for an unsupported database
// type returns a clear, non-nil error.
func TestNewProviderUnknownType(t *testing.T) {
	_, err := newProvider("does-not-exist-pg-test", nil)
	require.Error(t, err, "newProvider must return an error for an unsupported type")
	assert.Contains(t, err.Error(), "does-not-exist-pg-test")
}

// TestTakeSnapshotLoadsTablesAndColumns verifies that TakeSnapshot queries
// the catalog for tables and columns in the given schemas.
func TestTakeSnapshotLoadsTablesAndColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err)

	// Expect the database version query.
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow("16.4 (Ubuntu 16.4-0ubuntu0.24.04.2)"))

	// Expect the tables query (pg_class).
	tableRows := sqlmock.NewRows([]string{"oid", "schema", "name", "relkind"}).
		AddRow("16420", "public", "orders", "r").
		AddRow("16421", "public", "shipments", "p").
		AddRow("16422", "public", "foreign_t", "f")
	mock.ExpectQuery(`pg_class`).WillReturnRows(tableRows)

	// Expect the table links query (pg_inherits) — no links in this test.
	// Query order: pg_inherits runs inside loadTables, BEFORE pg_attribute.
	mock.ExpectQuery(`pg_inherits`).WillReturnRows(
		sqlmock.NewRows([]string{"child_oid", "child_schema", "child_name", "parent_oid", "parent_schema", "parent_name", "is_partition"}),
	)

	// Expect the columns query (pg_attribute).
	colRows := sqlmock.NewRows([]string{"table_oid", "attnum", "schema", "table_name", "col_name", "data_type", "not_null", "col_default"}).
		AddRow("16420", "1", "public", "orders", "id", "bigint", true, "").
		AddRow("16420", "2", "public", "orders", "amount", "numeric", false, "0")
	mock.ExpectQuery(`pg_attribute`).WillReturnRows(colRows)

	snap, err := p.TakeSnapshot(context.Background(), []string{"public"})
	require.NoError(t, err)
	require.NotNil(t, snap)

	require.Len(t, snap.Tables, 3)
	assert.Equal(t, "orders", snap.Tables[0].Name)
	assert.Equal(t, TableKindOrdinary, snap.Tables[0].Kind)
	assert.Equal(t, "16420", snap.Tables[0].ID)

	assert.Equal(t, "shipments", snap.Tables[1].Name)
	assert.Equal(t, TableKindPartitioned, snap.Tables[1].Kind)

	assert.Equal(t, "foreign_t", snap.Tables[2].Name)
	assert.Equal(t, TableKindForeign, snap.Tables[2].Kind)

	require.Len(t, snap.Columns, 2)
	assert.Equal(t, "id", snap.Columns[0].Name)
	assert.Equal(t, "bigint", snap.Columns[0].DataType)
	assert.True(t, snap.Columns[0].NotNull)
	assert.Equal(t, "16420:1", snap.Columns[0].ID)

	assert.Equal(t, "amount", snap.Columns[1].Name)
	assert.Equal(t, "numeric", snap.Columns[1].DataType)
	assert.False(t, snap.Columns[1].NotNull)
	assert.Equal(t, "0", snap.Columns[1].Default)
	assert.Equal(t, "16420:2", snap.Columns[1].ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestTakeSnapshotDatabaseVersionSet verifies that TakeSnapshot populates DatabaseVersion.
func TestTakeSnapshotDatabaseVersionSet(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err)

	// Query order: SHOW server_version → pg_class → pg_inherits → pg_attribute.
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow("14.11"))

	mock.ExpectQuery(`pg_class`).WillReturnRows(sqlmock.NewRows([]string{"oid", "schema", "name", "relkind"}))
	mock.ExpectQuery(`pg_inherits`).WillReturnRows(sqlmock.NewRows([]string{"child_oid", "child_schema", "child_name", "parent_oid", "parent_schema", "parent_name", "is_partition"}))
	mock.ExpectQuery(`pg_attribute`).WillReturnRows(sqlmock.NewRows([]string{"table_oid", "attnum", "schema", "table_name", "col_name", "data_type", "not_null", "col_default"}))

	snap, err := p.TakeSnapshot(context.Background(), []string{"public"})
	require.NoError(t, err)
	assert.Equal(t, "14.11", snap.DatabaseVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestTakeSnapshotPartitionLinkage verifies that TakeSnapshot wires up
// PartitionParent on child tables and PartitionChildren on parent tables.
// It also verifies that declarative partitions are NOT mislabeled as legacy inheritance.
func TestTakeSnapshotPartitionLinkage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	p, err := newProvider("postgresql", db)
	require.NoError(t, err)

	// Query order: SHOW server_version → pg_class → pg_inherits → pg_attribute.

	// version
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow("16.4"))

	// tables: one partitioned parent + one ordinary child
	tableRows := sqlmock.NewRows([]string{"oid", "schema", "name", "relkind"}).
		AddRow("1001", "public", "events", "p").
		AddRow("1002", "public", "events_2026", "r")
	mock.ExpectQuery(`pg_class`).WillReturnRows(tableRows)

	// table links: events_2026 is a declarative partition of events (is_partition=true)
	linkRows := sqlmock.NewRows([]string{"child_oid", "child_schema", "child_name", "parent_oid", "parent_schema", "parent_name", "is_partition"}).
		AddRow("1002", "public", "events_2026", "1001", "public", "events", true)
	mock.ExpectQuery(`pg_inherits`).WillReturnRows(linkRows)

	// columns: empty for simplicity
	mock.ExpectQuery(`pg_attribute`).WillReturnRows(
		sqlmock.NewRows([]string{"table_oid", "attnum", "schema", "table_name", "col_name", "data_type", "not_null", "col_default"}),
	)

	snap, err := p.TakeSnapshot(context.Background(), []string{"public"})
	require.NoError(t, err)
	require.NotNil(t, snap)

	// build lookup map
	byName := map[string]Table{}
	for _, tb := range snap.Tables {
		byName[tb.Name] = tb
	}

	// parent should have PartitionChildren populated
	parent := byName["events"]
	require.Len(t, parent.PartitionChildren, 1)
	assert.Equal(t, ObjectRef{Schema: "public", Name: "events_2026"}, parent.PartitionChildren[0])
	assert.Nil(t, parent.PartitionParent, "partitioned parent must have nil PartitionParent")
	assert.Empty(t, parent.InheritedBy, "partition parent must NOT be mislabeled as inheritance parent")

	// child should have PartitionParent set
	child := byName["events_2026"]
	require.NotNil(t, child.PartitionParent)
	assert.Equal(t, ObjectRef{Schema: "public", Name: "events"}, *child.PartitionParent)
	assert.Empty(t, child.PartitionChildren, "child partition must have empty PartitionChildren")
	assert.Empty(t, child.InheritsFrom, "declarative partition must NOT be mislabeled as legacy inheritance child")

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestTakeSnapshotInheritanceLinkage verifies that TakeSnapshot wires up
// InheritsFrom on child tables and InheritedBy on parent tables for legacy
// table inheritance (INHERITS), and does NOT mislabel them as partitions.
func TestTakeSnapshotInheritanceLinkage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	p, err := newProvider("postgresql", db)
	require.NoError(t, err)

	// Query order: SHOW server_version → pg_class → pg_inherits → pg_attribute.

	// version
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow("16.4"))

	// tables: parent "animals" (ordinary) and child "dogs" (ordinary)
	tableRows := sqlmock.NewRows([]string{"oid", "schema", "name", "relkind"}).
		AddRow("2001", "public", "animals", "r").
		AddRow("2002", "public", "dogs", "r")
	mock.ExpectQuery(`pg_class`).WillReturnRows(tableRows)

	// table links: dogs inherits from animals (is_partition=false — legacy INHERITS)
	linkRows := sqlmock.NewRows([]string{"child_oid", "child_schema", "child_name", "parent_oid", "parent_schema", "parent_name", "is_partition"}).
		AddRow("2002", "public", "dogs", "2001", "public", "animals", false)
	mock.ExpectQuery(`pg_inherits`).WillReturnRows(linkRows)

	// columns: empty for simplicity
	mock.ExpectQuery(`pg_attribute`).WillReturnRows(
		sqlmock.NewRows([]string{"table_oid", "attnum", "schema", "table_name", "col_name", "data_type", "not_null", "col_default"}),
	)

	snap, err := p.TakeSnapshot(context.Background(), []string{"public"})
	require.NoError(t, err)
	require.NotNil(t, snap)

	// build lookup map
	byName := map[string]Table{}
	for _, tb := range snap.Tables {
		byName[tb.Name] = tb
	}

	// dogs.InheritsFrom must contain animals
	dogs := byName["dogs"]
	require.Len(t, dogs.InheritsFrom, 1)
	assert.Equal(t, ObjectRef{Schema: "public", Name: "animals"}, dogs.InheritsFrom[0])
	// dogs must NOT be mislabeled as a declarative partition
	assert.Nil(t, dogs.PartitionParent, "legacy-inheritance child must NOT have PartitionParent set")
	assert.Empty(t, dogs.PartitionChildren, "legacy-inheritance child must have empty PartitionChildren")

	// animals.InheritedBy must contain dogs
	animals := byName["animals"]
	require.Len(t, animals.InheritedBy, 1)
	assert.Equal(t, ObjectRef{Schema: "public", Name: "dogs"}, animals.InheritedBy[0])
	// animals must NOT be mislabeled as a declarative partitioned table
	assert.Empty(t, animals.PartitionChildren, "legacy-inheritance parent must NOT have PartitionChildren set")
	assert.Nil(t, animals.PartitionParent, "legacy-inheritance parent must have nil PartitionParent")

	require.NoError(t, mock.ExpectationsWereMet())
}
