//go:build integration

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

package schemasnapshot_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

const testSchema = "drift_test"

// startCaptureTestDB starts a postgres container, creates the canonical test
// schema (the full set: ordinary/partitioned/foreign tables, columns, a dropped
// column, multi-level partitioning, single + multiple inheritance), and returns
// a live connection, display coordinates (DBMetadata), and a cleanup func.
//
// cfg is forwarded to NewTestContainer so callers can control the registry key
// (pass &testcontainers.ContainerConfig{ForLive: true} to get a distinct key
// and avoid singleton collisions when two tests run in the same process).
func startCaptureTestDB(t *testing.T, cfg *testcontainers.ContainerConfig) (*sql.DB, schemasnapshot.DBMetadata, func()) {
	t.Helper()
	ctx := context.Background()

	pg := testcontainers.NewTestContainer(testcontainers.POSTGRESQL, cfg)
	require.NoError(t, pg.Start(ctx), "start postgres container")

	pg.ExecuteSqls(
		`CREATE SCHEMA `+testSchema,
		`CREATE TABLE `+testSchema+`.orders (
			id        integer NOT NULL,
			customer  text,
			amount    numeric DEFAULT 0
		)`,
		`CREATE TABLE `+testSchema+`.events (
			id        bigint NOT NULL
		) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+testSchema+`.events_2026 PARTITION OF `+testSchema+`.events FOR VALUES FROM (1) TO (1000000)`,
		`CREATE TABLE `+testSchema+`.animals (id int, name text)`,
		`CREATE TABLE `+testSchema+`.dogs (breed text) INHERITS (`+testSchema+`.animals)`,

		// Foreign table (relkind 'f').
		`CREATE EXTENSION IF NOT EXISTS postgres_fdw`,
		`CREATE SERVER drift_fdw FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host 'localhost', dbname 'postgres')`,
		`CREATE FOREIGN TABLE `+testSchema+`.remote_accounts (id int, name text) SERVER drift_fdw`,

		// Dropped column: create with 3 cols, drop the middle one.
		`CREATE TABLE `+testSchema+`.dropcol (a int, b text, c int)`,
		`ALTER TABLE `+testSchema+`.dropcol DROP COLUMN b`,

		// Multi-level partitioning: regions → regions_lo → regions_lo_a.
		`CREATE TABLE `+testSchema+`.regions (id int, code text) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+testSchema+`.regions_lo PARTITION OF `+testSchema+`.regions FOR VALUES FROM (0) TO (100) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+testSchema+`.regions_lo_a PARTITION OF `+testSchema+`.regions_lo FOR VALUES FROM (0) TO (50)`,

		// Multiple inheritance: c12 inherits from both p1 and p2.
		`CREATE TABLE `+testSchema+`.p1 (c1 int)`,
		`CREATE TABLE `+testSchema+`.p2 (c2 text)`,
		`CREATE TABLE `+testSchema+`.c12 (own int) INHERITS (`+testSchema+`.p1, `+testSchema+`.p2)`,
	)

	db, err := pg.GetConnection()
	require.NoError(t, err)

	host, port, err := pg.GetHostPort()
	require.NoError(t, err)
	pgCfg := pg.GetConfig()

	// DBMetadata now contains only display coordinates (no DatabaseType or Side).
	dbMeta := schemasnapshot.DBMetadata{
		Host:     host,
		Port:     port,
		Database: pgCfg.DBName,
		User:     pgCfg.User,
	}

	cleanup := func() {
		db.Close()
		pg.Terminate(ctx)
	}
	return db, dbMeta, cleanup
}

// newIntegrationTestMetaDB creates a MetaDB backed by a fresh SQLite file in a temp dir.
func newIntegrationTestMetaDB(t *testing.T) *metadb.MetaDB {
	t.Helper()
	dir := t.TempDir()
	metainfoDir := filepath.Join(dir, "metainfo")
	require.NoError(t, os.MkdirAll(metainfoDir, 0o755))
	f, err := os.Create(filepath.Join(metainfoDir, "meta.db"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mdb, err := metadb.NewMetaDB(dir)
	require.NoError(t, err)
	return mdb
}

// TestCaptureAgainstLivePostgres starts a real PostgreSQL container, creates a
// schema with tables and columns, and asserts that schemasnapshot.Capture
// actually reads pg_catalog and returns the expected tables/columns.
// Run with: go test -tags integration -run TestCaptureAgainstLivePostgres ./src/schemasnapshot/
func TestCaptureAgainstLivePostgres(t *testing.T) {
	ctx := context.Background()

	db, dbMeta, cleanup := startCaptureTestDB(t, nil)
	defer cleanup()

	snap, err := schemasnapshot.Capture(ctx, db, schemasnapshot.CaptureParams{
		DatabaseType: constants.POSTGRESQL,
		Side:         schemasnapshot.SideSource,
		DBMetadata:   dbMeta,
		Schemas:      []string{testSchema},
		Label:        schemasnapshot.LabelExportSchema,
	})
	require.NoError(t, err, "Capture should succeed against live postgres")

	// Schema content fields.
	assert.Equal(t, 1, snap.Content.Version)
	assert.Equal(t, "postgresql", snap.Content.DatabaseType)

	// Header fields.
	assert.NotEmpty(t, snap.Header.DatabaseVersion, "DatabaseVersion should be probed")
	assert.Equal(t, []string{testSchema}, snap.Header.Schemas)
	assert.False(t, snap.Header.IsPlaceholder)

	// Tables: orders (ordinary) + events (partitioned) + events_2026 (partition child)
	//         + animals (ordinary, inheritance parent) + dogs (ordinary, inheritance child).
	byName := map[string]schemasnapshot.Table{}
	for _, tb := range snap.Content.Tables {
		byName[tb.Name] = tb
	}
	require.Contains(t, byName, "orders")
	require.Contains(t, byName, "events")
	require.Contains(t, byName, "events_2026")
	require.Contains(t, byName, "animals")
	require.Contains(t, byName, "dogs")
	assert.NotEmpty(t, byName["orders"].ID, "table OID should be populated")
	assert.Equal(t, schemasnapshot.TableKindOrdinary, byName["orders"].Kind)
	assert.Equal(t, schemasnapshot.TableKindPartitioned, byName["events"].Kind)
	// events_2026 is a declarative partition child — relkind is 'r' (ordinary).
	assert.Equal(t, schemasnapshot.TableKindOrdinary, byName["events_2026"].Kind)

	// Partition linkage: events must have events_2026 as a child.
	require.Len(t, byName["events"].PartitionChildren, 1)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: testSchema, Name: "events_2026"}, byName["events"].PartitionChildren[0])

	// Partition linkage: events_2026 must point back to events as its parent.
	require.NotNil(t, byName["events_2026"].PartitionParent)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: testSchema, Name: "events"}, *byName["events_2026"].PartitionParent)

	// Partition NOT mislabeled as inheritance: events_2026 must not appear in InheritsFrom.
	assert.Empty(t, byName["events_2026"].InheritsFrom, "declarative partition must not appear as legacy inheritance child")

	// Sanity: non-partitioned orders must have no partition links.
	assert.Nil(t, byName["orders"].PartitionParent, "orders must not have a partition parent")
	assert.Empty(t, byName["orders"].PartitionChildren, "orders must not have partition children")

	// Legacy inheritance linkage: dogs must inherit from animals.
	require.Len(t, byName["dogs"].InheritsFrom, 1)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: testSchema, Name: "animals"}, byName["dogs"].InheritsFrom[0])
	// dogs must NOT be mislabeled as a declarative partition child.
	assert.Nil(t, byName["dogs"].PartitionParent, "legacy inheritance child must not have PartitionParent set")
	assert.Empty(t, byName["dogs"].PartitionChildren, "legacy inheritance child must not have PartitionChildren")

	// Legacy inheritance linkage: animals must have dogs as an inherited-by child.
	require.Len(t, byName["animals"].InheritedBy, 1)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: testSchema, Name: "dogs"}, byName["animals"].InheritedBy[0])
	// animals must NOT be mislabeled as a declarative partitioned table.
	assert.Empty(t, byName["animals"].PartitionChildren, "legacy inheritance parent must not have PartitionChildren")
	assert.Nil(t, byName["animals"].PartitionParent, "legacy inheritance parent must have nil PartitionParent")

	// Columns of orders.
	cols := map[string]schemasnapshot.Column{}
	for _, c := range snap.Content.Columns {
		if c.Table.Name == "orders" {
			cols[c.Name] = c
		}
	}
	require.Contains(t, cols, "id")
	require.Contains(t, cols, "customer")
	require.Contains(t, cols, "amount")
	assert.Equal(t, "integer", cols["id"].DataType)
	assert.True(t, cols["id"].NotNull)
	assert.False(t, cols["customer"].NotNull)
	assert.NotEmpty(t, cols["amount"].Default, "amount has a DEFAULT 0")
	assert.NotEmpty(t, cols["id"].ID, "column ID should be {tableOID}:{attnum}")

	// ── Foreign table ─────────────────────────────────────────────────────────
	require.Contains(t, byName, "remote_accounts", "foreign table must appear in Tables")
	assert.Equal(t, schemasnapshot.TableKindForeign, byName["remote_accounts"].Kind,
		"remote_accounts relkind 'f' must map to TableKindForeign")

	remoteAcctCols := map[string]schemasnapshot.Column{}
	for _, c := range snap.Content.Columns {
		if c.Table.Name == "remote_accounts" {
			remoteAcctCols[c.Name] = c
		}
	}
	assert.Contains(t, remoteAcctCols, "id", "remote_accounts column 'id' must be captured")
	assert.Contains(t, remoteAcctCols, "name", "remote_accounts column 'name' must be captured")

	// ── Dropped column ────────────────────────────────────────────────────────
	require.Contains(t, byName, "dropcol")
	dropcolCols := map[string]schemasnapshot.Column{}
	for _, c := range snap.Content.Columns {
		if c.Table.Name == "dropcol" {
			dropcolCols[c.Name] = c
		}
	}
	assert.Len(t, dropcolCols, 2, "dropcol must have exactly 2 columns after DROP COLUMN b")
	assert.NotContains(t, dropcolCols, "b", "dropped column 'b' must not appear in snapshot")
	require.Contains(t, dropcolCols, "a")
	require.Contains(t, dropcolCols, "c")
	// attnum gaps must be preserved: a→1, b was 2 (dropped), c keeps attnum 3.
	assert.True(t, len(dropcolCols["a"].ID) > 0 && dropcolCols["a"].ID[len(dropcolCols["a"].ID)-1] == '1',
		"column 'a' ID must end with ':1' (attnum 1), got %s", dropcolCols["a"].ID)
	assert.True(t, len(dropcolCols["c"].ID) > 0 && dropcolCols["c"].ID[len(dropcolCols["c"].ID)-1] == '3',
		"column 'c' ID must end with ':3' (attnum 3, gap preserved), got %s", dropcolCols["c"].ID)

	// ── Multi-level partitioning ───────────────────────────────────────────────
	require.Contains(t, byName, "regions")
	require.Contains(t, byName, "regions_lo")
	require.Contains(t, byName, "regions_lo_a")

	// regions is a partitioned parent; regions_lo is its direct child.
	regionsRef := schemasnapshot.ObjectRef{Schema: testSchema, Name: "regions"}
	regionsLoRef := schemasnapshot.ObjectRef{Schema: testSchema, Name: "regions_lo"}
	regionsLoARef := schemasnapshot.ObjectRef{Schema: testSchema, Name: "regions_lo_a"}

	require.Len(t, byName["regions"].PartitionChildren, 1,
		"regions must have exactly 1 partition child (regions_lo)")
	assert.Equal(t, regionsLoRef, byName["regions"].PartitionChildren[0])

	// regions_lo is BOTH a child of regions AND a partitioned parent of regions_lo_a.
	require.NotNil(t, byName["regions_lo"].PartitionParent,
		"regions_lo must have a PartitionParent set")
	assert.Equal(t, regionsRef, *byName["regions_lo"].PartitionParent)
	require.Len(t, byName["regions_lo"].PartitionChildren, 1,
		"regions_lo must have exactly 1 partition child (regions_lo_a)")
	assert.Equal(t, regionsLoARef, byName["regions_lo"].PartitionChildren[0])

	// regions_lo_a is a leaf child of regions_lo.
	require.NotNil(t, byName["regions_lo_a"].PartitionParent,
		"regions_lo_a must have a PartitionParent set")
	assert.Equal(t, regionsLoRef, *byName["regions_lo_a"].PartitionParent)
	assert.Empty(t, byName["regions_lo_a"].PartitionChildren,
		"regions_lo_a is a leaf partition, must have no children")

	// ── Multiple inheritance ──────────────────────────────────────────────────
	require.Contains(t, byName, "p1")
	require.Contains(t, byName, "p2")
	require.Contains(t, byName, "c12")

	p1Ref := schemasnapshot.ObjectRef{Schema: testSchema, Name: "p1"}
	p2Ref := schemasnapshot.ObjectRef{Schema: testSchema, Name: "p2"}
	c12Ref := schemasnapshot.ObjectRef{Schema: testSchema, Name: "c12"}

	// c12 inherits from both p1 and p2.
	require.Len(t, byName["c12"].InheritsFrom, 2,
		"c12 must inherit from exactly 2 parents (p1, p2)")
	assert.Contains(t, byName["c12"].InheritsFrom, p1Ref)
	assert.Contains(t, byName["c12"].InheritsFrom, p2Ref)

	// p1 and p2 must each list c12 in InheritedBy.
	require.Len(t, byName["p1"].InheritedBy, 1)
	assert.Equal(t, c12Ref, byName["p1"].InheritedBy[0])
	require.Len(t, byName["p2"].InheritedBy, 1)
	assert.Equal(t, c12Ref, byName["p2"].InheritedBy[0])
}

// TestCapturePersistRoundTrip performs a full Capture → SaveSnapshot →
// ListSnapshots → LoadSnapshotByName round-trip against a real PostgreSQL
// container and a real in-process SQLite meta.db.
//
// Run with:
//
//	go test -tags integration -run TestCapturePersistRoundTrip ./src/schemasnapshot/
func TestCapturePersistRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Use a dedicated container config (ForLive=true creates a "postgresql-live-17"
	// registry key) so this test never collides with TestCaptureAgainstLivePostgres,
	// which terminates its container in a defer and would leave a dead entry in the
	// shared singleton registry.
	db, dbMeta, cleanup := startCaptureTestDB(t, &testcontainers.ContainerConfig{ForLive: true})
	defer cleanup()

	// Step 1: Capture.
	snap, err := schemasnapshot.Capture(ctx, db, schemasnapshot.CaptureParams{
		DatabaseType: constants.POSTGRESQL,
		Side:         schemasnapshot.SideSource,
		DBMetadata:   dbMeta,
		Schemas:      []string{testSchema},
		Label:        schemasnapshot.LabelExportSchema,
	})
	require.NoError(t, err, "Capture must succeed")
	require.NotEmpty(t, snap.Content.Tables, "captured Tables must be non-empty")
	require.NotEmpty(t, snap.Content.Columns, "captured Columns must be non-empty")

	// Verify partition and inheritance fields are non-empty before persisting.
	tablesByName := make(map[string]schemasnapshot.Table, len(snap.Content.Tables))
	for _, tb := range snap.Content.Tables {
		tablesByName[tb.Name] = tb
	}
	require.Contains(t, tablesByName, "events")
	require.NotEmpty(t, tablesByName["events"].PartitionChildren, "events must have partition children")
	require.Contains(t, tablesByName, "events_2026")
	require.NotNil(t, tablesByName["events_2026"].PartitionParent, "events_2026 must have a PartitionParent")
	require.Contains(t, tablesByName, "animals")
	require.NotEmpty(t, tablesByName["animals"].InheritedBy, "animals must have InheritedBy")
	require.Contains(t, tablesByName, "dogs")
	require.NotEmpty(t, tablesByName["dogs"].InheritsFrom, "dogs must have InheritsFrom")

	// Step 2: Create a temp meta.db.
	mdb := newIntegrationTestMetaDB(t)

	// Step 3: Persist. Capture already set the header; override Label/Reason to
	// use a label that carries a reason (export_data_from_source_exit + cutover).
	snap.Header.Label = schemasnapshot.LabelExportDataFromSourceExit
	snap.Header.Reason = schemasnapshot.ReasonCutover
	name, err := schemasnapshot.SaveSnapshot(mdb, snap)
	require.NoError(t, err, "SaveSnapshot must succeed")
	assert.NotEmpty(t, name)

	// Step 4: Verify the persisted row via ListSnapshots.
	metas, err := schemasnapshot.ListSnapshots(mdb)
	require.NoError(t, err)
	require.Len(t, metas, 1)

	meta := metas[0]
	assert.Equal(t, name, meta.Name())
	assert.Equal(t, schemasnapshot.LabelExportDataFromSourceExit, meta.Label)
	assert.Equal(t, "cutover", meta.Reason)
	assert.Equal(t, "source", meta.Side)
	assert.Equal(t, []string{testSchema}, meta.Schemas)
	assert.NotEmpty(t, meta.DatabaseVersion, "DatabaseVersion must be non-empty")
	assert.False(t, meta.IsPlaceholder, "real snapshot must not be a placeholder")

	// Step 5: Load by name and assert round-trip fidelity.
	loaded, err := schemasnapshot.LoadSnapshotByName(mdb, name)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// The blob (SnapshotContent) carries Version, DatabaseType, DBMetadata, Tables, Columns.
	assert.Equal(t, snap.Content.Version, loaded.Version)
	assert.Equal(t, snap.Content.DatabaseType, loaded.DatabaseType)
	assert.Equal(t, snap.Content.DBMetadata, loaded.DBMetadata)

	// Tables and Columns survive JSON serialization including ObjectRef slices.
	assert.Equal(t, snap.Content.Tables, loaded.Tables,
		"Tables must survive JSON round-trip (including partition/inheritance ObjectRef slices)")
	assert.Equal(t, snap.Content.Columns, loaded.Columns,
		"Columns must survive JSON round-trip")

	// Spot-check that partition + inheritance wiring survived serialization.
	loadedByName := make(map[string]schemasnapshot.Table, len(loaded.Tables))
	for _, tb := range loaded.Tables {
		loadedByName[tb.Name] = tb
	}

	require.Contains(t, loadedByName, "events")
	assert.NotEmpty(t, loadedByName["events"].PartitionChildren,
		"events.PartitionChildren must survive JSON round-trip")

	require.Contains(t, loadedByName, "events_2026")
	assert.NotNil(t, loadedByName["events_2026"].PartitionParent,
		"events_2026.PartitionParent must survive JSON round-trip")

	require.Contains(t, loadedByName, "animals")
	assert.NotEmpty(t, loadedByName["animals"].InheritedBy,
		"animals.InheritedBy must survive JSON round-trip")

	require.Contains(t, loadedByName, "dogs")
	assert.NotEmpty(t, loadedByName["dogs"].InheritsFrom,
		"dogs.InheritsFrom must survive JSON round-trip")
}
