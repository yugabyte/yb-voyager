package metrics

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

// noopRecorder is the default recorder used when metrics are disabled.
// Every method is a no-op with no heap allocations on hot paths.
type noopRecorder struct{}

// import snapshot
func (noopRecorder) RecordImportSnapshotBatchCreated(importerRole string, t sqlname.NameTuple)   {}
func (noopRecorder) RecordImportSnapshotBatchSubmitted(importerRole string, t sqlname.NameTuple) {}
func (noopRecorder) RecordImportSnapshotBatchIngested(importerRole string, t sqlname.NameTuple, rows, bytes int64) {
}
func (noopRecorder) ObserveImportSnapshotBatchSize(importerRole string, t sqlname.NameTuple, rows, bytes int64) {
}
func (noopRecorder) RecordImportError(importerRole string, t sqlname.NameTuple, kind ErrorKind, rows, bytes int64) {
}
func (noopRecorder) SetImportSnapshotTableExpectedRows(importerRole string, t sqlname.NameTuple, rows int64) {
}
func (noopRecorder) InitImportSnapshotTable(importerRole string, t sqlname.NameTuple) {}
func (noopRecorder) SetImportSnapshotTableStarted(importerRole string, t sqlname.NameTuple)   {}
func (noopRecorder) SetImportSnapshotTableCompleted(importerRole string, t sqlname.NameTuple) {}
func (noopRecorder) SetImportSnapshotTablesTotal(importerRole string, count int)              {}

// import CDC
func (noopRecorder) RecordImportCDCEvents(importerRole string, inserts, updates, deletes int64) {}
func (noopRecorder) SetImportCDCEventsPending(importerRole string, pending int64)               {}
func (noopRecorder) SetImportCDCEstimatedSecondsToCatchUp(importerRole string, seconds float64) {}
func (noopRecorder) SetImportCDCLastEventApplied(importerRole string)                           {}

// export snapshot
func (noopRecorder) RecordExportSnapshotRowCount(exporterRole string, t sqlname.NameTuple, cumulative int64) {
}
func (noopRecorder) SetExportSnapshotTableExpectedRows(exporterRole string, t sqlname.NameTuple, rows int64) {
}
func (noopRecorder) SetExportSnapshotTableStarted(exporterRole string, t sqlname.NameTuple)   {}
func (noopRecorder) SetExportSnapshotTableCompleted(exporterRole string, t sqlname.NameTuple) {}
func (noopRecorder) SetExportSnapshotTablesTotal(exporterRole string, count int)              {}

// export CDC
func (noopRecorder) RecordExportCDCEvents(exporterRole string, events int64) {}

// misc
func (noopRecorder) SetSourceReplicationSlotRetainedWALBytes(slotName string, bytes int64) {}
func (noopRecorder) SetImportParallelism(importerRole string, level int)                   {}
func (noopRecorder) SetExportParallelism(exporterRole string, level int)                   {}
func (noopRecorder) SetNodeCPUPercent(node string, pct float64)                            {}
