package schemadiff

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadSnapshot(t *testing.T) {
	tmpDir := t.TempDir()

	snap := &SchemaSnapshot{
		DatabaseType: "postgresql",
		Schemas:      []string{"public"},
		CapturedAt:   time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
		Tables: []Table{
			{Schema: "public", Name: "orders", IsPartitioned: false},
			{Schema: "public", Name: "users", IsPartitioned: false},
		},
		Columns: []Column{
			{Schema: "public", TableName: "orders", Name: "id", DataType: "int4", OrdinalPos: 1},
			{Schema: "public", TableName: "orders", Name: "amount", DataType: "numeric", TypeModifier: "10,2", OrdinalPos: 2},
		},
	}

	filename, err := SaveSnapshot(tmpDir, "source_export_schema", snap)
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	if filename != "source_export_schema_2026-03-25T10:00:00.json" {
		t.Errorf("unexpected filename: %s", filename)
	}

	path := filepath.Join(SnapshotPath(tmpDir), filename)
	loaded, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	if loaded.DatabaseType != "postgresql" {
		t.Errorf("DatabaseType: got %s", loaded.DatabaseType)
	}
	if len(loaded.Tables) != 2 {
		t.Errorf("Tables: got %d", len(loaded.Tables))
	}
	if len(loaded.Columns) != 2 {
		t.Errorf("Columns: got %d", len(loaded.Columns))
	}
	if loaded.Columns[1].TypeModifier != "10,2" {
		t.Errorf("TypeModifier: got %q", loaded.Columns[1].TypeModifier)
	}
}

func TestListSnapshots(t *testing.T) {
	tmpDir := t.TempDir()

	snap1 := &SchemaSnapshot{
		DatabaseType: "postgresql",
		Schemas:      []string{"public"},
		CapturedAt:   time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
	}
	snap2 := &SchemaSnapshot{
		DatabaseType: "postgresql",
		Schemas:      []string{"public"},
		CapturedAt:   time.Date(2026, 3, 26, 14, 0, 0, 0, time.UTC),
	}
	snap3 := &SchemaSnapshot{
		DatabaseType: "yugabytedb",
		Schemas:      []string{"public"},
		CapturedAt:   time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
	}

	SaveSnapshot(tmpDir, "source_export_schema", snap1)
	SaveSnapshot(tmpDir, "source_export_data_start", snap2)
	SaveSnapshot(tmpDir, "target_import_schema", snap3)

	infos, err := ListSnapshots(tmpDir)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}

	if len(infos) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(infos))
	}

	// Verify sorted by timestamp
	if !infos[0].Timestamp.Before(infos[1].Timestamp) {
		t.Errorf("snapshots not sorted by timestamp")
	}

	// Verify sides
	sourceCount, targetCount := 0, 0
	for _, info := range infos {
		switch info.Side {
		case "source":
			sourceCount++
		case "target":
			targetCount++
		}
	}
	if sourceCount != 2 {
		t.Errorf("expected 2 source snapshots, got %d", sourceCount)
	}
	if targetCount != 1 {
		t.Errorf("expected 1 target snapshot, got %d", targetCount)
	}
}

func TestLatestSnapshot(t *testing.T) {
	tmpDir := t.TempDir()

	snap1 := &SchemaSnapshot{
		DatabaseType: "postgresql",
		Schemas:      []string{"public"},
		CapturedAt:   time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
		Tables:       []Table{{Schema: "public", Name: "v1"}},
	}
	snap2 := &SchemaSnapshot{
		DatabaseType: "postgresql",
		Schemas:      []string{"public"},
		CapturedAt:   time.Date(2026, 3, 26, 14, 0, 0, 0, time.UTC),
		Tables:       []Table{{Schema: "public", Name: "v2"}},
	}

	SaveSnapshot(tmpDir, "source_export_schema", snap1)
	SaveSnapshot(tmpDir, "source_export_data_start", snap2)

	latest, err := LatestSnapshot(tmpDir, "source")
	if err != nil {
		t.Fatalf("LatestSnapshot: %v", err)
	}
	if latest == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if latest.Tables[0].Name != "v2" {
		t.Errorf("expected latest snapshot (v2), got %s", latest.Tables[0].Name)
	}
}

func TestLatestSnapshot_NoSnapshots(t *testing.T) {
	tmpDir := t.TempDir()

	latest, err := LatestSnapshot(tmpDir, "source")
	if err != nil {
		t.Fatalf("LatestSnapshot: %v", err)
	}
	if latest != nil {
		t.Errorf("expected nil for no snapshots")
	}
}

func TestLatestSnapshot_WrongSide(t *testing.T) {
	tmpDir := t.TempDir()

	snap := &SchemaSnapshot{
		DatabaseType: "postgresql",
		CapturedAt:   time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
	}
	SaveSnapshot(tmpDir, "source_export_schema", snap)

	latest, err := LatestSnapshot(tmpDir, "target")
	if err != nil {
		t.Fatalf("LatestSnapshot: %v", err)
	}
	if latest != nil {
		t.Errorf("expected nil for wrong side")
	}
}

func TestListSnapshots_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	infos, err := ListSnapshots(tmpDir)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if infos != nil {
		t.Errorf("expected nil for empty dir, got %v", infos)
	}
}

func TestSnapshotPath(t *testing.T) {
	got := SnapshotPath("/tmp/export")
	expected := filepath.Join("/tmp/export", "metainfo/schema/snapshots")
	if got != expected {
		t.Errorf("SnapshotPath: got %q, want %q", got, expected)
	}
}

func TestSaveSnapshot_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "nested", "deep")

	snap := &SchemaSnapshot{
		CapturedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	_, err := SaveSnapshot(dir, "source_test", snap)
	if err != nil {
		t.Fatalf("SaveSnapshot with nested dir: %v", err)
	}

	if _, err := os.Stat(SnapshotPath(dir)); os.IsNotExist(err) {
		t.Error("expected snapshot directory to be created")
	}
}
