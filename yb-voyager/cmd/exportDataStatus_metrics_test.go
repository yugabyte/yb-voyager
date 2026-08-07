//go:build unit

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metrics"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func newExportMetricsTestTuple(schema, table string) sqlname.NameTuple {
	obj := sqlname.NewObjectName(constants.POSTGRESQL, schema, schema, table)
	return sqlname.NameTuple{CurrentName: obj, SourceName: obj, TargetName: obj}
}

func TestInitExportSnapshotMetrics_RegistersAllTables(t *testing.T) {
	rec := metrics.NewRecordingRecorder()
	prev := metrics.Get()
	defer metrics.SetRecorder(prev)
	metrics.SetRecorder(rec)

	prevRole := exporterRole
	defer func() { exporterRole = prevRole }()
	exporterRole = SOURCE_DB_EXPORTER_ROLE

	md := map[string]*utils.TableProgressMetadata{
		"public.orders":   {TableName: newExportMetricsTestTuple("public", "orders"), CountTotalRows: 1000},
		"public.payments": {TableName: newExportMetricsTestTuple("public", "payments"), CountTotalRows: 0},
	}

	initExportSnapshotMetrics(md)

	assert.Equal(t, int64(1000), rec.ExportTableExpectedRows["public.orders"])
	assert.Equal(t, int64(0), rec.ExportTableExpectedRows["public.payments"])
	assert.Equal(t, int64(2), rec.ExportSnapshotTablesTotal[SOURCE_DB_EXPORTER_ROLE])
}
