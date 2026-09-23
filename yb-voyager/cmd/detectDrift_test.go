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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/errs"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
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
			// detectDrift rejects this with an operational error rather than
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
		name    string
		format  string
		wantErr bool
	}{
		{
			name:   "html,json is valid",
			format: "html,json",
		},
		{
			name:   "single valid format",
			format: "json",
		},
		{
			name:   "case-insensitive",
			format: "HTML,Json",
		},
		{
			name:    "empty string errors",
			format:  "",
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
		{
			name:    "duplicate format errors",
			format:  "html,html",
			wantErr: true,
		},
		{
			name:    "duplicate format errors case-insensitively",
			format:  "html,HTML",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDriftOutputFormat(tt.format)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
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
	t.Run("listing error is returned", func(t *testing.T) {
		failing := func(string) ([]string, error) { return nil, fmt.Errorf("connection reset") }
		got, err := driftTableUniverse(failing, []string{"public"}, nil, nil)
		require.EqualError(t, err, `list the tables in schema "public": connection reset`)
		assert.Nil(t, got)
	})

	t.Run("each schema's tables become universe entries", func(t *testing.T) {
		tablesBySchema := map[string][]string{"public": {"orders"}, "Sales": {"Invoices"}}
		listing := func(schema string) ([]string, error) { return tablesBySchema[schema], nil }
		got, err := driftTableUniverse(listing, []string{"public", "Sales"}, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string][]string{"public": {"orders"}, "Sales": {"Invoices"}}, got)
	})

	t.Run("snapshot-only (dropped) table is still in the universe; dedup across live+snapshot", func(t *testing.T) {
		// Headline universe-fix case: products is present ONLY in a historical
		// snapshot (dropped from the live catalog) yet must still be nameable.
		// orders appears in both live catalog and snapshot => deduped to one.
		listing := func(string) ([]string, error) { return []string{"orders", "customers"}, nil }
		got, err := driftTableUniverse(listing, []string{"public"}, []*schemasnapshot.SnapshotContent{
			snapContent(
				schemasnapshot.ObjectRef{Schema: "public", Name: "products"},
				schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
			),
		}, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"orders", "customers", "products"}, got["public"])
	})

	t.Run("live capture contributes an extra table", func(t *testing.T) {
		listing := func(string) ([]string, error) { return []string{"orders"}, nil }
		got, err := driftTableUniverse(listing, []string{"public"},
			[]*schemasnapshot.SnapshotContent{snapContent(schemasnapshot.ObjectRef{Schema: "public", Name: "products"})},
			snapContent(schemasnapshot.ObjectRef{Schema: "public", Name: "audit"}))
		require.NoError(t, err)
		assert.Equal(t, []string{"orders", "products", "audit"}, got["public"])
	})

	t.Run("same table in all three sources yields a single entry", func(t *testing.T) {
		orders := schemasnapshot.ObjectRef{Schema: "public", Name: "orders"}
		listing := func(string) ([]string, error) { return []string{"orders"}, nil }
		got, err := driftTableUniverse(listing, []string{"public"},
			[]*schemasnapshot.SnapshotContent{snapContent(orders)}, snapContent(orders))
		require.NoError(t, err)
		assert.Equal(t, []string{"orders"}, got["public"])
	})

	t.Run("nil snapshot content is skipped", func(t *testing.T) {
		listing := func(string) ([]string, error) { return []string{"orders"}, nil }
		got, err := driftTableUniverse(listing, []string{"public"},
			[]*schemasnapshot.SnapshotContent{nil, snapContent(schemasnapshot.ObjectRef{Schema: "public", Name: "products"})}, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"orders", "products"}, got["public"])
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
