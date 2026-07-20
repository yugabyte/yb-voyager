//go:build unit

package metrics

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

// RecordingRecorder is a test-only Recorder that counts calls, for verifying
// that call sites invoke metrics without a Prometheus registry.
type RecordingRecorder struct {
	CDCEventsPending          map[string]int64
	CDCEstimatedSeconds       map[string]float64
	CDCLastEventAppliedCalls  map[string]int
	ImportTableTotalRows      map[string]int64
	ImportSnapshotTableInit   map[string]int
	ExportTableTotalRows      map[string]int64
	ImportTableStartedCalls   map[string]int
	ImportTableCompletedCalls map[string]int
	ExportTableStartedCalls   map[string]int
	ExportTableCompletedCalls map[string]int
	ExportErrors              map[string]int64
	ExportedCDCEvents         map[string]int64
	ReplicationSlotWALBytes   map[string]int64
	SnapshotBatchesInFlight   map[string]int64
	PendingConnsToClose       map[string]int64
	ExportParallelism         map[string]int64
	DebeziumUp                map[string]bool
	SnapshotTablesTotal       map[string]int64
}

func NewRecordingRecorder() *RecordingRecorder {
	return &RecordingRecorder{
		CDCEventsPending: map[string]int64{}, CDCEstimatedSeconds: map[string]float64{},
		CDCLastEventAppliedCalls: map[string]int{}, ImportTableTotalRows: map[string]int64{},
		ImportSnapshotTableInit: map[string]int{},
		ExportTableTotalRows:    map[string]int64{},
		ImportTableStartedCalls: map[string]int{}, ImportTableCompletedCalls: map[string]int{},
		ExportTableStartedCalls: map[string]int{}, ExportTableCompletedCalls: map[string]int{},
		ExportErrors: map[string]int64{}, ExportedCDCEvents: map[string]int64{},
		ReplicationSlotWALBytes: map[string]int64{},
		SnapshotBatchesInFlight: map[string]int64{}, PendingConnsToClose: map[string]int64{},
		ExportParallelism: map[string]int64{}, DebeziumUp: map[string]bool{},
		SnapshotTablesTotal: map[string]int64{},
	}
}

func key(t sqlname.NameTuple) string { s, tb := t.ForKeyTableSchema(); return s + "." + tb }

func (r *RecordingRecorder) RecordSnapshotBatchCreated(role string, t sqlname.NameTuple) {}
func (r *RecordingRecorder) RecordSnapshotBatchSubmitted(role string, t sqlname.NameTuple) {
	r.SnapshotBatchesInFlight[key(t)]++
}
func (r *RecordingRecorder) RecordSnapshotBatchIngested(role string, t sqlname.NameTuple, rows, bytes int64) {
	r.SnapshotBatchesInFlight[key(t)]--
}
func (r *RecordingRecorder) ObserveSnapshotBatchSize(role string, t sqlname.NameTuple, rows, bytes int64) {
}
func (r *RecordingRecorder) RecordImportError(role string, t sqlname.NameTuple, kind ErrorKind, rows, bytes int64) {
}
func (r *RecordingRecorder) RecordCDCEventsImported(role string, inserts, updates, deletes int64) {}
func (r *RecordingRecorder) SetCDCImportRate(role string, eventsPerSec float64)                   {}
func (r *RecordingRecorder) SetCDCEventsPending(role string, pending int64) {
	r.CDCEventsPending[role] = pending
}
func (r *RecordingRecorder) SetCDCEstimatedSecondsToCatchUp(role string, seconds float64) {
	r.CDCEstimatedSeconds[role] = seconds
}
func (r *RecordingRecorder) SetCDCLastEventApplied(role string)                          { r.CDCLastEventAppliedCalls[role]++ }
func (r *RecordingRecorder) SetExportedSnapshotRowCount(t sqlname.NameTuple, rows int64) {}
func (r *RecordingRecorder) SetExportSnapshotTableTotalRows(t sqlname.NameTuple, rows int64) {
	r.ExportTableTotalRows[key(t)] = rows
}
func (r *RecordingRecorder) SetExportTableStarted(t sqlname.NameTuple) {
	r.ExportTableStartedCalls[key(t)]++
}
func (r *RecordingRecorder) SetExportTableCompleted(t sqlname.NameTuple) {
	r.ExportTableCompletedCalls[key(t)]++
}
func (r *RecordingRecorder) RecordExportedCDCEvents(role string, events int64) {
	r.ExportedCDCEvents[role] += events
}
func (r *RecordingRecorder) RecordExportError(operation string) { r.ExportErrors[operation]++ }
func (r *RecordingRecorder) SetImportSnapshotTableTotalRows(role string, t sqlname.NameTuple, rows int64) {
	r.ImportTableTotalRows[key(t)] = rows
}
func (r *RecordingRecorder) InitImportSnapshotTable(role string, t sqlname.NameTuple) {
	r.ImportSnapshotTableInit[key(t)]++
}
func (r *RecordingRecorder) SetImportTableStarted(role string, t sqlname.NameTuple) {
	r.ImportTableStartedCalls[key(t)]++
}
func (r *RecordingRecorder) SetImportTableCompleted(role string, t sqlname.NameTuple) {
	r.ImportTableCompletedCalls[key(t)]++
}
func (r *RecordingRecorder) SetParallelism(role string, level int)     {}
func (r *RecordingRecorder) SetParallelConnections(role string, n int) {}
func (r *RecordingRecorder) SetPendingConnsToClose(role string, n int) {
	r.PendingConnsToClose[role] = int64(n)
}
func (r *RecordingRecorder) SetNodeCPUPercent(node string, pct float64) {}
func (r *RecordingRecorder) SetExportParallelism(role string, level int) {
	r.ExportParallelism[role] = int64(level)
}
func (r *RecordingRecorder) SetSnapshotTablesTotal(role string, count int) {
	r.SnapshotTablesTotal[role] = int64(count)
}
func (r *RecordingRecorder) SetDebeziumUp(role string, up bool) { r.DebeziumUp[role] = up }
func (r *RecordingRecorder) SetSourceReplicationSlotRetainedWALBytes(slotName string, bytes int64) {
	r.ReplicationSlotWALBytes[slotName] = bytes
}
