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

// Package driftreport converts a sequence of captured schema snapshots (plus
// optionally one live snapshot) into a "schema drift" report: a
// JSON-serializable model, and JSON/HTML renderers for it.
//
// This package is a pure library: no cobra, no DB access, and no file I/O
// beyond one embedded HTML template used by RenderHTML. Callers (the future
// `schema detect-drift` command) are responsible for reading snapshots from
// storage and writing the rendered output to disk.
package driftreport

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
	Report      string      `json:"report"` // always "schema_drift"
	Version     int         `json:"version"`
	GeneratedAt time.Time   `json:"generated_at"`
	Source      Source      `json:"source"`
	Window      Window      `json:"window"`
	Comparing   Comparing   `json:"comparing"`
	Summary     Summary     `json:"summary"`
	Diffs       []DiffEntry `json:"diffs"`
	Captures    []Capture   `json:"captures"`
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

// Comparing describes the scope the report was generated over. Empty
// Tables/ObjectTypes mean "all" (no filtering was applied).
type Comparing struct {
	Schemas     []string `json:"schemas"`
	Tables      []string `json:"tables"`
	ObjectTypes []string `json:"object_types"`
}

// Summary carries report-wide counters.
type Summary struct {
	ChangeCount  int  `json:"change_count"`
	CaptureCount int  `json:"capture_count"`
	LiveCompared bool `json:"live_compared"`
}

// DiffEntry is a single schema change, enriched with severity classification,
// human guidance, and the capture-pair window/phase it was detected in.
type DiffEntry struct {
	Seq       int                      `json:"seq"`
	Type      string                   `json:"type"` // string(schemadiff.DiffType)
	Object    schemasnapshot.ObjectRef `json:"object"`
	SubObject string                   `json:"sub_object,omitempty"`
	Status    string                   `json:"status"`
	Property  string                   `json:"property,omitempty"`
	OldValue  any                      `json:"old_value,omitempty"`
	NewValue  any                      `json:"new_value,omitempty"`
	Window    Window                   `json:"window"`
	Phase     string                   `json:"phase,omitempty"`
	Guidance  string                   `json:"guidance,omitempty"`
}

// Capture is a single point on the report's timeline: either a stored
// snapshot (Series == its capture label) or the live read of the source
// (Series == SeriesSourceLive).
type Capture struct {
	Seq        int       `json:"seq"`
	Series     string    `json:"series"`
	Reason     string    `json:"reason,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
}
