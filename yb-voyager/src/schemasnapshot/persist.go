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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// currentSnapshotVersion is the snapshot JSON version this library understands.
const currentSnapshotVersion = 1

// Sentinel errors returned by LoadSnapshotByName.
var (
	ErrSnapshotNotFound    = errors.New("snapshot not found")
	ErrPlaceholderSnapshot = errors.New("snapshot is a placeholder: no schema content captured")
)

// SaveSnapshot persists a fully-populated SchemaSnapshot (header + schema content) to the
// metadata database. The header must have a non-empty Label. Returns the derived
// name "{label}_{second-precision-timestamp}" on success.
func SaveSnapshot(mdb *metadb.MetaDB, snap *SchemaSnapshot) (string, error) {
	if snap.Header.Label == "" {
		return "", goerrors.Errorf("schemasnapshot: snapshot has no label; cannot persist")
	}

	data, err := json.Marshal(snap.Content)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}

	side := snap.Header.Side
	if side == "" {
		side = SideSource
	}

	name := snap.Header.Name()
	row := metadb.SchemaSnapshotRow{
		Name:            name,
		Label:           snap.Header.Label,
		Reason:          snap.Header.Reason,
		Side:            side,
		CapturedAt:      snap.Header.CapturedAt,
		DatabaseVersion: snap.Header.DatabaseVersion,
		Schemas:         schemasToString(snap.Header.Schemas),
		IsPlaceholder:   snap.Header.IsPlaceholder,
		SnapshotJSON:    sql.NullString{String: string(data), Valid: true},
	}

	if err := mdb.InsertSchemaSnapshot(row); err != nil {
		return "", err
	}
	return name, nil
}

// SavePlaceholder records a metadata-only row (snapshot_json NULL) for when a capture
// attempt fails mid-process but the lifecycle moment still needs a timeline marker.
// h.Side defaults to SideSource if empty. An empty DatabaseVersion is accepted.
// Returns the derived name on success.
func SavePlaceholder(mdb *metadb.MetaDB, h SnapshotHeader) (string, error) {
	if h.Label == "" {
		return "", goerrors.Errorf("schemasnapshot: placeholder has no label; cannot persist")
	}

	side := h.Side
	if side == "" {
		side = SideSource
	}

	name := h.Name()
	row := metadb.SchemaSnapshotRow{
		Name:            name,
		Label:           h.Label,
		Reason:          h.Reason,
		Side:            side,
		CapturedAt:      h.CapturedAt,
		DatabaseVersion: h.DatabaseVersion,
		Schemas:         schemasToString(h.Schemas),
		SnapshotJSON:    sql.NullString{Valid: false},
	}

	if err := mdb.InsertSchemaSnapshotPlaceholder(row); err != nil {
		return "", err
	}
	return name, nil
}

// ListSnapshots returns the header for every snapshot (real and placeholder),
// sorted oldest-first by captured_at. Header columns only; no blob deserialization.
func ListSnapshots(mdb *metadb.MetaDB) ([]SnapshotHeader, error) {
	rows, err := mdb.ListSchemaSnapshots()
	if err != nil {
		return nil, err
	}
	result := make([]SnapshotHeader, 0, len(rows))
	for _, r := range rows {
		result = append(result, rowToHeader(r))
	}
	return result, nil
}

// LoadSnapshotByName fetches the snapshot with the given name and deserializes it.
// Returns ErrSnapshotNotFound if the row does not exist (or the table has not been created yet).
// Returns ErrPlaceholderSnapshot if the row is a placeholder (no schema content).
func LoadSnapshotByName(mdb *metadb.MetaDB, name string) (*SnapshotContent, error) {
	row, err := mdb.GetSchemaSnapshotByName(name)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrSnapshotNotFound
	}
	if row.IsPlaceholder {
		return nil, ErrPlaceholderSnapshot
	}
	return DecodeSnapshot([]byte(row.SnapshotJSON.String))
}

// DecodeSnapshot deserializes a SnapshotContent from raw JSON bytes.
// It enforces the versioning gate: a Version > currentSnapshotVersion is rejected,
// and a missing or zero Version is rejected as unreadable.
func DecodeSnapshot(data []byte) (*SnapshotContent, error) {
	var snap SnapshotContent
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	if snap.Version == 0 {
		return nil, goerrors.Errorf("snapshot has no version set (Version 0 or missing); this library requires Version %d", currentSnapshotVersion)
	}
	if snap.Version > currentSnapshotVersion {
		return nil, goerrors.Errorf("snapshot Version %d is newer than this library understands (expected Version %d); upgrade yb-voyager", snap.Version, currentSnapshotVersion)
	}
	return &snap, nil
}

// ─── helpers ────────────────────────────────────────────────────────────────────

// deriveName builds the primary-key name "{label}_{timestamp-at-second-precision}".
func deriveName(label string, capturedAt time.Time) string {
	return fmt.Sprintf("%s_%s", label, capturedAt.UTC().Format("20060102T150405Z"))
}

// schemasToString serializes the schema slice to a JSON array string.
// JSON encoding is delimiter-safe (a schema named "a,b" survives the round-trip).
func schemasToString(schemas []string) string {
	b, _ := json.Marshal(schemas)
	return string(b)
}

// schemasFromString deserializes a schema slice from a JSON array string.
// An empty string returns nil (not an empty slice) for cleanliness.
// For backward compatibility it falls back to comma-splitting when the value
// is not valid JSON (rows written by older voyager binaries).
func schemasFromString(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return strings.Split(s, ",") // legacy/non-JSON rows
	}
	return out
}

// rowToHeader converts a lightweight list row to a SnapshotHeader value.
// IsPlaceholder comes directly from the is_placeholder column; no blob field is present.
// Name is derived (not stored), so it is not set on the header struct.
func rowToHeader(r metadb.SchemaSnapshotListRow) SnapshotHeader {
	return SnapshotHeader{
		Label:           r.Label,
		Reason:          r.Reason,
		Side:            r.Side,
		CapturedAt:      r.CapturedAt,
		DatabaseVersion: r.DatabaseVersion,
		Schemas:         schemasFromString(r.Schemas),
		IsPlaceholder:   r.IsPlaceholder,
	}
}
