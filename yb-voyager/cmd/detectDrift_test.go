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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// ─── complementDriftTableRefs ────────────────────────────────────────────────

func TestComplementDriftTableRefs(t *testing.T) {
	refA := schemasnapshot.ObjectRef{Schema: "public", Name: "a"}
	refB := schemasnapshot.ObjectRef{Schema: "public", Name: "b"}
	refC := schemasnapshot.ObjectRef{Schema: "public", Name: "c"}
	refD := schemasnapshot.ObjectRef{Schema: "other", Name: "d"} // not among candidates

	candidates := []driftTableCandidate{
		{ref: refA},
		{ref: refB},
		{ref: refC},
	}

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
			name:     "exclude a ref not in candidates is a no-op",
			exclude:  []schemasnapshot.ObjectRef{refD},
			wantRefs: []schemasnapshot.ObjectRef{refA, refB, refC},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := complementDriftTableRefs(candidates, tt.exclude)
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

// ─── unionDriftTableCandidates (the universe fix) ────────────────────────────

// snapContent is a tiny helper to build a SnapshotContent whose Tables are the
// given (schema, name) refs -- only the embedded ObjectRef is set, which is all
// the union logic reads.
func snapContent(refs ...schemasnapshot.ObjectRef) *schemasnapshot.SnapshotContent {
	c := &schemasnapshot.SnapshotContent{}
	for _, r := range refs {
		c.Tables = append(c.Tables, schemasnapshot.Table{ObjectRef: r})
	}
	return c
}

func candidateRefs(candidates []driftTableCandidate) []schemasnapshot.ObjectRef {
	refs := make([]schemasnapshot.ObjectRef, 0, len(candidates))
	for _, c := range candidates {
		refs = append(refs, c.ref)
	}
	return refs
}

// A catalog read that fails must come back as an error: the caller turns it into
// exit 2, where the old GetAllTableNames path exited 1, which means "drift found".
func TestBuildDriftTableCandidates(t *testing.T) {
	origDBType := source.DBType
	t.Cleanup(func() { source.DBType = origDBType })
	source.DBType = POSTGRESQL

	t.Run("listing error is returned", func(t *testing.T) {
		failing := func(string) ([]string, error) { return nil, fmt.Errorf("connection reset") }
		got, err := buildDriftTableCandidates(failing, []string{"public"}, "public", nil, nil)
		require.EqualError(t, err, `list the tables in schema "public": connection reset`)
		assert.Nil(t, got)
	})

	t.Run("each schema's tables become candidates", func(t *testing.T) {
		tablesBySchema := map[string][]string{"public": {"orders"}, "Sales": {"Invoices"}}
		listing := func(schema string) ([]string, error) { return tablesBySchema[schema], nil }
		got, err := buildDriftTableCandidates(listing, []string{"public", "Sales"}, "public", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []schemasnapshot.ObjectRef{
			{Schema: "public", Name: "orders"},
			{Schema: "Sales", Name: "Invoices"},
		}, candidateRefs(got))
	})
}

func TestUnionDriftTableCandidates(t *testing.T) {
	orders := schemasnapshot.ObjectRef{Schema: "public", Name: "orders"}
	customers := schemasnapshot.ObjectRef{Schema: "public", Name: "customers"}
	products := schemasnapshot.ObjectRef{Schema: "public", Name: "products"} // dropped from live catalog
	audit := schemasnapshot.ObjectRef{Schema: "public", Name: "audit"}       // only in live capture

	tests := []struct {
		name             string
		liveRefs         []schemasnapshot.ObjectRef
		snapshotContents []*schemasnapshot.SnapshotContent
		liveContent      *schemasnapshot.SnapshotContent
		wantRefs         []schemasnapshot.ObjectRef
	}{
		{
			// Headline universe-fix case: products is present ONLY in a historical
			// snapshot (dropped from the live catalog) yet must still be a candidate.
			// orders appears in both live catalog and snapshot => deduped to one.
			name:             "snapshot-only (dropped) table is still a candidate; dedup across live+snapshot",
			liveRefs:         []schemasnapshot.ObjectRef{orders, customers},
			snapshotContents: []*schemasnapshot.SnapshotContent{snapContent(products, orders)},
			liveContent:      nil,
			wantRefs:         []schemasnapshot.ObjectRef{orders, customers, products},
		},
		{
			// A non-nil live capture contributes an additional table (audit) not seen
			// in the live catalog or the snapshot.
			name:             "live capture contributes an extra table",
			liveRefs:         []schemasnapshot.ObjectRef{orders},
			snapshotContents: []*schemasnapshot.SnapshotContent{snapContent(products)},
			liveContent:      snapContent(audit),
			wantRefs:         []schemasnapshot.ObjectRef{orders, products, audit},
		},
		{
			// The same table present in all three sources collapses to one candidate.
			name:             "same table in all three sources yields a single candidate",
			liveRefs:         []schemasnapshot.ObjectRef{orders},
			snapshotContents: []*schemasnapshot.SnapshotContent{snapContent(orders)},
			liveContent:      snapContent(orders),
			wantRefs:         []schemasnapshot.ObjectRef{orders},
		},
		{
			// A nil (placeholder / failed-to-load) snapshot in the chain is skipped.
			name:             "nil snapshot content is skipped",
			liveRefs:         []schemasnapshot.ObjectRef{orders},
			snapshotContents: []*schemasnapshot.SnapshotContent{nil, snapContent(products)},
			liveContent:      nil,
			wantRefs:         []schemasnapshot.ObjectRef{orders, products},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unionDriftTableCandidates("postgresql", "public", tt.liveRefs, tt.snapshotContents, tt.liveContent)
			gotRefs := candidateRefs(got)

			// Order is deterministic (live catalog -> snapshots -> live capture), so
			// assert the exact ordered candidate set.
			assert.Equal(t, tt.wantRefs, gotRefs)

			// Each candidate carries a non-nil sqlname.ObjectName view for glob matching.
			for _, c := range got {
				assert.NotNil(t, c.name, "candidate %v should have a non-nil ObjectName", c.ref)
			}
		})
	}
}

// ─── resolveDriftTableRefs ───────────────────────────────────────────────────

func TestResolveDriftTableRefs(t *testing.T) {
	// A multi-schema run with no "public": GetDefaultPGSchema reports no default,
	// so an unqualified pattern can match nothing at all.
	candidates := unionDriftTableCandidates("postgresql", "", nil, []*schemasnapshot.SnapshotContent{
		snapContent(
			schemasnapshot.ObjectRef{Schema: "sales", Name: "orders"},
			schemasnapshot.ObjectRef{Schema: "billing", Name: "invoices"},
		),
	}, nil)

	t.Run("unqualified pattern without a default schema is rejected by name", func(t *testing.T) {
		_, err := resolveDriftTableRefs(candidates, "orders", "table-list", false)
		require.Error(t, err)
		// The old behaviour reported "unknown table name", which sent the user
		// looking for a table that does exist.
		assert.Contains(t, err.Error(), "not schema-qualified")
		assert.NotContains(t, err.Error(), "unknown table name")
	})

	t.Run("qualified patterns still resolve without a default schema", func(t *testing.T) {
		refs, err := resolveDriftTableRefs(candidates, "sales.orders", "table-list", false)
		require.NoError(t, err)
		assert.Equal(t, []schemasnapshot.ObjectRef{{Schema: "sales", Name: "orders"}}, refs)
	})

	t.Run("a pattern matching nothing is still an unknown table", func(t *testing.T) {
		_, err := resolveDriftTableRefs(candidates, "sales.nope", "table-list", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown table name")
	})

	t.Run("case-sensitive names match only under their own quoting", func(t *testing.T) {
		mixed := unionDriftTableCandidates("postgresql", "public", nil, []*schemasnapshot.SnapshotContent{
			snapContent(
				schemasnapshot.ObjectRef{Schema: "public", Name: "Orders"},
				schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
			),
		}, nil)

		// An unquoted pattern folds case, so it reaches both spellings.
		refs, err := resolveDriftTableRefs(mixed, "orders", "table-list", true)
		require.NoError(t, err)
		assert.ElementsMatch(t, []schemasnapshot.ObjectRef{
			{Schema: "public", Name: "Orders"},
			{Schema: "public", Name: "orders"},
		}, refs)

		// A quoted pattern is exact, so it reaches only the capitalised one.
		refs, err = resolveDriftTableRefs(mixed, `public."Orders"`, "table-list", true)
		require.NoError(t, err)
		assert.Equal(t, []schemasnapshot.ObjectRef{{Schema: "public", Name: "Orders"}}, refs)
	})
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
