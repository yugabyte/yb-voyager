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

package schemasnapshot

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

// timeFromStr parses an RFC3339 time string for use in test assertions.
func timeFromStr(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return ts
}

// TestSchemaSnapshotJSONRoundTrip verifies SnapshotContent JSON round-trip with
// tables and columns only (v1 scope). Header fields (CapturedAt, DatabaseVersion,
// Schemas, Reason) are now in SnapshotHeader, not in SnapshotContent.
func TestSchemaSnapshotJSONRoundTrip(t *testing.T) {
	snap := &SnapshotContent{
		Version:      1,
		DatabaseType: "postgresql",
		DBMetadata: DBMetadata{
			Host:     "db.example.com",
			Port:     5432,
			Database: "mydb",
			User:     "voyager",
		},
		Tables: []Table{
			{
				ObjectRef: ObjectRef{Schema: "public", Name: "orders"},
				ID:        "16420",
				Kind:      TableKindOrdinary,
				Columns: []Column{
					{
						TableScopedRef: TableScopedRef{
							Table: ObjectRef{Schema: "public", Name: "orders"},
							Name:  "id",
						},
						ID:       "16420:1",
						DataType: "bigint",
						NotNull:  true,
					},
					{
						TableScopedRef: TableScopedRef{
							Table: ObjectRef{Schema: "public", Name: "orders"},
							Name:  "customer_id",
						},
						ID:       "16420:2",
						DataType: "integer",
						NotNull:  false,
						Default:  "0",
					},
				},
			},
			{
				ObjectRef: ObjectRef{Schema: "public", Name: "big_table"},
				ID:        "16421",
				Kind:      TableKindPartitioned,
			},
		},
	}

	data, err := json.Marshal(snap)
	require.NoError(t, err)

	var got SnapshotContent
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, snap.Version, got.Version)
	assert.Equal(t, snap.DatabaseType, got.DatabaseType)
	assert.Equal(t, snap.DBMetadata, got.DBMetadata)
	assert.Equal(t, snap.Tables, got.Tables)
}

// TestObjectRefForDisplayAndForKey verifies engine-aware rendering via ForDisplay
// and ForKey for PostgreSQL. For PG:
//   - ForDisplay uses minQuote2: no quotes for all-lowercase non-reserved names,
//     double-quotes otherwise.
//   - ForKey uses sqlname's Key(): the unquoted, dot-joined form (case preserved).
func TestObjectRefForDisplayAndForKey(t *testing.T) {
	const pg = constants.POSTGRESQL

	cases := []struct {
		name           string
		ref            ObjectRef
		wantForDisplay string
		wantForKey     string
	}{
		{
			name:           "lowercase table",
			ref:            ObjectRef{Schema: "public", Name: "orders"},
			wantForDisplay: `public.orders`,
			wantForKey:     `public.orders`,
		},
		{
			name:           "mixed-case table",
			ref:            ObjectRef{Schema: "public", Name: "Orders"},
			wantForDisplay: `public."Orders"`,
			wantForKey:     `public.Orders`,
		},
		{
			name:           "reserved word as table name",
			ref:            ObjectRef{Schema: "public", Name: "user"},
			wantForDisplay: `public."user"`,
			wantForKey:     `public.user`,
		},
		{
			name:           "name containing a dot",
			ref:            ObjectRef{Schema: "public", Name: "a.b"},
			wantForDisplay: `public."a.b"`,
			wantForKey:     `public.a.b`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantForDisplay, tc.ref.ForDisplay(pg), "ForDisplay")
			assert.Equal(t, tc.wantForKey, tc.ref.ForKey(pg), "ForKey")
		})
	}
}

// TestObjectRefForKeyDotCollisionKnownLimitation documents an accepted limitation of
// ForKey's unquoted, dot-joined key: two refs whose parts differ only in where the dot
// falls collapse to the same key — schema "a.b" / name "c" and schema "a" / name "b.c"
// both key to "a.b.c". This is inherent to every sqlname Key() use across the codebase;
// when sqlname gains quoting-aware keys, it resolves centrally with no change here.
func TestObjectRefForKeyDotCollisionKnownLimitation(t *testing.T) {
	const pg = constants.POSTGRESQL

	refDotInSchema := ObjectRef{Schema: "a.b", Name: "c"}
	refDotInName := ObjectRef{Schema: "a", Name: "b.c"}

	// KNOWN LIMITATION: the unquoted dot-join makes these collide.
	assert.Equal(t, refDotInSchema.ForKey(pg), refDotInName.ForKey(pg),
		"accepted limitation: dot-containing identifiers collide under the unquoted key")
	assert.Equal(t, "a.b.c", refDotInSchema.ForKey(pg))
}

// TestObjectRefForKeyCaseSensitivity verifies that case distinguishes identity:
// {public, Orders}.ForKey != {public, orders}.ForKey for PostgreSQL.
func TestObjectRefForKeyCaseSensitivity(t *testing.T) {
	const pg = constants.POSTGRESQL
	upper := ObjectRef{Schema: "public", Name: "Orders"}
	lower := ObjectRef{Schema: "public", Name: "orders"}
	assert.NotEqual(t, upper.ForKey(pg), lower.ForKey(pg),
		"mixed-case and lowercase table names must produce different ForKey values")
}

// TestColumnForKeyAndForDisplay verifies Column.ForKey and Column.ForDisplay
// produce the correct composite key/display strings.
func TestColumnForKeyAndForDisplay(t *testing.T) {
	const pg = constants.POSTGRESQL
	col := Column{
		TableScopedRef: TableScopedRef{
			Table: ObjectRef{Schema: "public", Name: "orders"},
			Name:  "Col",
		},
	}
	assert.Equal(t, `public.orders.Col`, col.ForKey(pg), "Column.ForKey")
	assert.Equal(t, `public.orders."Col"`, col.ForDisplay(pg), "Column.ForDisplay")
}

// TestTableKindConstants verifies the three table kind constants have correct values.
func TestTableKindConstants(t *testing.T) {
	assert.Equal(t, TableKind("ordinary"), TableKindOrdinary)
	assert.Equal(t, TableKind("partitioned"), TableKindPartitioned)
	assert.Equal(t, TableKind("foreign"), TableKindForeign)
}

// TestDBMetadataJSONFieldNames checks that DBMetadata marshals with the expected JSON keys.
// DatabaseType and Side have been removed from DBMetadata (they moved to SnapshotContent
// and SnapshotHeader respectively).
func TestDBMetadataJSONFieldNames(t *testing.T) {
	cs := DBMetadata{
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
		User:     "voyager",
	}
	data, err := json.Marshal(cs)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `"host"`)
	assert.Contains(t, s, `"port"`)
	assert.Contains(t, s, `"database"`)
	assert.Contains(t, s, `"user"`)
	// DatabaseType and Side are no longer in DBMetadata.
	assert.NotContains(t, s, `"database_type"`)
	assert.NotContains(t, s, `"side"`)
}

// TestSnapshotHeaderName verifies that SnapshotHeader.Name() derives the
// primary-key handle "{label}_{second-precision-timestamp}".
func TestSnapshotHeaderName(t *testing.T) {
	h := SnapshotHeader{
		Label:      LabelExportSchema,
		CapturedAt: timeFromStr(t, "2026-05-12T10:00:00Z"),
	}
	assert.Equal(t, "export_schema_20260512T100000Z", h.Name())
}

// TestSnapshotHeaderNameDifferentLabels verifies Name() with a longer label constant.
func TestSnapshotHeaderNameDifferentLabels(t *testing.T) {
	h := SnapshotHeader{
		Label:      LabelExportDataFromSourceExit,
		CapturedAt: timeFromStr(t, "2026-05-12T10:00:00Z"),
	}
	assert.Equal(t, "export_data_from_source_exit_20260512T100000Z", h.Name())
}
