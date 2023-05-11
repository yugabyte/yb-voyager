package cmd

import (
	"github.com/vbauerster/mpb/v7"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/pbreporter"
)

type ProgressTracker struct {
	totalRowCount map[string]int64

	inProgressTable *dbzm.TableExportStatus

	mpbProgress *mpb.Progress
	pb          pbreporter.ExportProgressReporter
}

func NewProgressTracker(totalRowCount map[string]int64) *ProgressTracker {
	return &ProgressTracker{
		totalRowCount: totalRowCount,
		mpbProgress:   mpb.New(),
	}
}

func (pt *ProgressTracker) UpdateProgress(status *dbzm.ExportStatus) {
	if status == nil || status.InProgressTable() == nil {
		return
	}

	inProgressTable := status.InProgressTable()
	if pt.inProgressTable == nil || pt.inProgressTable.Sno != inProgressTable.Sno {
		// Complete currently in-progress progress-bar.
		if pt.pb != nil {
			pt.pb.SetTotalRowCount(pt.totalRowCount[dbzm.QualifiedTableName(pt.inProgressTable)], true)
			pt.pb = nil
		}
		// Start new progress-bar.
		pt.inProgressTable = inProgressTable
		pt.pb = pbreporter.NewExportPB(pt.mpbProgress, dbzm.QualifiedTableName(pt.inProgressTable), false)
		pt.pb.SetTotalRowCount(pt.totalRowCount[dbzm.QualifiedTableName(pt.inProgressTable)], false)
	}
	exportedRowCount := status.GetTableExportedRowCount(pt.inProgressTable.SchemaName, pt.inProgressTable.TableName)
	if pt.totalRowCount[dbzm.QualifiedTableName(pt.inProgressTable)] <= exportedRowCount {
		pt.totalRowCount[dbzm.QualifiedTableName(pt.inProgressTable)] += int64(float64(exportedRowCount) * 1.05)
		pt.pb.SetTotalRowCount(pt.totalRowCount[dbzm.QualifiedTableName(pt.inProgressTable)], false)
	}
	pt.pb.SetExportedRowCount(exportedRowCount)
}

func (pt *ProgressTracker) Done(status *dbzm.ExportStatus) {
	if pt.pb != nil {
		exportedRowCount := status.GetTableExportedRowCount(pt.inProgressTable.SchemaName, pt.inProgressTable.TableName)
		pt.pb.SetTotalRowCount(exportedRowCount, true /* Mark complete */)
		pt.pb = nil
	}
	pt.mpbProgress.Wait()
}
