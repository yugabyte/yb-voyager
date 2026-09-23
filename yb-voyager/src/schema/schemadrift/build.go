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
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// DetectionConfig is the full, self-contained input to BuildReport. It carries no
// live connections or file handles — every field is plain data.
type DetectionConfig struct {
	Source Source
	// Oldest-first; BuildReport relies on the ordering. A nil Content is a failed
	// capture: still a point on the timeline, never diffed. The live read, when
	// the caller took one, is the last entry and is labelled LabelSourceLive.
	Snapshots []schemasnapshot.SchemaSnapshot
	// The exact sets compared, filtered or not; Comparing is rendered from it.
	Scope schemadiff.Scope
	// Whether the user narrowed each dimension. This is the one fact Scope cannot
	// carry: "these 12 tables" and "these 12 tables, which are all of them" are the
	// same set, and a report that cannot tell them apart lets a reader take "no
	// drift" across 12 of 400 tables for "no drift anywhere".
	TablesFiltered      bool
	ObjectTypesFiltered bool
}

// BuildReport assembles a Report from p, comparing each usable snapshot to the
// nearest preceding one.
//
// Two kinds of snapshot are bridged rather than compared -- skipped without
// disturbing the baseline, so the comparison reaches back past them:
//
//   - a failed capture, which has no content at all;
//   - a capture that did not cover every requested schema, where a table's absence
//     is not evidence of a drop but evidence that nobody looked.
//
// Both are recorded on their CapturePoint so a reader sees the gap. Captures that
// cover MORE than was requested are compared normally: the extra schemas' findings
// are removed by Scope's schema dimension, not by declining the comparison.
func BuildReport(p DetectionConfig) Report {
	capturePoints := make([]CapturePoint, len(p.Snapshots))
	for i, s := range p.Snapshots {
		capturePoints[i] = CapturePoint{
			Series:     s.Header.Label,
			Reason:     s.Header.Reason,
			CapturedAt: s.Header.CapturedAt,
		}
	}

	differ := schemadiff.NewDiffer(schemadiff.Config{Scope: p.Scope})

	var drifts []DriftEntry
	liveCompared := false
	comparedIntervals := 0
	prevIdx := -1
	for i := range p.Snapshots {
		switch {
		case p.Snapshots[i].Content == nil:
			capturePoints[i].Excluded = "the capture failed, so this point holds no schema"
			continue
		case !coversSchemas(p.Snapshots[i].Header, p.Scope.Schemas):
			missing, _ := lo.Difference(p.Scope.Schemas, p.Snapshots[i].Header.Schemas)
			capturePoints[i].Excluded = fmt.Sprintf("captured only %s, so it cannot answer for %s",
				strings.Join(p.Snapshots[i].Header.Schemas, ", "), strings.Join(missing, ", "))
			continue
		}
		if prevIdx == -1 {
			prevIdx = i
			continue // first usable snapshot: nothing to compare against yet
		}

		prev, next := p.Snapshots[prevIdx], p.Snapshots[i]
		comparedIntervals++
		intervalWindow := Window{From: prev.Header.CapturedAt, To: next.Header.CapturedAt}
		phase := phaseFor(capturePoints[prevIdx], capturePoints[i])
		if next.Header.Label == schemasnapshot.LabelSourceLive {
			liveCompared = true
		}

		for _, d := range differ.Diff(prev.Content, next.Content) {
			obj, subObj := splitIdentity(displayIdentity(d))
			drifts = append(drifts, DriftEntry{
				Diff: Diff{
					Type:       d.Type,
					Operation:  d.Operation,
					ObjectType: d.ObjectType,
					Attribute:  d.Attribute,
					Object:     obj,
					SubObject:  subObj,
					OldValue:   d.SideAValue,
					NewValue:   d.SideBValue,
				},
				Window:    intervalWindow,
				Phase:     phase,
				DriftInfo: getDriftInfo(d.Type),
			})
		}
		prevIdx = i
	}

	var reportWindow Window
	if len(capturePoints) > 0 {
		reportWindow = Window{From: capturePoints[0].CapturedAt, To: capturePoints[len(capturePoints)-1].CapturedAt}
	}

	return Report{
		Report:      "schema_drift",
		Version:     1,
		GeneratedAt: time.Now().UTC(),
		Source:      p.Source,
		Window:      reportWindow,
		Comparing: Comparing{
			Schemas: p.Scope.Schemas,
			Tables: lo.Map(p.Scope.Tables, func(r schemasnapshot.ObjectRef, _ int) string {
				return r.ForDisplay(p.Source.DatabaseType)
			}),
			TablesFiltered: p.TablesFiltered,
			ObjectTypes: lo.Map(p.Scope.ObjectTypes, func(t schemadiff.ObjectType, _ int) string {
				return string(t)
			}),
			ObjectTypesFiltered: p.ObjectTypesFiltered,
		},
		Summary: Summary{
			ChangeCount:           len(drifts),
			ComparedIntervalCount: comparedIntervals,
			StoredCaptureCount:    lo.CountBy(p.Snapshots, func(s schemasnapshot.SchemaSnapshot) bool { return s.Header.Label != schemasnapshot.LabelSourceLive }),
			LiveCompared:          liveCompared,
		},
		Drifts:        drifts,
		CapturePoints: capturePoints,
	}
}

// coversSchemas reports whether h was captured with every schema in requested, so
// that a table missing from its content is genuinely absent rather than never
// looked for. Capturing MORE than was requested still covers it.
func coversSchemas(h schemasnapshot.SnapshotHeader, requested []string) bool {
	missing, _ := lo.Difference(requested, h.Schemas)
	return len(missing) == 0
}

// phaseFor labels the migration phase an interval between two captures falls in.
// Returns "" when no label applies, and the caller shows the time window alone.
func phaseFor(prev, next CapturePoint) string {
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
	case next.Series == schemasnapshot.LabelSourceLive:
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
