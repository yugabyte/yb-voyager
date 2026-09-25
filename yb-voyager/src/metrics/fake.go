//go:build unit

package metrics

import (
	"sync"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// RecordingRecorder is a test-only Recorder that counts calls, for verifying
// that call sites invoke metrics without a Prometheus registry.
//
// Every recording method takes mu, because some call sites record from a
// background goroutine (the conflict cache records while holding its own lock).
// The exported maps are safe to read directly only from a single-goroutine test;
// a test whose recorder is written concurrently must read through an accessor
// that takes mu, as importCDCConflicts does.
type RecordingRecorder struct {
	mu sync.Mutex

	ImportCDCEventsPending    map[string]int64
	ImportCDCEstimatedSeconds map[string]float64
	ImportCDCLastEventApplied map[string]int
	ImportTableExpectedRows   map[string]int64
	ImportSnapshotTableInit   map[string]int
	ExportTableExpectedRows   map[string]int64
	ImportTableStartedCalls   map[string]int
	ImportTableCompletedCalls map[string]int
	ExportTableStartedCalls   map[string]int
	ExportTableCompletedCalls map[string]int
	ExportCDCEvents           map[string]int64
	ReplicationSlotWALBytes   map[string]int64
	ExportParallelism         map[string]int64
	ImportSnapshotTablesTotal map[string]int64
	ExportSnapshotTablesTotal map[string]int64

	// importCDCConflicts is unexported because the conflict cache records into it
	// from the streaming goroutine while a test polls for the count. Read it with
	// ImportCDCConflictsSnapshot, never directly.
	importCDCConflicts map[string]int
}

func NewRecordingRecorder() *RecordingRecorder {
	return &RecordingRecorder{
		ImportCDCEventsPending:    map[string]int64{},
		ImportCDCEstimatedSeconds: map[string]float64{},
		ImportCDCLastEventApplied: map[string]int{},
		ImportTableExpectedRows:   map[string]int64{},
		ImportSnapshotTableInit:   map[string]int{},
		ExportTableExpectedRows:   map[string]int64{},
		ImportTableStartedCalls:   map[string]int{},
		ImportTableCompletedCalls: map[string]int{},
		ExportTableStartedCalls:   map[string]int{},
		ExportTableCompletedCalls: map[string]int{},
		ExportCDCEvents:           map[string]int64{},
		ReplicationSlotWALBytes:   map[string]int64{},
		ExportParallelism:         map[string]int64{},
		ImportSnapshotTablesTotal: map[string]int64{},
		ExportSnapshotTablesTotal: map[string]int64{},
		importCDCConflicts:        map[string]int{},
	}
}

func key(t sqlname.NameTuple) string { s, tb := t.ForKeyTableSchema(); return s + "." + tb }

// import snapshot
func (r *RecordingRecorder) RecordImportSnapshotBatchCreated(importerRole string, t sqlname.NameTuple) {
}
func (r *RecordingRecorder) RecordImportSnapshotBatchSubmitted(importerRole string, t sqlname.NameTuple) {
}
func (r *RecordingRecorder) RecordImportSnapshotBatchIngested(importerRole string, t sqlname.NameTuple, rows, bytes int64) {
}
func (r *RecordingRecorder) ObserveImportSnapshotBatchSize(importerRole string, t sqlname.NameTuple, rows, bytes int64) {
}
func (r *RecordingRecorder) RecordImportError(importerRole string, t sqlname.NameTuple, kind ErrorKind, rows, bytes int64) {
}
func (r *RecordingRecorder) SetImportSnapshotTableExpectedRows(importerRole string, t sqlname.NameTuple, rows int64) {
	r.ImportTableExpectedRows[key(t)] = rows
}
func (r *RecordingRecorder) InitImportSnapshotTable(importerRole string, t sqlname.NameTuple) {
	r.ImportSnapshotTableInit[key(t)]++
}
func (r *RecordingRecorder) SetImportSnapshotTableStarted(importerRole string, t sqlname.NameTuple) {
	r.ImportTableStartedCalls[key(t)]++
}
func (r *RecordingRecorder) SetImportSnapshotTableCompleted(importerRole string, t sqlname.NameTuple) {
	r.ImportTableCompletedCalls[key(t)]++
}
func (r *RecordingRecorder) SetImportSnapshotTablesTotal(importerRole string, count int) {
	r.ImportSnapshotTablesTotal[importerRole] = int64(count)
}

// import CDC
func (r *RecordingRecorder) RecordImportCDCEvents(importerRole string, inserts, updates, deletes int64) {
}
func (r *RecordingRecorder) SetImportCDCEventsPending(importerRole string, pending int64) {
	r.ImportCDCEventsPending[importerRole] = pending
}
func (r *RecordingRecorder) SetImportCDCEstimatedSecondsToCatchUp(importerRole string, seconds float64) {
	r.ImportCDCEstimatedSeconds[importerRole] = seconds
}
func (r *RecordingRecorder) SetImportCDCLastEventApplied(importerRole string) {
	r.ImportCDCLastEventApplied[importerRole]++
}
func (r *RecordingRecorder) RecordImportCDCConflict(importerRole string, tableName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.importCDCConflicts[tableName]++
}

// ImportCDCConflictsSnapshot returns a copy of the per-table conflict counts. Callers
// must use this rather than touching the map, since the conflict cache records from the
// streaming goroutine while the test reads.
func (r *RecordingRecorder) ImportCDCConflictsSnapshot() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot := make(map[string]int, len(r.importCDCConflicts))
	for table, count := range r.importCDCConflicts {
		snapshot[table] = count
	}
	return snapshot
}

// export snapshot
func (r *RecordingRecorder) RecordExportSnapshotRowCount(exporterRole string, t sqlname.NameTuple, cumulative int64) {
}
func (r *RecordingRecorder) SetExportSnapshotTableExpectedRows(exporterRole string, t sqlname.NameTuple, rows int64) {
	r.ExportTableExpectedRows[key(t)] = rows
}
func (r *RecordingRecorder) SetExportSnapshotTableStarted(exporterRole string, t sqlname.NameTuple) {
	r.ExportTableStartedCalls[key(t)]++
}
func (r *RecordingRecorder) SetExportSnapshotTableCompleted(exporterRole string, t sqlname.NameTuple) {
	r.ExportTableCompletedCalls[key(t)]++
}
func (r *RecordingRecorder) SetExportSnapshotTablesTotal(exporterRole string, count int) {
	r.ExportSnapshotTablesTotal[exporterRole] = int64(count)
}

// export CDC
func (r *RecordingRecorder) RecordExportCDCEvents(exporterRole string, events int64) {
	r.ExportCDCEvents[exporterRole] += events
}

// misc
func (r *RecordingRecorder) SetSourceReplicationSlotRetainedWALBytes(slotName string, bytes int64) {
	r.ReplicationSlotWALBytes[slotName] = bytes
}
func (r *RecordingRecorder) SetImportParallelism(importerRole string, level int) {}
func (r *RecordingRecorder) SetExportParallelism(exporterRole string, level int) {
	r.ExportParallelism[exporterRole] = int64(level)
}
func (r *RecordingRecorder) SetNodeCPUPercent(node string, pct float64) {}
