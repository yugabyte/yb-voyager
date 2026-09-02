//go:build unit

package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metrics"
)

func TestStreamImportStatsReporterMetrics(t *testing.T) {
	t.Run("BatchImported records last-event-applied", func(t *testing.T) {
		rec := metrics.NewRecordingRecorder()
		prev := metrics.Get()
		defer metrics.SetRecorder(prev)
		metrics.SetRecorder(rec)

		s := NewStreamImportStatsReporter("target_db_importer")
		s.BatchImported(2, 1, 1)

		assert.Equal(t, 1, rec.ImportCDCLastEventApplied["target_db_importer"],
			"BatchImported should set the last-event-applied gauge once")
	})
}
