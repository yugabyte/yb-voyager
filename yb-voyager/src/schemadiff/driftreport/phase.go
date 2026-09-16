// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driftreport

import "github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"

// phaseFor labels the migration-lifecycle phase that the interval between two
// consecutive captures falls in, based on their Series (capture label, or
// SeriesSourceLive for a live read). Returns "" when no phase label applies;
// callers should fall back to showing the time window alone in that case.
func phaseFor(prev, next Capture) string {
	switch {
	case prev.Series == schemasnapshot.LabelExportSchema && next.Series == schemasnapshot.LabelExportDataFromSourceStart:
		return "export data: pending"
	case isExportDataRunningStart(prev.Series) && next.Series == schemasnapshot.LabelExportDataFromSourcePeriodic:
		return "export data: running"
	case prev.Series == schemasnapshot.LabelExportDataFromSourceExit && next.Series == schemasnapshot.LabelExportDataFromSourceStart:
		return "export data: paused"
	case next.Series == SeriesSourceLive:
		return "since last capture"
	default:
		return ""
	}
}

// isExportDataRunningStart reports whether series is one of the two labels
// that may precede a LabelExportDataFromSourcePeriodic capture while export
// data is running: the initial start capture, or a prior periodic capture.
func isExportDataRunningStart(series string) bool {
	return series == schemasnapshot.LabelExportDataFromSourceStart || series == schemasnapshot.LabelExportDataFromSourcePeriodic
}
