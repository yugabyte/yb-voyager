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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// currentSnapshotVersion is the snapshot JSON version this library understands.
const currentSnapshotVersion = 1

// Sentinel errors returned by LoadSnapshotByName.
var (
	ErrSnapshotNotFound    = errors.New("snapshot not found")
	ErrPlaceholderSnapshot = errors.New("snapshot is a placeholder: no schema content captured")
)

// SnapshotMetadata holds the header columns for a persisted snapshot.
// It is returned by ListSnapshots and never contains the full schema content.
type SnapshotMetadata struct {
	Name            string    // user-facing handle (primary key): "{label}_{timestamp-at-second-precision}"
	Label           string    // the capture label/series this snapshot was saved under (a labels.go constant).
	Reason          string    // capture reason where the series carries one; "" otherwise
	Side            string    // which side of the migration produced it; "source" in v1
	CapturedAt      time.Time // full-precision capture time (UTC)
	DatabaseVersion string    // source server version, truncated at the first space, e.g. "16.4"
	Schemas         []string  // schemas in scope for this snapshot
	IsPlaceholder   bool      // true when snapshot_json is NULL (capture-attempt marker, no schema content)
}

// deriveName builds the primary-key name "{label}_{timestamp-at-second-precision}".
func deriveName(label string, capturedAt time.Time) string {
	return fmt.Sprintf("%s_%s", label, capturedAt.UTC().Format("20060102T150405Z"))
}

// schemasToString joins a string slice to a comma-separated string.
func schemasToString(schemas []string) string {
	return strings.Join(schemas, ",")
}

// schemasFromString splits a comma-separated string back to a slice.
// An empty string returns nil (not an empty slice) for cleanliness.
func schemasFromString(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// rowToMetadata converts a metadb row to a SnapshotMetadata value.
func rowToMetadata(r metadb.SchemaSnapshotRow) SnapshotMetadata {
	return SnapshotMetadata{
		Name:            r.Name,
		Label:           r.Label,
		Reason:          r.Reason,
		Side:            r.Side,
		CapturedAt:      r.CapturedAt,
		DatabaseVersion: r.DatabaseVersion,
		Schemas:         schemasFromString(r.Schemas),
		IsPlaceholder:   !r.SnapshotJSON.Valid,
	}
}

// SaveSnapshot stamps label/reason into snap.Series/Reason, serializes snap to JSON,
// and persists it under the name "{label}_{second-precision-timestamp}".
// It mutates snap in place.
// Returns the derived name on success.
func SaveSnapshot(mdb *metadb.MetaDB, snap *SchemaSnapshot, label, reason string) (string, error) {
	if err := ValidateLabelReason(label, reason); err != nil {
		return "", err
	}

	// Stamp Series and Reason onto the snapshot before serializing.
	snap.Series = label
	snap.Reason = reason

	data, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}

	side := snap.CaptureSource.Role
	if side == "" {
		side = RoleSource
	}

	name := deriveName(label, snap.CapturedAt)
	row := metadb.SchemaSnapshotRow{
		Name:            name,
		Label:           label,
		Reason:          reason,
		Side:            side,
		CapturedAt:      snap.CapturedAt,
		DatabaseVersion: snap.DatabaseVersion,
		Schemas:         schemasToString(snap.Schemas),
		SnapshotJSON:    sql.NullString{String: string(data), Valid: true},
	}

	if err := mdb.InsertSchemaSnapshot(row); err != nil {
		return "", err
	}
	return name, nil
}

// SavePlaceholder records a metadata-only row (snapshot_json NULL) for when a capture
// attempt fails mid-process but the lifecycle moment still needs a timeline marker.
// role identifies which database the failed capture attempt targeted (e.g. RoleSource);
// an empty role defaults to RoleSource.
// An empty dbVersion is accepted.
// Returns the derived name on success.
func SavePlaceholder(mdb *metadb.MetaDB, label, reason, role string, capturedAt time.Time, dbVersion string, schemas []string) (string, error) {
	if err := ValidateLabelReason(label, reason); err != nil {
		return "", err
	}

	side := role
	if side == "" {
		side = RoleSource
	}

	name := deriveName(label, capturedAt)
	row := metadb.SchemaSnapshotRow{
		Name:            name,
		Label:           label,
		Reason:          reason,
		Side:            side,
		CapturedAt:      capturedAt,
		DatabaseVersion: dbVersion,
		Schemas:         schemasToString(schemas),
		SnapshotJSON:    sql.NullString{Valid: false},
	}

	if err := mdb.InsertSchemaSnapshotPlaceholder(row); err != nil {
		return "", err
	}
	return name, nil
}

// ListSnapshots returns metadata for every snapshot (real and placeholder),
// sorted oldest-first by captured_at. Header columns only; no blob deserialization.
func ListSnapshots(mdb *metadb.MetaDB) ([]SnapshotMetadata, error) {
	rows, err := mdb.ListSchemaSnapshots()
	if err != nil {
		return nil, err
	}
	result := make([]SnapshotMetadata, 0, len(rows))
	for _, r := range rows {
		result = append(result, rowToMetadata(r))
	}
	return result, nil
}

// LoadSnapshotByName fetches the snapshot with the given name and deserializes it.
// Returns ErrSnapshotNotFound if the row does not exist (or the table has not been created yet).
// Returns ErrPlaceholderSnapshot if the row exists but snapshot_json is NULL.
func LoadSnapshotByName(mdb *metadb.MetaDB, name string) (*SchemaSnapshot, error) {
	row, err := mdb.GetSchemaSnapshotByName(name)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrSnapshotNotFound
	}
	if !row.SnapshotJSON.Valid {
		return nil, ErrPlaceholderSnapshot
	}
	return DecodeSnapshot([]byte(row.SnapshotJSON.String))
}

// DecodeSnapshot deserializes a SchemaSnapshot from raw JSON bytes.
// It enforces the versioning gate: a Version > currentSnapshotVersion is rejected,
// and a missing or zero Version is rejected as unreadable.
func DecodeSnapshot(data []byte) (*SchemaSnapshot, error) {
	var snap SchemaSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	if snap.Version == 0 {
		return nil, fmt.Errorf("snapshot has no version set (Version 0 or missing); this library requires Version %d", currentSnapshotVersion)
	}
	if snap.Version > currentSnapshotVersion {
		return nil, fmt.Errorf("snapshot Version %d is newer than this library understands (expected Version %d); upgrade yb-voyager", snap.Version, currentSnapshotVersion)
	}
	return &snap, nil
}
