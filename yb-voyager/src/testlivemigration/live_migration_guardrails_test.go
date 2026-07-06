//go:build integration_live_migration

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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestLiveMigrationRejectsChangedNumEventChannelsOnResume verifies that
// NUM_EVENT_CHANNELS cannot be changed once a live migration has started.
//
// During the streaming phase, voyager seeds per-channel resumption metadata with
// exactly one row per channel (0..NUM_EVENT_CHANNELS-1), and routes events by
// hash(table + key) % NUM_EVENT_CHANNELS. Resuming import data with a different
// NUM_EVENT_CHANNELS would read metadata written for a different channel count and
// route events differently, silently skipping or re-applying events. Voyager must
// fail fast instead.
//
// This reproduces the support-ticket scenario: import data first ran with one
// channel count, then was restarted with a different NUM_EVENT_CHANNELS.
//
// Flow: export data -> import data (NUM_EVENT_CHANNELS=4) -> snapshot -> stream a
// few CDC events (so the channel metadata is seeded and committed) -> stop import
// -> resume import with NUM_EVENT_CHANNELS=1 and assert the command fails with the
// channel-count guard error.
func TestLiveMigrationRejectsChangedNumEventChannelsOnResume(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_num_event_channels_guard",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_num_event_channels_guard",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_live (
				id SERIAL PRIMARY KEY,
				name TEXT,
				email TEXT,
				description TEXT
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_live REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 10);`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO test_schema.test_live (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(1, 5);`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	// First import run: 4 event channels. The streaming phase seeds one metadata
	// row per channel in the target's import metadata tables.
	err = lm.StartImportDataWithEnv(true, nil, []string{"NUM_EVENT_CHANNELS=4"})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_live"`: 10,
	}, 60)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	// Stream a few CDC events so the streaming phase actually runs and commits the
	// per-channel metadata before we stop and attempt to resume.
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_live"`: {
			Inserts: 5,
			Updates: 0,
			Deletes: 0,
		},
	}, 90, 1)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	err = lm.StopImportData()
	testutils.FatalIfError(t, err, "failed to stop import data")

	// Resume import data with a different NUM_EVENT_CHANNELS. This must fail fast:
	// the stored metadata was seeded for 4 channels, so resuming with 1 is rejected
	// instead of silently resuming from an inconsistent position.
	err = lm.StartImportDataWithEnv(false, nil, []string{"NUM_EVENT_CHANNELS=1"})
	assert.Error(t, err, "resuming with a changed NUM_EVENT_CHANNELS must fail")
	assert.Contains(t, lm.GetImportCommandStderr(), "NUM_EVENT_CHANNELS cannot be changed",
		"import should fail with the channel-count guard error; stderr=%s", lm.GetImportCommandStderr())
}
