/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package pbreporter

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

type DisablePBReporter struct { // Each individual goroutine has the context of the table corresponding to the PBReporter, no need to add to struct.
	TotalRows       int64
	CurrentRows     int64
	IsCompleted     bool
	TriggerComplete bool
}

func newDisablePBReporter() *DisablePBReporter {
	return &DisablePBReporter{}
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

