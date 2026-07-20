//go:build unit

package cmd

import (
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

	prevRole := importerRole
	defer func() { importerRole = prevRole }()
	importerRole = TARGET_DB_IMPORTER_ROLE

	prevReportInBytes := reportProgressInBytes
	defer func() { reportProgressInBytes = prevReportInBytes }()
	reportProgressInBytes = false

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

	createInitialImportDataTableMetrics(tasks)

	assert.Equal(t, int64(1000), rec.ImportTableTotalRows["public.orders"])
	assert.Equal(t, int64(1), rec.SnapshotTablesTotal[TARGET_DB_IMPORTER_ROLE])
	assert.Equal(t, 1, rec.ImportSnapshotTableInit["public.orders"])
}
