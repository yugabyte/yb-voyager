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
	"time"

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

		saveSourceSchemaSnapshotPlaceholder(ctx, schemasnapshot.LabelExportDataFromSourceExit, schemasnapshot.ReasonError)

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

	t.Run("periodic capture persists on every tick, even for an unchanged schema (no dedup)", func(t *testing.T) {
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

		before := countPeriodicSnapshots()

		// Every periodic capture is persisted unconditionally — no dedup — so the drift
		// timeline records the source schema at each interval even when it hasn't changed.
		// The schema is NOT altered between these captures, so under the old dedup logic the
		// 2nd and 3rd would have been skipped; here all three must persist.
		//
		// Snapshot names are second-granularity ({label}_{YYYYMMDDThhmmssZ}); this test fires
		// captures back-to-back, so a >=1s wait between them avoids a UNIQUE-name collision.
		// Not a real-run concern: periodic captures are >=1 minute apart.
		const captures = 3
		for i := 0; i < captures; i++ {
			if i > 0 {
				time.Sleep(time.Second)
			}
			captureSourceSchemaSnapshot(ctx, schemasnapshot.LabelExportDataFromSourcePeriodic, "", false)
		}

		assert.Equal(t, before+captures, countPeriodicSnapshots(),
			"every periodic capture must persist a new snapshot, even with an unchanged schema")
	})

	t.Run("an expired context aborts the capture fast and falls back to a placeholder", func(t *testing.T) {
		suppressSchemaSnapshotCapture = utils.BoolStr(false)

		countExitPlaceholders := func() int {
			headers, err := schemasnapshot.ListSnapshots(metaDB)
			require.NoError(t, err)
			n := 0
			for _, h := range headers {
				if h.Label == schemasnapshot.LabelExportDataFromSourceExit && h.IsPlaceholder {
					n++
				}
			}
			return n
		}
		before := countExitPlaceholders()

		// A context whose deadline has already passed. The capture must not run a real
		// query and must not hang; with placeholderOnFailure=true (as the abnormal-exit
		// path uses) it falls back to a metadata-only marker.
		expiredCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond) // ensure the deadline has elapsed

		start := time.Now()
		captureSourceSchemaSnapshot(expiredCtx, schemasnapshot.LabelExportDataFromSourceExit, schemasnapshot.ReasonError, true)
		elapsed := time.Since(start)

		assert.Less(t, elapsed, 3*time.Second, "an expired context must abort the capture promptly, not hang")
		assert.Equal(t, before+1, countExitPlaceholders(),
			"an aborted capture with placeholderOnFailure must record exactly one exit placeholder")
	})
}
