package metrics

import (
	"sync/atomic"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// ErrorKind classifies import errors for the yb_voyager_import_errors_total label.
type ErrorKind string

const (
	ErrorKindRowProcessing  ErrorKind = "row_processing"
	ErrorKindBatchIngestion ErrorKind = "batch_ingestion"
)

// Recorder is the only type metrics call sites depend on. A no-op implementation
// is the default so call sites never need to check whether metrics are enabled.
type Recorder interface {
	// snapshot import counters (existing metric names, preserved)
	RecordSnapshotBatchCreated(role string, t sqlname.NameTuple)
	RecordSnapshotBatchSubmitted(role string, t sqlname.NameTuple)
	RecordSnapshotBatchIngested(role string, t sqlname.NameTuple, rows, bytes int64)

	// batch-size distribution
	ObserveSnapshotBatchSize(role string, t sqlname.NameTuple, rows, bytes int64)

	// import errors
	RecordImportError(role string, t sqlname.NameTuple, kind ErrorKind, rows, bytes int64)

	// cdc / streaming
	RecordCDCEventsImported(role string, inserts, updates, deletes int64)
	SetCDCImportRate(role string, eventsPerSec float64)
	SetCDCEventsPending(role string, pending int64)
	SetCDCEstimatedSecondsToCatchUp(role string, seconds float64)
	SetCDCLastEventApplied(role string)

	// export
	SetExportedSnapshotRowCount(t sqlname.NameTuple, rows int64)
	SetExportSnapshotTableTotalRows(t sqlname.NameTuple, rows int64)
	RecordExportedCDCEvents(role string, events int64)
	RecordExportError(operation string)

	// import progress / lifecycle
	SetImportSnapshotTableTotalRows(role string, t sqlname.NameTuple, rows int64)
	SetImportTableStarted(role string, t sqlname.NameTuple)
	SetImportTableCompleted(role string, t sqlname.NameTuple)

	// source health
	SetSourceReplicationSlotRetainedWALBytes(slotName string, bytes int64)

	// throughput / parallelism gauges
	SetParallelism(role string, level int)
	SetParallelConnections(role string, n int)
	SetPendingConnsToClose(role string, n int)
	SetNodeCPUPercent(node string, pct float64)
	SetExportParallelism(role string, level int)

	// process liveness
	SetDebeziumUp(role string, up bool)
}

// recorderHolder gives atomic.Value a single, consistent concrete type to
// store, since Store panics if the dynamic type changes between calls and
// Recorder implementations vary (noopRecorder, *PrometheusRecorder, ...).
type recorderHolder struct {
	r Recorder
}

var active atomic.Value // stores recorderHolder

func init() {
	active.Store(recorderHolder{r: noopRecorder{}})
}

// Get returns the active recorder. Never nil.
func Get() Recorder {
	return active.Load().(recorderHolder).r
}

// SetRecorder installs the active recorder. Call once at command startup,
// before worker goroutines are spawned.
func SetRecorder(r Recorder) {
	active.Store(recorderHolder{r: r})
}
