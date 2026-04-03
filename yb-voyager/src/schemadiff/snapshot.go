package schemadiff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const snapshotDir = "metainfo/schema/snapshots"

func SnapshotPath(exportDir string) string {
	return filepath.Join(exportDir, snapshotDir)
}

func SaveSnapshot(exportDir, label string, snapshot *SchemaSnapshot) (string, error) {
	dir := SnapshotPath(exportDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating snapshot directory: %w", err)
	}
	ts := snapshot.CapturedAt.UTC().Format("2006-01-02T15:04:05")
	filename := fmt.Sprintf("%s_%s.json", label, ts)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling snapshot: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("writing snapshot: %w", err)
	}
	return filename, nil
}

func LoadSnapshot(path string) (*SchemaSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}
	var snapshot SchemaSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("unmarshaling snapshot: %w", err)
	}
	return &snapshot, nil
}

type SnapshotInfo struct {
	Name      string
	Path      string
	Side      string // "source" or "target"
	Timestamp time.Time
}

func ListSnapshots(exportDir string) ([]SnapshotInfo, error) {
	dir := SnapshotPath(exportDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading snapshot directory: %w", err)
	}

	var infos []SnapshotInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		side := "unknown"
		if strings.HasPrefix(name, "source_") {
			side = "source"
		} else if strings.HasPrefix(name, "target_") {
			side = "target"
		}

		ts := parseTimestampFromName(name)
		infos = append(infos, SnapshotInfo{
			Name:      name,
			Path:      filepath.Join(dir, entry.Name()),
			Side:      side,
			Timestamp: ts,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Timestamp.Before(infos[j].Timestamp)
	})
	return infos, nil
}

func LatestSnapshot(exportDir, side string) (*SchemaSnapshot, error) {
	infos, err := ListSnapshots(exportDir)
	if err != nil {
		return nil, err
	}

	var latest *SnapshotInfo
	for i := range infos {
		if infos[i].Side == side {
			latest = &infos[i]
		}
	}
	if latest == nil {
		return nil, nil
	}
	return LoadSnapshot(latest.Path)
}

func parseTimestampFromName(name string) time.Time {
	// Name format: source_export_schema_2026-03-25T10:00:00
	// or target_import_data_start_2026-03-26T15:00:00
	// Find the last part that looks like a timestamp.
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return time.Time{}
	}
	tsStr := parts[len(parts)-1]
	t, err := time.Parse("2006-01-02T15:04:05", tsStr)
	if err != nil {
		return time.Time{}
	}
	return t
}
