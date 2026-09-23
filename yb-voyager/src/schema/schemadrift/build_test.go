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
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// ─── fixture helpers ─────────────────────────────────────────────────────────

func objRef(schema, name string) schemasnapshot.ObjectRef {
	return schemasnapshot.ObjectRef{Schema: schema, Name: name}
}

// fixtureTable builds an ordinary table with the given ID/schema/name and,
// optionally, its nested columns (the new SnapshotContent model has each
// table carry its own Columns rather than a flat top-level list).
func fixtureTable(id, schema, name string, cols ...schemasnapshot.Column) schemasnapshot.Table {
	return schemasnapshot.Table{
		ObjectRef: objRef(schema, name),
		ID:        id,
		Kind:      schemasnapshot.TableKindOrdinary,
		Columns:   cols,
	}
}

// fixtureColumn builds a column nested under (tableSchema, tableName), for
// passing into fixtureTable's variadic cols.
func fixtureColumn(tableSchema, tableName, id, name, dataType string) schemasnapshot.Column {
	return schemasnapshot.Column{
		TableScopedObjectRef: schemasnapshot.TableScopedObjectRef{
			Table: objRef(tableSchema, tableName),
			Name:  name,
		},
		ID:       id,
		DataType: dataType,
	}
}

func fixtureContent(tables ...schemasnapshot.Table) *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		Tables:       tables,
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

// buildUnfiltered runs BuildReport the way the command does when no --*-list
// narrows the run: with a Scope naming every table in the fixtures and every
// object type v1 emits. Scope is an exact keep-set, so a zero value keeps nothing
// -- tests about intervals and phases say "unfiltered" here rather than repeating
// the universe. A database type is set because Comparing renders identifiers.
func buildUnfiltered(p DetectionConfig) Report {
	seen := make(map[schemasnapshot.ObjectRef]bool)
	for _, sn := range p.Snapshots {
		if sn.Content == nil {
			continue
		}
		for _, t := range sn.Content.Tables {
			if !seen[t.ObjectRef] {
				seen[t.ObjectRef] = true
				p.Scope.Tables = append(p.Scope.Tables, t.ObjectRef)
			}
		}
	}
	p.Scope.ObjectTypes = []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn}
	// Requested schemas default to the union of what the fixtures captured, so every
	// snapshot covers the request and the coverage rule stays out of the way. Tests
	// ABOUT coverage set Scope.Schemas themselves and call BuildReport directly.
	for _, sn := range p.Snapshots {
		for _, sch := range sn.Header.Schemas {
			if !lo.Contains(p.Scope.Schemas, sch) {
				p.Scope.Schemas = append(p.Scope.Schemas, sch)
			}
		}
	}
	if p.Source.DatabaseType == "" {
		p.Source.DatabaseType = "postgresql"
	}
	return BuildReport(p)
}

func t1() time.Time { return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC) }
func t2() time.Time { return time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC) }
func t3() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }
func t4() time.Time { return time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC) }

// ─── tests ────────────────────────────────────────────────────────────────

func TestBuildReport_ConsecutivePairsProduceDiffEntries(t *testing.T) {
	t1Content := fixtureContent(fixtureTable("1", "public", "orders"))
	t2Content := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: t1Content},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: t2Content},
		},
	}

	report := buildUnfiltered(p)

	require.Len(t, report.Drifts, 1)
	d := report.Drifts[0]
	assert.Equal(t, schemadiff.TableAdded, d.Type)
	assert.Equal(t, objRef("public", "customers"), d.Object)
	assert.Equal(t, SeverityPotentialImpact, d.Severity)
	assert.Equal(t, Window{From: t1(), To: t2()}, d.Window)
	assert.Equal(t, "export data: pending", d.Phase)
	assert.Equal(t, getDriftInfo(schemadiff.TableAdded).Impact, d.Impact)
	assert.Equal(t, getDriftInfo(schemadiff.TableAdded).Action, d.Action)
}

func TestBuildReport_StampsGeneratedAt(t *testing.T) {
	before := time.Now().UTC()
	report := BuildReport(DetectionConfig{})
	after := time.Now().UTC()

	// Bounded both ways: the lower bound also fails the zero value a dropped
	// stamp would leave behind.
	assert.False(t, report.GeneratedAt.Before(before))
	assert.False(t, report.GeneratedAt.After(after))
	assert.Equal(t, time.UTC, report.GeneratedAt.Location())
}

func TestBuildReport_ZeroDiffIntervalProducesNoEntries(t *testing.T) {
	base := fixtureContent(fixtureTable("1", "public", "orders"))
	changed := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: base},
			// identical content: zero-diff interval
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: base},
			// a real change in the second interval
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: changed},
		},
	}

	report := buildUnfiltered(p)

	require.Len(t, report.Drifts, 1)
	assert.Equal(t, Window{From: t2(), To: t3()}, report.Drifts[0].Window)
	assert.Equal(t, "export data: running", report.Drifts[0].Phase)
}

func TestBuildReport_PlaceholderBridgesToNextContentBearingSnapshot(t *testing.T) {
	// a and c differ (c adds a table); the placeholder sitting between them
	// must not suppress that drift — it should be bridged across.
	a := fixtureContent(fixtureTable("1", "public", "orders"))
	c := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: nil},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: c},
		},
	}

	report := buildUnfiltered(p)

	require.Len(t, report.Drifts, 1, "drift between a and c must be reported, bridged across the placeholder")
	d := report.Drifts[0]
	assert.Equal(t, schemadiff.TableAdded, d.Type)
	assert.Equal(t, objRef("public", "customers"), d.Object)
	assert.Equal(t, Window{From: t1(), To: t3()}, d.Window, "window must span from the pre-placeholder snapshot to the post-placeholder snapshot")
	require.Len(t, report.CapturePoints, 3, "the placeholder itself still appears as a capture point")
	assert.Equal(t, 3, report.Summary.StoredCaptureCount,
		"a placeholder is a persisted row, so it counts as a stored capture even though no snapshot is behind it")
}

func TestBuildReport_PlaceholderAtChainEndProducesNoExtraEntries(t *testing.T) {
	a := fixtureContent(fixtureTable("1", "public", "orders"))

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: nil},
		},
	}

	report := buildUnfiltered(p)

	assert.Empty(t, report.Drifts, "a trailing placeholder with nothing after it contributes no diffs")
	require.Len(t, report.CapturePoints, 2)
}

func TestBuildReport_NonCoveringCaptureIsExcludedAndBridged(t *testing.T) {
	// Drift in public straddles a capture that only covered "other". Requesting
	// public, that middle capture cannot answer -- a public table missing from it
	// would mean "nobody looked", not "dropped" -- so it is bridged, and the two
	// captures either side of it are compared to each other.
	before := fixtureContent(fixtureTable("1", "public", "orders"))
	middle := fixtureContent(fixtureTable("9", "other", "unrelated"))
	after := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)

	report := BuildReport(DetectionConfig{
		Source: Source{DatabaseType: "postgresql"},
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: before},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "other"), Content: middle},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: after},
		},
		Scope: schemadiff.Scope{
			Schemas:     []string{"public"},
			Tables:      []schemasnapshot.ObjectRef{objRef("public", "orders"), objRef("public", "customers")},
			ObjectTypes: []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
		},
	})

	require.Len(t, report.Drifts, 1, "the drift must be found across the bridged capture")
	assert.Equal(t, objRef("public", "customers"), report.Drifts[0].Object)
	assert.Equal(t, Window{From: t1(), To: t3()}, report.Drifts[0].Window,
		"the window spans the bridged capture, as it does for a failed one")

	// The gap still has to be visible, or a reader cannot tell this from a clean run.
	require.Len(t, report.CapturePoints, 3)
	assert.Empty(t, report.CapturePoints[0].Excluded)
	assert.NotEmpty(t, report.CapturePoints[1].Excluded, "the non-covering capture must say why it was skipped")
	assert.Contains(t, report.CapturePoints[1].Excluded, "other")
	assert.Empty(t, report.CapturePoints[2].Excluded)
}

func TestBuildReport_SchemaScopeOrderInsensitive(t *testing.T) {
	a := fixtureContent(fixtureTable("1", "public", "orders"))
	b := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public", "sales"), Content: a},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "sales", "public"), Content: b},
		},
	}

	report := buildUnfiltered(p)

	require.Len(t, report.Drifts, 1, "same schema set in a different order must still be diffed")
}

func TestBuildReport_LivePairPhaseIsSinceLastCapture(t *testing.T) {
	a := fixtureContent(fixtureTable("1", "public", "orders"))
	live := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t1(), "public"), Content: a},
			{
				Header:  fixtureHeader(schemasnapshot.LabelSourceLive, t2(), "public"),
				Content: live,
			},
		},
	}

	report := buildUnfiltered(p)

	require.Len(t, report.Drifts, 1)
	assert.Equal(t, "since last capture", report.Drifts[0].Phase)
}

func TestBuildReport_SummaryCounts(t *testing.T) {
	a := fixtureContent(fixtureTable("1", "public", "orders"))
	b := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)
	live := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
		fixtureTable("3", "public", "invoices"),
	)

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: b},
			{
				Header:  fixtureHeader(schemasnapshot.LabelSourceLive, t3(), "public"),
				Content: live,
			},
		},
	}

	report := buildUnfiltered(p)

	assert.Equal(t, 2, report.Summary.StoredCaptureCount, "StoredCaptureCount counts the stored records only, not the live read")
	assert.Equal(t, 2, report.Summary.ChangeCount, "one TABLE_ADDED per interval (customers, then invoices)")
	assert.True(t, report.Summary.LiveCompared)
}

func TestBuildReport_SchemaFilterKeepsOnlyRequestedSchemas(t *testing.T) {
	// Both captures cover both schemas, and both schemas drifted. Asking only about
	// public must report only public -- before the schema dimension existed, a run
	// narrowed with --source-db-schema still reported everything the snapshots held.
	before := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("10", "sales", "customers"),
	)
	after := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "items"),
		fixtureTable("10", "sales", "customers"),
		fixtureTable("11", "sales", "invoices"),
	)

	report := BuildReport(DetectionConfig{
		Source: Source{DatabaseType: "postgresql"},
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public", "sales"), Content: before},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public", "sales"), Content: after},
		},
		Scope: schemadiff.Scope{
			Schemas: []string{"public"},
			Tables: []schemasnapshot.ObjectRef{
				objRef("public", "orders"), objRef("public", "items"),
				objRef("sales", "customers"), objRef("sales", "invoices"),
			},
			ObjectTypes: []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
		},
	})

	require.Len(t, report.Drifts, 1, "sales.invoices was added too, but sales was not requested")
	assert.Equal(t, objRef("public", "items"), report.Drifts[0].Object)
	assert.Equal(t, 1, report.Summary.ChangeCount,
		"the count drives the exit code, so it must not include out-of-scope drift")
	assert.Equal(t, []string{"public"}, report.Comparing.Schemas)
}

func TestBuildReport_ComparedIntervalCount(t *testing.T) {
	content := fixtureContent(fixtureTable("1", "public", "orders"))
	drifted := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "items"),
	)
	scope := schemadiff.Scope{
		Schemas:     []string{"public"},
		Tables:      []schemasnapshot.ObjectRef{objRef("public", "orders"), objRef("public", "items")},
		ObjectTypes: []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
	}

	t.Run("two usable snapshots is one interval", func(t *testing.T) {
		report := BuildReport(DetectionConfig{
			Source: Source{DatabaseType: "postgresql"},
			Snapshots: []schemasnapshot.SchemaSnapshot{
				{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: content},
				{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: drifted},
			},
			Scope: scope,
		})
		assert.Equal(t, 1, report.Summary.ComparedIntervalCount)
		assert.Equal(t, 1, report.Summary.ChangeCount)
	})

	t.Run("every stored capture failed, so nothing was compared", func(t *testing.T) {
		// The distinction this field exists for: ChangeCount is 0 here, but so is
		// the number of intervals examined. StoredCaptureCount says 2, because a
		// placeholder is a persisted row -- it cannot tell the reader that nothing
		// was looked at.
		report := BuildReport(DetectionConfig{
			Source: Source{DatabaseType: "postgresql"},
			Snapshots: []schemasnapshot.SchemaSnapshot{
				{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: nil},
				{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: nil},
				{Header: fixtureHeader(schemasnapshot.LabelSourceLive, t3(), "public"), Content: content},
			},
			Scope: scope,
		})
		assert.Equal(t, 0, report.Summary.ComparedIntervalCount, "one usable point cannot form an interval")
		assert.Equal(t, 0, report.Summary.ChangeCount)
		assert.Equal(t, 2, report.Summary.StoredCaptureCount, "placeholders still count as stored records")
		assert.False(t, report.Summary.LiveCompared)
	})

	t.Run("a bridged capture does not add an interval", func(t *testing.T) {
		report := BuildReport(DetectionConfig{
			Source: Source{DatabaseType: "postgresql"},
			Snapshots: []schemasnapshot.SchemaSnapshot{
				{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: content},
				{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "sales"), Content: content},
				{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: drifted},
			},
			Scope: scope,
		})
		assert.Equal(t, 1, report.Summary.ComparedIntervalCount,
			"three points with one bridged is still a single interval, not two")
		assert.Equal(t, 1, report.Summary.ChangeCount)
	})
}

func TestBuildReport_LiveComparedMeansActuallyCompared(t *testing.T) {
	content := fixtureContent(fixtureTable("1", "public", "orders"))

	t.Run("compared against a covering stored capture", func(t *testing.T) {
		report := buildUnfiltered(DetectionConfig{
			Snapshots: []schemasnapshot.SchemaSnapshot{
				{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: content},
				{Header: fixtureHeader(schemasnapshot.LabelSourceLive, t2(), "public"), Content: content},
			},
		})
		assert.True(t, report.Summary.LiveCompared)
	})

	t.Run("live read had nothing to compare against", func(t *testing.T) {
		// The only stored capture failed, so the live read becomes the baseline and
		// is never diffed. Reporting LiveCompared here would tell the user their
		// source was checked against history when it was not.
		report := buildUnfiltered(DetectionConfig{
			Snapshots: []schemasnapshot.SchemaSnapshot{
				{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: nil},
				{Header: fixtureHeader(schemasnapshot.LabelSourceLive, t2(), "public"), Content: content},
			},
		})
		assert.False(t, report.Summary.LiveCompared,
			"a live read with no baseline was not compared, whatever the summary used to say")
		assert.Equal(t, 1, report.Summary.StoredCaptureCount, "the failed capture is still a stored record")
	})
}

func TestBuildReport_SummaryLiveComparedFalseWhenNoLive(t *testing.T) {
	a := fixtureContent(fixtureTable("1", "public", "orders"))
	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a},
		},
	}

	report := buildUnfiltered(p)

	assert.False(t, report.Summary.LiveCompared)
	assert.Equal(t, 1, report.Summary.StoredCaptureCount)
}

func TestBuildReport_WindowFromToReflectFirstAndLastCapture(t *testing.T) {
	a := fixtureContent(fixtureTable("1", "public", "orders"))
	b := fixtureContent(fixtureTable("1", "public", "orders"))
	live := fixtureContent(fixtureTable("1", "public", "orders"))

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: a},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: b},
			{
				Header:  fixtureHeader(schemasnapshot.LabelSourceLive, t4(), "public"),
				Content: live,
			},
		},
	}

	report := buildUnfiltered(p)

	assert.Equal(t, t1(), report.Window.From)
	assert.Equal(t, t4(), report.Window.To)
}

func TestBuildReport_EmptyInputsProduceZeroValueWindowNoPanic(t *testing.T) {
	require.NotPanics(t, func() {
		report := BuildReport(DetectionConfig{})
		assert.Empty(t, report.CapturePoints)
		assert.Empty(t, report.Drifts)
		assert.True(t, report.Window.From.IsZero())
		assert.True(t, report.Window.To.IsZero())
		assert.Equal(t, 0, report.Summary.StoredCaptureCount)
		assert.False(t, report.Summary.LiveCompared)
	})
}

func TestBuildReport_DriftsFromEveryIntervalAreReported(t *testing.T) {
	// interval 1: two new tables added
	s1 := fixtureContent(fixtureTable("1", "public", "orders"))
	s2 := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
		fixtureTable("3", "public", "invoices"),
	)
	// interval 2: one more table added
	s3 := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
		fixtureTable("3", "public", "invoices"),
		fixtureTable("4", "public", "payments"),
	)

	p := DetectionConfig{
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: s1},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: s2},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourcePeriodic, t3(), "public"), Content: s3},
		},
	}

	report := buildUnfiltered(p)

	// Two findings from the first interval and one from the second, each attributed
	// to the interval it was detected in.
	require.Len(t, report.Drifts, 3)
	var windows []Window
	for _, d := range report.Drifts {
		windows = append(windows, d.Window)
	}
	assert.Equal(t, []Window{
		{From: t1(), To: t2()},
		{From: t1(), To: t2()},
		{From: t2(), To: t3()},
	}, windows)
}

func TestBuildReport_ScopeFilteringKeepsOnlyListedTable(t *testing.T) {
	before := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers"),
	)
	after := fixtureContent(
		fixtureTable("1", "public", "orders"),
		fixtureTable("2", "public", "customers", fixtureColumn("public", "customers", "2:2", "email", "text")),
		fixtureTable("3", "public", "invoices"),
	)

	// detect-drift only ever populates the include lists (it resolves any
	// --exclude-* form into a positive allow-list first): listing only
	// "invoices" keeps its TABLE_ADDED finding while filtering out customers'
	// COLUMN_ADDED finding, even though customers also changed in this interval.
	// Both dimensions are stated: Scope holds the exact set to keep, so leaving
	// ObjectTypes empty would keep nothing rather than every type.
	scope := schemadiff.Scope{
		Schemas:     []string{"public"},
		Tables:      []schemasnapshot.ObjectRef{objRef("public", "invoices")},
		ObjectTypes: []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
	}

	p := DetectionConfig{
		Source: Source{DatabaseType: "postgresql"},
		Snapshots: []schemasnapshot.SchemaSnapshot{
			{Header: fixtureHeader(schemasnapshot.LabelExportSchema, t1(), "public"), Content: before},
			{Header: fixtureHeader(schemasnapshot.LabelExportDataFromSourceStart, t2(), "public"), Content: after},
		},
		Scope:          scope,
		TablesFiltered: true,
	}

	report := BuildReport(p)

	require.Len(t, report.Drifts, 1, "only the allow-listed table's finding must appear")
	assert.Equal(t, objRef("public", "invoices"), report.Drifts[0].Object)

	// Comparing is rendered from Scope, so it cannot disagree with what was filtered.
	assert.Equal(t, []string{"public.invoices"}, report.Comparing.Tables)
	assert.Equal(t, []string{"TABLE", "COLUMN"}, report.Comparing.ObjectTypes)
	assert.True(t, report.Comparing.TablesFiltered)
	assert.False(t, report.Comparing.ObjectTypesFiltered, "object types were not narrowed, even though the set is stated")
}

func TestPhaseFor(t *testing.T) {
	cases := []struct {
		name       string
		prev, next string
		want       string
	}{
		{
			name: "export schema -> export data start",
			prev: schemasnapshot.LabelExportSchema,
			next: schemasnapshot.LabelExportDataFromSourceStart,
			want: "export data: pending",
		},
		{
			name: "export data start -> periodic",
			prev: schemasnapshot.LabelExportDataFromSourceStart,
			next: schemasnapshot.LabelExportDataFromSourcePeriodic,
			want: "export data: running",
		},
		{
			name: "periodic -> periodic",
			prev: schemasnapshot.LabelExportDataFromSourcePeriodic,
			next: schemasnapshot.LabelExportDataFromSourcePeriodic,
			want: "export data: running",
		},
		{
			name: "export data start -> exit: the export was running for that span",
			prev: schemasnapshot.LabelExportDataFromSourceStart,
			next: schemasnapshot.LabelExportDataFromSourceExit,
			want: "export data: running",
		},
		{
			name: "periodic -> exit: same span, regardless of how the run ended",
			prev: schemasnapshot.LabelExportDataFromSourcePeriodic,
			next: schemasnapshot.LabelExportDataFromSourceExit,
			want: "export data: running",
		},
		{
			name: "export data exit -> export data start (paused)",
			prev: schemasnapshot.LabelExportDataFromSourceExit,
			next: schemasnapshot.LabelExportDataFromSourceStart,
			want: "export data: paused",
		},
		{
			name: "anything -> live",
			prev: schemasnapshot.LabelExportDataFromSourcePeriodic,
			next: schemasnapshot.LabelSourceLive,
			want: "since last capture",
		},
		{
			name: "export schema -> live",
			prev: schemasnapshot.LabelExportSchema,
			next: schemasnapshot.LabelSourceLive,
			want: "since last capture",
		},
		{
			name: "fallback: unrelated pair",
			prev: schemasnapshot.LabelExportSchema,
			next: schemasnapshot.LabelExportDataFromSourceExit,
			want: "",
		},
		{
			name: "fallback: export data exit -> periodic (not a defined transition)",
			prev: schemasnapshot.LabelExportDataFromSourceExit,
			next: schemasnapshot.LabelExportDataFromSourcePeriodic,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prev := CapturePoint{Series: tc.prev}
			next := CapturePoint{Series: tc.next}
			assert.Equal(t, tc.want, phaseFor(prev, next))
		})
	}
}
