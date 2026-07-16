//go:build unit

package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func newTupleForTest(schema, table string) sqlname.NameTuple {
	obj := sqlname.NewObjectName(constants.YUGABYTEDB, schema, schema, table)
	return sqlname.NameTuple{CurrentName: obj, SourceName: obj, TargetName: obj}
}

func TestPrometheusRecorderSnapshotCounters(t *testing.T) {
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
	if err := testutil.CollectAndCompare(
		r.importRowsTotal, strings.NewReader(expected),
		"yb_voyager_import_snapshot_rows_total",
	); err != nil {
		t.Fatal(err)
	}
	if got := testutil.CollectAndCount(r.snapshotBatchIngested); got != 1 {
		t.Fatalf("expected 1 ingested series, got %d", got)
	}
}

func TestPrometheusRecorderBatchSizeAndTimestamp(t *testing.T) {
	r := NewPrometheusRecorder("uuid-1", "sess-1")
	tup := newTupleForTest("public", "orders")

	r.ObserveSnapshotBatchSize("target_db_importer", tup, 10, 100)
	r.RecordSnapshotBatchIngested("target_db_importer", tup, 10, 100)

	if got := testutil.CollectAndCount(r.batchSizeRows); got != 1 {
		t.Fatalf("expected 1 batch_size_rows series, got %d", got)
	}
	if got := testutil.ToFloat64(r.lastBatchIngestedTS.WithLabelValues(
		r.snapshotLabelValues("target_db_importer", tup)...)); got <= 0 {
		t.Fatalf("last-ingested timestamp gauge not set, got %v", got)
	}
}

func TestPrometheusRecorderImportErrors(t *testing.T) {
	r := NewPrometheusRecorder("uuid-1", "sess-1")
	tup := newTupleForTest("public", "orders")

	r.RecordImportError("target_db_importer", tup, ErrorKindRowProcessing, 3, 30)

	m := r.importErrorsTotal.WithLabelValues(
		"uuid-1", "sess-1", "target_db_importer", "orders", "public", string(ErrorKindRowProcessing))
	if got := testutil.ToFloat64(m); got != 3 {
		t.Fatalf("expected 3 error rows, got %v", got)
	}
}

func TestPrometheusRecorderCDC(t *testing.T) {
	r := NewPrometheusRecorder("uuid-1", "sess-1")
	r.RecordCDCEventsImported("target_db_importer", 5, 3, 2)
	r.SetCDCImportRate("target_db_importer", 12.5)

	ins := r.cdcEventsImported.WithLabelValues("uuid-1", "sess-1", "target_db_importer", "insert")
	if got := testutil.ToFloat64(ins); got != 5 {
		t.Fatalf("expected 5 inserts, got %v", got)
	}
	rate := r.cdcImportRate.WithLabelValues("uuid-1", "sess-1", "target_db_importer")
	if got := testutil.ToFloat64(rate); got != 12.5 {
		t.Fatalf("expected rate 12.5, got %v", got)
	}
}

func TestPrometheusRecorderExport(t *testing.T) {
	r := NewPrometheusRecorder("uuid-1", "sess-1")
	tup := newTupleForTest("public", "orders")

	r.SetExportedSnapshotRowCount(tup, 1234)
	r.RecordExportedCDCEvents(50)

	g := r.exportSnapshotRows.WithLabelValues("uuid-1", "sess-1", "orders", "public")
	if got := testutil.ToFloat64(g); got != 1234 {
		t.Fatalf("expected 1234 exported rows, got %v", got)
	}
	if got := testutil.ToFloat64(r.exportCDCEvents.WithLabelValues("uuid-1", "sess-1")); got != 50 {
		t.Fatalf("expected 50 exported cdc events, got %v", got)
	}
}

func TestPrometheusRecorderThroughputGauges(t *testing.T) {
	r := NewPrometheusRecorder("uuid-1", "sess-1")

	r.SetParallelism("target_db_importer", 8)
	r.SetParallelConnections("target_db_importer", 4)
	r.SetNodeCPUPercent("node-1", 55.5)

	if got := testutil.ToFloat64(r.parallelism.WithLabelValues("uuid-1", "sess-1", "target_db_importer")); got != 8 {
		t.Fatalf("parallelism = %v", got)
	}
	if got := testutil.ToFloat64(r.parallelConns.WithLabelValues("uuid-1", "sess-1", "target_db_importer")); got != 4 {
		t.Fatalf("parallel_connections = %v", got)
	}
	if got := testutil.ToFloat64(r.nodeCPU.WithLabelValues("uuid-1", "sess-1", "node-1")); got != 55.5 {
		t.Fatalf("node cpu = %v", got)
	}
}
