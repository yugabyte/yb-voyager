package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// snapshotLabels is the label set shared by the existing snapshot counters.
// Order is fixed and must not change (compat surface).
var snapshotLabels = []string{"migration_uuid", "session_id", "importer_role", "table_name", "schema_name"}
var errorLabels = append(append([]string{}, snapshotLabels...), "error_kind")
var cdcRateLabels = []string{"migration_uuid", "session_id", "importer_role"}
var cdcEventLabels = append(append([]string{}, cdcRateLabels...), "event_type")
var exportRowLabels = []string{"migration_uuid", "session_id", "table_name", "schema_name"}
var exportCDCLabels = []string{"migration_uuid", "session_id"}
var nodeCPULabels = []string{"migration_uuid", "session_id", "node"}

var sessionID = time.Now().Format("20060102-150405")

// SessionID returns the process-wide metrics session id (stable per run).
func SessionID() string { return sessionID }

// PrometheusRecorder implements Recorder backed by a dedicated registry, so it
// holds no global state and is safe to construct in tests.
type PrometheusRecorder struct {
	reg           *prometheus.Registry
	migrationUUID string
	sessionID     string

	importRowsTotal        *prometheus.CounterVec
	importBytesTotal       *prometheus.CounterVec
	snapshotBatchCreated   *prometheus.CounterVec
	snapshotBatchSubmitted *prometheus.CounterVec
	snapshotBatchIngested  *prometheus.CounterVec

	batchSizeRows       *prometheus.HistogramVec
	batchSizeBytes      *prometheus.HistogramVec
	lastBatchIngestedTS *prometheus.GaugeVec

	importErrorsTotal     *prometheus.CounterVec
	importErrorBytesTotal *prometheus.CounterVec

	cdcEventsImported *prometheus.CounterVec
	cdcImportRate     *prometheus.GaugeVec

	exportSnapshotRows *prometheus.GaugeVec
	exportCDCEvents    *prometheus.CounterVec

	parallelism   *prometheus.GaugeVec
	parallelConns *prometheus.GaugeVec
	nodeCPU       *prometheus.GaugeVec
}

func NewPrometheusRecorder(migrationUUID, sessionID string) *PrometheusRecorder {
	reg := prometheus.NewRegistry()
	f := promauto.With(reg)
	return &PrometheusRecorder{
		reg:           reg,
		migrationUUID: migrationUUID,
		sessionID:     sessionID,
		importRowsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_rows_total",
			Help: "Total rows imported during snapshot",
		}, snapshotLabels),
		importBytesTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_bytes_total",
			Help: "Total bytes imported during snapshot",
		}, snapshotLabels),
		snapshotBatchCreated: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_created_total",
			Help: "Total number of batches created for import",
		}, snapshotLabels),
		snapshotBatchSubmitted: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_submitted_total",
			Help: "Total number of batches submitted to worker pool",
		}, snapshotLabels),
		snapshotBatchIngested: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_ingested_total",
			Help: "Total number of batches successfully ingested",
		}, snapshotLabels),
		// PromQL: histogram_quantile(0.9, sum by (le) (rate(yb_voyager_import_snapshot_batch_size_rows_bucket[5m])))
		batchSizeRows: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "yb_voyager_import_snapshot_batch_size_rows",
			Help:    "Distribution of import batch sizes in rows",
			Buckets: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
		}, snapshotLabels),
		batchSizeBytes: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "yb_voyager_import_snapshot_batch_size_bytes",
			Help:    "Distribution of import batch sizes in bytes",
			Buckets: prometheus.ExponentialBuckets(1024, 4, 8), // 1KiB .. ~16MiB
		}, snapshotLabels),
		// PromQL: time() - yb_voyager_import_table_last_batch_ingested_timestamp_seconds > 600  (stalled >10m)
		lastBatchIngestedTS: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_table_last_batch_ingested_timestamp_seconds",
			Help: "Unix timestamp of the most recent successful batch ingest per table",
		}, snapshotLabels),
		// PromQL: sum by (table_name) (increase(yb_voyager_import_errors_total[1h]))
		importErrorsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_errors_total",
			Help: "Total rows that errored during import, by error_kind",
		}, errorLabels),
		importErrorBytesTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_error_bytes_total",
			Help: "Total bytes that errored during import, by error_kind",
		}, errorLabels),
		// PromQL: sum by (event_type) (rate(yb_voyager_cdc_events_imported_total[5m]))
		cdcEventsImported: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_cdc_events_imported_total",
			Help: "Total CDC events imported during streaming, by event_type",
		}, cdcEventLabels),
		cdcImportRate: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_cdc_import_rate_events_per_second",
			Help: "CDC events imported per second (3-minute average)",
		}, cdcRateLabels),
		exportSnapshotRows: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_export_snapshot_rows",
			Help: "Exported snapshot rows per table",
		}, exportRowLabels),
		exportCDCEvents: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_export_cdc_events_total",
			Help: "Total CDC events exported during streaming",
		}, exportCDCLabels),
		parallelism: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_parallelism",
			Help: "Current adaptive parallelism level",
		}, cdcRateLabels),
		parallelConns: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_parallel_connections",
			Help: "Current number of parallel import connections",
		}, cdcRateLabels),
		// PromQL: max by (node) (yb_voyager_cluster_node_cpu_percent)
		nodeCPU: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_cluster_node_cpu_percent",
			Help: "Per-node CPU usage percent of the target cluster",
		}, nodeCPULabels),
	}
}

// Registry exposes the recorder's registry for the HTTP handler.
func (p *PrometheusRecorder) Registry() *prometheus.Registry { return p.reg }

// snapshotLabelValues returns label values in snapshotLabels order.
func (p *PrometheusRecorder) snapshotLabelValues(role string, t sqlname.NameTuple) []string {
	schema, table := t.ForKeyTableSchema()
	return []string{p.migrationUUID, p.sessionID, role, table, schema}
}

func (p *PrometheusRecorder) RecordSnapshotBatchCreated(role string, t sqlname.NameTuple) {
	p.snapshotBatchCreated.WithLabelValues(p.snapshotLabelValues(role, t)...).Inc()
}

func (p *PrometheusRecorder) RecordSnapshotBatchSubmitted(role string, t sqlname.NameTuple) {
	p.snapshotBatchSubmitted.WithLabelValues(p.snapshotLabelValues(role, t)...).Inc()
}

func (p *PrometheusRecorder) RecordSnapshotBatchIngested(role string, t sqlname.NameTuple, rows, bytes int64) {
	lv := p.snapshotLabelValues(role, t)
	p.importRowsTotal.WithLabelValues(lv...).Add(float64(rows))
	p.importBytesTotal.WithLabelValues(lv...).Add(float64(bytes))
	p.snapshotBatchIngested.WithLabelValues(lv...).Inc()
	p.lastBatchIngestedTS.WithLabelValues(lv...).Set(float64(time.Now().Unix()))
}

func (p *PrometheusRecorder) ObserveSnapshotBatchSize(role string, t sqlname.NameTuple, rows, bytes int64) {
	lv := p.snapshotLabelValues(role, t)
	p.batchSizeRows.WithLabelValues(lv...).Observe(float64(rows))
	p.batchSizeBytes.WithLabelValues(lv...).Observe(float64(bytes))
}

func (p *PrometheusRecorder) RecordImportError(role string, t sqlname.NameTuple, kind ErrorKind, rows, bytes int64) {
	lv := append(p.snapshotLabelValues(role, t), string(kind))
	p.importErrorsTotal.WithLabelValues(lv...).Add(float64(rows))
	p.importErrorBytesTotal.WithLabelValues(lv...).Add(float64(bytes))
}
func (p *PrometheusRecorder) RecordCDCEventsImported(role string, inserts, updates, deletes int64) {
	p.cdcEventsImported.WithLabelValues(p.migrationUUID, p.sessionID, role, "insert").Add(float64(inserts))
	p.cdcEventsImported.WithLabelValues(p.migrationUUID, p.sessionID, role, "update").Add(float64(updates))
	p.cdcEventsImported.WithLabelValues(p.migrationUUID, p.sessionID, role, "delete").Add(float64(deletes))
}

func (p *PrometheusRecorder) SetCDCImportRate(role string, eventsPerSec float64) {
	p.cdcImportRate.WithLabelValues(p.migrationUUID, p.sessionID, role).Set(eventsPerSec)
}
func (p *PrometheusRecorder) SetExportedSnapshotRowCount(t sqlname.NameTuple, rows int64) {
	schema, table := t.ForKeyTableSchema()
	p.exportSnapshotRows.WithLabelValues(p.migrationUUID, p.sessionID, table, schema).Set(float64(rows))
}

func (p *PrometheusRecorder) RecordExportedCDCEvents(events int64) {
	p.exportCDCEvents.WithLabelValues(p.migrationUUID, p.sessionID).Add(float64(events))
}
func (p *PrometheusRecorder) SetParallelism(role string, level int) {
	p.parallelism.WithLabelValues(p.migrationUUID, p.sessionID, role).Set(float64(level))
}

func (p *PrometheusRecorder) SetParallelConnections(role string, n int) {
	p.parallelConns.WithLabelValues(p.migrationUUID, p.sessionID, role).Set(float64(n))
}

func (p *PrometheusRecorder) SetNodeCPUPercent(node string, pct float64) {
	p.nodeCPU.WithLabelValues(p.migrationUUID, p.sessionID, node).Set(pct)
}
