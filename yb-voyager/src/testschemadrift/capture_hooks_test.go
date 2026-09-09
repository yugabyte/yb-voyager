//go:build integration_voyager_command

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
package testschemadrift

import (
	"context"
	"testing"

	// Registers the sqlite3 driver the metaDB opens with. Package cmd pulled this in
	// transitively; a standalone test package has to ask for it.
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// openMetaDB opens the on-disk metaDB a voyager command just wrote.
func openMetaDB(t *testing.T, exportDir string) *metadb.MetaDB {
	t.Helper()
	mdb, err := metadb.NewMetaDB(exportDir)
	require.NoError(t, err, "open metaDB at %s", exportDir)
	return mdb
}

// TestSchemaSnapshotCaptureHooksFireDuringRealCommands proves (against a real
// PostgreSQL testcontainer and the real yb-voyager binary) that the schema-snapshot
// capture hooks wired into `export schema` and `export data` actually run and persist
// the expected rows to the on-disk metaDB.
//
// It runs export schema then offline (pg_dump-driven, non-BETA_FAST_DATA_EXPORT) export
// data against the same export dir/container, checking metaDB state after each step.
//
// Black-box on purpose: it drives the binary and reads the metaDB through the exported
// metadb/schemasnapshot API, so it needs nothing from package cmd.
func TestSchemaSnapshotCaptureHooksFireDuringRealCommands(t *testing.T) {
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	require.NoError(t, err, "failed to start postgres container")
	defer postgresContainer.Stop(context.Background())

	postgresContainer.ExecuteSqls(
		`CREATE TABLE public.orders(id int primary key, amount numeric);`,
		`CREATE TABLE public.customers(id int primary key, name text);`,
		// export data skips empty tables entirely (never reaching the capture hooks),
		// so seed a row in each to keep both tables in the export.
		`INSERT INTO public.customers(id, name) VALUES (1, 'alice');`,
		`INSERT INTO public.orders(id, amount) VALUES (1, 100);`,
	)
	defer postgresContainer.ExecuteSqls(
		`DROP TABLE IF EXISTS public.orders;`,
		`DROP TABLE IF EXISTS public.customers;`,
	)

	t.Run("export schema captures a snapshot", func(t *testing.T) {
		_, err := testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
			"--source-db-schema", "public",
			"--export-dir", exportDir,
			"--disable-schema-snapshot-capture", "false",
			"--yes",
		}, nil, false)
		require.NoError(t, err, "export schema command failed")

		mdb := openMetaDB(t, exportDir)
		headers, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err, "failed to list snapshots after export schema")
		require.Len(t, headers, 1, "expected exactly one snapshot header after export schema")

		header := headers[0]
		assert.Equal(t, schemasnapshot.LabelExportSchema, header.Label)
		assert.False(t, header.IsPlaceholder, "export schema snapshot should not be a placeholder")

		// Content is deliberately not re-checked here. This test's job is that the
		// command invoked the hook at all; what a capture contains is covered by the
		// capture tests in src/schemasnapshot, far more cheaply than through the binary.
		// Loading it is still worth it as a smoke check that a real snapshot was stored
		// rather than an empty or placeholder row.
		content, err := schemasnapshot.LoadSnapshotByName(mdb, header.Name())
		require.NoError(t, err, "failed to load export schema snapshot content")
		assert.NotEmpty(t, content.Tables, "a stored snapshot must carry captured tables")
	})

	t.Run("offline export data captures start and exit snapshots", func(t *testing.T) {
		_, err := testutils.RunVoyagerCommand(postgresContainer, "export data", []string{
			"--source-db-schema", "public",
			"--export-dir", exportDir,
			"--export-type", "snapshot-only",
			"--disable-pb", "true",
			"--disable-schema-snapshot-capture", "false",
			"--yes",
		}, nil, false)
		require.NoError(t, err, "export data command failed")

		mdb := openMetaDB(t, exportDir)
		headers, err := schemasnapshot.ListSnapshots(mdb)
		require.NoError(t, err, "failed to list snapshots after export data")

		var startHeader, exitHeader *schemasnapshot.SnapshotHeader
		for i := range headers {
			h := headers[i]
			switch {
			case h.Label == schemasnapshot.LabelExportDataFromSourceStart:
				startHeader = &h
			case h.Label == schemasnapshot.LabelExportDataFromSourceExit:
				exitHeader = &h
			}
		}

		require.NotNil(t, startHeader, "expected an %s header after export data; got headers: %+v", schemasnapshot.LabelExportDataFromSourceStart, headers)
		assert.Equal(t, schemasnapshot.ReasonInitial, startHeader.Reason)

		require.NotNil(t, exitHeader, "expected an %s header after export data; got headers: %+v", schemasnapshot.LabelExportDataFromSourceExit, headers)
		assert.Equal(t, schemasnapshot.ReasonComplete, exitHeader.Reason)
		assert.False(t, exitHeader.IsPlaceholder, "export data exit snapshot should not be a placeholder")
	})
}
