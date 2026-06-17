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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// newTestMetaDB creates a MetaDB backed by a fresh SQLite file in dir.
func newTestMetaDB(t *testing.T) *metadb.MetaDB {
	t.Helper()
	dir := t.TempDir()
	metainfoDir := filepath.Join(dir, "metainfo")
	require.NoError(t, os.MkdirAll(metainfoDir, 0o755))
	f, err := os.Create(filepath.Join(metainfoDir, "meta.db"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mdb, err := metadb.NewMetaDB(dir)
	require.NoError(t, err)
	return mdb
}

// makeSnapshot returns a minimal populated SchemaSnapshot with Version 1.
func makeSnapshot(capturedAt time.Time) *SchemaSnapshot {
	return &SchemaSnapshot{
		Version:         1,
		DatabaseType:    "postgresql",
		DatabaseVersion: "16.14",
		StableIdentity:  true,
		CapturedAt:      capturedAt,
		CaptureSource: CaptureSource{
			DatabaseType: "postgresql",
			Host:         "localhost",
			Port:         5432,
			Database:     "testdb",
			User:         "voyager",
			Role:         "source",
		},
		Schemas: []string{"public"},
		Tables: []Table{
			{
				ObjectRef: ObjectRef{Schema: "public", Name: "orders"},
				ID:        "16420",
				Kind:      TableKindOrdinary,
			},
		},
	}
}

// ─── Save → Load round-trip ───────────────────────────────────────────────────

func TestSaveSnapshotRoundTrip(t *testing.T) {
	mdb := newTestMetaDB(t)

	capturedAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	snap := makeSnapshot(capturedAt)

	name, err := SaveSnapshot(mdb, snap, LabelExportDataFromSourceExit, "cutover")
	require.NoError(t, err)
	assert.Equal(t, "export_data_from_source_exit_20260512T100000Z", name)

	// Series and Reason must be stamped in place.
	assert.Equal(t, LabelExportDataFromSourceExit, snap.Series)
	assert.Equal(t, "cutover", snap.Reason)

	loaded, err := LoadSnapshotByName(mdb, name)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, 1, loaded.Version)
	assert.Equal(t, "postgresql", loaded.DatabaseType)
	assert.Equal(t, "16.14", loaded.DatabaseVersion)
	assert.Equal(t, LabelExportDataFromSourceExit, loaded.Series)
	assert.Equal(t, "cutover", loaded.Reason)
	assert.Equal(t, []string{"public"}, loaded.Schemas)
	assert.True(t, capturedAt.Equal(loaded.CapturedAt), "CapturedAt mismatch")
	require.Len(t, loaded.Tables, 1)
	assert.Equal(t, "orders", loaded.Tables[0].Name)
}

// ─── ListSnapshots: ordered oldest-first, mixes real + placeholder ────────────

func TestListSnapshotsOrder(t *testing.T) {
	mdb := newTestMetaDB(t)

	t1 := time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	// Insert in non-chronological order.
	_, err := SaveSnapshot(mdb, makeSnapshot(t3), LabelExportDataFromSourceExit, "complete")
	require.NoError(t, err)

	_, err = SavePlaceholder(mdb, LabelExportDataFromSourceStart, "initial", t1, "16.14", []string{"public"})
	require.NoError(t, err)

	_, err = SaveSnapshot(mdb, makeSnapshot(t2), LabelExportDataFromSourcePeriodic, "")
	require.NoError(t, err)

	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	require.Len(t, list, 3)

	// Must be oldest-first.
	assert.True(t, list[0].CapturedAt.Equal(t1), "first should be t1")
	assert.True(t, list[1].CapturedAt.Equal(t2), "second should be t2")
	assert.True(t, list[2].CapturedAt.Equal(t3), "third should be t3")

	// Placeholder detection.
	assert.True(t, list[0].IsPlaceholder, "placeholder row should have IsPlaceholder=true")
	assert.False(t, list[1].IsPlaceholder, "real snapshot should have IsPlaceholder=false")
	assert.False(t, list[2].IsPlaceholder)

	// Header columns on the placeholder.
	assert.Equal(t, LabelExportDataFromSourceStart, list[0].Label)
	assert.Equal(t, "initial", list[0].Reason)
	assert.Equal(t, "source", list[0].Side)
	assert.Equal(t, "16.14", list[0].DatabaseVersion)
	assert.Equal(t, []string{"public"}, list[0].Schemas)

	// Real snapshots must also carry Side == "source".
	assert.Equal(t, "source", list[1].Side, "real snapshot (t2) side should be source")
	assert.Equal(t, "source", list[2].Side, "real snapshot (t3) side should be source")
}

// ─── ListSnapshots on empty DB (table does not exist yet) ─────────────────────

func TestListSnapshotsEmptyDB(t *testing.T) {
	mdb := newTestMetaDB(t)
	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	assert.Empty(t, list)
}

// ─── SavePlaceholder → LoadSnapshotByName → ErrPlaceholderSnapshot ───────────

func TestLoadPlaceholderReturnsError(t *testing.T) {
	mdb := newTestMetaDB(t)
	capturedAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	name, err := SavePlaceholder(mdb, LabelExportSchema, "", capturedAt, "", []string{"public"})
	require.NoError(t, err)

	_, err = LoadSnapshotByName(mdb, name)
	assert.ErrorIs(t, err, ErrPlaceholderSnapshot)
}

// ─── Load missing name → ErrSnapshotNotFound ─────────────────────────────────

func TestLoadMissingNameReturnsNotFound(t *testing.T) {
	mdb := newTestMetaDB(t)

	_, err := LoadSnapshotByName(mdb, "no_such_name")
	assert.ErrorIs(t, err, ErrSnapshotNotFound)
}

// ErrSnapshotNotFound is also returned when the table has never been written.
func TestLoadMissingNameNoTable(t *testing.T) {
	mdb := newTestMetaDB(t)
	_, err := LoadSnapshotByName(mdb, "anything")
	assert.ErrorIs(t, err, ErrSnapshotNotFound)
}

// ─── Version gate in DecodeSnapshot ──────────────────────────────────────────

func TestDecodeSnapshotVersionTooNew(t *testing.T) {
	snap := makeSnapshot(time.Now())
	snap.Version = 2 // future version
	data, err := json.Marshal(snap)
	require.NoError(t, err)

	_, err = DecodeSnapshot(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newer than this library understands")
	assert.Contains(t, err.Error(), "expected Version 1")
}

func TestDecodeSnapshotVersionZero(t *testing.T) {
	snap := makeSnapshot(time.Now())
	snap.Version = 0
	data, err := json.Marshal(snap)
	require.NoError(t, err)

	_, err = DecodeSnapshot(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Version 0 or missing")
}

func TestDecodeSnapshotVersionMissing(t *testing.T) {
	// Omit "version" field entirely (Version is 0 by default in Go).
	raw := `{"database_type":"postgresql","stable_identity":true,"captured_at":"2026-05-12T10:00:00Z","capture_source":{},"schemas":["public"]}`
	_, err := DecodeSnapshot([]byte(raw))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Version 0 or missing")
}

func TestDecodeSnapshotVersionOne(t *testing.T) {
	snap := makeSnapshot(time.Now())
	snap.Version = 1
	data, err := json.Marshal(snap)
	require.NoError(t, err)

	got, err := DecodeSnapshot(data)
	require.NoError(t, err)
	assert.Equal(t, 1, got.Version)
}

// ─── Name collision → error ───────────────────────────────────────────────────

func TestSaveSnapshotNameCollision(t *testing.T) {
	mdb := newTestMetaDB(t)
	capturedAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	snap1 := makeSnapshot(capturedAt)
	_, err := SaveSnapshot(mdb, snap1, LabelExportSchema, "")
	require.NoError(t, err)

	// Same label + same second → same derived name → collision.
	snap2 := makeSnapshot(capturedAt)
	_, err = SaveSnapshot(mdb, snap2, LabelExportSchema, "")
	require.Error(t, err)
}

// ─── SavePlaceholder with empty dbVersion ────────────────────────────────────

func TestSavePlaceholderEmptyDbVersion(t *testing.T) {
	mdb := newTestMetaDB(t)
	capturedAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	name, err := SavePlaceholder(mdb, LabelExportDataFromSourceStart, "resume", capturedAt, "", []string{"public", "sales"})
	require.NoError(t, err)
	assert.NotEmpty(t, name)

	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "", list[0].DatabaseVersion)
	assert.Equal(t, []string{"public", "sales"}, list[0].Schemas)
	assert.True(t, list[0].IsPlaceholder)
}

// ─── Confirm ErrSnapshotNotFound and ErrPlaceholderSnapshot are distinct ──────

func TestSentinelErrorsAreDistinct(t *testing.T) {
	assert.False(t, errors.Is(ErrSnapshotNotFound, ErrPlaceholderSnapshot))
	assert.False(t, errors.Is(ErrPlaceholderSnapshot, ErrSnapshotNotFound))
}

// ─── SaveSnapshot with empty CaptureSource.Role defaults Side to "source" ────

func TestSaveSnapshotEmptyRoleDefaultsSideToSource(t *testing.T) {
	mdb := newTestMetaDB(t)

	capturedAt := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	snap := makeSnapshot(capturedAt)
	// Clear the role so the fallback logic is exercised.
	snap.CaptureSource.Role = ""

	name, err := SaveSnapshot(mdb, snap, LabelExportSchema, "")
	require.NoError(t, err)
	assert.NotEmpty(t, name)

	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "source", list[0].Side, "empty Role should default Side to 'source'")
}

// ─── SaveSnapshot rejects unknown label ───────────────────────────────────────

func TestSaveSnapshotBadLabel(t *testing.T) {
	mdb := newTestMetaDB(t)
	snap := makeSnapshot(time.Now())
	_, err := SaveSnapshot(mdb, snap, "invalid_label", "")
	require.Error(t, err)
}
