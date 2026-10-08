//go:build unit

/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/google/uuid"
	pgconnv5 "github.com/jackc/pgx/v5/pgconn"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/errs"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// ─── complementDriftTableRefs ────────────────────────────────────────────────

func TestComplementDriftTableRefs(t *testing.T) {
	refA := schemasnapshot.ObjectRef{Schema: "public", Name: "a"}
	refB := schemasnapshot.ObjectRef{Schema: "public", Name: "b"}
	refC := schemasnapshot.ObjectRef{Schema: "public", Name: "c"}
	refD := schemasnapshot.ObjectRef{Schema: "other", Name: "d"} // not in universe

	universe := []schemasnapshot.ObjectRef{refA, refB, refC}

	tests := []struct {
		name     string
		exclude  []schemasnapshot.ObjectRef
		wantRefs []schemasnapshot.ObjectRef
	}{
		{
			name:     "exclude a subset keeps the rest",
			exclude:  []schemasnapshot.ObjectRef{refB},
			wantRefs: []schemasnapshot.ObjectRef{refA, refC},
		},
		{
			name:     "exclude nothing keeps all",
			exclude:  nil,
			wantRefs: []schemasnapshot.ObjectRef{refA, refB, refC},
		},
		{
			name:     "exclude everything yields empty",
			exclude:  []schemasnapshot.ObjectRef{refA, refB, refC},
			wantRefs: nil,
		},
		{
			name:     "exclude a ref not in the universe is a no-op",
			exclude:  []schemasnapshot.ObjectRef{refD},
			wantRefs: []schemasnapshot.ObjectRef{refA, refB, refC},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := complementDriftTableRefs(universe, tt.exclude)
			assert.Equal(t, tt.wantRefs, got)
		})
	}
}

// ─── complementDriftObjectTypes ──────────────────────────────────────────────

func TestComplementDriftObjectTypes(t *testing.T) {
	tests := []struct {
		name    string
		exclude []schemadiff.ObjectType
		want    []schemadiff.ObjectType
	}{
		{
			name:    "exclude TABLE leaves COLUMN",
			exclude: []schemadiff.ObjectType{schemadiff.ObjectTypeTable},
			want:    []schemadiff.ObjectType{schemadiff.ObjectTypeColumn},
		},
		{
			name:    "exclude COLUMN leaves TABLE",
			exclude: []schemadiff.ObjectType{schemadiff.ObjectTypeColumn},
			want:    []schemadiff.ObjectType{schemadiff.ObjectTypeTable},
		},
		{
			// detectDrift rejects this with an error rather than
			// forwarding it to schemadiff.Scope, which keeps nothing for an empty
			// dimension -- the run would compare nothing and report it as clean.
			name:    "exclude both yields empty",
			exclude: []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
			want:    nil,
		},
		{
			name:    "exclude none keeps both, in universe order",
			exclude: nil,
			want:    []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := complementDriftObjectTypes(tt.exclude)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ─── parseDriftObjectTypeList ────────────────────────────────────────────────

func TestNormalizeDriftListFlag(t *testing.T) {
	// The point of this helper is that a value which LOOKS set but names nothing
	// is treated as unset -- otherwise it resolves to an empty keep-set, and the
	// run drops every finding and reports no drift.
	tests := map[string]string{
		"":                  "",
		"   ":               "",
		",":                 "",
		" , , ":             "",
		"orders":            "orders",
		"  orders  ":        "orders",
		"orders,customers":  "orders,customers",
		"orders, customers": "orders,customers",
		"orders,,customers": "orders,customers",
		",orders,":          "orders",
	}
	for raw, want := range tests {
		t.Run(fmt.Sprintf("%q", raw), func(t *testing.T) {
			assert.Equal(t, want, normalizeDriftListFlag(raw))
		})
	}
}

func TestResolveDetectDriftFlagDefaultsTrimsSourceSchemas(t *testing.T) {
	saved := source
	t.Cleanup(func() { source = saved })

	source.SchemaConfig = "public, sales ,"
	resolveDetectDriftFlagDefaults()
	assert.Equal(t, "public,sales", source.SchemaConfig)
}

func TestDetectDriftNeedsInitialisedExportDirAndTakesLock(t *testing.T) {
	require.Equal(t, "yb-voyager schema detect-drift", detectDriftCmd.CommandPath())
	assert.True(t, shouldRunExportDirInitialisedCheck(detectDriftCmd))
	assert.True(t, shouldLock(detectDriftCmd))
}

func TestParseDriftObjectTypeList(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []schemadiff.ObjectType
		wantErr bool
	}{
		{
			name: "empty string means no filter",
			raw:  "",
			want: nil,
		},
		{
			name: "whitespace-only string means no filter",
			raw:  "   ",
			want: nil,
		},
		{
			name: "TABLE,COLUMN parses both",
			raw:  "TABLE,COLUMN",
			want: []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
		},
		{
			name: "single lowercase type is case-insensitive",
			raw:  "table",
			want: []schemadiff.ObjectType{schemadiff.ObjectTypeTable},
		},
		{
			name: "mixed case with surrounding whitespace is tolerated",
			raw:  " Table , coLUMN ",
			want: []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn},
		},
		{
			name: "a type named twice is kept once",
			raw:  "TABLE,table",
			want: []schemadiff.ObjectType{schemadiff.ObjectTypeTable},
		},
		{
			name:    "unknown type errors",
			raw:     "SEQUENCE",
			wantErr: true,
		},
		{
			name:    "old export-schema vocabulary (INDEX) is no longer supported",
			raw:     "INDEX",
			wantErr: true,
		},
		{
			name:    "one bad type among good ones still errors",
			raw:     "TABLE,INDEX",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDriftObjectTypeList(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ─── validateDriftOutputFormat ───────────────────────────────────────────────

func TestValidateDriftOutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		wantFormats []string
		wantErr     bool
	}{
		{
			name:        "unset writes both",
			format:      "",
			wantFormats: []string{"html", "json"},
		},
		{
			name:        "json alone",
			format:      "json",
			wantFormats: []string{"json"},
		},
		{
			name:        "case-insensitive",
			format:      "HTML",
			wantFormats: []string{"html"},
		},
		{
			name:    "a list is not accepted",
			format:  "html,json",
			wantErr: true,
		},
		{
			name:    "whitespace-only string errors",
			format:  "   ",
			wantErr: true,
		},
		{
			name:    "unsupported format errors",
			format:  "xml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDriftOutputFormat(tt.format)
			if tt.wantErr {
				assert.ErrorContains(t, err, "invalid report output format")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantFormats, driftReportFormats(tt.format))
		})
	}
}

// ─── driftTableUniverse (the universe fix) ───────────────────────────────────

// snapContent is a tiny helper to build a SnapshotContent whose Tables are the
// given (schema, name) refs -- only the embedded ObjectRef is set, which is all
// the union logic reads unless a test also attaches PartitionChildren.
func snapContent(refs ...schemasnapshot.ObjectRef) *schemasnapshot.SnapshotContent {
	c := &schemasnapshot.SnapshotContent{}
	for _, r := range refs {
		c.Tables = append(c.Tables, schemasnapshot.Table{ObjectRef: r})
	}
	return c
}

func TestDriftTableUniverse(t *testing.T) {
	t.Run("snapshot-only (dropped) table is still in the universe; dedup across snapshots", func(t *testing.T) {
		// Headline universe-fix case: products is present ONLY in an older snapshot
		// (dropped from the source since) yet must still be nameable. orders appears
		// in both snapshots => deduped to one.
		got := driftTableUniverse([]*schemasnapshot.SnapshotContent{
			snapContent(
				schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
				schemasnapshot.ObjectRef{Schema: "public", Name: "products"},
			),
			snapContent(schemasnapshot.ObjectRef{Schema: "public", Name: "orders"}),
		}, nil)
		assert.Equal(t, map[string][]string{"public": {"orders", "products"}}, got)
	})

	t.Run("live capture contributes an extra table", func(t *testing.T) {
		got := driftTableUniverse(
			[]*schemasnapshot.SnapshotContent{snapContent(schemasnapshot.ObjectRef{Schema: "public", Name: "products"})},
			snapContent(schemasnapshot.ObjectRef{Schema: "public", Name: "audit"}))
		assert.Equal(t, map[string][]string{"public": {"products", "audit"}}, got)
	})

	t.Run("same table in a snapshot and the live capture yields a single entry", func(t *testing.T) {
		orders := schemasnapshot.ObjectRef{Schema: "public", Name: "orders"}
		got := driftTableUniverse([]*schemasnapshot.SnapshotContent{snapContent(orders)}, snapContent(orders))
		assert.Equal(t, map[string][]string{"public": {"orders"}}, got)
	})

	t.Run("each schema keeps its own tables, case preserved", func(t *testing.T) {
		got := driftTableUniverse(nil, snapContent(
			schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
			schemasnapshot.ObjectRef{Schema: "Sales", Name: "Invoices"},
		))
		assert.Equal(t, map[string][]string{"public": {"orders"}, "Sales": {"Invoices"}}, got)
	})

	t.Run("nil snapshot content is skipped", func(t *testing.T) {
		got := driftTableUniverse(
			[]*schemasnapshot.SnapshotContent{nil, snapContent(schemasnapshot.ObjectRef{Schema: "public", Name: "products"})}, nil)
		assert.Equal(t, map[string][]string{"public": {"products"}}, got)
	})
}

// ─── requireDriftTableListQualified ──────────────────────────────────────────

func TestRequireDriftTableListQualified(t *testing.T) {
	t.Run("unqualified pattern without a default schema is rejected by name", func(t *testing.T) {
		err := requireDriftTableListQualified("orders", "table-list", false)
		require.Error(t, err)
		// The old behaviour reported "unknown table name", which sent the user
		// looking for a table that does exist.
		assert.Contains(t, err.Error(), "not schema-qualified")
		assert.NotContains(t, err.Error(), "unknown table name")
	})

	t.Run("qualified pattern needs no default schema", func(t *testing.T) {
		require.NoError(t, requireDriftTableListQualified("sales.orders", "table-list", false))
	})

	t.Run("unqualified pattern is fine once a default schema exists", func(t *testing.T) {
		require.NoError(t, requireDriftTableListQualified("orders", "table-list", true))
	})

	t.Run("one unqualified entry among qualified ones still errors", func(t *testing.T) {
		err := requireDriftTableListQualified("sales.orders,customers", "exclude-table-list", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"customers"`)
	})
}

// ─── table-list resolution via the in-memory registry + extractTableListFromString ──

func TestDriftTableListResolutionViaRegistry(t *testing.T) {
	universe := map[string][]string{
		"sales":   {"orders"},
		"billing": {"invoices"},
	}
	reg, err := namereg.NewInMemorySourceNameRegistry(POSTGRESQL, []string{"sales", "billing"}, universe)
	require.NoError(t, err)
	allTuples, err := reg.GetRegisteredTableList(false)
	require.NoError(t, err)

	t.Run("qualified pattern resolves", func(t *testing.T) {
		refs, err := extractTableListFromString(allTuples, "sales.orders", "include")
		require.NoError(t, err)
		assert.Equal(t, []schemasnapshot.ObjectRef{{Schema: "sales", Name: "orders"}}, driftObjectRefs(refs))
	})

	t.Run("a pattern matching nothing is an UnknownTableErr", func(t *testing.T) {
		_, err := extractTableListFromString(allTuples, "sales.nope", "include")
		require.Error(t, err)
		var unknownErr *errs.UnknownTableErr
		assert.True(t, errors.As(err, &unknownErr))
	})

	t.Run("case-sensitive names match only under their own quoting", func(t *testing.T) {
		mixedReg, err := namereg.NewInMemorySourceNameRegistry(POSTGRESQL, []string{"public"},
			map[string][]string{"public": {"Orders", "orders"}})
		require.NoError(t, err)
		mixedTuples, err := mixedReg.GetRegisteredTableList(false)
		require.NoError(t, err)

		// An unquoted pattern folds case, so it reaches both spellings.
		refs, err := extractTableListFromString(mixedTuples, "orders", "include")
		require.NoError(t, err)
		assert.ElementsMatch(t, []schemasnapshot.ObjectRef{
			{Schema: "public", Name: "Orders"},
			{Schema: "public", Name: "orders"},
		}, driftObjectRefs(refs))

		// A quoted pattern is exact, so it reaches only the capitalised one.
		refs, err = extractTableListFromString(mixedTuples, `public."Orders"`, "include")
		require.NoError(t, err)
		assert.Equal(t, []schemasnapshot.ObjectRef{{Schema: "public", Name: "Orders"}}, driftObjectRefs(refs))
	})
}

// ─── partition expansion ─────────────────────────────────────────────────────

var (
	driftPartRoot  = schemasnapshot.ObjectRef{Schema: "public", Name: "events"}
	driftPartMid   = schemasnapshot.ObjectRef{Schema: "public", Name: "events_2024"}
	driftPartLeaf1 = schemasnapshot.ObjectRef{Schema: "public", Name: "events_2024_01"}
	driftPartLeaf2 = schemasnapshot.ObjectRef{Schema: "public", Name: "events_2024_02"}
	driftPartOther = schemasnapshot.ObjectRef{Schema: "public", Name: "customers"} // not a partition at all
)

// driftPartitionContent builds a SnapshotContent for the root -> mid -> {leaf1, leaf2}
// hierarchy shared by the partition tests below, plus one unrelated table.
func driftPartitionContent() *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{Tables: []schemasnapshot.Table{
		{ObjectRef: driftPartRoot, PartitionChildren: []schemasnapshot.ObjectRef{driftPartMid}},
		{ObjectRef: driftPartMid, PartitionChildren: []schemasnapshot.ObjectRef{driftPartLeaf1, driftPartLeaf2}},
		{ObjectRef: driftPartLeaf1},
		{ObjectRef: driftPartLeaf2},
		{ObjectRef: driftPartOther},
	}}
}

func TestExpandDriftPartitions(t *testing.T) {
	children := driftPartitionChildren([]*schemasnapshot.SnapshotContent{driftPartitionContent()}, nil)

	tests := []struct {
		name string
		refs []schemasnapshot.ObjectRef
		want []schemasnapshot.ObjectRef
	}{
		{
			name: "root in include expands to root + intermediate + leaves, depth-first",
			refs: []schemasnapshot.ObjectRef{driftPartRoot},
			want: []schemasnapshot.ObjectRef{driftPartRoot, driftPartMid, driftPartLeaf1, driftPartLeaf2},
		},
		{
			name: "a leaf named alone expands to only itself",
			refs: []schemasnapshot.ObjectRef{driftPartLeaf1},
			want: []schemasnapshot.ObjectRef{driftPartLeaf1},
		},
		{
			name: "a non-partitioned table is unaffected",
			refs: []schemasnapshot.ObjectRef{driftPartOther},
			want: []schemasnapshot.ObjectRef{driftPartOther},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, expandDriftPartitions(tt.refs, children))
		})
	}
}

func TestExpandDriftPartitionsThenComplement(t *testing.T) {
	// Mirrors resolveDriftScope's exclude branch: the matched exclude refs are
	// expanded to their full partition subtree before complementing against the
	// universe, so excluding a root removes every partition beneath it too.
	universe := []schemasnapshot.ObjectRef{driftPartRoot, driftPartMid, driftPartLeaf1, driftPartLeaf2, driftPartOther}
	children := driftPartitionChildren([]*schemasnapshot.SnapshotContent{driftPartitionContent()}, nil)

	excludeExpanded := expandDriftPartitions([]schemasnapshot.ObjectRef{driftPartRoot}, children)
	got := complementDriftTableRefs(universe, excludeExpanded)
	assert.Equal(t, []schemasnapshot.ObjectRef{driftPartOther}, got)
}

func TestDriftPartitionChildrenUnionAcrossSnapshots(t *testing.T) {
	// A partition present only in an older snapshot (dropped since) must still
	// expand: driftPartitionChildren unions PartitionChildren across every content
	// rather than letting a newer snapshot's list overwrite an older one's.
	droppedLeaf := schemasnapshot.ObjectRef{Schema: "public", Name: "events_2023_12"}
	older := &schemasnapshot.SnapshotContent{Tables: []schemasnapshot.Table{
		{ObjectRef: driftPartRoot, PartitionChildren: []schemasnapshot.ObjectRef{driftPartMid, droppedLeaf}},
	}}
	newer := &schemasnapshot.SnapshotContent{Tables: []schemasnapshot.Table{
		{ObjectRef: driftPartRoot, PartitionChildren: []schemasnapshot.ObjectRef{driftPartMid}},
	}}

	children := driftPartitionChildren([]*schemasnapshot.SnapshotContent{older, newer}, nil)
	got := expandDriftPartitions([]schemasnapshot.ObjectRef{driftPartRoot}, children)
	assert.Contains(t, got, droppedLeaf)
}

func TestExpandDriftPartitionsCaseSensitive(t *testing.T) {
	root := schemasnapshot.ObjectRef{Schema: "Sales", Name: "Events"}
	leaf := schemasnapshot.ObjectRef{Schema: "Sales", Name: "Events_2024"}
	content := &schemasnapshot.SnapshotContent{Tables: []schemasnapshot.Table{
		{ObjectRef: root, PartitionChildren: []schemasnapshot.ObjectRef{leaf}},
	}}
	children := driftPartitionChildren([]*schemasnapshot.SnapshotContent{content}, nil)

	assert.Equal(t, []schemasnapshot.ObjectRef{root, leaf}, expandDriftPartitions([]schemasnapshot.ObjectRef{root}, children))
}

// ─── nothingComparedError ────────────────────────────────────────────────────

func TestNothingComparedError(t *testing.T) {
	tests := []struct {
		name   string
		points []schemadrift.CapturePoint
		want   string
		absent string
	}{
		{
			name:   "no captures stored at all",
			points: nil,
			want:   "holds no schema snapshots",
			// Capture cannot be enabled retroactively, so the message must not
			// suggest re-running the export as a fix for this run.
			absent: "Re-run",
		},
		{
			name: "one usable capture forms no interval",
			points: []schemadrift.CapturePoint{
				{Label: schemasnapshot.LabelExportSchema},
			},
			want:   "a single capture forms no interval",
			absent: "skipped because",
		},
		{
			name: "every capture excluded, reasons named",
			points: []schemadrift.CapturePoint{
				{Label: schemasnapshot.LabelExportSchema, Excluded: "the capture failed, so this point holds no schema"},
				{Label: schemasnapshot.LabelExportDataFromSourceStart, Excluded: "captured only sales, so it cannot answer for public"},
			},
			want:   "captured only sales, so it cannot answer for public",
			absent: "single capture",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nothingComparedError(schemadrift.Report{CapturePoints: tt.points})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.NotContains(t, err.Error(), tt.absent)
		})
	}
}

// ─── buildSchemaDriftPayload ─────────────────────────────────────────────────

func TestBuildSchemaDriftPayload(t *testing.T) {
	report := schemadrift.Report{
		Comparing: schemadrift.Comparing{
			Schemas:     []string{"public", "sales"},
			Tables:      []string{"public.orders", "sales.items", `sales."Customers"`},
			ObjectTypes: []string{"TABLE", "COLUMN"},
		},
		Summary: schemadrift.Summary{
			ChangeCount:           3,
			ComparedIntervalCount: 2,
			StoredCaptureCount:    4,
			LiveCompared:          true,
		},
		Drifts: []schemadrift.DriftEntry{
			{Diff: schemadrift.Diff{Type: schemadiff.ColumnAdded}, DriftInfo: schemadrift.DriftInfo{Severity: schemadrift.SeverityAdvisory}},
			{Diff: schemadrift.Diff{Type: schemadiff.TableDropped}, DriftInfo: schemadrift.DriftInfo{Severity: schemadrift.SeverityBreaksUnrecoverable}},
		},
	}

	t.Run("populated report", func(t *testing.T) {
		got := buildSchemaDriftPayload(nil, &report, driftInvokerDetectDrift)

		assert.Equal(t, callhome.SCHEMA_DRIFT_CALLHOME_PAYLOAD_VERSION, got.PayloadVersion)
		assert.Equal(t, "detect-drift", got.InvokedBy)
		assert.Equal(t, 3, got.ChangeCount)
		assert.Equal(t, 2, got.ComparedIntervalCount)
		assert.Equal(t, 4, got.StoredCaptureCount)
		assert.True(t, got.LiveCompared)
		assert.Equal(t, 3, got.TableCount)
		assert.Equal(t, []string{"COLUMN", "TABLE"}, got.ObjectTypes)
		assert.Equal(t, map[string]int{
			string(schemadiff.ColumnAdded):  1,
			string(schemadiff.TableDropped): 1,
		}, got.DriftsByType)
		assert.Equal(t, map[string]int{
			string(schemadrift.SeverityAdvisory):            1,
			string(schemadrift.SeverityBreaksUnrecoverable): 1,
		}, got.DriftsBySeverity)
		assert.Empty(t, got.Error)
	})

	// The run failed before a report existed. Everything report-derived must stay
	// zero rather than be invented, and the histograms must drop out of the JSON.
	t.Run("nil report", func(t *testing.T) {
		got := buildSchemaDriftPayload(errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_CAPTURE_LIVE_SCHEMA, fmt.Errorf("source is unreachable")), nil, driftInvokerDetectDrift)

		assert.Equal(t, callhome.SCHEMA_DRIFT_CALLHOME_PAYLOAD_VERSION, got.PayloadVersion)
		assert.Zero(t, got.ChangeCount)
		assert.Zero(t, got.ComparedIntervalCount)
		assert.Zero(t, got.StoredCaptureCount)
		assert.False(t, got.LiveCompared)
		assert.Zero(t, got.TableCount)
		assert.Nil(t, got.ObjectTypes)
		assert.Nil(t, got.DriftsByType)
		assert.Nil(t, got.DriftsBySeverity)
		assert.Equal(t, `{"msg":"schema drift","step":"capture_live_schema"}`, got.Error)
	})

	t.Run("a report with no drift sends no histograms", func(t *testing.T) {
		clean := report
		clean.Drifts = nil
		raw, err := json.Marshal(buildSchemaDriftPayload(nil, &clean, driftInvokerDetectDrift))
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw, &fields))
		assert.NotContains(t, fields, "drifts_by_type")
		assert.NotContains(t, fields, "drifts_by_severity")
	})

	t.Run("an in-process check names its invoker", func(t *testing.T) {
		got := buildSchemaDriftPayload(nil, &report, driftInvoker("export-data"))
		assert.Equal(t, "export-data", got.InvokedBy)
	})
}

func TestSanitizeDriftError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "no error", err: nil, want: ""},
		{
			name: "a table pattern before the first colon",
			err:  errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_RESOLVE_SCOPE, fmt.Errorf(`invalid table name pattern "proddb.sales.customer_pii": syntax error`)),
			want: `{"msg":"schema drift","step":"resolve_scope"}`,
		},
		{
			name: "a table list with no colon at all",
			err:  errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_RESOLVE_SCOPE, fmt.Errorf(`--exclude-table-list "sales.\"Customer_PII\"" excludes every table in the comparison; nothing left to compare`)),
			want: `{"msg":"schema drift","step":"resolve_scope"}`,
		},
		{
			name: "an export-dir path",
			err:  errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_WRITE_REPORTS, fmt.Errorf(`failed to write json drift report to "/home/alice/acme-prod/reports/drift_analysis_report.json": no space left on device`)),
			want: `{"msg":"schema drift","step":"write_reports"}`,
		},
		{
			name: "schema names in the nothing-compared reasons",
			err:  errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_NOTHING_COMPARED, fmt.Errorf("no two comparable schema snapshots: captured only public, sales, so it cannot answer for hr")),
			want: `{"msg":"schema drift","step":"nothing_compared"}`,
		},
		{
			name: "an untagged error from flag validation",
			err:  fmt.Errorf(`schema detect-drift currently supports PostgreSQL sources only (got --source-db-type="acme_prod")`),
			want: `{"msg":"schema drift","step":"setup"}`,
		},
		{
			name: "the SQLSTATE survives",
			err:  errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_CONNECT_TO_SOURCE, fmt.Errorf("failed to connect to source database: %w", &pgconnv5.PgError{Code: "28P01", Message: `password authentication failed for user "alice"`})),
			want: `{"msg":"schema drift","pg_error_code":"28P01","step":"connect_to_source"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeDriftError(tt.err))
		})
	}

	// The exit handler receives the run's error re-wrapped by utils.ErrExit, which
	// adds a go-errors stack, so only msg, step and the stack trace may be present.
	t.Run("the step survives utils.ErrExit's wrapping", func(t *testing.T) {
		tagged := errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_RESOLVE_SCOPE, fmt.Errorf(`invalid table name pattern "proddb.sales.customer_pii": syntax error`))
		got := sanitizeDriftError(goerrors.Errorf("%w", tagged))

		var fields map[string]string
		require.NoError(t, json.Unmarshal([]byte(got), &fields))
		assert.Equal(t, "schema drift", fields["msg"])
		assert.Equal(t, "resolve_scope", fields["step"])
		assert.ElementsMatch(t, []string{"msg", "step", "stack_trace"}, lo.Keys(fields))
		assert.NotContains(t, got, "customer_pii")
	})
}

// redirectCallhomeToTestServer points callhome at a local server and returns its
// request count. It turns diagnostics on and restores every global it sets.
func redirectCallhomeToTestServer(t *testing.T) *atomic.Int32 {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	t.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", serverURL.Hostname())
	t.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", serverURL.Port())

	origSend, origStart, origMetaDB, origUUID := callhome.SendDiagnostics, startTime, metaDB, migrationUUID
	origHost, origPort := callhome.CALL_HOME_SERVICE_HOST, callhome.CALL_HOME_SERVICE_PORT
	origAnonymizer, origSent := anonymizer, callHomeErrorOrCompletePayloadSent
	t.Cleanup(func() {
		callhome.SendDiagnostics, startTime, metaDB, migrationUUID = origSend, origStart, origMetaDB, origUUID
		callhome.CALL_HOME_SERVICE_HOST, callhome.CALL_HOME_SERVICE_PORT = origHost, origPort
		anonymizer, callHomeErrorOrCompletePayloadSent = origAnonymizer, origSent
	})
	callhome.SendDiagnostics = true
	startTime = time.Now()
	migrationUUID = uuid.New()

	// Proves the redirect works, so a missing request below means nothing was sent.
	require.NoError(t, callhome.SendPayload(&callhome.Payload{}))
	require.Equal(t, int32(1), requests.Load())
	return &requests
}

// checkExportDirInitialised exits before initMetaDB when no migration has started,
// so the exit handler reaches the drift sender with metaDB unset.
func TestPackAndSendSchemaDriftPayloadWithoutMetaDB(t *testing.T) {
	requests := redirectCallhomeToTestServer(t)

	metaDB = nil
	packAndSendSchemaDriftPayload(ERROR, fmt.Errorf("Migration has not started yet"), nil, driftInvokerDetectDrift)
	assert.Equal(t, int32(1), requests.Load(), "no payload may be sent without metaDB")
}

// captureCallhomePayloads points callhome at a local server with metaDB open and
// returns the payloads it receives. It restores every global it sets.
func captureCallhomePayloads(t *testing.T) func() []callhome.Payload {
	var mu sync.Mutex
	var payloads []callhome.Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p callhome.Payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode callhome request: %v", err)
		}
		mu.Lock()
		payloads = append(payloads, p)
		mu.Unlock()
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	t.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", serverURL.Hostname())
	t.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", serverURL.Port())

	origSend, origStart, origMetaDB, origUUID := callhome.SendDiagnostics, startTime, metaDB, migrationUUID
	origHost, origPort := callhome.CALL_HOME_SERVICE_HOST, callhome.CALL_HOME_SERVICE_PORT
	origAnonymizer, origSent := anonymizer, callHomeErrorOrCompletePayloadSent
	origCommand, origExitErr := currentCommand, utils.ErrExitErr
	t.Cleanup(func() {
		callhome.SendDiagnostics, startTime, metaDB, migrationUUID = origSend, origStart, origMetaDB, origUUID
		callhome.CALL_HOME_SERVICE_HOST, callhome.CALL_HOME_SERVICE_PORT = origHost, origPort
		anonymizer, callHomeErrorOrCompletePayloadSent = origAnonymizer, origSent
		currentCommand, utils.ErrExitErr = origCommand, origExitErr
	})
	callhome.SendDiagnostics = true
	startTime = time.Now()
	callHomeErrorOrCompletePayloadSent = false
	currentCommand = detectDriftCmd.CommandPath()
	metaDB = initMetaDB(t.TempDir())

	return func() []callhome.Payload {
		mu.Lock()
		defer mu.Unlock()
		return append([]callhome.Payload(nil), payloads...)
	}
}

func TestSchemaDriftErrorWithReportSentOnce(t *testing.T) {
	received := captureCallhomePayloads(t)
	migrationUUID = uuid.New()

	report := schemadrift.Report{}
	report.Summary.StoredCaptureCount = 1
	failure := errs.NewSchemaDriftError(errs.SCHEMA_DRIFT_STEP_NOTHING_COMPARED, fmt.Errorf("captured only sales"))
	packAndSendSchemaDriftPayload(ERROR, failure, &report, driftInvokerDetectDrift)
	utils.ErrExitErr = failure
	PackAndSendCallhomePayloadOnExit()

	got := received()
	require.Len(t, got, 1, "the exit handler must not send a second ERROR row")
	assert.Equal(t, ERROR, got[0].Status)
	var phase callhome.SchemaDriftPhasePayload
	require.NoError(t, json.Unmarshal([]byte(got[0].PhasePayload), &phase))
	assert.Equal(t, 1, phase.StoredCaptureCount)
	var errFields map[string]string
	require.NoError(t, json.Unmarshal([]byte(phase.Error), &errFields))
	assert.Equal(t, "schema drift", errFields["msg"])
	assert.Equal(t, "nothing_compared", errFields["step"])
	assert.NotContains(t, phase.Error, "sales")
}

// The root pre-run can exit after initMetaDB and before PreRun reads the UUID.
func TestSchemaDriftErrorSentWithoutMigrationUUID(t *testing.T) {
	received := captureCallhomePayloads(t)
	migrationUUID = uuid.Nil

	utils.ErrExitErr = fmt.Errorf("export directory was created by an incompatible voyager version")
	PackAndSendCallhomePayloadOnExit()

	got := received()
	require.Len(t, got, 1)
	assert.Equal(t, ERROR, got[0].Status)
	assert.Equal(t, uuid.Nil, got[0].MigrationUUID)
}

func TestPackAndSendSchemaDriftPayloadSentGuard(t *testing.T) {
	tests := []struct {
		name      string
		invoker   driftInvoker
		wantGuard bool
	}{
		{name: "the command marks its phase as reported", invoker: driftInvokerDetectDrift, wantGuard: true},
		{name: "an in-process check leaves the guard to its host command", invoker: driftInvoker("export-data"), wantGuard: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := redirectCallhomeToTestServer(t)
			metaDB = initMetaDB(t.TempDir())
			callHomeErrorOrCompletePayloadSent = false

			packAndSendSchemaDriftPayload(ERROR, fmt.Errorf("export failed"), nil, tt.invoker)

			assert.Equal(t, int32(2), requests.Load(), "the drift payload must be sent")
			assert.Equal(t, tt.wantGuard, callHomeErrorOrCompletePayloadSent)
		})
	}
}

// ─── checkSchemaDrift (the in-process entry point) ────────────────────────────

func TestDriftReportBaseName(t *testing.T) {
	assert.Equal(t, "drift_analysis_report", driftReportBaseName(driftInvokerDetectDrift))
	assert.Equal(t, "drift_analysis_report_export_data", driftReportBaseName(driftInvoker("export-data")))
	assert.Equal(t, "drift_analysis_report_cutover_to_target", driftReportBaseName(driftInvoker("cutover-to-target")))
}

func TestCheckSchemaDriftRejectsAnUnsetInvoker(t *testing.T) {
	report, paths, err := checkSchemaDrift(driftCheckInput{Schemas: []string{"public"}, Formats: []string{"json"}})
	var sde errs.SchemaDriftError
	require.ErrorAs(t, err, &sde)
	assert.Equal(t, errs.SCHEMA_DRIFT_STEP_SETUP, sde.Step())
	assert.Nil(t, report)
	assert.Nil(t, paths)
}

func TestWriteDriftReportsRoutesTheOverwriteNote(t *testing.T) {
	saved := exportDir
	t.Cleanup(func() { exportDir = saved })
	exportDir = t.TempDir()

	var notes []string
	info := func(format string, args ...interface{}) { notes = append(notes, fmt.Sprintf(format, args...)) }

	_, err := writeDriftReports(schemadrift.Report{}, []string{"json"}, "drift_analysis_report_export_data", info)
	require.NoError(t, err)
	assert.Empty(t, notes, "a first write overwrites nothing")

	_, err = writeDriftReports(schemadrift.Report{}, []string{"json"}, "drift_analysis_report_export_data", info)
	require.NoError(t, err)
	assert.Equal(t, []string{"\ndrift_analysis_report_export_data.json already exists, overwriting it with a new generated report\n"}, notes)
}

// The export-data caller's shape: stored snapshots only, no live read.
func TestCheckSchemaDriftWithoutLiveRead(t *testing.T) {
	savedMetaDB, savedExportDir, savedSource := metaDB, exportDir, source
	t.Cleanup(func() { metaDB, exportDir, source = savedMetaDB, savedExportDir, savedSource })
	exportDir = t.TempDir()
	metaDB = initMetaDB(exportDir)
	source.DBType = constants.POSTGRESQL

	table := func(name, id string) schemasnapshot.Table {
		return schemasnapshot.Table{ObjectRef: schemasnapshot.ObjectRef{Schema: "public", Name: name}, ID: id, Kind: schemasnapshot.TableKindOrdinary}
	}
	start := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	for i, snap := range []schemasnapshot.SchemaSnapshot{
		{
			Header:  schemasnapshot.SnapshotHeader{Label: schemasnapshot.LabelExportDataFromSourceStart, Reason: schemasnapshot.ReasonInitial, CapturedAt: start, Schemas: []string{"public"}},
			Content: &schemasnapshot.SnapshotContent{Version: 1, DatabaseType: constants.POSTGRESQL, Tables: []schemasnapshot.Table{table("orders", "1")}},
		},
		{
			Header:  schemasnapshot.SnapshotHeader{Label: schemasnapshot.LabelExportDataFromSourceExit, Reason: schemasnapshot.ReasonError, CapturedAt: start.Add(time.Hour), Schemas: []string{"public"}},
			Content: &schemasnapshot.SnapshotContent{Version: 1, DatabaseType: constants.POSTGRESQL, Tables: []schemasnapshot.Table{table("orders", "1"), table("products", "2")}},
		},
	} {
		_, err := schemasnapshot.SaveSnapshot(context.Background(), metaDB, &snap)
		require.NoError(t, err, "snapshot %d", i)
	}

	report, paths, err := checkSchemaDrift(driftCheckInput{
		Schemas: []string{"public"},
		Formats: []string{"json"},
		Invoker: driftInvoker("export-data"),
	})
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.HasLiveRead())
	assert.False(t, report.Summary.LiveCompared)
	assert.Equal(t, 1, report.Summary.ComparedIntervalCount)
	assert.Equal(t, 2, report.Summary.StoredCaptureCount)
	require.Len(t, report.Drifts, 1)
	assert.Equal(t, schemadiff.TableAdded, report.Drifts[0].Type)
	assert.Equal(t, "products", report.Drifts[0].Object.Name)
	assert.Equal(t, []string{filepath.Join(exportDir, "reports", "drift_analysis_report_export_data.json")}, paths)
}
