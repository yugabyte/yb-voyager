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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// ─── fixture helpers ─────────────────────────────────────────────────────────

func objRef(schema, name string) schemasnapshot.ObjectRef {
	return schemasnapshot.ObjectRef{Schema: schema, Name: name}
}

func fixtureTable(id, schema, name string) schemasnapshot.Table {
	return schemasnapshot.Table{
		ObjectRef: objRef(schema, name),
		ID:        id,
		Kind:      schemasnapshot.TableKindOrdinary,
	}
}

func fixtureColumn(schema, tableName, id, name, dataType string) schemasnapshot.Column {
	return schemasnapshot.Column{
		Table:    objRef(schema, tableName),
		ID:       id,
		Name:     name,
		DataType: dataType,
	}
}

func fixtureContent(tables []schemasnapshot.Table, cols []schemasnapshot.Column) *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Tables:       tables,
		Columns:      cols,
	}
}

func fixtureHeader(label string, capturedAt time.Time, schemas ...string) schemasnapshot.SnapshotHeader {
	return schemasnapshot.SnapshotHeader{
		Label:      label,
		Side:       schemasnapshot.SideSource,
		CapturedAt: capturedAt,
		Schemas:    schemas,
	}
}

func t1() time.Time { return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC) }
func t2() time.Time { return time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC) }
func t3() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }
func t4() time.Time { return time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC) }

// ─── tests ────────────────────────────────────────────────────────────────

func TestBuildReport_ConsecutivePairsProduceDiffEntries(t *testing.T) {
	t1Content := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	t2Content := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: t1Content, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: t2Content, Series: schemasnapshot.LabelExportDataFromSourceStart},
		},
		GeneratedAt: t2(),
	}

	report := BuildReport(p)

	require.Len(t, report.Diffs, 1)
	d := report.Diffs[0]
	assert.Equal(t, 1, d.Seq)
	assert.Equal(t, string(schemadiff.TableAdded), d.Type)
	assert.Equal(t, objRef("public", "customers"), d.Object)
	assert.Equal(t, string(StatusPotentialImpact), d.Status)
	assert.Equal(t, Window{From: t1(), To: t2()}, d.Window)
	assert.Equal(t, "export data: pending", d.Phase)
	assert.Equal(t, Guidance(schemadiff.TableAdded), d.Guidance)
}

func TestBuildReport_EmptyIntervalsProduceNoEntriesButKeepSequencing(t *testing.T) {
	base := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	changed := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: base, Series: schemasnapshot.LabelExportSchema},
			// identical content: zero-diff interval
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: base, Series: schemasnapshot.LabelExportDataFromSourceStart},
			// a real change in the second interval
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: changed, Series: schemasnapshot.LabelExportDataFromSourcePeriodic},
		},
	}

	report := BuildReport(p)

	require.Len(t, report.Diffs, 1)
	assert.Equal(t, 1, report.Diffs[0].Seq, "seq should start at 1 even though the first interval had zero diffs")
	assert.Equal(t, Window{From: t2(), To: t3()}, report.Diffs[0].Window)
	assert.Equal(t, "export data: running", report.Diffs[0].Phase)
}

func TestBuildReport_PlaceholderPairSkippedEntirely(t *testing.T) {
	a := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	c := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: nil, Series: schemasnapshot.LabelExportDataFromSourceStart},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: c, Series: schemasnapshot.LabelExportDataFromSourcePeriodic},
		},
	}

	report := BuildReport(p)

	assert.Empty(t, report.Diffs, "both pairs touching the placeholder must be skipped, even though a and c differ")
	require.Len(t, report.Captures, 3, "the placeholder itself still appears as a capture point")
}

func TestBuildReport_SchemaScopeMismatchSkippedEntirely(t *testing.T) {
	a := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	b := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "other"), Content: b, Series: schemasnapshot.LabelExportDataFromSourceStart},
		},
	}

	report := BuildReport(p)

	assert.Empty(t, report.Diffs)
}

func TestBuildReport_SchemaScopeOrderInsensitive(t *testing.T) {
	a := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	b := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public", "sales"), Content: a, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "sales", "public"), Content: b, Series: schemasnapshot.LabelExportDataFromSourceStart},
		},
	}

	report := BuildReport(p)

	require.Len(t, report.Diffs, 1, "same schema set in a different order must still be diffed")
}

func TestBuildReport_LivePairPhaseIsSinceLastCapture(t *testing.T) {
	a := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	live := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t1(), "public"), Content: a, Series: schemasnapshot.LabelExportDataFromSourcePeriodic},
		},
		Live: &SnapshotInput{
			Header:  fixtureHeader("", t2(), "public"),
			Content: live,
			Series:  SeriesSourceLive,
		},
	}

	report := BuildReport(p)

	require.Len(t, report.Diffs, 1)
	assert.Equal(t, "since last capture", report.Diffs[0].Phase)
}

func TestBuildReport_SummaryCounts(t *testing.T) {
	a := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	b := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)
	live := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
		fixtureTable("3", "public", "invoices"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: b, Series: schemasnapshot.LabelExportDataFromSourceStart},
		},
		Live: &SnapshotInput{
			Header:  fixtureHeader("", t3(), "public"),
			Content: live,
			Series:  SeriesSourceLive,
		},
	}

	report := BuildReport(p)

	assert.Equal(t, 2, report.Summary.CaptureCount, "CaptureCount counts stored snapshots only, not the live read")
	assert.Equal(t, 2, report.Summary.ChangeCount, "one TABLE_ADDED per interval (customers, then invoices)")
	assert.True(t, report.Summary.LiveCompared)
}

func TestBuildReport_SummaryLiveComparedFalseWhenNoLive(t *testing.T) {
	a := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a, Series: schemasnapshot.LabelExportSchema},
		},
	}

	report := BuildReport(p)

	assert.False(t, report.Summary.LiveCompared)
	assert.Equal(t, 1, report.Summary.CaptureCount)
}

func TestBuildReport_WindowFromToReflectFirstAndLastCapture(t *testing.T) {
	a := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	b := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	live := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: b, Series: schemasnapshot.LabelExportDataFromSourceStart},
		},
		Live: &SnapshotInput{
			Header:  fixtureHeader("", t4(), "public"),
			Content: live,
			Series:  SeriesSourceLive,
		},
	}

	report := BuildReport(p)

	assert.Equal(t, t1(), report.Window.From)
	assert.Equal(t, t4(), report.Window.To)
}

func TestBuildReport_EmptyInputsProduceZeroValueWindowNoPanic(t *testing.T) {
	require.NotPanics(t, func() {
		report := BuildReport(BuildParams{})
		assert.Empty(t, report.Captures)
		assert.Empty(t, report.Diffs)
		assert.True(t, report.Window.From.IsZero())
		assert.True(t, report.Window.To.IsZero())
		assert.Equal(t, 0, report.Summary.CaptureCount)
		assert.False(t, report.Summary.LiveCompared)
	})
}

func TestBuildReport_GlobalSeqNumberingIsSequentialAcrossIntervals(t *testing.T) {
	// interval 1: two new tables added
	s1 := fixtureContent([]schemasnapshot.Table{fixtureTable("1", "public", "orders")}, nil)
	s2 := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
		fixtureTable("3", "public", "invoices"),
	}, nil)
	// interval 2: one more table added
	s3 := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
		fixtureTable("3", "public", "invoices"),
		fixtureTable("4", "public", "payments"),
	}, nil)

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: s1, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: s2, Series: schemasnapshot.LabelExportDataFromSourceStart},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: s3, Series: schemasnapshot.LabelExportDataFromSourcePeriodic},
		},
	}

	report := BuildReport(p)

	require.Len(t, report.Diffs, 3)
	var seqs []int
	for _, d := range report.Diffs {
		seqs = append(seqs, d.Seq)
	}
	assert.Equal(t, []int{1, 2, 3}, seqs)
}

func TestBuildReport_ScopeFilteringExcludesTable(t *testing.T) {
	before := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	}, nil)
	after := fixtureContent([]schemasnapshot.Table{
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
		fixtureTable("3", "public", "invoices"),
	}, []schemasnapshot.Column{
		fixtureColumn("public", "customers", "2:2", "email", "text"),
	})

	scope := schemadiff.Scope{
		ExcludeTables: []schemasnapshot.ObjectRef{objRef("public", "customers")},
	}

	p := BuildParams{
		Snapshots: []SnapshotInput{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: before, Series: schemasnapshot.LabelExportSchema},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: after, Series: schemasnapshot.LabelExportDataFromSourceStart},
		},
		Scope: scope,
	}

	report := BuildReport(p)

	require.Len(t, report.Diffs, 1, "the excluded table's column-added finding must not appear")
	assert.Equal(t, objRef("public", "invoices"), report.Diffs[0].Object)
}
