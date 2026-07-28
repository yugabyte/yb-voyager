//go:build unit

package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metrics"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func TestResolveMetricsPort(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		override       int
		legacyProfile  bool
		wantPort       int
		wantEnabled    bool
		failureMessage string
	}{
		{
			name:           "port 0 with no legacy fallback disables metrics",
			role:           "target_db_importer",
			override:       0,
			legacyProfile:  false,
			wantPort:       0,
			wantEnabled:    false,
			failureMessage: "port 0 with no legacy fallback must disable metrics",
		},
		{
			name:           "explicit port enables metrics",
			role:           "target_db_importer",
			override:       9200,
			legacyProfile:  false,
			wantPort:       9200,
			wantEnabled:    true,
			failureMessage: "explicit port must enable",
		},
		{
			name:           "explicit port wins over legacy fallback",
			role:           "target_db_importer",
			override:       9200,
			legacyProfile:  true,
			wantPort:       9200,
			wantEnabled:    true,
			failureMessage: "explicit port must win over legacy fallback",
		},
		{
			name:           "legacy fallback uses target_db_importer default port",
			role:           "target_db_importer",
			override:       0,
			legacyProfile:  true,
			wantPort:       9101,
			wantEnabled:    true,
			failureMessage: "legacy --profile fallback must use role default port",
		},
		{
			name:           "legacy fallback uses import_file default port",
			role:           "import_file",
			override:       0,
			legacyProfile:  true,
			wantPort:       9102,
			wantEnabled:    true,
			failureMessage: "legacy --profile fallback must use role default port",
		},
		{
			name:           "legacy fallback has no default for export roles",
			role:           "source_db_exporter",
			override:       0,
			legacyProfile:  true,
			wantPort:       0,
			wantEnabled:    false,
			failureMessage: "legacy --profile fallback has no default for export roles",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, on := resolveMetricsPort(tc.role, tc.override, tc.legacyProfile)
			if on != tc.wantEnabled || p != tc.wantPort {
				t.Fatalf("%s; got (%d,%t), want (%d,%t)", tc.failureMessage, p, on, tc.wantPort, tc.wantEnabled)
			}
		})
	}
}

func makeTasksForTest(n int) []*ImportFileTask {
	tasks := make([]*ImportFileTask, n)
	for i := 0; i < n; i++ {
		obj := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", fmt.Sprintf("table_%d", i))
		tup := sqlname.NameTuple{CurrentName: obj, SourceName: obj, TargetName: obj}
		tasks[i] = &ImportFileTask{
			ID:           i,
			FilePath:     fmt.Sprintf("/tmp/table_%d.sql", i),
			TableNameTup: tup,
			RowCount:     100,
		}
	}
	return tasks
}

// TestInitialImportMetricsUsesAllTasks guards against createInitialImportDataTableMetrics
// under-reporting yb_voyager_import_data_snapshot_tables_total on resume, when only the
// not-yet-imported (pending) tasks would otherwise be counted instead of all tasks.
func TestInitialImportMetricsUsesAllTasks(t *testing.T) {
	prevRole := importerRole
	importerRole = TARGET_DB_IMPORTER_ROLE
	defer func() { importerRole = prevRole }()

	prev := metrics.Get()
	rec := metrics.NewRecordingRecorder()
	metrics.SetRecorder(rec)
	defer metrics.SetRecorder(prev)

	state := NewImportDataState(t.TempDir())

	all := makeTasksForTest(3)
	pending := all[2:] // 2 tables already completed in a prior run, 1 pending

	result := createInitialImportDataTableMetrics(state, all, pending)

	assert.Equal(t, int64(3), rec.ImportSnapshotTablesTotal[importerRole])
	assert.Len(t, rec.ImportSnapshotTableInit, 3)
	assert.Len(t, result, 1, "control-plane event list must still cover pending tasks only")
}
