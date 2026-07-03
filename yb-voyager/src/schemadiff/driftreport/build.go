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
// Consecutive pairs (in chronological order, stored snapshots followed by the
// live read if present) are diffed via schemadiff.NewDiffer(p.Scope).Diff.
// A pair is skipped (contributing no DiffEntries, and no error) when either
// side is a placeholder (Content == nil) or when the two sides' Header.Schemas
// sets differ (order-insensitive). DiffEntry.Seq is a single counter running
// across the whole report, not reset per interval.
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
	for i := 0; i+1 < len(all); i++ {
		prev, next := all[i], all[i+1]
		if prev.Content == nil || next.Content == nil {
			continue // placeholder pair: skip entirely
		}
		if !sameSchemaScope(prev.Header.Schemas, next.Header.Schemas) {
			continue // schema-scope mismatch: skip entirely
		}

		window := Window{From: prev.Header.CapturedAt, To: next.Header.CapturedAt}
		phase := phaseFor(captures[i], captures[i+1])

		for _, d := range differ.Diff(prev.Content, next.Content) {
			seq++
			diffs = append(diffs, DiffEntry{
				Seq:       seq,
				Type:      string(d.Type),
				Object:    d.Object,
				SubObject: d.SubObject,
				Status:    string(Classify(d.Type)),
				Property:  d.Property,
				OldValue:  d.OldValue,
				NewValue:  d.NewValue,
				Window:    window,
				Phase:     phase,
				Guidance:  Guidance(d.Type),
			})
		}
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
