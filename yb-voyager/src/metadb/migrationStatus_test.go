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
package metadb

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitMigrationStatusRecordMigrationUUID(t *testing.T) {
	t.Run("uses valid UUID from environment", func(t *testing.T) {
		externalUUID := uuid.New().String()
		t.Setenv(migrationUUIDEnvVarName, externalUUID)
		mdb := newTestMetaDB(t)

		require.NoError(t, mdb.InitMigrationStatusRecord("config.yaml"))
		record, err := mdb.GetMigrationStatusRecord()
		require.NoError(t, err)
		require.NotNil(t, record)
		assert.Equal(t, externalUUID, record.MigrationUUID)
	})

	t.Run("generates UUID when environment is absent", func(t *testing.T) {
		t.Setenv(migrationUUIDEnvVarName, "")
		mdb := newTestMetaDB(t)

		require.NoError(t, mdb.InitMigrationStatusRecord("config.yaml"))
		record, err := mdb.GetMigrationStatusRecord()
		require.NoError(t, err)
		require.NotNil(t, record)

		generatedUUID, err := uuid.Parse(record.MigrationUUID)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, generatedUUID)
	})

	t.Run("errors when environment is set but invalid", func(t *testing.T) {
		t.Setenv(migrationUUIDEnvVarName, "not-a-uuid")
		mdb := newTestMetaDB(t)

		err := mdb.InitMigrationStatusRecord("config.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), migrationUUIDEnvVarName)

		// nothing should have been persisted for a rejected value
		record, err := mdb.GetMigrationStatusRecord()
		require.NoError(t, err)
		assert.Nil(t, record)
	})

	t.Run("preserves persisted UUID", func(t *testing.T) {
		persistedUUID := uuid.New().String()
		t.Setenv(migrationUUIDEnvVarName, uuid.New().String())
		mdb := newTestMetaDB(t)
		require.NoError(t, mdb.UpdateMigrationStatusRecord(func(record *MigrationStatusRecord) {
			record.MigrationUUID = persistedUUID
			record.ConfigFile = "persisted-config.yaml"
		}))

		require.NoError(t, mdb.InitMigrationStatusRecord("new-config.yaml"))
		record, err := mdb.GetMigrationStatusRecord()
		require.NoError(t, err)
		require.NotNil(t, record)
		assert.Equal(t, persistedUUID, record.MigrationUUID)
		assert.Equal(t, "persisted-config.yaml", record.ConfigFile)
	})
}
