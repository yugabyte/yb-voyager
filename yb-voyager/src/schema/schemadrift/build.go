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

package schemadrift

import (
	"time"

	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// SnapshotInput is one point in the chronological sequence of schemas fed
// into BuildReport: either a stored snapshot or the live read of the source.
type SnapshotInput struct {
	Header schemasnapshot.SnapshotHeader
	// nil for a failed capture: still a Capture on the timeline, never diffed.
	Content *schemasnapshot.SnapshotContent
	// Series is the timeline identity phaseFor and the renderers key off, and
	// is not always Header.Label: the live read borrows LabelDetectDrift only to
	// satisfy label validation, so its Series is SeriesSourceLive instead.
	Series string
}

// BuildParams is the full, self-contained input to BuildReport. It carries no
// live connections or file handles — every field is plain data.
type BuildParams struct {
	Source Source
	// Display only: diffing works on whatever the snapshots captured.
	Schemas []string
	// Oldest-first; BuildReport relies on the ordering. The live read, when the
	// caller took one, is the last entry and carries Series SeriesSourceLive.
	Snapshots []SnapshotInput
	Scope     schemadiff.Scope
	// Always populated, filtered or not; see Comparing.
	Tables              []string
	TablesFiltered      bool
	ObjectTypes         []string
	ObjectTypesFiltered bool
	GeneratedAt         time.Time
}

// BuildReport assembles a Report from p, comparing each content-bearing
// snapshot to the nearest preceding one.
//
// Two cases are not obvious from the walk below:
//
//   - A failed capture is bridged, not a boundary: it is skipped and the
//     comparison reaches back past it.
//   - A pair whose Header.Schemas sets differ is skipped instead, and the
//     later snapshot becomes the new baseline -- snapshots covering different
//     schemas cannot be meaningfully compared.
func BuildReport(p BuildParams) Report {
	captures := make([]Capture, len(p.Snapshots))
	for i, s := range p.Snapshots {
		captures[i] = Capture{
			Series:     s.Series,
			Reason:     s.Header.Reason,
			CapturedAt: s.Header.CapturedAt,
		}
	}

	differ := schemadiff.NewDiffer(schemadiff.Config{Scope: p.Scope})

	var diffs []DiffEntry
	var skipped []SkippedInterval
	seq := 0
	prevIdx := -1
	for i := range p.Snapshots {
		if p.Snapshots[i].Content == nil {
			continue // placeholder: skip without disturbing prevIdx, so the bridge spans it
		}
		if prevIdx == -1 {
			prevIdx = i
			continue // first content-bearing snapshot: nothing to compare against yet
		}

		prev, next := p.Snapshots[prevIdx], p.Snapshots[i]
		if !sameSchemaScope(prev.Header.Schemas, next.Header.Schemas) {
			skipped = append(skipped, SkippedInterval{
				From:   prev.Series,
				To:     next.Series,
				Window: Window{From: prev.Header.CapturedAt, To: next.Header.CapturedAt},
				Reason: "the two captures cover different schemas, so they cannot be compared",
			})
			prevIdx = i // this snapshot becomes the new baseline
			continue
		}

		intervalWindow := Window{From: prev.Header.CapturedAt, To: next.Header.CapturedAt}
		phase := phaseFor(captures[prevIdx], captures[i])

		for _, d := range differ.Diff(prev.Content, next.Content) {
			seq++
			obj, subObj := splitIdentity(displayIdentity(d))
			class := Classify(d.Type)
			diffs = append(diffs, DiffEntry{
				Seq:        seq,
				Type:       string(d.Type),
				Operation:  string(d.Operation),
				ObjectType: string(d.ObjectType),
				Attribute:  string(d.Attribute),
				Object:     obj,
				SubObject:  subObj,
				Status:     string(class.Status),
				OldValue:   d.SideAValue,
				NewValue:   d.SideBValue,
				Window:     intervalWindow,
				Phase:      phase,
				Impact:     class.Impact,
				Action:     class.Action,
			})
		}
		prevIdx = i
	}

	var reportWindow Window
	if len(captures) > 0 {
		reportWindow = Window{From: captures[0].CapturedAt, To: captures[len(captures)-1].CapturedAt}
	}

	return Report{
		Report:      "schema_drift",
		Version:     1,
		GeneratedAt: p.GeneratedAt,
		Source:      p.Source,
		Window:      reportWindow,
		Comparing: Comparing{
			Schemas:             p.Schemas,
			Tables:              p.Tables,
			TablesFiltered:      p.TablesFiltered,
			ObjectTypes:         p.ObjectTypes,
			ObjectTypesFiltered: p.ObjectTypesFiltered,
		},
		Summary: Summary{
			ChangeCount:  len(diffs),
			CaptureCount: lo.CountBy(p.Snapshots, func(s SnapshotInput) bool { return s.Series != SeriesSourceLive }),
			LiveCompared: lo.ContainsBy(p.Snapshots, func(s SnapshotInput) bool { return s.Series == SeriesSourceLive }),
		},
		Diffs:    diffs,
		Captures: captures,
		Skipped:  skipped,
	}
}

// phaseFor labels the migration phase an interval between two captures falls in.
// Returns "" when no label applies, and the caller shows the time window alone.
func phaseFor(prev, next Capture) string {
	switch {
	case prev.Series == schemasnapshot.LabelExportSchema && next.Series == schemasnapshot.LabelExportDataFromSourceStart:
		return "export data: pending"
	// The export was running for this whole span, whether the span ends at
	// another periodic capture or at the exit. The exit marker says how the run
	// ended; this says what was happening while the drift appeared.
	case isExportDataRunningStart(prev.Series) && isExportDataRunningEnd(next.Series):
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
func isExportDataRunningEnd(series string) bool {
	return series == schemasnapshot.LabelExportDataFromSourcePeriodic || series == schemasnapshot.LabelExportDataFromSourceExit
}

func isExportDataRunningStart(series string) bool {
	return series == schemasnapshot.LabelExportDataFromSourceStart || series == schemasnapshot.LabelExportDataFromSourcePeriodic
}

// displayIdentity prefers side-B (new) over side-A (old), the convention
// Difference documents for ObjectA/ObjectB.
func displayIdentity(d schemadiff.Difference) schemadiff.ObjectIdent {
	if d.ObjectB != nil {
		return d.ObjectB
	}
	return d.ObjectA
}

// splitIdentity maps a table-scoped identity (a column) to its parent table as
// Object and its own name as SubObject; a table maps to Object alone.
func splitIdentity(id schemadiff.ObjectIdent) (obj schemasnapshot.ObjectRef, subObject string) {
	switch it := id.(type) {
	case schemasnapshot.ObjectRef:
		return it, ""
	case schemasnapshot.TableScopedObjectRef:
		return it.Table, it.Name
	default:
		return schemasnapshot.ObjectRef{}, ""
	}
}

// sameSchemaScope reports whether a and b cover the same set of schemas,
// ignoring order and duplicates.
func sameSchemaScope(a, b []string) bool {
	onlyA, onlyB := lo.Difference(a, b)
	return len(onlyA) == 0 && len(onlyB) == 0
}
