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

// Package schemadrift converts a sequence of captured schema snapshots (plus
// optionally one live snapshot) into a "schema drift" report: a
// JSON-serializable model, and JSON/HTML renderers for it.
//
// Reading snapshots from storage and writing the rendered output is the
// caller's job; nothing here does I/O beyond the embedded HTML template.
package schemadrift

import (
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// SeriesSourceLive is the Capture.Series / SnapshotInput.Series value used for
// a live read of the source database (as opposed to a stored snapshot series
// identified by its capture label).
const SeriesSourceLive = "source_live"

// Report is the top-level, JSON-serializable schema drift report. Its shape
// is contractual: field names and JSON tags must not change without a
// deliberate compatibility review, since downstream tooling (and users) may
// consume the JSON directly.
type Report struct {
	Report        string         `json:"report"` // always "schema_drift"
	Version       int            `json:"version"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Source        Source         `json:"source"`
	Window        Window         `json:"window"`
	Comparing     Comparing      `json:"comparing"`
	Summary       Summary        `json:"summary"`
	Diffs         []DiffEntry    `json:"diffs"`
	CapturePoints []CapturePoint `json:"capture_points"`
	// Intervals BuildReport declined to compare. Empty on a normal run; a
	// non-empty list means the report covers less than its Window suggests.
	Skipped []SkippedInterval `json:"skipped,omitempty"`
}

// Source identifies the source database the report was generated for.
type Source struct {
	DatabaseType    string `json:"database_type"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Database        string `json:"database"`
	DatabaseVersion string `json:"database_version"`
}

// Window is a [From, To] time interval, used both for the report as a whole
// and for each individual diff's capture-pair interval.
type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Comparing states what was actually compared, not what the user typed: the
// whole universe when unfiltered, the resolved keep-set when a --*-list
// narrowed it. The *Filtered flags tell the two apart.
type Comparing struct {
	Schemas             []string `json:"schemas"`
	Tables              []string `json:"tables"`
	TablesFiltered      bool     `json:"tables_filtered"`
	ObjectTypes         []string `json:"object_types"`
	ObjectTypesFiltered bool     `json:"object_types_filtered"`
}

// Summary carries report-wide counters.
type Summary struct {
	ChangeCount        int  `json:"change_count"`
	StoredCaptureCount int  `json:"stored_capture_count"`
	LiveCompared       bool `json:"live_compared"`
}

// DiffEntry is a single schema change, enriched with severity classification,
// human guidance, and the capture-pair window/phase it was detected in.
type DiffEntry struct {
	Seq        int                      `json:"seq"`
	Type       string                   `json:"type"`                // string(schemadiff.DiffType)
	Operation  string                   `json:"operation"`           // string(schemadiff.Operation): ADDED | DROPPED | CHANGED
	ObjectType string                   `json:"object_type"`         // string(schemadiff.ObjectType): TABLE | COLUMN
	Attribute  string                   `json:"attribute,omitempty"` // string(schemadiff.Attribute); "" for ADDED/DROPPED
	Object     schemasnapshot.ObjectRef `json:"object"`
	SubObject  string                   `json:"sub_object,omitempty"`
	OldValue   any                      `json:"old_value,omitempty"`
	NewValue   any                      `json:"new_value,omitempty"`
	Window     Window                   `json:"window"`
	Phase      string                   `json:"phase,omitempty"`
	// Severity, Impact and Action, flattened into this object by encoding/json.
	// They describe the DiffType, so every entry of a kind carries the same ones.
	DiffTypeInfo
}

// SkippedInterval is a capture pair that was not compared, so a reader can tell
// "nothing changed here" from "this interval was never examined".
type SkippedInterval struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Window Window `json:"window"`
	Reason string `json:"reason"`
}

// CapturePoint is one point on the report's timeline: a moment at which the
// source schema was captured, or capture was attempted. Three kinds appear -- a
// stored capture, a stored placeholder (the capture failed, so no schema content
// exists behind it), and the live read (Series == SeriesSourceLive, never
// persisted). It is a projection: the snapshot content itself stays out of the
// report.
type CapturePoint struct {
	Series     string    `json:"series"`
	Reason     string    `json:"reason,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
}
