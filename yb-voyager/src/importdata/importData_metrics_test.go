//go:build unit

package importdata

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metrics"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func newImportMetricsTestTuple(schema, table string) sqlname.NameTuple {
	obj := sqlname.NewObjectName(constants.YUGABYTEDB, schema, schema, table)
	return sqlname.NameTuple{CurrentName: obj, SourceName: obj, TargetName: obj}
}

func TestCreateInitialImportDataTableMetrics_SetsTotalRows(t *testing.T) {
	rec := metrics.NewRecordingRecorder()
	prev := metrics.Get()
	defer metrics.SetRecorder(prev)
	metrics.SetRecorder(rec)

	imp := &Importer{cfg: Config{ImporterRole: constants.TARGET_DB_IMPORTER_ROLE, ReportProgressInBytes: false}}

	tup := newImportMetricsTestTuple("public", "orders")
	tasks := []*ImportFileTask{
		{
			ID:           1,
			FilePath:     "orders_data.sql",
			TableNameTup: tup,
			RowCount:     1000,
			FileSize:     2048,
		},
	}

	imp.createInitialImportDataTableMetrics(tasks, tasks)

	assert.Equal(t, int64(1000), rec.ImportTableExpectedRows["public.orders"])
	assert.Equal(t, int64(1), rec.ImportSnapshotTablesTotal[constants.TARGET_DB_IMPORTER_ROLE])
	assert.Equal(t, 1, rec.ImportSnapshotTableInit["public.orders"])
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
	imp := &Importer{cfg: Config{ImporterRole: constants.TARGET_DB_IMPORTER_ROLE}}

	prev := metrics.Get()
	rec := metrics.NewRecordingRecorder()
	metrics.SetRecorder(rec)
	defer metrics.SetRecorder(prev)

	all := makeTasksForTest(3)
	pending := all[2:] // 2 tables already completed in a prior run, 1 pending

	result := imp.createInitialImportDataTableMetrics(all, pending)

	assert.Equal(t, int64(3), rec.ImportSnapshotTablesTotal[constants.TARGET_DB_IMPORTER_ROLE])
	assert.Len(t, rec.ImportSnapshotTableInit, 3)
	assert.Len(t, result, 1, "control-plane event list must still cover pending tasks only")
}
