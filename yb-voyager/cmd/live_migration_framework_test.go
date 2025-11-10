//go:build integration_voyager_command

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
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLiveMigrationWithImportResumptionOnFailureAtRestoreSequences_New tests live migration
// with import resumption after failure during sequence restoration using the new framework
func TestLiveMigrationWithImportResumptionOnFailureAtRestoreSequences_New(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize with config
	config := &TestConfig{
		SourceDB:   ContainerConfig{Type: "postgresql", ForLive: true},
		TargetDB:   ContainerConfig{Type: "yugabytedb"},
		SchemaName: "test_schema",
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;`,
			`CREATE TABLE test_schema.test_live (
				id SERIAL PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
			 SELECT md5(random()::text), md5(random()::text) || '@example.com', 
					repeat(md5(random()::text), 10)
			 FROM generate_series(1, 20);`,
		},
		CleanupSQL: []string{`DROP SCHEMA IF EXISTS test_schema CASCADE;`},
		DefaultExportArgs: map[string]string{
			"--source-db-schema": "test_schema",
			"--export-type":      SNAPSHOT_AND_CHANGES,
			"--disable-pb":       "true",
		},
		DefaultImportArgs: map[string]string{
			"--disable-pb": "true",
		},
		SnapshotTimeout:  30 * time.Second,
		StreamingTimeout: 30 * time.Second,
		CutoverTimeout:   30 * time.Second,
	}

	lm := NewLiveMigrationTest(t, config)

	// 2. Setup source and target containers
	err := lm.SetupContainers(ctx)
	require.NoError(t, err)

	// 3. Setup schema on both, register cleanup
	err = lm.SetupSchema()
	require.NoError(t, err)
	defer lm.Cleanup()

	// 4. Start export data (async)
	err = lm.StartExportData(true, nil)
	require.NoError(t, err)

	// 5. Start import data (async)
	err = lm.StartImportData(true, nil)
	require.NoError(t, err)

	// 6. Wait until snapshot is completed
	err = lm.WaitForSnapshotComplete(`test_schema."test_live"`, 20)
	require.NoError(t, err)

	// 7. Validate data consistency
	err = lm.ValidateDataConsistency([]string{"test_schema.test_live"}, "id")
	require.NoError(t, err)

	// 8. Execute SQL to produce CDC events
	err = lm.ExecuteOnSource(
		`INSERT INTO test_schema.test_live (name, email, description)
		 SELECT md5(random()::text), md5(random()::text) || '@example.com',
				repeat(md5(random()::text), 10)
		 FROM generate_series(1, 15);`,
	)
	require.NoError(t, err)

	// 9. Wait until streaming is completed
	err = lm.WaitForStreamingComplete(`test_schema."test_live"`, 15)
	require.NoError(t, err)

	// 10. Validate data again
	err = lm.ValidateDataConsistency([]string{"test_schema.test_live"}, "id")
	require.NoError(t, err)

	// 11. Stop import command
	err = lm.StopImportData()
	require.NoError(t, err)

	err = lm.WaitForImportExit()
	require.NoError(t, err)

	// 12. Initiate cutover
	err = lm.InitiateCutover(false, nil)
	require.NoError(t, err)

	// 13. Inject failure (drop sequence on target)
	err = lm.ExecuteOnTarget(`DROP SEQUENCE test_schema.test_live_id_seq CASCADE;`)
	require.NoError(t, err)

	time.Sleep(10 * time.Second)

	// 14. Restart import data (sync mode to catch error immediately)
	err = lm.StartImportData(false, nil)
	require.Error(t, err)

	// 15. Verify error message
	stderr := lm.GetImportCommandStderr()
	assert.Contains(t, stderr, "failed to restore sequences:")

	// 16. Remove failure (recreate sequence)
	err = lm.ExecuteOnTarget(
		`CREATE SEQUENCE test_schema.test_live_id_seq;`,
		`ALTER SEQUENCE test_schema.test_live_id_seq OWNED BY test_schema.test_live.id;`,
		`ALTER TABLE test_schema.test_live ALTER COLUMN id SET DEFAULT nextval('test_schema.test_live_id_seq');`,
	)
	require.NoError(t, err)

	// 17. Restart import data again (async mode this time)
	err = lm.StartImportData(true, nil)
	require.NoError(t, err)

	// 18. Wait for cutover to complete
	err = lm.WaitForCutoverComplete()
	require.NoError(t, err)

	// 19. Final custom validation using connection helper
	err = lm.WithTargetConn(func(conn *sql.DB) error {
		// Insert some rows to test sequence
		_, err := conn.Exec(`INSERT INTO test_schema.test_live (name, email, description)
			SELECT md5(random()::text), md5(random()::text) || '@example.com',
				   repeat(md5(random()::text), 10)
			FROM generate_series(1, 10);`)
		if err != nil {
			return err
		}

		// Validate sequence values (check if IDs 36-45 exist)
		rows, err := conn.Query(`SELECT id from test_schema.test_live 
			WHERE id BETWEEN 36 AND 45 ORDER BY id;`)
		if err != nil {
			return err
		}
		defer rows.Close()

		var ids []int
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}

		// Assert we got all expected IDs
		expected := []int{36, 37, 38, 39, 40, 41, 42, 43, 44, 45}
		assert.Equal(t, expected, ids)
		return nil
	})
	require.NoError(t, err)
}
