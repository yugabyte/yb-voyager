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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const driftTestSchema = "public"

// runDetectDrift runs `schema detect-drift` against exportDir, requires it to exit 0,
// and returns the JSON report it wrote. The command exits 0 whether or not it finds
// drift, so only the report says whether it did.
func runDetectDrift(t *testing.T, pg testcontainers.TestContainer, exportDir string) schemadrift.Report {
	t.Helper()

	_, err := testutils.RunVoyagerCommand(pg, "schema detect-drift", []string{
		"--source-db-schema", driftTestSchema,
		"--export-dir", exportDir,
	}, nil, false)
	require.NoError(t, err, "detect-drift must exit 0 once it has written the report")

	jsonPath := filepath.Join(exportDir, "reports", "drift_analysis_report.json")
	raw, err := os.ReadFile(jsonPath)
	require.NoError(t, err, "failed to read %s", jsonPath)

	// Decoding into schemadrift.Report is the point. The JSON shape is contractual
	// for downstream tooling, so checking the real field names catches a break that
	// scraping the rendered text would not.
	var report schemadrift.Report
	require.NoError(t, json.Unmarshal(raw, &report),
		"report JSON must decode into schemadrift.Report")
	return report
}

// TestDetectDriftEndToEnd changes the source schema in the middle of a real
// migration, then runs `yb-voyager schema detect-drift` and asserts what it reports.
//
// This is the only test that drives the whole chain through the CLI: the capture
// hooks on export schema / export data, snapshot persistence in metaDB, the diff
// engine, the report assembler, and the renderers. Every layer has unit tests; none
// of them prove the command wires them together.
//
// The drift is deliberately one added column. The point is that the command detects
// and reports a change at all; what each change type means is pinned far
// more cheaply in schemadrift's unit tests.
func TestDetectDriftEndToEnd(t *testing.T) {
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// A dedicated container config, so this test neither shares nor outlives the
	// registry entry used by TestSchemaSnapshotCaptureHooksFireDuringRealCommands --
	// that test terminates its container in a defer, and the registry is keyed by
	// config, so a shared key would hand one test a container the other just killed.
	pg := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{ForLive: true})
	require.NoError(t, pg.Start(context.Background()), "failed to start postgres container")
	defer pg.Stop(context.Background())

	pg.ExecuteSqls(
		`CREATE TABLE public.orders(id int PRIMARY KEY, amount numeric);`,
		`CREATE TABLE public.customers(id int PRIMARY KEY, name text);`,
		// export data skips empty tables entirely, so seed a row in each to keep
		// both in the export.
		`INSERT INTO public.orders(id, amount) VALUES (1, 100);`,
		`INSERT INTO public.customers(id, name) VALUES (1, 'alice');`,
	)
	defer pg.ExecuteSqls(
		`DROP TABLE IF EXISTS public.orders;`,
		`DROP TABLE IF EXISTS public.customers;`,
	)

	t.Run("an export dir with no migration project fails without creating one", func(t *testing.T) {
		emptyDir := testutils.CreateTempExportDir()
		defer testutils.RemoveTempExportDir(emptyDir)

		_, err := testutils.RunVoyagerCommand(pg, "schema detect-drift", []string{
			"--source-db-schema", driftTestSchema,
			"--export-dir", emptyDir,
		}, nil, false)
		require.Error(t, err)
		assert.NoFileExists(t, filepath.Join(emptyDir, "metainfo", "meta.db"))
		assert.NoDirExists(t, filepath.Join(emptyDir, "schema"))
	})

	// No capture flag is passed: the export commands capture by default, and this test
	// is what shows it. Without that history the drift assertions below would fail.

	// export schema: the first capture, taken before the schema changes.
	_, err := testutils.RunVoyagerCommand(pg, "export schema", []string{
		"--source-db-schema", driftTestSchema,
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)
	require.NoError(t, err, "export schema command failed")

	t.Run("an unchanged source reports no drift", func(t *testing.T) {
		assert.Zero(t, runDetectDrift(t, pg, exportDir).Summary.ChangeCount,
			"nothing has changed yet, so the report must find no drift")
	})

	// The drift: a column the migration has not seen yet.
	pg.ExecuteSqls(`ALTER TABLE public.orders ADD COLUMN note text;`)

	// export data: start and exit captures, both taken after the ALTER, so the
	// export_schema -> export_data_start pair is where the change shows up.
	_, err = testutils.RunVoyagerCommand(pg, "export data", []string{
		"--source-db-schema", driftTestSchema,
		"--export-dir", exportDir,
		"--export-type", "snapshot-only",
		"--disable-pb", "true",
		"--yes",
	}, nil, false)
	require.NoError(t, err, "export data command failed")

	report := runDetectDrift(t, pg, exportDir)

	reportsDir := filepath.Join(exportDir, "reports")
	jsonPath := filepath.Join(reportsDir, "drift_analysis_report.json")
	htmlPath := filepath.Join(reportsDir, "drift_analysis_report.html")

	t.Run("both formats are written by default", func(t *testing.T) {
		for _, path := range []string{jsonPath, htmlPath} {
			info, err := os.Stat(path)
			require.NoError(t, err, "expected a report at %s", path)
			assert.NotZero(t, info.Size(), "%s must not be empty", filepath.Base(path))
		}
	})

	t.Run("the JSON report describes the drift", func(t *testing.T) {
		assert.Equal(t, "schema_drift", report.Report)
		assert.Contains(t, report.Comparing.Schemas, driftTestSchema)
		assert.True(t, report.Summary.LiveCompared,
			"detect-drift reads the live source, so the report must say it compared it")
		assert.Equal(t, 1, report.Summary.ChangeCount, "exactly one column was added")

		// Three stored captures: export_schema, and export data's start and exit.
		// Exactly three also pins that the live read is not persisted -- the earlier
		// detect-drift run in this test took one, and it must not have become a
		// fourth snapshot.
		assert.Equal(t, 3, report.Summary.StoredCaptureCount,
			"expected the three export captures and no persisted live read")

		// Object is the parent table and SubObject the column, so a column change
		// is addressed as (public.orders, note).
		var added *schemadrift.DriftEntry
		for i := range report.Drifts {
			d := &report.Drifts[i]
			if d.Type == schemadiff.ColumnAdded && d.Object.Name == "orders" && d.SubObject == "note" {
				added = d
				break
			}
		}
		require.NotNil(t, added,
			"expected a %s entry for public.orders.note; got diffs: %+v",
			schemadiff.ColumnAdded, report.Drifts)

		assert.Equal(t, driftTestSchema, added.Object.Schema)
		assert.NotEmpty(t, added.Severity, "every entry carries a severity")
		assert.NotEmpty(t, added.Phase, "every entry says which migration phase it happened in")
	})
}
