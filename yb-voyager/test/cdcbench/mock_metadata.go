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

package cdcbench

import (
	"database/sql"
	"fmt"

	// registered by the metadb package too; imported here for clarity since
	// this file opens its own sqlite handle
	_ "github.com/mattn/go-sqlite3"
)

/*
The import streaming bootstrap (streamChanges -> ImportDataState) keeps its
live-migration bookkeeping — per-channel last-applied VSNs and per-table event
counts — in tables on the TARGET database, accessed through TargetDB's
Query/QueryRow/Exec/WithTx, which return concrete *sql.Rows / *sql.Row values.
Those can only be produced by a real database/sql driver, so the mock backs
them with an in-memory SQLite database holding just those two tables (schema
mirrors TargetYugabyteDB.CreateVoyagerSchema). ExecuteBatch stays a no-op, so
the bookkeeping keeps its fresh-migration values (last_applied_vsn = -1)
throughout a run.
*/
func newMockMetadataStore() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	// a fresh pooled connection would see a fresh empty :memory: database;
	// pin everything to a single connection
	db.SetMaxOpenConns(1)

	stmts := []string{
		`ATTACH DATABASE ':memory:' AS ybvoyager_metadata`,
		`CREATE TABLE ybvoyager_metadata.ybvoyager_import_data_event_channels_metainfo (
			migration_uuid TEXT,
			channel_no INTEGER,
			last_applied_vsn INTEGER,
			num_inserts INTEGER,
			num_deletes INTEGER,
			num_updates INTEGER,
			PRIMARY KEY (migration_uuid, channel_no))`,
		`CREATE TABLE ybvoyager_metadata.ybvoyager_imported_event_count_by_table (
			migration_uuid TEXT,
			table_name TEXT,
			channel_no INTEGER,
			total_events INTEGER,
			num_inserts INTEGER,
			num_deletes INTEGER,
			num_updates INTEGER,
			PRIMARY KEY (migration_uuid, table_name, channel_no))`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("init mock metadata store: %w", err)
		}
	}
	return db, nil
}

// PrepareForStreaming is a no-op: the real implementation only readies the
// target connection for streaming.
func (m *MockTargetDB) PrepareForStreaming() {}

func (m *MockTargetDB) Query(query string) (*sql.Rows, error) {
	return m.metadata.Query(query)
}

func (m *MockTargetDB) QueryRow(query string) *sql.Row {
	return m.metadata.QueryRow(query)
}

func (m *MockTargetDB) Exec(query string) (int64, error) {
	res, err := m.metadata.Exec(query)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}

func (m *MockTargetDB) WithTx(fn func(tx *sql.Tx) error) error {
	tx, err := m.metadata.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
