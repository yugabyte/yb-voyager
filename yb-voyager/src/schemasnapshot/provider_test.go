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
	"context"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

// TestNewProviderUnknownTypeFromCapture asserts that Capture returns a clear error
// for a database type that is not supported by the switch in newProvider.
func TestNewProviderUnknownTypeFromCapture(t *testing.T) {
	_, err := newProvider("does-not-exist-schemasnapshot", nil)
	require.Error(t, err, "newProvider must return an error for an unsupported type")
	assert.Contains(t, err.Error(), "does-not-exist-schemasnapshot",
		"error message should include the unrecognised database type")
}

// setupSuccessfulPGMock registers the minimal pg_catalog expectations for one
// round of TakeSnapshot: server_version, pg_class (empty), pg_inherits (empty),
// pg_attribute (empty). Use this when testing Capture orchestration (header
// stamping, tx commit/rollback) rather than the loader logic itself.
func setupSuccessfulPGMock(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow("16.4"))
	mock.ExpectQuery(`pg_class`).
		WillReturnRows(sqlmock.NewRows([]string{"oid", "schema", "name", "relkind"}))
	mock.ExpectQuery(`pg_inherits`).
		WillReturnRows(sqlmock.NewRows([]string{"child_oid", "child_schema", "child_name", "parent_oid", "parent_schema", "parent_name", "is_partition"}))
	mock.ExpectQuery(`pg_attribute`).
		WillReturnRows(sqlmock.NewRows([]string{"table_oid", "attnum", "schema", "table_name", "col_name", "data_type", "not_null", "col_default"}))
	mock.ExpectCommit()
}

// TestCaptureRollbackOnSnapshotError verifies that when TakeSnapshot returns an
// error (simulated by making the server_version query fail), Capture rolls back
// the transaction and propagates the error.
func TestCaptureRollbackOnSnapshotError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	// Make detectDatabaseVersion fail — this causes TakeSnapshot to return an error.
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnError(fmt.Errorf("snapshot intentionally failed"))
	mock.ExpectRollback()

	source := DBMetadata{DatabaseType: constants.POSTGRESQL}
	snap, err := Capture(context.Background(), db, CaptureParams{Source: source, Schemas: []string{"public"}, Label: LabelExportSchema})

	assert.Nil(t, snap, "Capture must return nil snapshot on TakeSnapshot error")
	require.Error(t, err, "Capture must return an error when TakeSnapshot fails")
	assert.Contains(t, err.Error(), "snapshot intentionally failed")

	// All mock expectations (Begin + query error + Rollback) must have been satisfied.
	require.NoError(t, mock.ExpectationsWereMet(), "transaction must have been rolled back")
}

// TestCaptureStampsHeaders verifies that Capture stamps Version, CapturedAt,
// DatabaseType, StableIdentity, and Schemas on the returned snapshot.
func TestCaptureStampsHeaders(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	setupSuccessfulPGMock(mock)

	source := DBMetadata{
		DatabaseType: constants.POSTGRESQL,
		Host:         "localhost",
		Port:         5432,
		Database:     "mydb",
		User:         "voyager",
		Side:         "source",
	}
	snap, err := Capture(context.Background(), db, CaptureParams{Source: source, Schemas: []string{"public"}, Label: LabelExportSchema})
	require.NoError(t, err)
	require.NotNil(t, snap)

	assert.Equal(t, 1, snap.Version)
	assert.Equal(t, constants.POSTGRESQL, snap.DatabaseType)
	assert.True(t, snap.StableIdentity)
	assert.Equal(t, []string{"public"}, snap.Schemas)
	assert.Equal(t, source, snap.DBMetadata)
	assert.False(t, snap.CapturedAt.IsZero(), "CapturedAt must be set")

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCaptureAndSaveSnapshotSuccess verifies that CaptureAndSaveSnapshot saves a
// snapshot and returns its name when capture succeeds.
func TestCaptureAndSaveSnapshotSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	setupSuccessfulPGMock(mock)

	mdb := newTestMetaDB(t)

	source := DBMetadata{
		DatabaseType: constants.POSTGRESQL,
		Host:         "localhost",
		Port:         5432,
		Database:     "mydb",
		User:         "voyager",
		Side:         "source",
	}

	name, err := CaptureAndSaveSnapshot(context.Background(), db, mdb, CaptureRequest{
		CaptureParams:        CaptureParams{Source: source, Schemas: []string{"public"}, Label: LabelExportSchema},
		PlaceholderOnFailure: false,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	require.NoError(t, mock.ExpectationsWereMet())

	// Verify it was actually saved.
	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, name, list[0].Name)
}

// TestCaptureAndSaveSnapshotFailurePlaceholderTrue verifies that on capture failure
// with placeholderOnFailure=true, a placeholder is saved and the capture error is returned.
func TestCaptureAndSaveSnapshotFailurePlaceholderTrue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnError(fmt.Errorf("snapshot intentionally failed"))
	mock.ExpectRollback()

	mdb := newTestMetaDB(t)

	source := DBMetadata{DatabaseType: constants.POSTGRESQL}
	name, err := CaptureAndSaveSnapshot(context.Background(), db, mdb, CaptureRequest{
		CaptureParams:        CaptureParams{Source: source, Schemas: []string{"public"}, Label: LabelExportSchema},
		PlaceholderOnFailure: true,
	})

	assert.Empty(t, name, "name must be empty on capture failure")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot intentionally failed")
	require.NoError(t, mock.ExpectationsWereMet())

	// A placeholder should have been written.
	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	require.Len(t, list, 1, "a placeholder must be written when placeholderOnFailure=true")
	assert.True(t, list[0].IsPlaceholder, "the written row must be a placeholder")
}

// TestCaptureEmptySchemasReturnsError verifies that Capture returns an error when
// the schemas slice is nil or empty, before touching the database.
func TestCaptureEmptySchemasReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// No mock expectations: the guard must return before any DB access.
	source := DBMetadata{DatabaseType: constants.POSTGRESQL}

	// nil schemas
	snap, err := Capture(context.Background(), db, CaptureParams{Source: source, Schemas: nil, Label: LabelExportSchema})
	assert.Nil(t, snap)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no schemas in scope")

	// empty slice
	snap, err = Capture(context.Background(), db, CaptureParams{Source: source, Schemas: []string{}, Label: LabelExportSchema})
	assert.Nil(t, snap)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no schemas in scope")

	// No Begin/Rollback/Commit should have been called.
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCaptureAndSaveSnapshotFailurePlaceholderFalse verifies that on capture failure
// with placeholderOnFailure=false, nothing is written and the capture error is returned.
func TestCaptureAndSaveSnapshotFailurePlaceholderFalse(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SHOW server_version`).
		WillReturnError(fmt.Errorf("snapshot intentionally failed"))
	mock.ExpectRollback()

	mdb := newTestMetaDB(t)

	source := DBMetadata{DatabaseType: constants.POSTGRESQL}
	name, err := CaptureAndSaveSnapshot(context.Background(), db, mdb, CaptureRequest{
		CaptureParams:        CaptureParams{Source: source, Schemas: []string{"public"}, Label: LabelExportSchema},
		PlaceholderOnFailure: false,
	})

	assert.Empty(t, name, "name must be empty on capture failure")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot intentionally failed")
	require.NoError(t, mock.ExpectationsWereMet())

	// Nothing should have been written.
	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	assert.Empty(t, list, "nothing must be written when placeholderOnFailure=false")
}
