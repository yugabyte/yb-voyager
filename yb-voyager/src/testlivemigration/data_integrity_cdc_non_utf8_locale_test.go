//go:build integration_live_migration

package testlivemigration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestDataIntegrityCDCNonUTF8LocaleCorruptsUnicode pins mechanism M5 (value transformation):
// export data runs under a non-UTF-8 locale (LANG/LC_ALL=C, the default in many containers and
// service units). On JDK 17 the Debezium exporter's queue writer (QueueSegment, new FileWriter)
// uses the platform charset, so every non-ASCII character in a streamed change is written to the
// queue as '?'. The importer applies the corrupted value without any error.
// Correct behaviour: the target matches the source regardless of the exporter's locale.
func TestDataIntegrityCDCNonUTF8LocaleCorruptsUnicode(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:             ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: "di_cdc_non_utf8_locale"},
		TargetDB:             ContainerConfig{Type: "yugabytedb", DatabaseName: "di_cdc_non_utf8_locale"},
		SchemaNames:          []string{"diu"},
		SchemaSQL:            []string{`CREATE SCHEMA IF NOT EXISTS diu;`, `CREATE TABLE diu.t (id int PRIMARY KEY, s text);`},
		SourceSetupSchemaSQL: []string{`ALTER TABLE diu.t REPLICA IDENTITY FULL;`},
		InitialDataSQL:       []string{`INSERT INTO diu.t VALUES (1, 'plain');`},
		SourceDeltaSQL:       []string{`INSERT INTO diu.t VALUES (2, 'café');`},
		CleanupSQL:           []string{`DROP SCHEMA IF EXISTS diu CASCADE;`},
	})
	defer lm.Cleanup()

	testutils.FatalIfError(t, lm.SetupContainers(context.Background()), "setup containers")
	testutils.FatalIfError(t, lm.SetupSchema(), "setup schema")
	testutils.FatalIfError(t, lm.StartExportDataWithEnv(true, nil, []string{"LANG=C", "LC_ALL=C"}), "start export data")
	testutils.FatalIfError(t, lm.StartImportData(true, map[string]string{"--cdc-partition-key": "auto"}), "start import data")
	testutils.FatalIfError(t, lm.WaitForSnapshotComplete(map[string]int64{`"diu"."t"`: 1}, 240), "snapshot")
	testutils.FatalIfError(t, lm.ExecuteSourceDelta(), "source delta")
	testutils.FatalIfError(t, lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"diu"."t"`: {Inserts: 1},
	}, 120, 5), "streaming")

	require.False(t, lm.GetImportRunner().IsStopped(), "importer should still be running")
	require.NoError(t, lm.ValidateDataConsistency([]string{"diu.t"}, "id"),
		"target diverged from source: non-ASCII text streamed under a non-UTF-8 locale was written as '?'")
}
