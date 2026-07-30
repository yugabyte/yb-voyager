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

import (
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// SnapshotInput is one point in the chronological sequence of schemas fed
// into BuildReport: either a stored snapshot or the live read of the source.
type SnapshotInput struct {
	Header schemasnapshot.SnapshotHeader
	// Content is nil for a placeholder (failed-capture marker); such an entry
	// still appears on the timeline (as a Capture) but is never diffed.
	Content *schemasnapshot.SnapshotContent
	// Series names this point's series on the timeline: usually Header.Label
	// for a stored snapshot, or SeriesSourceLive for the live read.
	Series string
}

// BuildParams is the full, self-contained input to BuildReport. It carries no
// live connections or file handles — every field is plain data.
type BuildParams struct {
	Source Source
	// Schemas is the set of schemas the report is scoped to (display only;
	// does not affect diffing, which operates on whatever the snapshots
	// captured).
	Schemas []string
	// Snapshots is the chronological (oldest-first) sequence of stored
	// snapshots to diff pairwise.
	Snapshots []SnapshotInput
	// Live is the optional live read of the source, appended after the last
	// stored snapshot. nil if the source was unreachable or the caller chose
	// to skip it.
	Live        *SnapshotInput
	Scope       schemadiff.Scope
	Tables      []string // comparing.tables (display names); nil => all
	ObjectTypes []string // comparing.object_types; nil => all
	GeneratedAt time.Time
}

// BuildReport assembles a Report from p. It performs no I/O: every diff is
// computed from the SnapshotContent values already present in p.
//
// Diffing walks the chronological sequence (stored snapshots followed by the
// live read if present) and compares each content-bearing snapshot to the
// nearest PRECEDING content-bearing snapshot, bridging across any
// placeholders (Content == nil) in between — a placeholder never severs the
// comparison, it is simply skipped. A pair is otherwise skipped (contributing
// no DiffEntries, and no error) when the two sides' Header.Schemas sets
// differ (order-insensitive); in that case the later snapshot becomes the new
// baseline for subsequent comparisons, since snapshots on either side of a
// schema-scope mismatch cannot be compared. Diffs are computed via
// schemadiff.NewDiffer(p.Scope).Diff. DiffEntry.Seq is a single counter
// running across the whole report, not reset per interval.
//
// BuildReport never panics on nil/empty inputs: zero snapshots and no live
// read yields a Report with empty Captures/Diffs and a zero-value Window.
func BuildReport(p BuildParams) Report {
	all := make([]SnapshotInput, 0, len(p.Snapshots)+1)
	all = append(all, p.Snapshots...)
	if p.Live != nil {
		all = append(all, *p.Live)
	}

	captures := make([]Capture, len(all))
	for i, s := range all {
		captures[i] = Capture{
			Seq:        i + 1,
			Series:     s.Series,
			Reason:     s.Header.Reason,
			CapturedAt: s.Header.CapturedAt,
		}
	}

	differ := schemadiff.NewDiffer(schemadiff.Config{Scope: p.Scope})

	var diffs []DiffEntry
	seq := 0
	prevIdx := -1
	for i := range all {
		if all[i].Content == nil {
			continue // placeholder: skip without disturbing prevIdx, so the bridge spans it
		}
		if prevIdx == -1 {
			prevIdx = i
			continue // first content-bearing snapshot: nothing to compare against yet
		}

		prev, next := all[prevIdx], all[i]
		if !sameSchemaScope(prev.Header.Schemas, next.Header.Schemas) {
			prevIdx = i // schema-scope mismatch: skip the pair, but this snapshot becomes the new baseline
			continue
		}

		window := Window{From: prev.Header.CapturedAt, To: next.Header.CapturedAt}
		phase := phaseFor(captures[prevIdx], captures[i])

		for _, d := range differ.Diff(prev.Content, next.Content) {
			seq++
			obj, subObj := splitIdentity(displayIdentity(d))
			diffs = append(diffs, DiffEntry{
				Seq:        seq,
				Type:       string(d.Type),
				Operation:  string(d.Operation),
				ObjectType: string(d.ObjectType),
				Attribute:  string(d.Attribute),
				Object:     obj,
				SubObject:  subObj,
				Status:     string(Classify(d.Type)),
				OldValue:   d.SideAValue,
				NewValue:   d.SideBValue,
				Window:     window,
				Phase:      phase,
				Guidance:   Guidance(d.Type),
			})
		}
		prevIdx = i
	}

	var window Window
	if len(captures) > 0 {
		window = Window{From: captures[0].CapturedAt, To: captures[len(captures)-1].CapturedAt}
	}

	return Report{
		Report:      "schema_drift",
		Version:     1,
		GeneratedAt: p.GeneratedAt,
		Source:      p.Source,
		Window:      window,
		Comparing: Comparing{
			Schemas:     p.Schemas,
			Tables:      p.Tables,
			ObjectTypes: p.ObjectTypes,
		},
		Summary: Summary{
			ChangeCount:  len(diffs),
			CaptureCount: len(p.Snapshots),
			LiveCompared: p.Live != nil,
		},
		Diffs:    diffs,
		Captures: captures,
	}
}

// displayIdentity picks the finding's display identity: side-B (new) when
// present, else side-A (old) — the same "prefer new, fall back to old"
// convention Difference itself documents for ObjectA/ObjectB (nil ObjectB for
// *_DROPPED, nil ObjectA for *_ADDED, both set for *_CHANGED where they may
// differ, e.g. a rename).
func displayIdentity(d schemadiff.Difference) schemadiff.ObjectIdent {
	if d.ObjectB != nil {
		return d.ObjectB
	}
	return d.ObjectA
}

// splitIdentity derives DiffEntry's Object/SubObject pair from a finding's
// identity. A schema-level identity (schemasnapshot.ObjectRef, e.g. a table)
// maps directly to Object with no SubObject. A table-scoped identity
// (schemasnapshot.TableScopedObjectRef, e.g. a column) maps to its parent table as
// Object and its own name as SubObject.
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

// sameSchemaScope reports whether a and b contain the same set of schema
// names, ignoring order and duplicates.
func sameSchemaScope(a, b []string) bool {
	setA := make(map[string]struct{}, len(a))
	for _, s := range a {
		setA[s] = struct{}{}
	}
	setB := make(map[string]struct{}, len(b))
	for _, s := range b {
		setB[s] = struct{}{}
	}
	if len(setA) != len(setB) {
		return false
	}
	for s := range setA {
		if _, ok := setB[s]; !ok {
			return false
		}
	}
	return true
}
