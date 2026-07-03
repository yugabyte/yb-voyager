//go:build unit

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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

func fixtureReport() Report {
	generatedAt := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	from := time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 15, 8, 0, 0, 0, time.UTC)

	return Report{
		Report:      "schema_drift",
		Version:     1,
		GeneratedAt: generatedAt,
		Source: Source{
			DatabaseType:    "postgresql",
			Host:            "db.example.internal",
			Port:            5432,
			Database:        "orders_db",
			DatabaseVersion: "16.4",
		},
		Window: Window{From: from, To: to},
		Comparing: Comparing{
			Schemas: []string{"public"},
		},
		Summary: Summary{
			ChangeCount:  2,
			CaptureCount: 2,
			LiveCompared: true,
		},
		Diffs: []DiffEntry{
			{
				Seq:      1,
				Type:     string(schemadiff.TableAdded),
				Object:   schemasnapshot.ObjectRef{Schema: "public", Name: "invoices"},
				Status:   string(StatusPotentialImpact),
				Window:   Window{From: from, To: to},
				Phase:    "export data: running",
				Guidance: Guidance(schemadiff.TableAdded),
			},
			{
				Seq:       2,
				Type:      string(schemadiff.ColumnTypeChanged),
				Object:    schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
				SubObject: "amount",
				Status:    string(StatusBreaksRecoverable),
				Property:  "data_type",
				OldValue:  "integer",
				NewValue:  "numeric",
				Window:    Window{From: from, To: to},
				Phase:     "export data: running",
				Guidance:  Guidance(schemadiff.ColumnTypeChanged),
			},
		},
		Captures: []Capture{
			{Seq: 1, Series: schemasnapshot.LabelExportDataFromSourceStart, CapturedAt: from},
			{Seq: 2, Series: schemasnapshot.LabelExportDataFromSourcePeriodic, CapturedAt: to},
		},
	}
}

func TestRenderJSON(t *testing.T) {
	r := fixtureReport()

	out, err := RenderJSON(r)
	require.NoError(t, err)
	require.NotEmpty(t, out)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))

	assert.Equal(t, "schema_drift", got["report"])
	assert.Equal(t, float64(1), got["version"])
	assert.Contains(t, got, "generated_at")
	assert.Contains(t, got, "source")
	assert.Contains(t, got, "window")
	assert.Contains(t, got, "comparing")
	assert.Contains(t, got, "summary")
	assert.Contains(t, got, "diffs")
	assert.Contains(t, got, "captures")

	summary, ok := got["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2), summary["change_count"])
	assert.Equal(t, float64(2), summary["capture_count"])
	assert.Equal(t, true, summary["live_compared"])

	// Round-trip through the real struct too.
	var roundTripped Report
	require.NoError(t, json.Unmarshal(out, &roundTripped))
	assert.Equal(t, r.Report, roundTripped.Report)
	assert.Equal(t, r.Version, roundTripped.Version)
	require.Len(t, roundTripped.Diffs, 2)
	assert.Equal(t, r.Diffs[0].Type, roundTripped.Diffs[0].Type)
}

func TestRenderHTML(t *testing.T) {
	r := fixtureReport()

	out, err := RenderHTML(r)
	require.NoError(t, err)
	require.NotEmpty(t, out)

	html := string(out)
	assert.Contains(t, html, "db.example.internal", "source host should appear")
	assert.Contains(t, html, "breaks_migration_recoverable", "status badge label should appear")
	assert.Contains(t, html, "TABLE_ADDED", "a diff type string should appear")
	assert.Contains(t, html, "COLUMN_TYPE_CHANGED")
	assert.Contains(t, html, "export data: running", "phase label should appear")
	assert.Contains(t, html, "informational only", "standing disclaimer footer should appear")
}

func TestRenderHTML_EmptyReportDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		out, err := RenderHTML(Report{Report: "schema_drift", Version: 1})
		require.NoError(t, err)
		assert.NotEmpty(t, out)
	})
}
