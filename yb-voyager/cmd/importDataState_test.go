//go:build unit

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
package cmd

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

// sqlmockTargetDB is a minimal tgtdb.TargetDB backed by a sqlmock *sql.DB. Only
// the methods that initChannelMetaInfo exercises (QueryRow, WithTx) are
// implemented; every other TargetDB method comes from the embedded (nil)
// interface and will panic if called, which is fine for these focused tests.
type sqlmockTargetDB struct {
	tgtdb.TargetDB
	db *sql.DB
}

func (m *sqlmockTargetDB) QueryRow(query string) *sql.Row {
	return m.db.QueryRow(query)
}

func (m *sqlmockTargetDB) WithTx(fn func(tx *sql.Tx) error) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// withMockTdb swaps the package-level tdb for a sqlmock-backed one and returns
// the mock controller plus a cleanup func to restore the original tdb.
func withMockTdb(t *testing.T) (sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	origTdb := tdb
	tdb = &sqlmockTargetDB{db: db}

	return mock, func() {
		tdb = origTdb
		db.Close()
	}
}

// TestInitChannelMetaInfoRejectsChangedChannelCount verifies the guard that
// prevents NUM_EVENT_CHANNELS from being changed once a migration has started.
// The per-channel resumption metadata is seeded with exactly one row per channel,
// so a stored row count that differs from the configured channel count means the
// value was changed mid-migration; resuming would silently skip or re-apply
// events, so initChannelMetaInfo must fail instead of proceeding.
func TestInitChannelMetaInfoRejectsChangedChannelCount(t *testing.T) {
	tests := []struct {
		name               string
		storedChannels     int64
		configuredChannels int
	}{
		{name: "decreasing channels", storedChannels: 100, configuredChannels: 1},
		{name: "increasing channels", storedChannels: 4, configuredChannels: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := withMockTdb(t)
			defer cleanup()

			// Previous run seeded storedChannels rows for this migration.
			mock.ExpectQuery("SELECT count").
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.storedChannels))
			// No ExpectBegin/ExpectExec: the guard must reject before touching the
			// metadata, so any attempt to open a transaction would fail the test.

			state := NewImportDataState("")
			err := state.initChannelMetaInfo(uuid.New(), tt.configuredChannels)

			require.Error(t, err)
			require.Contains(t, err.Error(), "NUM_EVENT_CHANNELS cannot be changed")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestInitChannelMetaInfoAllowsSameChannelCount verifies that a normal resume
// (stored channel count matches the configured count) skips re-init without error
// and without re-inserting metadata.
func TestInitChannelMetaInfoAllowsSameChannelCount(t *testing.T) {
	mock, cleanup := withMockTdb(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(4)))

	state := NewImportDataState("")
	err := state.initChannelMetaInfo(uuid.New(), 4)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInitChannelMetaInfoFreshInitInsertsOneRowPerChannel verifies that a fresh
// run (no stored metadata) still seeds exactly one row per channel.
func TestInitChannelMetaInfoFreshInitInsertsOneRowPerChannel(t *testing.T) {
	mock, cleanup := withMockTdb(t)
	defer cleanup()

	numChans := 3
	mock.ExpectQuery("SELECT count").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectBegin()
	for i := 0; i < numChans; i++ {
		mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	state := NewImportDataState("")
	err := state.initChannelMetaInfo(uuid.New(), numChans)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
