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

package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

// TestCaptureAgainstLivePostgres starts a real PostgreSQL container, creates a
// schema with tables and columns, and asserts that schemasnapshot.Capture
// actually reads pg_catalog and returns the expected tables/columns.
// Run with: go test -tags integration -run TestCaptureAgainstLivePostgres ./src/schemasnapshot/databases/postgres/
func TestCaptureAgainstLivePostgres(t *testing.T) {
	ctx := context.Background()

	pg := testcontainers.NewTestContainer(testcontainers.POSTGRESQL, nil)
	require.NoError(t, pg.Start(ctx), "start postgres container")
	defer pg.Terminate(ctx)

	const schema = "drift_test"
	pg.ExecuteSqls(
		`CREATE SCHEMA `+schema,
		`CREATE TABLE `+schema+`.orders (
			id        integer NOT NULL,
			customer  text,
			amount    numeric DEFAULT 0
		)`,
		`CREATE TABLE `+schema+`.events (
			id        bigint NOT NULL
		) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+schema+`.events_2026 PARTITION OF `+schema+`.events FOR VALUES FROM (1) TO (1000000)`,
		`CREATE TABLE `+schema+`.animals (id int, name text)`,
		`CREATE TABLE `+schema+`.dogs (breed text) INHERITS (`+schema+`.animals)`,
	)

	db, err := pg.GetConnection()
	require.NoError(t, err)
	defer db.Close()

	host, port, err := pg.GetHostPort()
	require.NoError(t, err)
	cfg := pg.GetConfig()

	snap, err := schemasnapshot.Capture(ctx, db, schemasnapshot.CaptureSource{
		DatabaseType: constants.POSTGRESQL,
		Host:         host,
		Port:         port,
		Database:     cfg.DBName,
		User:         cfg.User,
		Role:         "source",
	}, []string{schema})
	require.NoError(t, err, "Capture should succeed against live postgres")

	// Header stamping.
	assert.Equal(t, 1, snap.Version)
	assert.Equal(t, "postgresql", snap.DatabaseType)
	assert.True(t, snap.StableIdentity)
	assert.NotEmpty(t, snap.DatabaseVersion, "DatabaseVersion should be probed")
	assert.Equal(t, []string{schema}, snap.Schemas)

	// Tables: orders (ordinary) + events (partitioned) + events_2026 (partition child)
	//         + animals (ordinary, inheritance parent) + dogs (ordinary, inheritance child).
	byName := map[string]schemasnapshot.Table{}
	for _, tb := range snap.Tables {
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
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: schema, Name: "events_2026"}, byName["events"].PartitionChildren[0])

	// Partition linkage: events_2026 must point back to events as its parent.
	require.NotNil(t, byName["events_2026"].PartitionParent)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: schema, Name: "events"}, *byName["events_2026"].PartitionParent)

	// Partition NOT mislabeled as inheritance: events_2026 must not appear in InheritsFrom.
	assert.Empty(t, byName["events_2026"].InheritsFrom, "declarative partition must not appear as legacy inheritance child")

	// Sanity: non-partitioned orders must have no partition links.
	assert.Nil(t, byName["orders"].PartitionParent, "orders must not have a partition parent")
	assert.Empty(t, byName["orders"].PartitionChildren, "orders must not have partition children")

	// Legacy inheritance linkage: dogs must inherit from animals.
	require.Len(t, byName["dogs"].InheritsFrom, 1)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: schema, Name: "animals"}, byName["dogs"].InheritsFrom[0])
	// dogs must NOT be mislabeled as a declarative partition child.
	assert.Nil(t, byName["dogs"].PartitionParent, "legacy inheritance child must not have PartitionParent set")
	assert.Empty(t, byName["dogs"].PartitionChildren, "legacy inheritance child must not have PartitionChildren")

	// Legacy inheritance linkage: animals must have dogs as an inherited-by child.
	require.Len(t, byName["animals"].InheritedBy, 1)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: schema, Name: "dogs"}, byName["animals"].InheritedBy[0])
	// animals must NOT be mislabeled as a declarative partitioned table.
	assert.Empty(t, byName["animals"].PartitionChildren, "legacy inheritance parent must not have PartitionChildren")
	assert.Nil(t, byName["animals"].PartitionParent, "legacy inheritance parent must have nil PartitionParent")

	// Columns of orders.
	cols := map[string]schemasnapshot.Column{}
	for _, c := range snap.Columns {
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
}
