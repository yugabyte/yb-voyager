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

// TestSchemaSnapshotJSONRoundTrip verifies SchemaSnapshot JSON round-trip with
// tables and columns only (v1 scope).
func TestSchemaSnapshotJSONRoundTrip(t *testing.T) {
	capturedAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	snap := &SchemaSnapshot{
		Version:         1,
		DatabaseType:    "postgresql",
		DatabaseVersion: "16.14",
		StableIdentity:  true,
		CapturedAt:      capturedAt,
		CaptureSource: CaptureSource{
			DatabaseType: "postgresql",
			Host:         "db.example.com",
			Port:         5432,
			Database:     "mydb",
			User:         "voyager",
			Side:         SideSource,
		},
		Schemas: []string{"public", "sales"},
		Series:  LabelExportDataFromSourceExit,
		Reason:  ReasonCutover,
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

	var got SchemaSnapshot
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, snap.Version, got.Version)
	assert.Equal(t, snap.DatabaseType, got.DatabaseType)
	assert.Equal(t, snap.DatabaseVersion, got.DatabaseVersion)
	assert.Equal(t, snap.StableIdentity, got.StableIdentity)
	assert.True(t, snap.CapturedAt.Equal(got.CapturedAt), "CapturedAt mismatch")
	assert.Equal(t, snap.CaptureSource, got.CaptureSource)
	assert.Equal(t, snap.Schemas, got.Schemas)
	assert.Equal(t, snap.Series, got.Series)
	assert.Equal(t, snap.Reason, got.Reason)
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

// TestCaptureSourceJSONFieldNames checks that CaptureSource marshals with the expected JSON keys.
func TestCaptureSourceJSONFieldNames(t *testing.T) {
	cs := CaptureSource{
		DatabaseType: "postgresql",
		Host:         "localhost",
		Port:         5432,
		Database:     "mydb",
		User:         "voyager",
		Side:         SideSource,
	}
	data, err := json.Marshal(cs)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `"database_type"`)
	assert.Contains(t, s, `"host"`)
	assert.Contains(t, s, `"port"`)
	assert.Contains(t, s, `"database"`)
	assert.Contains(t, s, `"user"`)
	assert.Contains(t, s, `"side"`)
}
