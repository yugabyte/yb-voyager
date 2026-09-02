package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// snapshotLabels is the label set shared by the import snapshot counters.
// Order is fixed and must not change (compat surface).
var snapshotLabels = []string{"migration_uuid", "session_id", "importer_role", "table_name", "schema_name"}
var errorLabels = append(append([]string{}, snapshotLabels...), "error_kind")
var importerRoleLabels = []string{"migration_uuid", "session_id", "importer_role"}
var cdcEventLabels = append(append([]string{}, importerRoleLabels...), "event_type")
var exportSnapshotLabels = []string{"migration_uuid", "session_id", "exporter_role", "table_name", "schema_name"}
var exporterRoleLabels = []string{"migration_uuid", "session_id", "exporter_role"}
var nodeCPULabels = []string{"migration_uuid", "session_id", "node"}
var replicationSlotLabels = []string{"migration_uuid", "session_id", "slot_name"}
var buildInfoLabels = []string{"migration_uuid", "session_id", "version", "commit"}

var sessionID = time.Now().Format("20060102-150405")

// SessionID returns the process-wide metrics session id (stable per run).
func SessionID() string { return sessionID }

// PrometheusRecorder implements Recorder backed by a dedicated registry, so it
// holds no global state and is safe to construct in tests.
type PrometheusRecorder struct {
	reg           *prometheus.Registry
	migrationUUID string
	sessionID     string

	// import snapshot
	importRowsTotal              *prometheus.CounterVec
	importBytesTotal             *prometheus.CounterVec
	importSnapshotBatchCreated   *prometheus.CounterVec
	importSnapshotBatchSubmitted *prometheus.CounterVec
	importSnapshotBatchIngested  *prometheus.CounterVec
	importBatchSizeRows          *prometheus.HistogramVec
	importBatchSizeBytes         *prometheus.HistogramVec
	importLastBatchIngestedTS    *prometheus.GaugeVec
	importTableExpectedRows      *prometheus.GaugeVec
	importTableStartTS           *prometheus.GaugeVec
	importTableCompletedTS       *prometheus.GaugeVec
	importSnapshotTablesTotal    *prometheus.GaugeVec

	// import errors
	importErrorsTotal     *prometheus.CounterVec
	importErrorBytesTotal *prometheus.CounterVec

	// import CDC
	importCDCEventsTotal               *prometheus.CounterVec
	importCDCEventsPending             *prometheus.GaugeVec
	importCDCEstimatedSecondsToCatchUp *prometheus.GaugeVec
	importCDCLastEventApplied          *prometheus.GaugeVec

	// export snapshot
	exportSnapshotRows        *prometheus.CounterVec
	exportRowsMu              sync.Mutex
	exportRowsLast            map[string]int64
	exportTableExpectedRows   *prometheus.GaugeVec
	exportTableStartTS        *prometheus.GaugeVec
	exportTableCompletedTS    *prometheus.GaugeVec
	exportSnapshotTablesTotal *prometheus.GaugeVec

	// export CDC
	exportCDCEventsTotal *prometheus.CounterVec

	// misc
	replicationSlotWAL *prometheus.GaugeVec
	buildInfo          *prometheus.GaugeVec
	importParallelism  *prometheus.GaugeVec
	nodeCPU            *prometheus.GaugeVec
	exportParallelism  *prometheus.GaugeVec
}

func NewPrometheusRecorder(migrationUUID, sessionID string) *PrometheusRecorder {
	reg := prometheus.NewRegistry()
	f := promauto.With(reg)
	rec := &PrometheusRecorder{
		reg:            reg,
		migrationUUID:  migrationUUID,
		sessionID:      sessionID,
		exportRowsLast: map[string]int64{},

		importRowsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_snapshot_rows_total",
			Help: "Total rows imported during snapshot",
		}, snapshotLabels),
		importBytesTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_snapshot_bytes_total",
			Help: "Total bytes imported during snapshot",
		}, snapshotLabels),
		importSnapshotBatchCreated: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_snapshot_batch_created_total",
			Help: "Total number of batches created for import",
		}, snapshotLabels),
		importSnapshotBatchSubmitted: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_snapshot_batch_submitted_total",
			Help: "Total number of batches submitted to worker pool",
		}, snapshotLabels),
		importSnapshotBatchIngested: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_snapshot_batch_ingested_total",
			Help: "Total number of batches successfully ingested",
		}, snapshotLabels),
		// PromQL: histogram_quantile(0.9, sum by (le) (rate(yb_voyager_import_data_snapshot_batch_size_rows_bucket[5m])))
		importBatchSizeRows: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "yb_voyager_import_data_snapshot_batch_size_rows",
			Help:    "Distribution of import batch sizes in rows",
			Buckets: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
		}, snapshotLabels),
		importBatchSizeBytes: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "yb_voyager_import_data_snapshot_batch_size_bytes",
			Help:    "Distribution of import batch sizes in bytes",
			Buckets: prometheus.ExponentialBuckets(1024, 4, 8), // 1KiB .. ~16MiB
		}, snapshotLabels),
		// PromQL: time() - yb_voyager_import_data_snapshot_table_last_batch_ingested_timestamp_seconds > 600  (stalled >10m)
		importLastBatchIngestedTS: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_snapshot_table_last_batch_ingested_timestamp_seconds",
			Help: "Unix timestamp of the most recent successful batch ingest per table",
		}, snapshotLabels),
		importTableExpectedRows: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_snapshot_table_expected_rows",
			Help: "Expected total rows for the table during snapshot import",
		}, snapshotLabels),
		importTableStartTS: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_snapshot_table_start_timestamp_seconds",
			Help: "Unix timestamp when import of the table started",
		}, snapshotLabels),
		importTableCompletedTS: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_snapshot_table_completed_timestamp_seconds",
			Help: "Unix timestamp when import of the table completed",
		}, snapshotLabels),
		importSnapshotTablesTotal: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_snapshot_tables_total",
			Help: "Number of tables in scope for the import snapshot phase; set once at phase start",
		}, importerRoleLabels),
		// PromQL: sum by (table_name) (increase(yb_voyager_import_data_errors_total[1h]))
		importErrorsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_errors_total",
			Help: "Total rows that errored during import, by error_kind",
		}, errorLabels),
		importErrorBytesTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_error_bytes_total",
			Help: "Total bytes that errored during import, by error_kind",
		}, errorLabels),
		// PromQL: sum by (event_type) (rate(yb_voyager_import_data_cdc_events_total[5m]))
		importCDCEventsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_import_data_cdc_events_total",
			Help: "Total CDC events imported during streaming, by event_type",
		}, cdcEventLabels),
		importCDCEventsPending: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_cdc_events_pending",
			Help: "CDC events exported but not yet applied on the importer (exported minus imported)",
		}, importerRoleLabels),
		importCDCEstimatedSecondsToCatchUp: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_cdc_estimated_seconds_to_catch_up",
			Help: "Estimated seconds for the importer to apply all pending CDC events at the current rate",
		}, importerRoleLabels),
		importCDCLastEventApplied: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_cdc_last_event_applied_timestamp_seconds",
			Help: "Unix timestamp of the most recent successfully applied CDC event batch",
		}, importerRoleLabels),
		exportSnapshotRows: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_export_data_snapshot_rows_total",
			Help: "Total rows exported during snapshot",
		}, exportSnapshotLabels),
		exportTableExpectedRows: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_export_data_snapshot_table_expected_rows",
			Help: "Expected total rows for the table during snapshot export",
		}, exportSnapshotLabels),
		exportTableStartTS: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_export_data_snapshot_table_start_timestamp_seconds",
			Help: "Unix timestamp when export of the table started",
		}, exportSnapshotLabels),
		exportTableCompletedTS: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_export_data_snapshot_table_completed_timestamp_seconds",
			Help: "Unix timestamp when export of the table completed",
		}, exportSnapshotLabels),
		exportSnapshotTablesTotal: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_export_data_snapshot_tables_total",
			Help: "Number of tables in scope for the export snapshot phase; set once at phase start",
		}, exporterRoleLabels),
		exportCDCEventsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yb_voyager_export_data_cdc_events_total",
			Help: "Total CDC events exported during streaming",
		}, exporterRoleLabels),
		replicationSlotWAL: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_source_replication_slot_retained_wal_bytes",
			Help: "Bytes of WAL retained by the source logical replication slot",
		}, replicationSlotLabels),
		buildInfo: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_build_info",
			Help: "Build identification for the running yb-voyager process (always 1)",
		}, buildInfoLabels),
		importParallelism: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_import_data_parallelism",
			Help: "Current import parallelism (adaptive level, or the fixed --parallel-jobs value)",
		}, importerRoleLabels),
		// PromQL: max by (node) (yb_voyager_cluster_node_cpu_percent)
		nodeCPU: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_cluster_node_cpu_percent",
			Help: "Per-node CPU usage percent (user+system) of the target cluster; one series per node",
		}, nodeCPULabels),
		exportParallelism: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "yb_voyager_export_data_parallelism",
			Help: "Configured export parallelism (--parallel-jobs); static for the lifetime of the process",
		}, exporterRoleLabels),
	}
	rec.buildInfo.WithLabelValues(migrationUUID, sessionID, utils.YB_VOYAGER_VERSION, utils.GitCommitHash()).Set(1)
	return rec
}

// Registry exposes the recorder's registry for the HTTP handler.
func (p *PrometheusRecorder) Registry() *prometheus.Registry { return p.reg }

// snapshotLabelValues returns label values in snapshotLabels order.
func (p *PrometheusRecorder) snapshotLabelValues(importerRole string, t sqlname.NameTuple) []string {
	schema, table := t.ForKeyTableSchema()
	return []string{p.migrationUUID, p.sessionID, importerRole, table, schema}
}

// exportSnapshotLabelValues returns label values in exportSnapshotLabels order.
func (p *PrometheusRecorder) exportSnapshotLabelValues(exporterRole string, t sqlname.NameTuple) []string {
	schema, table := t.ForKeyTableSchema()
	return []string{p.migrationUUID, p.sessionID, exporterRole, table, schema}
}

// export snapshot

func (p *PrometheusRecorder) RecordExportSnapshotRowCount(exporterRole string, t sqlname.NameTuple, cumulative int64) {
	schema, table := t.ForKeyTableSchema()
	key := exporterRole + "." + schema + "." + table
	p.exportRowsMu.Lock()
	prev := p.exportRowsLast[key]
	if cumulative < prev { // source restarted / recount: treat cumulative as a fresh baseline
		prev = 0
	}
	delta := cumulative - prev
	p.exportRowsLast[key] = cumulative
	p.exportRowsMu.Unlock()
	if delta > 0 {
		p.exportSnapshotRows.WithLabelValues(p.exportSnapshotLabelValues(exporterRole, t)...).Add(float64(delta))
	}
}

func (p *PrometheusRecorder) SetExportSnapshotTableExpectedRows(exporterRole string, t sqlname.NameTuple, rows int64) {
	p.exportTableExpectedRows.WithLabelValues(p.exportSnapshotLabelValues(exporterRole, t)...).Set(float64(rows))
}

func (p *PrometheusRecorder) SetExportSnapshotTableStarted(exporterRole string, t sqlname.NameTuple) {
	p.exportTableStartTS.WithLabelValues(p.exportSnapshotLabelValues(exporterRole, t)...).Set(float64(time.Now().Unix()))
}

func (p *PrometheusRecorder) SetExportSnapshotTableCompleted(exporterRole string, t sqlname.NameTuple) {
	p.exportTableCompletedTS.WithLabelValues(p.exportSnapshotLabelValues(exporterRole, t)...).Set(float64(time.Now().Unix()))
}

func (p *PrometheusRecorder) SetExportSnapshotTablesTotal(exporterRole string, count int) {
	p.exportSnapshotTablesTotal.WithLabelValues(p.migrationUUID, p.sessionID, exporterRole).Set(float64(count))
}

// export CDC

func (p *PrometheusRecorder) RecordExportCDCEvents(exporterRole string, events int64) {
	p.exportCDCEventsTotal.WithLabelValues(p.migrationUUID, p.sessionID, exporterRole).Add(float64(events))
}

// import snapshot

func (p *PrometheusRecorder) RecordImportSnapshotBatchCreated(importerRole string, t sqlname.NameTuple) {
	p.importSnapshotBatchCreated.WithLabelValues(p.snapshotLabelValues(importerRole, t)...).Inc()
}

func (p *PrometheusRecorder) RecordImportSnapshotBatchSubmitted(importerRole string, t sqlname.NameTuple) {
	p.importSnapshotBatchSubmitted.WithLabelValues(p.snapshotLabelValues(importerRole, t)...).Inc()
}

func (p *PrometheusRecorder) RecordImportSnapshotBatchIngested(importerRole string, t sqlname.NameTuple, rows, bytes int64) {
	lv := p.snapshotLabelValues(importerRole, t)
	p.importRowsTotal.WithLabelValues(lv...).Add(float64(rows))
	p.importBytesTotal.WithLabelValues(lv...).Add(float64(bytes))
	p.importSnapshotBatchIngested.WithLabelValues(lv...).Inc()
	p.importLastBatchIngestedTS.WithLabelValues(lv...).Set(float64(time.Now().Unix()))
}

func (p *PrometheusRecorder) ObserveImportSnapshotBatchSize(importerRole string, t sqlname.NameTuple, rows, bytes int64) {
	lv := p.snapshotLabelValues(importerRole, t)
	p.importBatchSizeRows.WithLabelValues(lv...).Observe(float64(rows))
	p.importBatchSizeBytes.WithLabelValues(lv...).Observe(float64(bytes))
}

func (p *PrometheusRecorder) RecordImportError(importerRole string, t sqlname.NameTuple, kind ErrorKind, rows, bytes int64) {
	lv := append(p.snapshotLabelValues(importerRole, t), string(kind))
	p.importErrorsTotal.WithLabelValues(lv...).Add(float64(rows))
	p.importErrorBytesTotal.WithLabelValues(lv...).Add(float64(bytes))
}

func (p *PrometheusRecorder) SetImportSnapshotTableExpectedRows(importerRole string, t sqlname.NameTuple, rows int64) {
	p.importTableExpectedRows.WithLabelValues(p.snapshotLabelValues(importerRole, t)...).Set(float64(rows))
}

// InitImportSnapshotTable pre-registers the table's rows/bytes/batch series at
// zero so panels aren't empty before the first batch is ingested. Counters are
// not seeded with any cross-resume total; they reset to 0 per process.
func (p *PrometheusRecorder) InitImportSnapshotTable(importerRole string, t sqlname.NameTuple) {
	lv := p.snapshotLabelValues(importerRole, t)
	p.importRowsTotal.WithLabelValues(lv...)
	p.importBytesTotal.WithLabelValues(lv...)
	p.importSnapshotBatchCreated.WithLabelValues(lv...)
	p.importSnapshotBatchSubmitted.WithLabelValues(lv...)
	p.importSnapshotBatchIngested.WithLabelValues(lv...)
}

func (p *PrometheusRecorder) SetImportSnapshotTableStarted(importerRole string, t sqlname.NameTuple) {
	p.importTableStartTS.WithLabelValues(p.snapshotLabelValues(importerRole, t)...).Set(float64(time.Now().Unix()))
}

func (p *PrometheusRecorder) SetImportSnapshotTableCompleted(importerRole string, t sqlname.NameTuple) {
	p.importTableCompletedTS.WithLabelValues(p.snapshotLabelValues(importerRole, t)...).Set(float64(time.Now().Unix()))
}

func (p *PrometheusRecorder) SetImportSnapshotTablesTotal(importerRole string, count int) {
	p.importSnapshotTablesTotal.WithLabelValues(p.migrationUUID, p.sessionID, importerRole).Set(float64(count))
}

// import CDC

func (p *PrometheusRecorder) RecordImportCDCEvents(importerRole string, inserts, updates, deletes int64) {
	p.importCDCEventsTotal.WithLabelValues(p.migrationUUID, p.sessionID, importerRole, "insert").Add(float64(inserts))
	p.importCDCEventsTotal.WithLabelValues(p.migrationUUID, p.sessionID, importerRole, "update").Add(float64(updates))
	p.importCDCEventsTotal.WithLabelValues(p.migrationUUID, p.sessionID, importerRole, "delete").Add(float64(deletes))
}

func (p *PrometheusRecorder) SetImportCDCEventsPending(importerRole string, pending int64) {
	p.importCDCEventsPending.WithLabelValues(p.migrationUUID, p.sessionID, importerRole).Set(float64(pending))
}

func (p *PrometheusRecorder) SetImportCDCEstimatedSecondsToCatchUp(importerRole string, seconds float64) {
	p.importCDCEstimatedSecondsToCatchUp.WithLabelValues(p.migrationUUID, p.sessionID, importerRole).Set(seconds)
}

func (p *PrometheusRecorder) SetImportCDCLastEventApplied(importerRole string) {
	p.importCDCLastEventApplied.WithLabelValues(p.migrationUUID, p.sessionID, importerRole).Set(float64(time.Now().Unix()))
}

// misc

func (p *PrometheusRecorder) SetSourceReplicationSlotRetainedWALBytes(slotName string, bytes int64) {
	p.replicationSlotWAL.WithLabelValues(p.migrationUUID, p.sessionID, slotName).Set(float64(bytes))
}

func (p *PrometheusRecorder) SetImportParallelism(importerRole string, level int) {
	p.importParallelism.WithLabelValues(p.migrationUUID, p.sessionID, importerRole).Set(float64(level))
}

func (p *PrometheusRecorder) SetNodeCPUPercent(node string, pct float64) {
	p.nodeCPU.WithLabelValues(p.migrationUUID, p.sessionID, node).Set(pct)
}

func (p *PrometheusRecorder) SetExportParallelism(exporterRole string, level int) {
	p.exportParallelism.WithLabelValues(p.migrationUUID, p.sessionID, exporterRole).Set(float64(level))
}
