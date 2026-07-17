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
	t.Run("snapshot counters", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")

		r.RecordSnapshotBatchCreated("target_db_importer", tup)
		r.RecordSnapshotBatchSubmitted("target_db_importer", tup)
		r.RecordSnapshotBatchIngested("target_db_importer", tup, 10, 100)

		expected := `
# HELP yb_voyager_import_snapshot_rows_total Total rows imported during snapshot
# TYPE yb_voyager_import_snapshot_rows_total counter
yb_voyager_import_snapshot_rows_total{importer_role="target_db_importer",migration_uuid="uuid-1",schema_name="public",session_id="sess-1",table_name="orders"} 10
`
		err := testutil.CollectAndCompare(
			r.importRowsTotal, strings.NewReader(expected),
			"yb_voyager_import_snapshot_rows_total",
		)
		assert.NoError(t, err)
		assert.Equal(t, 1, testutil.CollectAndCount(r.snapshotBatchIngested), "expected 1 ingested series")
	})

	t.Run("batch size and timestamp", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")

		r.ObserveSnapshotBatchSize("target_db_importer", tup, 10, 100)
		r.RecordSnapshotBatchIngested("target_db_importer", tup, 10, 100)

		assert.Equal(t, 1, testutil.CollectAndCount(r.batchSizeRows), "expected 1 batch_size_rows series")

		val := testutil.ToFloat64(r.lastBatchIngestedTS.WithLabelValues(
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

	t.Run("cdc", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.RecordCDCEventsImported("target_db_importer", 5, 3, 2)
		r.SetCDCImportRate("target_db_importer", 12.5)

		ins := r.cdcEventsImported.WithLabelValues("uuid-1", "sess-1", "target_db_importer", "insert")
		assert.Equal(t, float64(5), testutil.ToFloat64(ins), "expected 5 inserts")

		rate := r.cdcImportRate.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, 12.5, testutil.ToFloat64(rate), "expected rate 12.5")
	})

	t.Run("export", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")

		r.SetExportedSnapshotRowCount(tup, 1234)
		r.RecordExportedCDCEvents("source_db_exporter", 50)

		g := r.exportSnapshotRows.WithLabelValues("uuid-1", "sess-1", "orders", "public")
		assert.Equal(t, float64(1234), testutil.ToFloat64(g), "expected 1234 exported rows")

		cdcEvents := r.exportCDCEvents.WithLabelValues("uuid-1", "sess-1", "source_db_exporter")
		assert.Equal(t, float64(50), testutil.ToFloat64(cdcEvents), "expected 50 exported cdc events")
	})

	t.Run("throughput gauges", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")

		r.SetParallelism("target_db_importer", 8)
		r.SetParallelConnections("target_db_importer", 4)
		r.SetNodeCPUPercent("node-1", 55.5)

		parallelism := r.parallelism.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, float64(8), testutil.ToFloat64(parallelism))

		parallelConns := r.parallelConns.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, float64(4), testutil.ToFloat64(parallelConns))

		nodeCPU := r.nodeCPU.WithLabelValues("uuid-1", "sess-1", "node-1")
		assert.Equal(t, 55.5, testutil.ToFloat64(nodeCPU))
	})

	t.Run("cdc lag gauges", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.SetCDCEventsPending("target_db_importer", 42)
		r.SetCDCEstimatedSecondsToCatchUp("target_db_importer", 12.5)
		r.SetCDCLastEventApplied("target_db_importer")

		pending := r.cdcEventsPending.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, float64(42), testutil.ToFloat64(pending), "expected 42 pending")
		eta := r.cdcEstimatedSecondsToCatchUp.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, 12.5, testutil.ToFloat64(eta), "expected eta 12.5")
		ts := r.cdcLastEventApplied.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Greater(t, testutil.ToFloat64(ts), float64(0), "last-event-applied gauge not set")
	})

	t.Run("import progress and lifecycle", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")
		r.SetImportSnapshotTableTotalRows("target_db_importer", tup, 1000)
		r.SetImportTableStarted("target_db_importer", tup)
		r.SetImportTableCompleted("target_db_importer", tup)

		total := r.importTableTotalRows.WithLabelValues(r.snapshotLabelValues("target_db_importer", tup)...)
		assert.Equal(t, float64(1000), testutil.ToFloat64(total), "expected total 1000")
		start := r.importTableStartTS.WithLabelValues(r.snapshotLabelValues("target_db_importer", tup)...)
		assert.Greater(t, testutil.ToFloat64(start), float64(0), "start ts not set")
		done := r.importTableCompletedTS.WithLabelValues(r.snapshotLabelValues("target_db_importer", tup)...)
		assert.Greater(t, testutil.ToFloat64(done), float64(0), "completed ts not set")
	})

	t.Run("export errors and exporter_role on cdc events", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.RecordExportError("get_initial_table_list")
		r.RecordExportedCDCEvents("source_db_exporter", 7)

		errs := r.exportErrorsTotal.WithLabelValues("uuid-1", "sess-1", "get_initial_table_list")
		assert.Equal(t, float64(1), testutil.ToFloat64(errs), "expected 1 export error")
		ev := r.exportCDCEvents.WithLabelValues("uuid-1", "sess-1", "source_db_exporter")
		assert.Equal(t, float64(7), testutil.ToFloat64(ev), "expected 7 exported events")
	})

	t.Run("replication slot wal and build info", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.SetSourceReplicationSlotRetainedWALBytes("voyager_slot", 4096)
		wal := r.replicationSlotWAL.WithLabelValues("uuid-1", "sess-1", "voyager_slot")
		assert.Equal(t, float64(4096), testutil.ToFloat64(wal), "expected 4096 wal bytes")
		// build_info is always 1; label values come from the version package.
		assert.Equal(t, 1, testutil.CollectAndCount(r.buildInfo), "expected 1 build_info series")
	})

	t.Run("batches in flight tracks submit/ingest lifecycle", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		tup := newTupleForTest("public", "orders")
		lv := r.snapshotLabelValues("target_db_importer", tup)

		r.RecordSnapshotBatchSubmitted("target_db_importer", tup)
		r.RecordSnapshotBatchSubmitted("target_db_importer", tup)
		inFlight := r.snapshotBatchesInFlight.WithLabelValues(lv...)
		assert.Equal(t, float64(2), testutil.ToFloat64(inFlight), "expected 2 batches in flight after 2 submits")

		r.RecordSnapshotBatchIngested("target_db_importer", tup, 10, 100)
		assert.Equal(t, float64(1), testutil.ToFloat64(inFlight), "expected 1 batch in flight after 1 ingest")
	})

	t.Run("pending conns to close and export parallelism", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.SetPendingConnsToClose("target_db_importer", 3)
		pending := r.pendingConnsToClose.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
		assert.Equal(t, float64(3), testutil.ToFloat64(pending), "expected 3 pending conns to close")

		r.SetExportParallelism("source_db_exporter", 8)
		parallelism := r.exportParallelism.WithLabelValues("uuid-1", "sess-1", "source_db_exporter")
		assert.Equal(t, float64(8), testutil.ToFloat64(parallelism), "expected export parallelism 8")
	})

	t.Run("debezium liveness", func(t *testing.T) {
		r := NewPrometheusRecorder("uuid-1", "sess-1")
		r.SetDebeziumUp("source_db_exporter", true)
		up := r.debeziumUp.WithLabelValues("uuid-1", "sess-1", "source_db_exporter")
		assert.Equal(t, float64(1), testutil.ToFloat64(up), "expected debezium up=1")

		r.SetDebeziumUp("source_db_exporter", false)
		assert.Equal(t, float64(0), testutil.ToFloat64(up), "expected debezium up=0 after stop")
	})
}
