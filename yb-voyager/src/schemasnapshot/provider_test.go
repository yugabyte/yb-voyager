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
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSnapshotProviderUnknownType asserts that requesting a provider for a
// database type that was never registered returns a clear, non-nil error.
func TestNewSnapshotProviderUnknownType(t *testing.T) {
	_, err := NewSnapshotProvider("does-not-exist-schemasnapshot")
	require.Error(t, err, "NewSnapshotProvider must return an error for an unregistered type")
	assert.Contains(t, err.Error(), "does-not-exist-schemasnapshot",
		"error message should include the unrecognised database type")
}

// errSnapshotProvider is a test-only SnapshotProvider whose TakeSnapshot always
// returns an error, exercising the Capture rollback path.
type errSnapshotProvider struct{}

func (e *errSnapshotProvider) DatabaseType() string    { return "test-error-provider-schemasnapshot" }
func (e *errSnapshotProvider) HasStableIdentity() bool { return false }
func (e *errSnapshotProvider) TakeSnapshot(_ context.Context, _ QueryExecutor, _ []string) (*SchemaSnapshot, error) {
	return nil, errors.New("snapshot intentionally failed")
}

// TestCaptureRollbackOnSnapshotError verifies that when TakeSnapshot returns an
// error, Capture rolls back the transaction and returns the error.
func TestCaptureRollbackOnSnapshotError(t *testing.T) {
	// Register the error provider under a unique type name.
	RegisterProvider("test-error-provider-schemasnapshot", func() SnapshotProvider {
		return &errSnapshotProvider{}
	})

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()

	source := CaptureSource{DatabaseType: "test-error-provider-schemasnapshot"}
	snap, err := Capture(context.Background(), db, source, []string{"public"})

	assert.Nil(t, snap, "Capture must return nil snapshot on TakeSnapshot error")
	require.Error(t, err, "Capture must return an error when TakeSnapshot fails")
	assert.Contains(t, err.Error(), "snapshot intentionally failed")

	// All mock expectations (Begin + Rollback) must have been satisfied.
	require.NoError(t, mock.ExpectationsWereMet(), "transaction must have been rolled back")
}

// successSnapshotProvider is a test-only SnapshotProvider that returns a minimal snapshot.
type successSnapshotProvider struct{}

func (s *successSnapshotProvider) DatabaseType() string    { return "test-success-provider" }
func (s *successSnapshotProvider) HasStableIdentity() bool { return true }
func (s *successSnapshotProvider) TakeSnapshot(_ context.Context, _ QueryExecutor, schemas []string) (*SchemaSnapshot, error) {
	return &SchemaSnapshot{
		Tables: []Table{
			{ObjectRef: ObjectRef{Schema: schemas[0], Name: "t1"}, ID: "100", Kind: TableKindOrdinary},
		},
	}, nil
}

// TestCaptureStampsHeaders verifies that Capture stamps Version, CapturedAt,
// DatabaseType, StableIdentity, and Schemas on the returned snapshot.
func TestCaptureStampsHeaders(t *testing.T) {
	RegisterProvider("test-success-provider", func() SnapshotProvider {
		return &successSnapshotProvider{}
	})

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()

	source := CaptureSource{
		DatabaseType: "test-success-provider",
		Host:         "localhost",
		Port:         5432,
		Database:     "mydb",
		User:         "voyager",
		Role:         "source",
	}
	snap, err := Capture(context.Background(), db, source, []string{"public"})
	require.NoError(t, err)
	require.NotNil(t, snap)

	assert.Equal(t, 1, snap.Version)
	assert.Equal(t, "test-success-provider", snap.DatabaseType)
	assert.True(t, snap.StableIdentity)
	assert.Equal(t, []string{"public"}, snap.Schemas)
	assert.Equal(t, source, snap.CaptureSource)
	assert.False(t, snap.CapturedAt.IsZero(), "CapturedAt must be set")

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCaptureAndSaveSnapshotSuccess verifies that CaptureAndSaveSnapshot saves a
// snapshot and returns its name when capture succeeds.
func TestCaptureAndSaveSnapshotSuccess(t *testing.T) {
	RegisterProvider("test-success-provider", func() SnapshotProvider {
		return &successSnapshotProvider{}
	})

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()

	mdb := newTestMetaDB(t)

	source := CaptureSource{
		DatabaseType: "test-success-provider",
		Host:         "localhost",
		Port:         5432,
		Database:     "mydb",
		User:         "voyager",
		Role:         "source",
	}

	name, err := CaptureAndSaveSnapshot(context.Background(), db, mdb, source, []string{"public"},
		LabelExportSchema, "", false)
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
	RegisterProvider("test-error-provider-schemasnapshot", func() SnapshotProvider {
		return &errSnapshotProvider{}
	})

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()

	mdb := newTestMetaDB(t)

	source := CaptureSource{DatabaseType: "test-error-provider-schemasnapshot"}
	name, err := CaptureAndSaveSnapshot(context.Background(), db, mdb, source, []string{"public"},
		LabelExportSchema, "", true)

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

// TestCaptureAndSaveSnapshotFailurePlaceholderFalse verifies that on capture failure
// with placeholderOnFailure=false, nothing is written and the capture error is returned.
func TestCaptureAndSaveSnapshotFailurePlaceholderFalse(t *testing.T) {
	RegisterProvider("test-error-provider-schemasnapshot", func() SnapshotProvider {
		return &errSnapshotProvider{}
	})

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()

	mdb := newTestMetaDB(t)

	source := CaptureSource{DatabaseType: "test-error-provider-schemasnapshot"}
	name, err := CaptureAndSaveSnapshot(context.Background(), db, mdb, source, []string{"public"},
		LabelExportSchema, "", false)

	assert.Empty(t, name, "name must be empty on capture failure")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot intentionally failed")
	require.NoError(t, mock.ExpectationsWereMet())

	// Nothing should have been written.
	list, err := ListSnapshots(mdb)
	require.NoError(t, err)
	assert.Empty(t, list, "nothing must be written when placeholderOnFailure=false")
}
