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

package schemadrift

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
			ChangeCount:        2,
			StoredCaptureCount: 2,
			LiveCompared:       true,
		},
		Drifts: []DriftEntry{
			{
				Diff: Diff{
					Type:       schemadiff.TableAdded,
					Operation:  schemadiff.OpAdded,
					ObjectType: schemadiff.ObjectTypeTable,
					Object:     schemasnapshot.ObjectRef{Schema: "public", Name: "invoices"},
				},
				Window:    Window{From: from, To: to},
				Phase:     "export data: running",
				DriftInfo: getDriftInfo(schemadiff.TableAdded),
			},
			{
				Diff: Diff{
					Type:       schemadiff.ColumnTypeChanged,
					Operation:  schemadiff.OpChanged,
					ObjectType: schemadiff.ObjectTypeColumn,
					Attribute:  schemadiff.AttrType,
					Object:     schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
					SubObject:  "amount",
					OldValue:   "integer",
					NewValue:   "numeric",
				},
				Window:    Window{From: from, To: to},
				Phase:     "export data: running",
				DriftInfo: getDriftInfo(schemadiff.ColumnTypeChanged),
			},
		},
		CapturePoints: []CapturePoint{
			{Label: schemasnapshot.LabelExportDataFromSourceStart, CapturedAt: from},
			{Label: schemasnapshot.LabelExportDataFromSourcePeriodic, CapturedAt: to},
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
	assert.Contains(t, got, "drifts")
	assert.Contains(t, got, "capture_points")

	summary, ok := got["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2), summary["change_count"])
	assert.Equal(t, float64(2), summary["stored_capture_count"])
	assert.Equal(t, true, summary["live_compared"])

	// The DriftInfo embed has to stay anonymous and untagged: a json tag on it
	// would nest severity/impact/action under a sub-object, which no consumer of
	// the flat shape would notice until it read a null.
	drifts, ok := got["drifts"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, drifts)
	firstDrift, ok := drifts[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(SeverityPotentialImpact), firstDrift["severity"])
	assert.Contains(t, firstDrift, "impact")
	assert.Contains(t, firstDrift, "action")
	assert.NotContains(t, firstDrift, "DriftInfo")

	// excluded is omitempty, so its absence has to be pinned too: otherwise a
	// renderer that dropped the field entirely would still pass.
	points, ok := got["capture_points"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, points)
	first, ok := points[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, first, "excluded", "a capture that was used omits the key")

	// Round-trip through the real struct too.
	var roundTripped Report
	require.NoError(t, json.Unmarshal(out, &roundTripped))
	assert.Equal(t, r.Report, roundTripped.Report)
	assert.Equal(t, r.Version, roundTripped.Version)
	require.Len(t, roundTripped.Drifts, 2)
	assert.Equal(t, r.Drifts[0].Type, roundTripped.Drifts[0].Type)
}

// TestRenderJSON_ExclusionIsSerialized covers the other side of excluded's
// omitempty: when the assembler bridged a capture, the JSON consumer must be able
// to tell that gap from a stretch that genuinely contributed nothing.
func TestRenderJSON_ExclusionIsSerialized(t *testing.T) {
	r := fixtureReport()
	r.CapturePoints[1].Excluded = "captured only sales, so it cannot answer for public"

	out, err := RenderJSON(r)
	require.NoError(t, err)

	var roundTripped Report
	require.NoError(t, json.Unmarshal(out, &roundTripped))
	require.Len(t, roundTripped.CapturePoints, 2)
	assert.Equal(t, r.CapturePoints[1].Excluded, roundTripped.CapturePoints[1].Excluded)
}

func TestRenderHTML(t *testing.T) {
	r := fixtureReport()

	out, err := RenderHTML(r)
	require.NoError(t, err)
	require.NotEmpty(t, out)

	html := string(out)
	assert.Contains(t, html, "db.example.internal", "source host should appear")
	assert.Contains(t, html, "⛔ Breaks the migration — recoverable", "severity label should appear")
	assert.Contains(t, html, "table added", "a diff kind label should appear")
	assert.Contains(t, html, "column type changed")
	assert.Contains(t, html, "public.orders.", "object qualifier should appear")
	assert.Contains(t, html, `<span class="s">amount</span>`, "object subject should appear")
	assert.Contains(t, html, "export data: running", "phase label should appear")
	assert.Contains(t, html, "does not apply any of these changes", "standing disclaimer banner should appear")
}

// TestObjectPathMinQuotesIdentifiers pins that rendered object identities are
// minimally quoted: a case-sensitive or space-containing identifier must render
// as valid, copy-pasteable SQL (sales."MixedCase"), never as the ambiguous
// sales.MixedCase, while an all-lowercase name stays unquoted.
func TestObjectPathMinQuotesIdentifiers(t *testing.T) {
	tests := []struct {
		name  string
		entry DriftEntry
		wantQ string
		wantS string
	}{
		{
			name: "lowercase table needs no quoting",
			entry: DriftEntry{
				Diff: Diff{
					ObjectType: schemadiff.ObjectTypeTable,
					Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "orders"},
				},
			},
			wantQ: "sales.", wantS: "orders",
		},
		{
			name: "mixed-case table is quoted",
			entry: DriftEntry{
				Diff: Diff{
					ObjectType: schemadiff.ObjectTypeTable,
					Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "MixedCase"},
				},
			},
			wantQ: "sales.", wantS: `"MixedCase"`,
		},
		{
			name: "column with a space, under a mixed-case table, quotes both parts",
			entry: DriftEntry{
				Diff: Diff{
					ObjectType: schemadiff.ObjectTypeColumn,
					Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "MixedCase"},
					SubObject:  "Extra Col",
				},
			},
			wantQ: `sales."MixedCase".`, wantS: `"Extra Col"`,
		},
		{
			name: "lowercase column under a lowercase table stays unquoted",
			entry: DriftEntry{
				Diff: Diff{
					ObjectType: schemadiff.ObjectTypeColumn,
					Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "orders"},
					SubObject:  "discount",
				},
			},
			wantQ: "sales.orders.", wantS: "discount",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, s, err := objectPath(tt.entry, "postgresql")
			require.NoError(t, err)
			assert.Equal(t, tt.wantQ, q)
			assert.Equal(t, tt.wantS, s)
			// q+s must equal the ref's own ForDisplay rendering.
			if tt.entry.ObjectType == schemadiff.ObjectTypeTable {
				assert.Equal(t, tt.entry.Object.ForDisplay("postgresql"), q+s)
			}
		})
	}
}

// TestRenderHTML_ExcludedCaptureIsVisible is the point of CapturePoint.Excluded: a
// capture the assembler bridged must appear on the timeline, and its reason in the
// footer, rather than vanishing into a stretch that reads as simply quiet.
//
// The excluded point is a THIRD capture appended after the two the fixture's drifts
// span, so the test also shows the surrounding findings still render.
func TestRenderHTML_ExcludedCaptureIsVisible(t *testing.T) {
	r := fixtureReport()
	r.CapturePoints = append(r.CapturePoints, CapturePoint{
		Label:      schemasnapshot.LabelExportDataFromSourcePeriodic,
		CapturedAt: r.CapturePoints[1].CapturedAt.Add(time.Hour),
		Excluded:   "captured only sales, so it cannot answer for public",
	})

	out, err := RenderHTML(r)
	require.NoError(t, err)
	html := string(out)

	assert.Contains(t, html, "not compared", "the spine must show the gap")
	assert.Contains(t, html, "cannot answer for public", "the footer must carry the reason")
	// The unrelated interval's findings must survive alongside it.
	assert.Contains(t, html, "table added")
	assert.Contains(t, html, "column type changed")
}

// TestRenderHTML_BridgedIntervalsRender pins the renderer against BuildReport's
// placeholder bridging: a failed capture is kept on the timeline but never
// diffed, so the interval around it spans two capture steps. Matching intervals
// on consecutive capture pairs drops such an interval entirely -- the banner
// still counts the change while the timeline renders nothing, and the
// "no changes" note does not fire either because the event markers are present.
func TestRenderHTML_BridgedIntervalsRender(t *testing.T) {
	before := time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC)
	failed := before.Add(time.Hour)
	after := failed.Add(time.Hour)

	t.Run("bridged findings reach the HTML", func(t *testing.T) {
		r := fixtureReport()
		r.CapturePoints = []CapturePoint{
			{Label: schemasnapshot.LabelExportSchema, CapturedAt: before},
			{Label: schemasnapshot.LabelExportDataFromSourceStart, CapturedAt: failed},
			{Label: schemasnapshot.LabelExportDataFromSourcePeriodic, CapturedAt: after},
		}
		// One finding, in the window that bridges the failed capture.
		r.Drifts = r.Drifts[:1]
		r.Drifts[0].Window = Window{From: before, To: after}
		r.Summary.ChangeCount = 1

		out, err := RenderHTML(r)
		require.NoError(t, err)
		html := string(out)

		assert.Contains(t, html, "table added", "the bridged interval's finding must render")
		assert.Contains(t, html, `<span class="s">invoices</span>`, "the bridged finding's object must render")
		assert.Contains(t, html, formatTime(before)+" → "+formatTime(after),
			"the interval must show the bridged window, spanning the failed capture")
		assert.NotContains(t, html, "No changes detected", "a report with a finding must not read as drift-free")
	})

	t.Run("an excluded middle capture is marked, and the bridged interval still renders", func(t *testing.T) {
		r := fixtureReport()
		r.CapturePoints = []CapturePoint{
			{Label: schemasnapshot.LabelExportSchema, CapturedAt: before},
			{
				Label:      schemasnapshot.LabelExportDataFromSourceStart,
				CapturedAt: failed,
				Excluded:   "captured only sales, so it cannot answer for public",
			},
			{Label: schemasnapshot.LabelExportDataFromSourcePeriodic, CapturedAt: after},
		}
		// The interval spans the excluded point, as it does a failed capture.
		for i := range r.Drifts {
			r.Drifts[i].Window = Window{From: before, To: after}
		}

		out, err := RenderHTML(r)
		require.NoError(t, err)
		html := string(out)

		assert.Contains(t, html, "not compared", "the excluded point must be marked")
		assert.Contains(t, html, "cannot answer for public")
		assert.Contains(t, html, "table added", "and the interval bridged across it must still render")
	})
}

func TestRenderHTML_EmptyReportDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		out, err := RenderHTML(Report{Report: "schema_drift", Version: 1})
		require.NoError(t, err)
		assert.NotEmpty(t, out)
	})
}

// The scope line states counts only: the report does not know whether a list
// flag narrowed the set, so it must not claim "all".
func TestComparingSummary(t *testing.T) {
	assert.Equal(t, "schema public · 2 tables · 1 object type", comparingSummary(Comparing{
		Schemas:     []string{"public"},
		Tables:      []string{"public.orders", "public.invoices"},
		ObjectTypes: []string{"TABLE"},
	}))
	assert.Equal(t, "no schemas · no tables · no object types", comparingSummary(Comparing{}))
}

// An added table's definition names its columns, and a case-sensitive one must
// stay quoted there too, or the printed definition is not valid SQL.
func TestRenderHTML_AddedTableColumnsAreMinQuoted(t *testing.T) {
	r := fixtureReport()
	r.Drifts = r.Drifts[:1]
	invoices := schemasnapshot.ObjectRef{Schema: "public", Name: "invoices"}
	r.Drifts[0].NewValue = schemasnapshot.Table{
		ObjectRef: invoices,
		Columns: []schemasnapshot.Column{
			{TableScopedObjectRef: schemasnapshot.TableScopedObjectRef{Table: invoices, Name: "InvoiceNo"}, DataType: "text", NotNull: true},
			{TableScopedObjectRef: schemasnapshot.TableScopedObjectRef{Table: invoices, Name: "amount"}, DataType: "numeric"},
		},
	}
	r.Summary.ChangeCount = 1

	out, err := RenderHTML(r)
	require.NoError(t, err)

	assert.Contains(t, string(out), "&#34;InvoiceNo&#34; text NOT NULL, amount numeric")
}

// An empty dimension compared nothing, so the dropdown must agree with the
// summary's "no tables" instead of offering an "all" chip.
func TestRenderHTML_EmptyScopeDimensionShowsNoChip(t *testing.T) {
	r := fixtureReport()
	r.Comparing = Comparing{Schemas: []string{"public"}, ObjectTypes: []string{"TABLE"}}

	out, err := RenderHTML(r)
	require.NoError(t, err)
	html := string(out)

	assert.Contains(t, html, "no tables")
	assert.Contains(t, html, `<span class="scope-chip">TABLE</span>`, "a non-empty dimension still lists its chips")
	assert.NotContains(t, html, `<span class="scope-chip">all</span>`)
}

// Each case is a report the HTML cannot show faithfully. Rendering past any of
// them would drop or misprint a finding in a page that still looks complete.
func TestRenderHTML_RejectsReportsItCannotDisplay(t *testing.T) {
	tests := []struct {
		name    string
		breakIt func(r *Report)
		wantErr string
	}{
		{
			name: "an interval no capture opens",
			breakIt: func(r *Report) {
				r.Drifts[0].Window.From = r.Drifts[0].Window.From.Add(-time.Hour)
			},
			wantErr: "no capture point opens it",
		},
		{
			name: "an interval no capture closes",
			breakIt: func(r *Report) {
				r.Drifts[0].Window.To = r.Drifts[0].Window.To.Add(time.Hour)
			},
			wantErr: "no capture point closes it",
		},
		{
			name:    "an unknown operation",
			breakIt: func(r *Report) { r.Drifts[0].Operation = "RENAMED" },
			wantErr: `unexpected operation "RENAMED"`,
		},
		{
			name:    "an unknown severity",
			breakIt: func(r *Report) { r.Drifts[0].Severity = "catastrophic" },
			wantErr: `unexpected severity "catastrophic"`,
		},
		{
			name:    "a value of an undocumented type",
			breakIt: func(r *Report) { r.Drifts[1].NewValue = 42 },
			wantErr: `unexpected value type int for attribute "TYPE"`,
		},
		{
			name:    "a column finding without a column",
			breakIt: func(r *Report) { r.Drifts[1].SubObject = "" },
			wantErr: "column finding on public.orders has no column name",
		},
		{
			name:    "a finding without an object name",
			breakIt: func(r *Report) { r.Drifts[0].Object.Name = "" },
			wantErr: "has an empty schema or name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fixtureReport()
			tt.breakIt(&r)
			_, err := RenderHTML(r)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// A capture label from a newer voyager only loses its timeline marker; failing
// the whole report over it would throw away every finding.
func TestRenderHTML_UnknownCaptureLabelStillRenders(t *testing.T) {
	r := fixtureReport()
	r.CapturePoints[0].Label = "import_data_start"

	out, err := RenderHTML(r)
	require.NoError(t, err)
	html := string(out)
	assert.Contains(t, html, "table added", "the findings must still render")
	assert.Contains(t, html, "import_data_start", "the footer still lists the capture")
}
