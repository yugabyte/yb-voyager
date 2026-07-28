//go:build unit

package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func newTupleForTest(schema, table string) sqlname.NameTuple {
	obj := sqlname.NewObjectName(constants.YUGABYTEDB, schema, schema, table)
	return sqlname.NameTuple{CurrentName: obj, SourceName: obj, TargetName: obj}
}

func TestPrometheusRecorder(t *testing.T) {
	t.Run("import snapshot counters", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")

		r.RecordImportSnapshotBatchCreated("target_db_importer", tup)
		r.RecordImportSnapshotBatchSubmitted("target_db_importer", tup)
		r.RecordImportSnapshotBatchIngested("target_db_importer", tup, 10, 100)

		expected := `
# HELP yb_voyager_import_data_snapshot_rows_total Total rows imported during snapshot
# TYPE yb_voyager_import_data_snapshot_rows_total counter
yb_voyager_import_data_snapshot_rows_total{importer_role="target_db_importer",migration_uuid="uuid-1",schema_name="public",session_id="sess-1",table_name="orders"} 10
`
		err := testutil.CollectAndCompare(
			r.importRowsTotal, strings.NewReader(expected),
			"yb_voyager_import_data_snapshot_rows_total",
		)
		assert.NoError(t, err)
		assert.Equal(t, 1, testutil.CollectAndCount(r.importSnapshotBatchIngested), "expected 1 ingested series")
	})

	t.Run("batch size and timestamp", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")

		r.ObserveImportSnapshotBatchSize("target_db_importer", tup, 10, 100)
		r.RecordImportSnapshotBatchIngested("target_db_importer", tup, 10, 100)

		assert.Equal(t, 1, testutil.CollectAndCount(r.importBatchSizeRows), "expected 1 batch_size_rows series")

		val := testutil.ToFloat64(r.importLastBatchIngestedTS.WithLabelValues(
			r.snapshotLabelValues("target_db_importer", tup)...))
		assert.Greater(t, val, float64(0), "last-ingested timestamp gauge not set")
	})

	t.Run("import errors", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")

		r.RecordImportError("target_db_importer", tup, ErrorKindRowProcessing, 3, 30)

		m := r.importErrorsTotal.WithLabelValues(
			"uuid-1", "sess-1", "target_db_importer", "orders", "public", string(ErrorKindRowProcessing))
		assert.Equal(t, float64(3), testutil.ToFloat64(m), "expected 3 error rows")
	})

	t.Run("import cdc", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.RecordImportCDCEvents("target_db_importer", 5, 3, 2)

		ins := r.importCDCEventsTotal.WithLabelValues("uuid-1", "sess-1", "target_db_importer", "insert")
		assert.Equal(t, float64(5), testutil.ToFloat64(ins), "expected 5 inserts")
	})

	t.Run("export snapshot rows counter accumulates deltas", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")
		r.RecordExportSnapshotRowCount("source_db_exporter", tup, 100) // first obs: +100
		r.RecordExportSnapshotRowCount("source_db_exporter", tup, 250) // delta +150
		g := r.exportSnapshotRows.WithLabelValues("uuid-1", "sess-1", "source_db_exporter", "orders", "public")
		assert.Equal(t, float64(250), testutil.ToFloat64(g))
	})

	t.Run("export snapshot rows counter reseeds after process reset", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")
		r.RecordExportSnapshotRowCount("source_db_exporter", tup, 500) // resume: already-exported 500 seeded
		assert.Equal(t, float64(500), testutil.ToFloat64(
			r.exportSnapshotRows.WithLabelValues("uuid-1", "sess-1", "source_db_exporter", "orders", "public")))
	})

	t.Run("export cdc events", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.RecordExportCDCEvents("source_db_exporter", 50)

		cdcEvents := r.exportCDCEventsTotal.WithLabelValues("uuid-1", "sess-1", "source_db_exporter")
		assert.Equal(t, float64(50), testutil.ToFloat64(cdcEvents), "expected 50 exported cdc events")
	})

	t.Run("throughput gauges", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")

		r.SetImportParallelism("target_db_importer", 8)
		r.SetNodeCPUPercent("node-1", 55.5)

		parallelism := r.importParallelism.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, float64(8), testutil.ToFloat64(parallelism))

		nodeCPU := r.nodeCPU.WithLabelValues("uuid-1", "sess-1", "node-1")
		assert.Equal(t, 55.5, testutil.ToFloat64(nodeCPU))
	})

	t.Run("import cdc lag gauges", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.SetImportCDCEventsPending("target_db_importer", 42)
		r.SetImportCDCEstimatedSecondsToCatchUp("target_db_importer", 12.5)
		r.SetImportCDCLastEventApplied("target_db_importer")

		pending := r.importCDCEventsPending.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, float64(42), testutil.ToFloat64(pending), "expected 42 pending")
		eta := r.importCDCEstimatedSecondsToCatchUp.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, 12.5, testutil.ToFloat64(eta), "expected eta 12.5")
		ts := r.importCDCLastEventApplied.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Greater(t, testutil.ToFloat64(ts), float64(0), "last-event-applied gauge not set")
	})

	t.Run("import progress and lifecycle", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")
		r.SetImportSnapshotTableExpectedRows("target_db_importer", tup, 1000)
		r.SetImportSnapshotTableStarted("target_db_importer", tup)
		r.SetImportSnapshotTableCompleted("target_db_importer", tup)

		total := r.importTableExpectedRows.WithLabelValues(r.snapshotLabelValues("target_db_importer", tup)...)
		assert.Equal(t, float64(1000), testutil.ToFloat64(total), "expected total 1000")
		start := r.importTableStartTS.WithLabelValues(r.snapshotLabelValues("target_db_importer", tup)...)
		assert.Greater(t, testutil.ToFloat64(start), float64(0), "start ts not set")
		done := r.importTableCompletedTS.WithLabelValues(r.snapshotLabelValues("target_db_importer", tup)...)
		assert.Greater(t, testutil.ToFloat64(done), float64(0), "completed ts not set")
	})

	t.Run("export table lifecycle timestamps", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")
		r.SetExportSnapshotTableStarted("source_db_exporter", tup)
		r.SetExportSnapshotTableCompleted("source_db_exporter", tup)

		start := r.exportTableStartTS.WithLabelValues("uuid-1", "sess-1", "source_db_exporter", "orders", "public")
		assert.Greater(t, testutil.ToFloat64(start), float64(0), "export start ts not set")
		done := r.exportTableCompletedTS.WithLabelValues("uuid-1", "sess-1", "source_db_exporter", "orders", "public")
		assert.Greater(t, testutil.ToFloat64(done), float64(0), "export completed ts not set")
	})

	t.Run("replication slot wal and build info", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.SetSourceReplicationSlotRetainedWALBytes("voyager_slot", 4096)
		wal := r.replicationSlotWAL.WithLabelValues("uuid-1", "sess-1", "voyager_slot")
		assert.Equal(t, float64(4096), testutil.ToFloat64(wal), "expected 4096 wal bytes")
		// build_info is always 1; label values come from the version package.
		assert.Equal(t, 1, testutil.CollectAndCount(r.buildInfo), "expected 1 build_info series")
	})

	t.Run("export parallelism", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.SetExportParallelism("source_db_exporter", 8)
		parallelism := r.exportParallelism.WithLabelValues("uuid-1", "sess-1", "source_db_exporter")
		assert.Equal(t, float64(8), testutil.ToFloat64(parallelism), "expected export parallelism 8")
	})

	t.Run("init seeds cumulative rows and bytes on resume", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")
		r.InitImportSnapshotTable("target_db_importer", tup, 500, 5000)
		lv := r.snapshotLabelValues("target_db_importer", tup)
		assert.Equal(t, float64(500), testutil.ToFloat64(r.importRowsTotal.WithLabelValues(lv...)))
		assert.Equal(t, float64(5000), testutil.ToFloat64(r.importBytesTotal.WithLabelValues(lv...)))
	})
}

func TestImportExportSnapshotTablesTotal(t *testing.T) {
	rec := NewPrometheusRecorder("uuid-1", "sess-1")
	rec.SetImportSnapshotTablesTotal("target_db_importer", 37)
	rec.SetExportSnapshotTablesTotal("source_db_exporter", 12)

	assert.Equal(t, float64(37), testutil.ToFloat64(
		rec.importSnapshotTablesTotal.WithLabelValues("uuid-1", "sess-1", "target_db_importer")))
	assert.Equal(t, float64(12), testutil.ToFloat64(
		rec.exportSnapshotTablesTotal.WithLabelValues("uuid-1", "sess-1", "source_db_exporter")))
}

func TestInitImportSnapshotTableCreatesZeroSeriesWithoutSeed(t *testing.T) {
	rec := NewPrometheusRecorder("uuid-1", "sess-1")
	tup := newTupleForTest("public", "orders")
	rec.InitImportSnapshotTable("target_db_importer", tup, 0, 0)

	lv := rec.snapshotLabelValues("target_db_importer", tup)
	assert.Equal(t, float64(0), testutil.ToFloat64(rec.importRowsTotal.WithLabelValues(lv...)))
	assert.Equal(t, 1, testutil.CollectAndCount(rec.importRowsTotal))
}

func TestDroppedMetricsAreAbsent(t *testing.T) {
	rec := NewPrometheusRecorder("uuid-1", "sess-1")
	mfs, err := rec.Registry().Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	dropped := map[string]bool{
		"yb_voyager_import_snapshot_batches_in_flight":     true,
		"yb_voyager_cdc_import_rate_events_per_second":     true,
		"yb_voyager_export_errors_total":                   true,
		"yb_voyager_import_pool_pending_close_connections": true,
		"yb_voyager_cdc_debezium_up":                       true,
	}
	for _, mf := range mfs {
		if dropped[mf.GetName()] {
			t.Fatalf("dropped metric %s is still registered", mf.GetName())
		}
	}
}
