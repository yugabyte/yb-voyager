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
			},
			{
				ObjectRef: ObjectRef{Schema: "public", Name: "big_table"},
				ID:        "16421",
				Kind:      TableKindPartitioned,
			},
		},
		Columns: []Column{
			{
				Table:    ObjectRef{Schema: "public", Name: "orders"},
				ID:       "16420:1",
				Name:     "id",
				DataType: "bigint",
				NotNull:  true,
			},
			{
				Table:    ObjectRef{Schema: "public", Name: "orders"},
				ID:       "16420:2",
				Name:     "customer_id",
				DataType: "integer",
				NotNull:  false,
				Default:  "0",
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
	assert.Equal(t, snap.Columns, got.Columns)
}

// TestObjectRefString verifies ObjectRef.String() returns "schema.name".
func TestObjectRefString(t *testing.T) {
	ref := ObjectRef{Schema: "public", Name: "orders"}
	assert.Equal(t, "public.orders", ref.String())
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
