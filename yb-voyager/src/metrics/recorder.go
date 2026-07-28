package metrics

import (
	"sync/atomic"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// ErrorKind classifies import errors for the yb_voyager_import_data_errors_total label.
type ErrorKind string

const (
	ErrorKindRowProcessing  ErrorKind = "row_processing"
	ErrorKindBatchIngestion ErrorKind = "batch_ingestion"
)

// Recorder is the only type metrics call sites depend on. A no-op implementation
// is the default so call sites never need to check whether metrics are enabled.
type Recorder interface {
	// import snapshot
	RecordImportSnapshotBatchCreated(importerRole string, t sqlname.NameTuple)
	RecordImportSnapshotBatchSubmitted(importerRole string, t sqlname.NameTuple)
	RecordImportSnapshotBatchIngested(importerRole string, t sqlname.NameTuple, rows, bytes int64)
	ObserveImportSnapshotBatchSize(importerRole string, t sqlname.NameTuple, rows, bytes int64)
	RecordImportError(importerRole string, t sqlname.NameTuple, kind ErrorKind, rows, bytes int64)
	SetImportSnapshotTableExpectedRows(importerRole string, t sqlname.NameTuple, rows int64)
	InitImportSnapshotTable(importerRole string, t sqlname.NameTuple, seedRows, seedBytes int64)
	SetImportSnapshotTableStarted(importerRole string, t sqlname.NameTuple)
	SetImportSnapshotTableCompleted(importerRole string, t sqlname.NameTuple)
	SetImportSnapshotTablesTotal(importerRole string, count int)

	// import CDC
	RecordImportCDCEvents(importerRole string, inserts, updates, deletes int64)
	SetImportCDCEventsPending(importerRole string, pending int64)
	SetImportCDCEstimatedSecondsToCatchUp(importerRole string, seconds float64)
	SetImportCDCLastEventApplied(importerRole string)

	// export snapshot
	RecordExportSnapshotRowCount(exporterRole string, t sqlname.NameTuple, cumulative int64)
	SetExportSnapshotTableExpectedRows(exporterRole string, t sqlname.NameTuple, rows int64)
	SetExportSnapshotTableStarted(exporterRole string, t sqlname.NameTuple)
	SetExportSnapshotTableCompleted(exporterRole string, t sqlname.NameTuple)
	SetExportSnapshotTablesTotal(exporterRole string, count int)

	// export CDC
	RecordExportCDCEvents(exporterRole string, events int64)

	// misc
	SetSourceReplicationSlotRetainedWALBytes(slotName string, bytes int64)
	SetImportParallelism(importerRole string, level int)
	SetExportParallelism(exporterRole string, level int)
	SetNodeCPUPercent(node string, pct float64)
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
