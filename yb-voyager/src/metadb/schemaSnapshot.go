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
package metadb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// schemaSnapshotsTableName is the SQLite table that persists schema snapshots.
const schemaSnapshotsTableName = "schema_snapshots"

// SchemaSnapshotRow is the primitive row stored in schema_snapshots.
// snapshot_json is NULLable: NULL means the row is a placeholder (no schema content captured).
// This struct intentionally contains no schemadiff types — the metadb package must not import schemadiff.
type SchemaSnapshotRow struct {
	Name            string         // PRIMARY KEY: "{label}_{timestamp-at-second-precision}"
	Label           string         // one of the four capture labels
	Reason          string         // capture reason; "" when the series carries none
	Side            string         // "source"
	CapturedAt      time.Time      // full-precision capture time (UTC)
	DatabaseVersion string         // server_version truncated at first space, e.g. "16.14"
	Schemas         string         // comma-joined schema list
	SnapshotJSON    sql.NullString // the full SchemaSnapshot JSON; NULL for a placeholder
}

// createSchemaSnapshotsTable creates the schema_snapshots table if it does not exist.
// It is called lazily on first write, not at MetaDB init.
func (m *MetaDB) createSchemaSnapshotsTable() error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		name             TEXT    PRIMARY KEY,
		label            TEXT    NOT NULL,
		reason           TEXT    NOT NULL DEFAULT '',
		side             TEXT    NOT NULL,
		captured_at      TEXT    NOT NULL,
		database_version TEXT    NOT NULL DEFAULT '',
		schemas          TEXT    NOT NULL DEFAULT '',
		snapshot_json    TEXT
	);`, schemaSnapshotsTableName)
	_, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("create schema_snapshots table: %w", err)
	}
	log.Infof("Executed query on meta db - %s", query)
	return nil
}

// InsertSchemaSnapshot inserts a full snapshot row (snapshot_json is set).
// Returns an error if the name (primary key) already exists.
func (m *MetaDB) InsertSchemaSnapshot(row SchemaSnapshotRow) error {
	if err := m.createSchemaSnapshotsTable(); err != nil {
		return err
	}
	capturedAtStr := row.CapturedAt.UTC().Format(time.RFC3339Nano)
	query := fmt.Sprintf(`INSERT INTO %s
		(name, label, reason, side, captured_at, database_version, schemas, snapshot_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, schemaSnapshotsTableName)
	_, err := m.db.Exec(query,
		row.Name,
		row.Label,
		row.Reason,
		row.Side,
		capturedAtStr,
		row.DatabaseVersion,
		row.Schemas,
		row.SnapshotJSON,
	)
	if err != nil {
		// Surface primary-key collision as-is; the schemadiff layer interprets it.
		return fmt.Errorf("insert schema snapshot %q: %w", row.Name, err)
	}
	log.Infof("Executed query on meta db - %s", query)
	return nil
}

// InsertSchemaSnapshotPlaceholder inserts a placeholder row (snapshot_json NULL).
// Returns an error if the name (primary key) already exists.
func (m *MetaDB) InsertSchemaSnapshotPlaceholder(row SchemaSnapshotRow) error {
	row.SnapshotJSON = sql.NullString{Valid: false}
	return m.InsertSchemaSnapshot(row)
}

// ListSchemaSnapshots returns all rows ordered oldest-first by captured_at.
// snapshot_json is populated for each row (callers that need to check placeholder use SnapshotJSON.Valid).
// This method tolerates the table not existing: in that case it returns an empty slice.
func (m *MetaDB) ListSchemaSnapshots() ([]SchemaSnapshotRow, error) {
	query := fmt.Sprintf(`SELECT name, label, reason, side, captured_at, database_version, schemas, snapshot_json
		FROM %s ORDER BY captured_at ASC;`, schemaSnapshotsTableName)
	rows, err := m.db.Query(query)
	if err != nil {
		// If the table doesn't exist, SQLite returns an error containing "no such table".
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("list schema snapshots: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Errorf("failed to close rows while fetching schema snapshots from query %s : %v", query, err)
		}
	}()

	var result []SchemaSnapshotRow
	for rows.Next() {
		var r SchemaSnapshotRow
		var capturedAtStr string
		if err := rows.Scan(
			&r.Name,
			&r.Label,
			&r.Reason,
			&r.Side,
			&capturedAtStr,
			&r.DatabaseVersion,
			&r.Schemas,
			&r.SnapshotJSON,
		); err != nil {
			return nil, fmt.Errorf("scan schema snapshot row: %w", err)
		}
		t, err := time.Parse(time.RFC3339Nano, capturedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse captured_at %q: %w", capturedAtStr, err)
		}
		r.CapturedAt = t.UTC()
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema snapshot rows: %w", err)
	}
	return result, nil
}

// GetSchemaSnapshotByName returns the row with the given name.
// If the table does not exist or the name is not found, it returns (nil, nil).
func (m *MetaDB) GetSchemaSnapshotByName(name string) (*SchemaSnapshotRow, error) {
	query := fmt.Sprintf(`SELECT name, label, reason, side, captured_at, database_version, schemas, snapshot_json
		FROM %s WHERE name = ?;`, schemaSnapshotsTableName)
	row := m.db.QueryRow(query, name)

	var r SchemaSnapshotRow
	var capturedAtStr string
	err := row.Scan(
		&r.Name,
		&r.Label,
		&r.Reason,
		&r.Side,
		&capturedAtStr,
		&r.DatabaseVersion,
		&r.Schemas,
		&r.SnapshotJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		// Table may not exist yet.
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("get schema snapshot %q: %w", name, err)
	}
	t, err := time.Parse(time.RFC3339Nano, capturedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse captured_at %q: %w", capturedAtStr, err)
	}
	r.CapturedAt = t.UTC()
	return &r, nil
}
