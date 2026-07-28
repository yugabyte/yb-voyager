//go:build unit

package metrics

import (
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func emptyTuple() sqlname.NameTuple { return sqlname.NameTuple{} }

func TestRecorder(t *testing.T) {
	t.Run("Get returns no-op by default", func(t *testing.T) {
		// A fresh process must never return nil; default is the no-op recorder.
		if Get() == nil {
			t.Fatal("Get() returned nil; expected no-op recorder")
		}
	})

	t.Run("No-op recorder does not panic", func(t *testing.T) {
		r := noopRecorder{}
		tup := emptyTuple()
		r.RecordImportSnapshotBatchCreated("target_db_importer", tup)
		r.RecordImportSnapshotBatchSubmitted("target_db_importer", tup)
		r.RecordImportSnapshotBatchIngested("target_db_importer", tup, 10, 100)
		r.ObserveImportSnapshotBatchSize("target_db_importer", tup, 10, 100)
		r.RecordImportError("target_db_importer", tup, ErrorKindRowProcessing, 1, 5)
		r.SetImportSnapshotTableExpectedRows("target_db_importer", tup, 1000)
		r.InitImportSnapshotTable("target_db_importer", tup, 0, 0)
		r.SetImportSnapshotTableStarted("target_db_importer", tup)
		r.SetImportSnapshotTableCompleted("target_db_importer", tup)
		r.SetImportSnapshotTablesTotal("target_db_importer", 10)
		r.RecordImportCDCEvents("target_db_importer", 1, 2, 3)
		r.SetImportCDCEventsPending("target_db_importer", 42)
		r.SetImportCDCEstimatedSecondsToCatchUp("target_db_importer", 12.5)
		r.SetImportCDCLastEventApplied("target_db_importer")
		r.RecordExportSnapshotRowCount("target_db_exporter", tup, 99)
		r.SetExportSnapshotTableExpectedRows("target_db_exporter", tup, 1000)
		r.SetExportSnapshotTableStarted("target_db_exporter", tup)
		r.SetExportSnapshotTableCompleted("target_db_exporter", tup)
		r.SetExportSnapshotTablesTotal("target_db_exporter", 10)
		r.RecordExportCDCEvents("target_db_exporter", 7)
		r.SetSourceReplicationSlotRetainedWALBytes("voyager_slot", 4096)
		r.SetImportParallelism("target_db_importer", 8)
		r.SetExportParallelism("target_db_exporter", 8)
		r.SetNodeCPUPercent("node-1", 55.5)
	})

	t.Run("SetRecorder installs active recorder", func(t *testing.T) {
		prev := Get()
		defer SetRecorder(prev)
		sentinel := &countingRecorder{}
		SetRecorder(sentinel)
		if Get() != sentinel {
			t.Fatal("SetRecorder did not install the recorder")
		}
		Get().SetImportParallelism("target_db_importer", 3)
		if sentinel.setImportParallelismCalls != 1 {
			t.Fatalf("expected 1 SetImportParallelism call, got %d", sentinel.setImportParallelismCalls)
		}
	})
}

// countingRecorder is a test double embedding noopRecorder and counting one method.
type countingRecorder struct {
	noopRecorder
	setImportParallelismCalls int
}

func (c *countingRecorder) SetImportParallelism(importerRole string, level int) {
	c.setImportParallelismCalls++
}
