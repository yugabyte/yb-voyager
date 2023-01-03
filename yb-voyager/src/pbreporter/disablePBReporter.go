package pbreporter

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

type DisablePBReporter struct { // Each individual goroutine has the context of the table corresponding to the PBReporter, no need to add to struct.
	TotalRows       int64
	CurrentRows     int64
	IsCompleted     bool
	TriggerComplete bool
}

func newDisablePBReporter() *DisablePBReporter {
	return &DisablePBReporter{TotalRows: int64(0), CurrentRows: int64(0), IsCompleted: false, TriggerComplete: false}
}

func (pbr *DisablePBReporter) SetTotalRowCount(totalRowCount int64, triggerComplete bool) {
	pbr.TriggerComplete = triggerComplete
	if totalRowCount < 0 {
		pbr.TotalRows = pbr.CurrentRows
	} else {
		pbr.TotalRows = totalRowCount
	}
	if triggerComplete && !pbr.IsCompleted {
		pbr.IsCompleted = true
		pbr.CurrentRows = pbr.TotalRows
	}

}
func (pbr *DisablePBReporter) SetExportedRowCount(exportedRowCount int64) {
	if exportedRowCount < 0 {
		utils.ErrExit("cannot maintain negative exported row count in PB")
	}
	pbr.CurrentRows = exportedRowCount
	if pbr.TriggerComplete && pbr.CurrentRows >= pbr.TotalRows {
		pbr.CurrentRows = pbr.TotalRows
		pbr.IsCompleted = true
	}
}
func (pbr *DisablePBReporter) IsComplete() bool {
	return pbr.IsCompleted
}
