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
		r.RecordSnapshotBatchCreated("target_db_importer", tup)
		r.RecordSnapshotBatchSubmitted("target_db_importer", tup)
		r.RecordSnapshotBatchIngested("target_db_importer", tup, 10, 100)
		r.ObserveSnapshotBatchSize("target_db_importer", tup, 10, 100)
		r.RecordImportError("target_db_importer", tup, ErrorKindRowProcessing, 1, 5)
		r.RecordCDCEventsImported("target_db_importer", 1, 2, 3)
		r.SetCDCImportRate("target_db_importer", 4.2)
		r.SetCDCEventsPending("target_db_importer", 42)
		r.SetCDCEstimatedSecondsToCatchUp("target_db_importer", 12.5)
		r.SetCDCLastEventApplied("target_db_importer")
		r.SetExportedSnapshotRowCount(tup, 99)
		r.RecordExportedCDCEvents("target_db_exporter", 7)
		r.RecordExportError("get_initial_table_list")
		r.SetImportSnapshotTableTotalRows("target_db_importer", tup, 1000)
		r.SetImportTableStarted("target_db_importer", tup)
		r.SetImportTableCompleted("target_db_importer", tup)
		r.SetSourceReplicationSlotRetainedWALBytes("voyager_slot", 4096)
		r.SetParallelism("target_db_importer", 8)
		r.SetParallelConnections("target_db_importer", 4)
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
		Get().SetParallelism("target_db_importer", 3)
		if sentinel.setParallelismCalls != 1 {
			t.Fatalf("expected 1 SetParallelism call, got %d", sentinel.setParallelismCalls)
		}
	})
}

// countingRecorder is a test double embedding noopRecorder and counting one method.
type countingRecorder struct {
	noopRecorder
	setParallelismCalls int
}

func (c *countingRecorder) SetParallelism(role string, level int) { c.setParallelismCalls++ }
