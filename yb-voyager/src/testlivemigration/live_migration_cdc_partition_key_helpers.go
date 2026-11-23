//go:build integration_live_migration || integration_live_migration_with_failpoint

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
package testlivemigration

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
)

// requireCustomCdcPartitionKeyPersisted checks that table was persisted with the custom CDC
// partitioning strategy and exactly cols as its key columns. This is the precondition that makes
// a custom-key test's conflict assertions meaningful, so it stops the test rather than only
// recording a failure.
func requireCustomCdcPartitionKeyPersisted(t *testing.T, lm *LiveMigrationTest, table string, cols []string) {
	t.Helper()
	require.NoError(t, lm.InitMetaDB(), "failed to initialize meta db")
	importDataStatus, err := lm.GetMetaDB().GetImportDataStatusRecord()
	require.NoError(t, err, "failed to get import data status record")
	partitionKey, ok := importDataStatus.TableToCDCPartitionKey[table]
	require.True(t, ok, "no persisted cdc partition key for %s", table)
	require.Equal(t, cmd.PARTITION_BY_CUSTOM, partitionKey.Strategy, "%s should use the custom partition strategy", table)
	require.Equal(t, cols, partitionKey.Columns, "%s custom key columns should be persisted", table)
}
