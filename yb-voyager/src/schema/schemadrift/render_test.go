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
			ChangeCount:   2,
			SnapshotCount: 2,
			LiveCompared:  true,
		},
		Diffs: []DiffEntry{
			{
				Seq:        1,
				Type:       string(schemadiff.TableAdded),
				Operation:  string(schemadiff.OpAdded),
				ObjectType: string(schemadiff.ObjectTypeTable),
				Object:     schemasnapshot.ObjectRef{Schema: "public", Name: "invoices"},
				Status:     string(StatusPotentialImpact),
				Window:     Window{From: from, To: to},
				Phase:      "export data: running",
				Impact:     classify(schemadiff.TableAdded).Impact,
				Action:     classify(schemadiff.TableAdded).Action,
			},
			{
				Seq:        2,
				Type:       string(schemadiff.ColumnTypeChanged),
				Operation:  string(schemadiff.OpChanged),
				ObjectType: string(schemadiff.ObjectTypeColumn),
				Attribute:  string(schemadiff.AttrType),
				Object:     schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
				SubObject:  "amount",
				Status:     string(StatusBreaksRecoverable),
				OldValue:   "integer",
				NewValue:   "numeric",
				Window:     Window{From: from, To: to},
				Phase:      "export data: running",
				Impact:     classify(schemadiff.ColumnTypeChanged).Impact,
				Action:     classify(schemadiff.ColumnTypeChanged).Action,
			},
		},
		Timeline: []Capture{
			{Series: schemasnapshot.LabelExportDataFromSourceStart, CapturedAt: from},
			{Series: schemasnapshot.LabelExportDataFromSourcePeriodic, CapturedAt: to},
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
	assert.Contains(t, got, "timeline")

	summary, ok := got["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2), summary["change_count"])
	assert.Equal(t, float64(2), summary["snapshot_count"])
	assert.Equal(t, true, summary["live_compared"])

	// skipped is omitempty, so its absence has to be pinned too: otherwise a
	// renderer that dropped the field entirely would still pass.
	assert.NotContains(t, got, "skipped", "no skipped intervals means the key is omitted")

	// Round-trip through the real struct too.
	var roundTripped Report
	require.NoError(t, json.Unmarshal(out, &roundTripped))
	assert.Equal(t, r.Report, roundTripped.Report)
	assert.Equal(t, r.Version, roundTripped.Version)
	require.Len(t, roundTripped.Diffs, 2)
	assert.Equal(t, r.Diffs[0].Type, roundTripped.Diffs[0].Type)
}

// TestRenderJSON_SkippedIntervalsAreSerialized covers the other side of
// skipped's omitempty: when the assembler declined a pair, the JSON consumer
// must be able to tell that interval from one that genuinely had no changes.
func TestRenderJSON_SkippedIntervalsAreSerialized(t *testing.T) {
	r := fixtureReport()
	r.Skipped = []SkippedInterval{{
		From:   schemasnapshot.LabelExportSchema,
		To:     schemasnapshot.LabelExportDataFromSourceStart,
		Window: Window{From: r.Timeline[0].CapturedAt, To: r.Timeline[1].CapturedAt},
		Reason: "the two captures cover different schemas, so they cannot be compared",
	}}

	out, err := RenderJSON(r)
	require.NoError(t, err)

	var roundTripped Report
	require.NoError(t, json.Unmarshal(out, &roundTripped))
	require.Len(t, roundTripped.Skipped, 1)
	assert.Equal(t, r.Skipped[0], roundTripped.Skipped[0])
}

func TestRenderHTML(t *testing.T) {
	r := fixtureReport()

	out, err := RenderHTML(r)
	require.NoError(t, err)
	require.NotEmpty(t, out)

	html := string(out)
	assert.Contains(t, html, "db.example.internal", "source host should appear")
	assert.Contains(t, html, "⛔ Breaks the migration — recoverable", "status action label should appear")
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
		entry DiffEntry
		wantQ string
		wantS string
	}{
		{
			name: "lowercase table needs no quoting",
			entry: DiffEntry{
				ObjectType: string(schemadiff.ObjectTypeTable),
				Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "orders"},
			},
			wantQ: "sales.", wantS: "orders",
		},
		{
			name: "mixed-case table is quoted",
			entry: DiffEntry{
				ObjectType: string(schemadiff.ObjectTypeTable),
				Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "MixedCase"},
			},
			wantQ: "sales.", wantS: `"MixedCase"`,
		},
		{
			name: "column with a space, under a mixed-case table, quotes both parts",
			entry: DiffEntry{
				ObjectType: string(schemadiff.ObjectTypeColumn),
				Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "MixedCase"},
				SubObject:  "Extra Col",
			},
			wantQ: `sales."MixedCase".`, wantS: `"Extra Col"`,
		},
		{
			name: "lowercase column under a lowercase table stays unquoted",
			entry: DiffEntry{
				ObjectType: string(schemadiff.ObjectTypeColumn),
				Object:     schemasnapshot.ObjectRef{Schema: "sales", Name: "orders"},
				SubObject:  "discount",
			},
			wantQ: "sales.orders.", wantS: "discount",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, s := objectPath(tt.entry, "postgresql")
			assert.Equal(t, tt.wantQ, q)
			assert.Equal(t, tt.wantS, s)
			// q+s must equal the ref's own ForDisplay rendering.
			if tt.entry.ObjectType == string(schemadiff.ObjectTypeTable) {
				assert.Equal(t, tt.entry.Object.ForDisplay("postgresql"), q+s)
			}
		})
	}
}

// TestRenderHTML_SkippedIntervalIsVisible is the point of Report.Skipped: a pair
// the assembler declined to compare must appear on the timeline with its reason,
// not vanish and read as a window that simply had no changes.
//
// The skipped pair is a THIRD capture appended after the two the fixture's diffs
// span. A skipped window never carries diffs (BuildReport bails before diffing),
// so overlaying it on the fixture's own window would both be an impossible report
// and hide whether the findings still render.
func TestRenderHTML_SkippedIntervalIsVisible(t *testing.T) {
	r := fixtureReport()
	skipFrom := r.Timeline[1].CapturedAt
	skipTo := skipFrom.Add(time.Hour)
	r.Timeline = append(r.Timeline, Capture{
		Series:     schemasnapshot.LabelExportDataFromSourcePeriodic,
		CapturedAt: skipTo,
	})
	r.Skipped = []SkippedInterval{{
		From:   schemasnapshot.LabelExportDataFromSourcePeriodic,
		To:     schemasnapshot.LabelExportDataFromSourcePeriodic,
		Window: Window{From: skipFrom, To: skipTo},
		Reason: "the two captures cover different schemas, so they cannot be compared",
	}}

	out, err := RenderHTML(r)
	require.NoError(t, err)
	html := string(out)

	assert.Contains(t, html, "not compared")
	assert.Contains(t, html, "cover different schemas")
	assert.Contains(t, html, `class="interval skipped"`)
	// The unrelated interval's findings must survive alongside the skipped block.
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
		r.Timeline = []Capture{
			{Series: schemasnapshot.LabelExportSchema, CapturedAt: before},
			{Series: schemasnapshot.LabelExportDataFromSourceStart, CapturedAt: failed},
			{Series: schemasnapshot.LabelExportDataFromSourcePeriodic, CapturedAt: after},
		}
		// One finding, in the window that bridges the failed capture.
		r.Diffs = r.Diffs[:1]
		r.Diffs[0].Window = Window{From: before, To: after}
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

	t.Run("bridged skipped interval reaches the HTML", func(t *testing.T) {
		r := fixtureReport()
		r.Timeline = []Capture{
			{Series: schemasnapshot.LabelExportSchema, CapturedAt: before},
			{Series: schemasnapshot.LabelExportDataFromSourceStart, CapturedAt: failed},
			{Series: schemasnapshot.LabelExportDataFromSourcePeriodic, CapturedAt: after},
		}
		r.Diffs = nil
		r.Summary.ChangeCount = 0
		r.Skipped = []SkippedInterval{{
			From:   schemasnapshot.LabelExportSchema,
			To:     schemasnapshot.LabelExportDataFromSourcePeriodic,
			Window: Window{From: before, To: after},
			Reason: "the two captures cover different schemas, so they cannot be compared",
		}}

		out, err := RenderHTML(r)
		require.NoError(t, err)
		html := string(out)

		assert.Contains(t, html, `class="interval skipped"`)
		assert.Contains(t, html, "cover different schemas")
	})
}

// TestRenderHTML_SkippedAndRealIntervalShareACapture covers the capture that closes a
// skipped interval and opens the next real one -- the normal shape after a scope
// mismatch, since BuildReport makes the mismatching snapshot the new baseline. Both
// blocks must render: buildTimeline's early continue is only safe because the two
// lookups are keyed on Window.From, where a capture appears at most once. Keyed on
// Window.To instead, the skipped block would swallow the findings that follow it.
func TestRenderHTML_SkippedAndRealIntervalShareACapture(t *testing.T) {
	a := fixtureContent(fixtureTable("1", "public", "orders"))
	b := fixtureContent(fixtureTable("1", "sales", "orders"))
	c := fixtureContent(
		fixtureTable("1", "sales", "orders"),
		fixtureTable("2", "sales", "customers"),
	)

	report := BuildReport(DetectionInput{
		Source: Source{DatabaseType: "postgresql"},
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "sales"), Content: b, Series: schemasnapshot.LabelExportDataFromSourceStart},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "sales"), Content: c, Series: schemasnapshot.LabelExportDataFromSourcePeriodic},
		},
	})
	require.Len(t, report.Skipped, 1, "the scope mismatch must be recorded")
	require.Len(t, report.Diffs, 1, "the pair after the mismatch must still be diffed")
	require.Equal(t, t2(), report.Skipped[0].Window.To, "the shared capture closes the skipped interval")
	require.Equal(t, t2(), report.Diffs[0].Window.From, "...and opens the real one")

	out, err := RenderHTML(report)
	require.NoError(t, err)
	html := string(out)

	assert.Contains(t, html, `class="interval skipped"`, "the skipped interval must render")
	assert.Contains(t, html, "table added", "the finding after it must render too")
	assert.Contains(t, html, `<span class="s">customers</span>`)
}

func TestRenderHTML_EmptyReportDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		out, err := RenderHTML(Report{Report: "schema_drift", Version: 1})
		require.NoError(t, err)
		assert.NotEmpty(t, out)
	})
}
