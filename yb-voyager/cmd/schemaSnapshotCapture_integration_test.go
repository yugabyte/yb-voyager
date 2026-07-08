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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// TestSchemaSnapshotCaptureIntegration exercises captureSourceSchemaSnapshot and
// saveSourceSchemaSnapshotPlaceholder end-to-end against a REAL PostgreSQL
// database and a REAL metaDB.
//
// It reuses the postgres container + srcdb.Source already spun up by this
// package's TestMain (see cmd/exportData_test.go) rather than starting a
// second container, mirroring the exact idiom used by the other integration
// tests in this package (setupPostgreDBAndExportDependencies /
// TestTableListInFreshRunOfExportDataBasicPG etc.): connect
// testPostgresSource.DB(), then copy *testPostgresSource.Source into the
// package-level `source` global so captureSourceSchemaSnapshot's
// `source.DB().(*srcdb.PostgreSQL)` type assertion resolves to the same,
// already-connected handle.
//
// Run with: go test -tags integration -run TestSchemaSnapshotCaptureIntegration ./cmd/...
func TestSchemaSnapshotCaptureIntegration(t *testing.T) {
	ctx := context.Background()

	seedSqls := []string{
		`CREATE TABLE public.orders(id int primary key, amount numeric)`,
		`CREATE TABLE public.customers(id int primary key, name text)`,
	}
	cleanupSqls := []string{
		`DROP TABLE IF EXISTS public.orders`,
		`DROP TABLE IF EXISTS public.customers`,
	}

	testPostgresSource.ExecuteSqls(seedSqls...)
	t.Cleanup(func() { testPostgresSource.ExecuteSqls(cleanupSqls...) })

	sqlname.SourceDBType = testPostgresSource.DBType
	testPostgresSource.Schemas = sqlname.ParseIdentifiersFromString(constants.POSTGRESQL, "public", "|")

	err := testPostgresSource.DB().Connect()
	require.NoError(t, err, "connect to postgres source")
	t.Cleanup(func() { testPostgresSource.DB().Disconnect() })

	testExportDir, err := os.MkdirTemp("/tmp", "schemasnapshot-capture-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(testExportDir) })

	mdb := initMetaDB(testExportDir)

	// Populate the package globals consumed by captureSourceSchemaSnapshot /
	// saveSourceSchemaSnapshotPlaceholder. `source` is a value copy of
	// *testPostgresSource.Source taken AFTER Connect(), so source.DB() returns
	// the same already-connected *srcdb.PostgreSQL handle.
	source = *testPostgresSource.Source
	metaDB = mdb
	exporterRole = SOURCE_DB_EXPORTER_ROLE
	suppressSchemaSnapshotCapture = utils.BoolStr(false)

	t.Cleanup(func() {
		source = srcdb.Source{}
		metaDB = nil
		exporterRole = SOURCE_DB_EXPORTER_ROLE
		suppressSchemaSnapshotCapture = utils.BoolStr(false)
	})

	require.Equal(t, []string{"public"}, source.GetSchemaList(), "source must be scoped to the public schema")

	t.Run("happy path captures and persists a real snapshot", func(t *testing.T) {
		suppressSchemaSnapshotCapture = utils.BoolStr(false)

		captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportSchema, "", true)

		headers, err := schemasnapshot.ListSnapshots(metaDB)
		require.NoError(t, err)
		require.Len(t, headers, 1, "exactly one snapshot must have been persisted")

		h := headers[0]
		assert.Equal(t, schemasnapshot.LabelExportSchema, h.Label)
		assert.False(t, h.IsPlaceholder, "a successful capture must not be a placeholder")

		content, err := schemasnapshot.LoadSnapshotByName(metaDB, h.Name())
		require.NoError(t, err)
		require.NotNil(t, content)

		tableNames := make(map[string]bool, len(content.Tables))
		for _, tb := range content.Tables {
			tableNames[tb.Name] = true
		}
		assert.True(t, tableNames["orders"], "captured schema content must include the seeded 'orders' table")
		assert.True(t, tableNames["customers"], "captured schema content must include the seeded 'customers' table")
	})

	t.Run("suppression is honored and results in a no-op", func(t *testing.T) {
		before, err := schemasnapshot.ListSnapshots(metaDB)
		require.NoError(t, err)

		suppressSchemaSnapshotCapture = utils.BoolStr(true)
		t.Cleanup(func() { suppressSchemaSnapshotCapture = utils.BoolStr(false) })

		captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", true)

		after, err := schemasnapshot.ListSnapshots(metaDB)
		require.NoError(t, err)
		assert.Equal(t, len(before), len(after), "a suppressed capture must not add a snapshot row")
	})

	t.Run("placeholder writes a metadata-only marker", func(t *testing.T) {
		suppressSchemaSnapshotCapture = utils.BoolStr(false)

		before, err := schemasnapshot.ListSnapshots(metaDB)
		require.NoError(t, err)

		saveSourceSchemaSnapshotPlaceholder(schemasnapshot.LabelExportDataFromSourceExit, schemasnapshot.ReasonError)

		after, err := schemasnapshot.ListSnapshots(metaDB)
		require.NoError(t, err)
		require.Len(t, after, len(before)+1, "the placeholder must add exactly one snapshot row")

		var placeholder *schemasnapshot.SnapshotHeader
		for i := range after {
			if after[i].Label == schemasnapshot.LabelExportDataFromSourceExit {
				placeholder = &after[i]
				break
			}
		}
		require.NotNil(t, placeholder, "a header with the exit label must be present")
		assert.True(t, placeholder.IsPlaceholder, "the marker must be flagged as a placeholder")
		assert.Equal(t, schemasnapshot.ReasonError, placeholder.Reason)
	})

	t.Run("periodic capture dedups against an unchanged schema and persists on a real change", func(t *testing.T) {
		suppressSchemaSnapshotCapture = utils.BoolStr(false)

		countPeriodicSnapshots := func() int {
			headers, err := schemasnapshot.ListSnapshots(metaDB)
			require.NoError(t, err)
			n := 0
			for _, h := range headers {
				if h.Label == schemasnapshot.LabelExportDataFromSourcePeriodic {
					n++
				}
			}
			return n
		}

		// Force the schema to differ from whatever earlier subtests in this file may have
		// already captured (e.g. the "happy path" export_schema snapshot), so the first
		// periodic capture below is guaranteed to be a real change and persist.
		testPostgresSource.ExecuteSqls(`ALTER TABLE public.orders ADD COLUMN dedup_marker_1 numeric`)
		t.Cleanup(func() {
			testPostgresSource.ExecuteSqls(`ALTER TABLE public.orders DROP COLUMN IF EXISTS dedup_marker_1`)
		})

		before := countPeriodicSnapshots()

		// First periodic capture after the schema change above: must persist.
		captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false)
		afterFirst := countPeriodicSnapshots()
		require.Equal(t, before+1, afterFirst, "a periodic capture following a real schema change must persist")

		// Second periodic capture with no further schema change: must be deduped (no new row).
		captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false)
		afterSecond := countPeriodicSnapshots()
		assert.Equal(t, afterFirst, afterSecond, "an unchanged schema must not add a second periodic snapshot")

		// Alter the schema again, then capture again: must persist a new row.
		testPostgresSource.ExecuteSqls(`ALTER TABLE public.orders ADD COLUMN dedup_marker_2 numeric`)
		t.Cleanup(func() {
			testPostgresSource.ExecuteSqls(`ALTER TABLE public.orders DROP COLUMN IF EXISTS dedup_marker_2`)
		})

		captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false)
		afterSchemaChange := countPeriodicSnapshots()
		assert.Equal(t, afterSecond+1, afterSchemaChange, "a real schema change must persist a new periodic snapshot")
	})
}
