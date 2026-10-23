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
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

// TestNewProviderPostgres asserts that newProvider("postgresql", db) returns a
// provider whose DatabaseType() is "postgresql".
func TestNewProviderPostgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	_ = mock // no queries expected; just constructing the provider

	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err, "newProvider must succeed for postgresql")
	assert.Equal(t, "postgresql", p.DatabaseType())
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
	expectEmptyConstraintsAndIndexes(mock)

	snap, dbVersion, err := p.TakeSnapshot(context.Background(), []string{"public"})
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.Equal(t, "16.4", dbVersion, "dbVersion must be truncated at first space")

	require.Len(t, snap.Tables, 3)
	assert.Equal(t, "orders", snap.Tables[0].Name)
	assert.Equal(t, TableKindOrdinary, snap.Tables[0].Kind)
	assert.Equal(t, "16420", snap.Tables[0].ID)

	assert.Equal(t, "shipments", snap.Tables[1].Name)
	assert.Equal(t, TableKindPartitioned, snap.Tables[1].Kind)

	assert.Equal(t, "foreign_t", snap.Tables[2].Name)
	assert.Equal(t, TableKindForeign, snap.Tables[2].Kind)

	// Both columns belong to "orders" (oid 16420) and are nested under it, in
	// attnum order; "shipments" and "foreign_t" have no columns in this test.
	require.Len(t, snap.Tables[0].Columns, 2)
	assert.Equal(t, "id", snap.Tables[0].Columns[0].Name)
	assert.Equal(t, "bigint", snap.Tables[0].Columns[0].DataType)
	assert.True(t, snap.Tables[0].Columns[0].NotNull)
	assert.Equal(t, "16420:1", snap.Tables[0].Columns[0].ID)

	assert.Equal(t, "amount", snap.Tables[0].Columns[1].Name)
	assert.Equal(t, "numeric", snap.Tables[0].Columns[1].DataType)
	assert.False(t, snap.Tables[0].Columns[1].NotNull)
	assert.Equal(t, "0", snap.Tables[0].Columns[1].Default)
	assert.Equal(t, "16420:2", snap.Tables[0].Columns[1].ID)
	assert.Empty(t, snap.Tables[1].Columns)
	assert.Empty(t, snap.Tables[2].Columns)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestTakeSnapshotDatabaseVersionReturned verifies that TakeSnapshot returns the
// probed database version as the second return value (not stuffed into the snapshot).
func TestTakeSnapshotDatabaseVersionReturned(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err)

	// Query order: SHOW server_version → pg_class → pg_inherits → pg_attribute → pg_constraint → pg_index.
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow("14.11"))

	mock.ExpectQuery(`pg_class`).WillReturnRows(sqlmock.NewRows([]string{"oid", "schema", "name", "relkind"}))
	mock.ExpectQuery(`pg_inherits`).WillReturnRows(sqlmock.NewRows([]string{"child_oid", "child_schema", "child_name", "parent_oid", "parent_schema", "parent_name", "is_partition"}))
	mock.ExpectQuery(`pg_attribute`).WillReturnRows(sqlmock.NewRows([]string{"table_oid", "attnum", "schema", "table_name", "col_name", "data_type", "not_null", "col_default"}))
	expectEmptyConstraintsAndIndexes(mock)

	snap, dbVersion, err := p.TakeSnapshot(context.Background(), []string{"public"})
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.Equal(t, "14.11", dbVersion, "dbVersion must be the second return value")
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

	// Query order: SHOW server_version → pg_class → pg_inherits → pg_attribute → pg_constraint → pg_index.

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
	expectEmptyConstraintsAndIndexes(mock)

	snap, _, err := p.TakeSnapshot(context.Background(), []string{"public"})
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

	// Query order: SHOW server_version → pg_class → pg_inherits → pg_attribute → pg_constraint → pg_index.

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
	expectEmptyConstraintsAndIndexes(mock)

	snap, _, err := p.TakeSnapshot(context.Background(), []string{"public"})
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

var (
	constraintColumns = []string{"table_oid", "name", "oid", "contype", "definition", "from_parent"}
	indexColumns      = []string{"table_oid", "name", "oid", "is_unique", "quoted_name", "index_def", "table_regclass", "from_parent"}
)

func expectEmptyConstraintsAndIndexes(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`pg_get_constraintdef`).WillReturnRows(sqlmock.NewRows(constraintColumns))
	mock.ExpectQuery(`pg_get_indexdef`).WillReturnRows(sqlmock.NewRows(indexColumns))
}

// expectTables registers the version, tables, links and (empty) columns queries for the
// given tables, each as (oid, schema, name).
func expectTables(mock sqlmock.Sqlmock, tables ...[3]string) {
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow("17.2"))
	tableRows := sqlmock.NewRows([]string{"oid", "schema", "name", "relkind"})
	for _, tb := range tables {
		tableRows.AddRow(tb[0], tb[1], tb[2], "r")
	}
	mock.ExpectQuery(`pg_class`).WillReturnRows(tableRows)
	mock.ExpectQuery(`pg_inherits`).WillReturnRows(
		sqlmock.NewRows([]string{"child_oid", "child_schema", "child_name", "parent_oid", "parent_schema", "parent_name", "is_partition"}))
	mock.ExpectQuery(`pg_attribute`).WillReturnRows(
		sqlmock.NewRows([]string{"table_oid", "attnum", "schema", "table_name", "col_name", "data_type", "not_null", "col_default"}))
}

func TestTakeSnapshotAttachesConstraintsAndIndexes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err)

	expectTables(mock, [3]string{"100", "MixedCase", "My Table"}, [3]string{"200", "public", "plain"})
	mock.ExpectQuery(`pg_get_constraintdef`).WillReturnRows(sqlmock.NewRows(constraintColumns).
		AddRow("100", "My Check", "11", "c", "CHECK (qty > 0)", false).
		AddRow("100", "My FK", "12", "f", "FOREIGN KEY (ref_id) REFERENCES other.ref(id) ON DELETE CASCADE", false).
		AddRow("100", "My PK", "13", "p", `PRIMARY KEY ("Id")`, false).
		AddRow("100", "My Unique", "14", "u", "UNIQUE (email)", false).
		AddRow("100", "inherited_chk", "15", "c", "CHECK (a > 0)", true).
		AddRow("999", "out_of_scope_pkey", "16", "p", "PRIMARY KEY (id)", false))
	mock.ExpectQuery(`pg_get_indexdef`).WillReturnRows(sqlmock.NewRows(indexColumns).
		AddRow("100", "My Idx", "21", false, `"My Idx"`,
			`CREATE INDEX "My Idx" ON "MixedCase"."My Table" USING btree (qty)`, `"MixedCase"."My Table"`, false).
		AddRow("100", "My Uidx", "22", true, `"My Uidx"`,
			`CREATE UNIQUE INDEX "My Uidx" ON "MixedCase"."My Table" USING btree (lower(email)) WHERE (email IS NOT NULL)`, `"MixedCase"."My Table"`, true).
		AddRow("999", "out_of_scope_idx", "23", false, "out_of_scope_idx",
			"CREATE INDEX out_of_scope_idx ON other.t USING btree (a)", "other.t", false))

	snap, _, err := p.TakeSnapshot(context.Background(), []string{"MixedCase", "public"})
	require.NoError(t, err)
	require.Len(t, snap.Tables, 2)

	tableRef := ObjectRef{Schema: "MixedCase", Name: "My Table"}
	scoped := func(name string) TableScopedObjectRef { return TableScopedObjectRef{Table: tableRef, Name: name} }
	mixed := snap.Tables[0]
	assert.Equal(t, &Constraint{TableScopedObjectRef: scoped("My PK"), ID: "13", Definition: `PRIMARY KEY ("Id")`}, mixed.PrimaryKey)
	assert.Equal(t, []Constraint{{TableScopedObjectRef: scoped("My Unique"), ID: "14", Definition: "UNIQUE (email)"}}, mixed.UniqueKeys)
	assert.Equal(t, []Constraint{
		{TableScopedObjectRef: scoped("My Check"), ID: "11", Definition: "CHECK (qty > 0)"},
		{TableScopedObjectRef: scoped("inherited_chk"), ID: "15", Definition: "CHECK (a > 0)", FromParent: true},
	}, mixed.Checks)
	assert.Equal(t, []Constraint{{TableScopedObjectRef: scoped("My FK"), ID: "12",
		Definition: "FOREIGN KEY (ref_id) REFERENCES other.ref(id) ON DELETE CASCADE"}}, mixed.ForeignKeys)
	assert.Equal(t, []Index{{TableScopedObjectRef: scoped("My Idx"), ID: "21", Definition: "USING btree (qty)"}}, mixed.Indexes)
	assert.Equal(t, []Index{{TableScopedObjectRef: scoped("My Uidx"), ID: "22",
		Definition: "USING btree (lower(email)) WHERE (email IS NOT NULL)", FromParent: true}}, mixed.UniqueIndexes)

	plain := snap.Tables[1]
	assert.Nil(t, plain.PrimaryKey)
	assert.Empty(t, plain.UniqueKeys)
	assert.Empty(t, plain.Checks)
	assert.Empty(t, plain.ForeignKeys)
	assert.Empty(t, plain.Indexes)
	assert.Empty(t, plain.UniqueIndexes)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTakeSnapshotTwoPrimaryKeysIsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err)

	expectTables(mock, [3]string{"100", "public", "orders"})
	mock.ExpectQuery(`pg_get_constraintdef`).WillReturnRows(sqlmock.NewRows(constraintColumns).
		AddRow("100", "orders_pkey", "1", "p", "PRIMARY KEY (id)", false).
		AddRow("100", "orders_pkey2", "2", "p", "PRIMARY KEY (id)", false))

	_, _, err = p.TakeSnapshot(context.Background(), []string{"public"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `table public.orders has two primary keys: "orders_pkey" and "orders_pkey2"`)
}

func TestTakeSnapshotUnexpectedContypeIsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p, err := newProvider(constants.POSTGRESQL, db)
	require.NoError(t, err)

	expectTables(mock, [3]string{"100", "public", "orders"})
	mock.ExpectQuery(`pg_get_constraintdef`).WillReturnRows(sqlmock.NewRows(constraintColumns).
		AddRow("100", "weird", "1", "x", "EXCLUDE USING gist (a WITH &&)", false))

	_, _, err = p.TakeSnapshot(context.Background(), []string{"public"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unexpected contype "x"`)
}

func TestStripIndexDefPrefix(t *testing.T) {
	cases := []struct {
		name      string
		indexDef  string
		quoted    string
		table     string
		isUnique  bool
		want      string
		wantError bool
	}{
		{name: "plain", indexDef: "CREATE INDEX idx ON public.orders USING btree (a)",
			quoted: "idx", table: "public.orders", want: "USING btree (a)"},
		{name: "unique", indexDef: "CREATE UNIQUE INDEX idx ON public.orders USING btree (a)",
			quoted: "idx", table: "public.orders", isUnique: true, want: "USING btree (a)"},
		{name: "on only", indexDef: "CREATE INDEX idx ON ONLY public.orders USING btree (a)",
			quoted: "idx", table: "public.orders", want: "USING btree (a)"},
		{name: "unique on only", indexDef: "CREATE UNIQUE INDEX idx ON ONLY public.orders USING btree (a)",
			quoted: "idx", table: "public.orders", isUnique: true, want: "USING btree (a)"},
		{name: "quoted spaced names", indexDef: `CREATE INDEX "My Idx" ON "Mixed Case"."My Table" USING btree (qty)`,
			quoted: `"My Idx"`, table: `"Mixed Case"."My Table"`, want: "USING btree (qty)"},
		{name: "keeps expression and predicate",
			indexDef: `CREATE UNIQUE INDEX "My Uidx" ON "MixedCase"."T" USING btree (lower(email)) WHERE (email IS NOT NULL)`,
			quoted:   `"My Uidx"`, table: `"MixedCase"."T"`, isUnique: true,
			want: "USING btree (lower(email)) WHERE (email IS NOT NULL)"},
		{name: "index name containing USING",
			indexDef: `CREATE INDEX "a USING b" ON public.t USING btree (x)`,
			quoted:   `"a USING b"`, table: "public.t", want: "USING btree (x)"},
		{name: "wrong index name", indexDef: "CREATE INDEX other ON public.orders USING btree (a)",
			quoted: "idx", table: "public.orders", wantError: true},
		{name: "wrong table", indexDef: "CREATE INDEX idx ON public.other USING btree (a)",
			quoted: "idx", table: "public.orders", wantError: true},
		{name: "unique flag disagrees", indexDef: "CREATE UNIQUE INDEX idx ON public.orders USING btree (a)",
			quoted: "idx", table: "public.orders", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stripIndexDefPrefix(tc.indexDef, tc.quoted, tc.table, tc.isUnique)
			if tc.wantError {
				require.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), "index "+tc.quoted))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
