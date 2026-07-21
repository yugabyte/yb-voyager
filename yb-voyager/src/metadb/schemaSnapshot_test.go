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

package metadb

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSchemaSnapshotMetaDB creates a minimal MetaDB backed by a temp SQLite
// file for schema-snapshot-specific tests.
func newTestSchemaSnapshotMetaDB(t *testing.T) *MetaDB {
	t.Helper()
	dir := t.TempDir()
	metainfoDir := filepath.Join(dir, "metainfo")
	require.NoError(t, os.MkdirAll(metainfoDir, 0o755))
	f, err := os.Create(filepath.Join(metainfoDir, "meta.db"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mdb, err := NewMetaDB(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mdb.db.Close() })
	return mdb
}

// makeSchemaSnapshotRow constructs a minimal SchemaSnapshotRow for testing.
func makeSchemaSnapshotRow(name string, capturedAt time.Time) SchemaSnapshotRow {
	return SchemaSnapshotRow{
		Name:       name,
		Label:      "export_schema",
		Reason:     "",
		Side:       "source",
		CapturedAt: capturedAt,
		Schemas:    "public",
		SnapshotJSON: sql.NullString{
			String: `{"version":1}`,
			Valid:  true,
		},
	}
}

// TestListSchemaSnapshotsSubSecondOrder verifies that ListSchemaSnapshots returns
// rows in true chronological order even when timestamps differ only in sub-second
// fractions. This test exposes the RFC3339Nano bug: that format strips trailing
// zeros, producing variable-length strings whose lexicographic order diverges from
// chronological order in two known cases:
//
//  1. Zero fraction ("...00Z") sorts AFTER a half-second fraction ("...00.5Z") because
//     'Z' (ASCII 90) > '.' (ASCII 46) — but 0ms is before 500ms.
//  2. 120ms ("...00.12Z") sorts AFTER 123ms ("...00.123Z") because 'Z' (90) > '3' (51) —
//     but 120ms is before 123ms.
//
// The fixed-width capturedAtLayout ("2006-01-02T15:04:05.000000000Z") avoids both by
// always emitting nine decimal digits, making lexicographic == chronological.
func TestListSchemaSnapshotsSubSecondOrder(t *testing.T) {
	mdb := newTestSchemaSnapshotMetaDB(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Four timestamps that differ only in sub-second fraction.
	t0 := base                             // 0 ns (zero fraction)
	t1 := base.Add(120 * time.Millisecond) // 120 ms
	t2 := base.Add(123 * time.Millisecond) // 123 ms  (> t1)
	t3 := base.Add(500 * time.Millisecond) // 500 ms  (> t0 via the Z-vs-dot case)

	// Insert in an order that is NOT chronological to prevent SQLite insertion
	// order from masking a sort bug.
	rows := []SchemaSnapshotRow{
		makeSchemaSnapshotRow("snap_t2", t2),
		makeSchemaSnapshotRow("snap_t3", t3),
		makeSchemaSnapshotRow("snap_t0", t0),
		makeSchemaSnapshotRow("snap_t1", t1),
	}
	for _, r := range rows {
		require.NoError(t, mdb.InsertSchemaSnapshot(r))
	}

	list, err := mdb.ListSchemaSnapshots()
	require.NoError(t, err)
	require.Len(t, list, 4)

	// Must be in strict chronological (ascending) order.
	assert.True(t, list[0].CapturedAt.Equal(t0), "row[0] should be t0 (0 ms), got %v", list[0].CapturedAt)
	assert.True(t, list[1].CapturedAt.Equal(t1), "row[1] should be t1 (120 ms), got %v", list[1].CapturedAt)
	assert.True(t, list[2].CapturedAt.Equal(t2), "row[2] should be t2 (123 ms), got %v", list[2].CapturedAt)
	assert.True(t, list[3].CapturedAt.Equal(t3), "row[3] should be t3 (500 ms), got %v", list[3].CapturedAt)
}

// TestListSchemaSnapshotsIsPlaceholder verifies that ListSchemaSnapshots correctly
// sets IsPlaceholder by reading the explicit is_placeholder column directly, without
// ever selecting the snapshot_json blob. A full snapshot must have IsPlaceholder=false
// and a placeholder row must have IsPlaceholder=true.
func TestListSchemaSnapshotsIsPlaceholder(t *testing.T) {
	mdb := newTestSchemaSnapshotMetaDB(t)

	t1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)

	// Insert a full snapshot (snapshot_json is set).
	fullRow := makeSchemaSnapshotRow("snap_full", t1)
	require.NoError(t, mdb.InsertSchemaSnapshot(fullRow))

	// Insert a placeholder row (snapshot_json NULL).
	placeholderRow := makeSchemaSnapshotRow("snap_placeholder", t2)
	require.NoError(t, mdb.InsertSchemaSnapshotPlaceholder(placeholderRow))

	list, err := mdb.ListSchemaSnapshots()
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Rows are ordered oldest-first: full snapshot (t1) then placeholder (t2).
	assert.Equal(t, "snap_full", list[0].Name)
	assert.False(t, list[0].IsPlaceholder, "full snapshot must have IsPlaceholder=false")

	assert.Equal(t, "snap_placeholder", list[1].Name)
	assert.True(t, list[1].IsPlaceholder, "placeholder row must have IsPlaceholder=true")
}
