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

import "github.com/vbauerster/mpb/v8"

type ExportProgressReporter interface { // Bare minimum required to simulate mpb.bar for exportDataStatus
	SetTotalRowCount(totalRowCount int64, triggerComplete bool)
	SetExportedRowCount(exportedRowCount int64)
	IsComplete() bool
}

func NewExportPB(progressContainer *mpb.Progress, tableName string, disablePb bool) ExportProgressReporter {
	if disablePb {
		return newDisablePBReporter()
	}
	return newEnablePBReporter(progressContainer, tableName)
}
