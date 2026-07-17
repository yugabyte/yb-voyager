package metrics

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

// noopRecorder is the default recorder used when metrics are disabled.
// Every method is a no-op with no heap allocations on hot paths.
type noopRecorder struct{}

func (noopRecorder) RecordSnapshotBatchCreated(role string, t sqlname.NameTuple)              {}
func (noopRecorder) RecordSnapshotBatchSubmitted(role string, t sqlname.NameTuple)            {}
func (noopRecorder) RecordSnapshotBatchIngested(role string, t sqlname.NameTuple, r, b int64) {}
func (noopRecorder) ObserveSnapshotBatchSize(role string, t sqlname.NameTuple, r, b int64)    {}
func (noopRecorder) RecordImportError(role string, t sqlname.NameTuple, k ErrorKind, r, b int64) {
}
func (noopRecorder) RecordCDCEventsImported(role string, i, u, d int64)                        {}
func (noopRecorder) SetCDCImportRate(role string, eventsPerSec float64)                        {}
func (noopRecorder) SetExportedSnapshotRowCount(t sqlname.NameTuple, rows int64)               {}
func (noopRecorder) SetExportSnapshotTableTotalRows(t sqlname.NameTuple, rows int64)           {}
func (noopRecorder) RecordExportedCDCEvents(role string, events int64)                         {}
func (noopRecorder) RecordExportError(operation string)                                        {}
func (noopRecorder) SetCDCEventsPending(role string, pending int64)                            {}
func (noopRecorder) SetCDCEstimatedSecondsToCatchUp(role string, seconds float64)              {}
func (noopRecorder) SetCDCLastEventApplied(role string)                                        {}
func (noopRecorder) SetImportSnapshotTableTotalRows(role string, t sqlname.NameTuple, r int64) {}
func (noopRecorder) SetImportTableStarted(role string, t sqlname.NameTuple)                    {}
func (noopRecorder) SetImportTableCompleted(role string, t sqlname.NameTuple)                  {}
func (noopRecorder) SetSourceReplicationSlotRetainedWALBytes(slotName string, bytes int64)     {}
func (noopRecorder) SetParallelism(role string, level int)                                     {}
func (noopRecorder) SetParallelConnections(role string, n int)                                 {}
func (noopRecorder) SetPendingConnsToClose(role string, n int)                                 {}
func (noopRecorder) SetNodeCPUPercent(node string, pct float64)                                {}
func (noopRecorder) SetExportParallelism(role string, level int)                               {}
func (noopRecorder) SetDebeziumUp(role string, up bool)                                        {}
