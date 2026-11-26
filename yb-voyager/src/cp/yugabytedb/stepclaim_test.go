//go:build integration

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
package yugabytedb

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// stepClaimSchemaDDL is what the platform adds for step claiming. Voyager
// never creates it, so the test does.
var stepClaimSchemaDDL = []string{
	`CREATE TABLE voyager_core.operation (
		id              uuid PRIMARY KEY,
		migration_id    uuid,
		sequence_id     bigint,
		issued_by       text,
		name            text,
		config          jsonb,
		idempotency_key text,
		created_at      timestamptz DEFAULT now()
	)`,
	`CREATE TABLE voyager_core.operation_step (
		id           bigserial PRIMARY KEY,
		operation_id uuid,
		step         text,
		ordinal      int,
		UNIQUE (operation_id, step)
	)`,
	`CREATE TABLE voyager_core.step_execution (
		id                bigserial PRIMARY KEY,
		migration_id      uuid NOT NULL,
		operation_step_id bigint NOT NULL,
		step              text NOT NULL,
		executor_ref      jsonb,
		started_at        timestamptz DEFAULT now(),
		last_heartbeat_at timestamptz DEFAULT now(),
		status            text DEFAULT 'running' CHECK (status IN ('running','succeeded','failed','stopped')),
		reason            text,
		error_message     text,
		finished_at       timestamptz
	)`,
	`CREATE UNIQUE INDEX one_running_per_step ON voyager_core.step_execution (migration_id, step) WHERE status = 'running'`,
	`ALTER TABLE voyager_core.migration_state ADD COLUMN step_execution_id bigint`,
	`ALTER TABLE voyager_core.metadata ADD COLUMN step_execution_id bigint`,
}

func TestStepClaiming(t *testing.T) {
	ctx := context.Background()

	yugabyteDBContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	require.NoError(t, yugabyteDBContainer.Start(ctx), "Failed to start yugabytedb container")

	// The schema name voyager_core is fixed in the control plane, so this test
	// works in a database of its own and leaves the container's default
	// database to TestYugabyteDBControlPlaneWrites.
	adminDSN := yugabyteDBContainer.GetConnectionString()
	adminDB, err := sql.Open("pgx", adminDSN)
	require.NoError(t, err)
	defer adminDB.Close()
	require.NoError(t, testutils.WaitForDBToBeReady(adminDB))
	_, err = adminDB.Exec(`DROP DATABASE IF EXISTS step_claiming`)
	require.NoError(t, err)
	_, err = adminDB.Exec(`CREATE DATABASE step_claiming`)
	require.NoError(t, err)
	dsnURL, err := url.Parse(adminDSN)
	require.NoError(t, err)
	dsnURL.Path = "/step_claiming"
	dsn := dsnURL.String()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, testutils.WaitForDBToBeReady(db))

	t.Setenv(DB_CONN_STRING_ENV_VAR, dsn)
	t.Setenv(OPERATION_ID_ENV_VAR, "")
	t.Setenv(EXECUTOR_REF_ENV_VAR, "")

	for _, ddl := range controlPlaneSchemaDDL {
		_, err = db.Exec(ddl)
		require.NoError(t, err, ddl)
	}

	operationID := uuid.New()
	// newControlPlane is Init'ed with the operation env vars as set by the caller.
	newControlPlane := func(t *testing.T) *YugabyteDB {
		controlPlane := New(t.TempDir())
		controlPlane.heartbeatInterval = 100 * time.Millisecond
		require.NoError(t, controlPlane.Init())
		t.Cleanup(controlPlane.Finalize)
		return controlPlane
	}
	// plan registers an operation of migrationID with the given steps.
	plan := func(t *testing.T, migrationID uuid.UUID, steps ...string) uuid.UUID {
		id := uuid.New()
		_, err := db.Exec(`INSERT INTO voyager_core.operation (id, migration_id) VALUES ($1, $2)`, id, migrationID)
		require.NoError(t, err)
		for i, step := range steps {
			_, err := db.Exec(`INSERT INTO voyager_core.operation_step (operation_id, step, ordinal) VALUES ($1, $2, $3)`, id, step, i)
			require.NoError(t, err)
		}
		return id
	}
	withOperation := func(t *testing.T, id uuid.UUID) {
		t.Setenv(OPERATION_ID_ENV_VAR, id.String())
	}
	neverLost := func(t *testing.T) func(error) {
		return func(err error) { t.Errorf("unexpected loss of the step: %v", err) }
	}
	type attemptRow struct {
		id                    int64
		status                string
		reason, errorMessage  sql.NullString
		executorRef           sql.NullString
		finished              bool
		heartbeatAfterStarted bool
	}
	attempts := func(t *testing.T, migrationID uuid.UUID, step string) []attemptRow {
		rows, err := db.Query(`SELECT id, status, reason, error_message, executor_ref::text, finished_at IS NOT NULL,
				last_heartbeat_at > started_at
			FROM voyager_core.step_execution WHERE migration_id = $1 AND step = $2 ORDER BY id`, migrationID, step)
		require.NoError(t, err)
		defer rows.Close()
		var result []attemptRow
		for rows.Next() {
			var r attemptRow
			require.NoError(t, rows.Scan(&r.id, &r.status, &r.reason, &r.errorMessage, &r.executorRef, &r.finished, &r.heartbeatAfterStarted))
			result = append(result, r)
		}
		require.NoError(t, rows.Err())
		return result
	}
	stampedEntries := func(t *testing.T, migrationID uuid.UUID) map[string]sql.NullInt64 {
		rows, err := db.Query(`SELECT name, step_execution_id FROM voyager_core.migration_state WHERE migration_id = $1`, migrationID)
		require.NoError(t, err)
		defer rows.Close()
		result := map[string]sql.NullInt64{}
		for rows.Next() {
			var name string
			var id sql.NullInt64
			require.NoError(t, rows.Scan(&name, &id))
			result[name] = id
		}
		require.NoError(t, rows.Err())
		return result
	}
	waitFor := func(t *testing.T, what string, cond func() bool) {
		deadline := time.Now().Add(15 * time.Second)
		for !cond() {
			require.True(t, time.Now().Before(deadline), "timed out waiting for %s", what)
			time.Sleep(50 * time.Millisecond)
		}
	}
	sendEvent := func(t *testing.T, controlPlane *YugabyteDB, migrationID uuid.UUID, eventType, status, payload string) error {
		return controlPlane.sendMigrationEvent(MigrationEvent{
			MigrationUUID: migrationID, EventType: eventType, Status: status, Payload: payload})
	}

	t.Run("environment is validated", func(t *testing.T) {
		t.Setenv(OPERATION_ID_ENV_VAR, "not-a-uuid")
		assert.ErrorContains(t, New(t.TempDir()).Init(), OPERATION_ID_ENV_VAR)

		t.Setenv(OPERATION_ID_ENV_VAR, uuid.NewString())
		for _, executorRef := range []string{"not json", `["a"]`, "null", `"x"`} {
			t.Setenv(EXECUTOR_REF_ENV_VAR, executorRef)
			assert.ErrorContains(t, New(t.TempDir()).Init(), EXECUTOR_REF_ENV_VAR, executorRef)
		}
	})

	t.Run("init fails fast when the platform tables are missing", func(t *testing.T) {
		withOperation(t, operationID)
		assert.ErrorContains(t, New(t.TempDir()).Init(), "table voyager_core.operation does not exist")
	})

	for _, ddl := range stepClaimSchemaDDL {
		_, err = db.Exec(ddl)
		require.NoError(t, err, ddl)
	}

	t.Run("init fails fast when a column is missing", func(t *testing.T) {
		withOperation(t, operationID)
		_, err := db.Exec(`ALTER TABLE voyager_core.metadata DROP COLUMN step_execution_id`)
		require.NoError(t, err)
		assert.ErrorContains(t, New(t.TempDir()).Init(),
			"column step_execution_id of table voyager_core.metadata does not exist")
		_, err = db.Exec(`ALTER TABLE voyager_core.metadata ADD COLUMN step_execution_id bigint`)
		require.NoError(t, err)
	})

	t.Run("operation id unset: no claim, no rows, no stamp", func(t *testing.T) {
		migrationID := uuid.New()
		controlPlane := newControlPlane(t)
		assert.False(t, controlPlane.StepClaimingEnabled())
		require.NoError(t, sendEvent(t, controlPlane, migrationID, "EXPORT SCHEMA", STATUS_IN_PROGRESS, ""))
		require.NoError(t, sendEvent(t, controlPlane, migrationID, "EXPORT SCHEMA", STATUS_COMPLETED, ""))

		assert.Equal(t, map[string]sql.NullInt64{
			"schema.export_started": {}, "schema.exported": {},
		}, stampedEntries(t, migrationID))
		var count int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM voyager_core.step_execution WHERE migration_id = $1`, migrationID).Scan(&count))
		assert.Equal(t, 0, count)
	})

	t.Run("claim, heartbeat, done entry and close together", func(t *testing.T) {
		migrationID := uuid.New()
		withOperation(t, plan(t, migrationID, STEP_ASSESS_MIGRATION, STEP_EXPORT_SCHEMA))
		t.Setenv(EXECUTOR_REF_ENV_VAR, `{"executor_id":"pod-1"}`)
		controlPlane := newControlPlane(t)
		require.True(t, controlPlane.StepClaimingEnabled())

		require.NoError(t, controlPlane.ClaimStep(migrationID, STEP_EXPORT_SCHEMA, neverLost(t)))
		running := attempts(t, migrationID, STEP_EXPORT_SCHEMA)
		require.Len(t, running, 1)
		assert.Equal(t, "running", running[0].status)
		assert.JSONEq(t, `{"executor_id":"pod-1"}`, running[0].executorRef.String)
		attemptID := running[0].id

		waitFor(t, "a heartbeat", func() bool { return attempts(t, migrationID, STEP_EXPORT_SCHEMA)[0].heartbeatAfterStarted })

		require.NoError(t, sendEvent(t, controlPlane, migrationID, "EXPORT SCHEMA", STATUS_IN_PROGRESS, ""))
		// A phase that is not the attempt's own step is stamped but does not close it.
		require.NoError(t, sendEvent(t, controlPlane, migrationID, "ASSESS MIGRATION", STATUS_COMPLETED, `{"PayloadVersion":"1.8"}`))
		assert.Equal(t, "running", attempts(t, migrationID, STEP_EXPORT_SCHEMA)[0].status)
		var metadataStamp sql.NullInt64
		require.NoError(t, db.QueryRow(`SELECT step_execution_id FROM voyager_core.metadata WHERE migration_id = $1`, migrationID).Scan(&metadataStamp))
		assert.Equal(t, sql.NullInt64{Int64: attemptID, Valid: true}, metadataStamp)

		require.NoError(t, sendEvent(t, controlPlane, migrationID, "EXPORT SCHEMA", STATUS_COMPLETED, ""))
		closed := attempts(t, migrationID, STEP_EXPORT_SCHEMA)
		require.Len(t, closed, 1)
		assert.Equal(t, "succeeded", closed[0].status)
		assert.True(t, closed[0].finished)
		stamp := sql.NullInt64{Int64: attemptID, Valid: true}
		assert.Equal(t, map[string]sql.NullInt64{
			"schema.export_started": stamp, "schema.exported": stamp, "assessment.completed": stamp,
		}, stampedEntries(t, migrationID))

		// Later closes are no-ops, and later rows are no longer stamped.
		require.NoError(t, controlPlane.CloseStep(errors.New("late")))
		assert.Equal(t, "succeeded", attempts(t, migrationID, STEP_EXPORT_SCHEMA)[0].status)
		require.NoError(t, sendEvent(t, controlPlane, migrationID, "IMPORT SCHEMA", STATUS_IN_PROGRESS, ""))
		assert.False(t, stampedEntries(t, migrationID)["schema.import_started"].Valid)
	})

	t.Run("a lost attempt gets no done entry", func(t *testing.T) {
		migrationID := uuid.New()
		withOperation(t, plan(t, migrationID, STEP_EXPORT_SCHEMA))
		controlPlane := newControlPlane(t)
		lost := make(chan error, 1)
		require.NoError(t, controlPlane.ClaimStep(migrationID, STEP_EXPORT_SCHEMA, func(err error) { lost <- err }))

		_, err := db.Exec(`UPDATE voyager_core.step_execution SET status = 'stopped', finished_at = now() WHERE migration_id = $1`, migrationID)
		require.NoError(t, err)
		select {
		case err := <-lost:
			assert.ErrorContains(t, err, "closed by another process")
		case <-time.After(15 * time.Second):
			t.Fatal("the heartbeat did not report the lost attempt")
		}

		assert.ErrorIs(t, sendEvent(t, controlPlane, migrationID, "EXPORT SCHEMA", STATUS_COMPLETED, ""), errAttemptNotRunning)
		assert.NotContains(t, stampedEntries(t, migrationID), "schema.exported")
		assert.Equal(t, "stopped", attempts(t, migrationID, STEP_EXPORT_SCHEMA)[0].status)
		// The exit path must not overwrite what closed it.
		assert.Error(t, controlPlane.CloseStep(errors.New("exit")))
		assert.Equal(t, "stopped", attempts(t, migrationID, STEP_EXPORT_SCHEMA)[0].status)
	})

	t.Run("failure closes the attempt with the error", func(t *testing.T) {
		migrationID := uuid.New()
		withOperation(t, plan(t, migrationID, STEP_IMPORT_DATA_TO_TARGET))
		controlPlane := newControlPlane(t)
		require.NoError(t, controlPlane.ClaimStep(migrationID, STEP_IMPORT_DATA_TO_TARGET, neverLost(t)))

		require.NoError(t, controlPlane.CloseStep(errors.New("target unreachable")))
		got := attempts(t, migrationID, STEP_IMPORT_DATA_TO_TARGET)
		require.Len(t, got, 1)
		assert.Equal(t, "failed", got[0].status)
		assert.Equal(t, "target unreachable", got[0].errorMessage.String)
		assert.True(t, got[0].finished)
		assert.False(t, got[0].executorRef.Valid)
	})

	t.Run("a step with no done entry closes succeeded", func(t *testing.T) {
		migrationID := uuid.New()
		withOperation(t, plan(t, migrationID, STEP_FINALIZE_SCHEMA))
		controlPlane := newControlPlane(t)
		require.NoError(t, controlPlane.ClaimStep(migrationID, STEP_FINALIZE_SCHEMA, neverLost(t)))
		require.NoError(t, controlPlane.CloseStep(nil))
		assert.Equal(t, "succeeded", attempts(t, migrationID, STEP_FINALIZE_SCHEMA)[0].status)
	})

	t.Run("an expired attempt is closed and a new one claimed", func(t *testing.T) {
		migrationID := uuid.New()
		planned := plan(t, migrationID, STEP_EXPORT_SCHEMA)
		withOperation(t, planned)
		_, err := db.Exec(`INSERT INTO voyager_core.step_execution (migration_id, operation_step_id, step, last_heartbeat_at)
			SELECT $1, id, step, now() - interval '10 minutes' FROM voyager_core.operation_step WHERE operation_id = $2`,
			migrationID, planned)
		require.NoError(t, err)

		controlPlane := newControlPlane(t)
		require.NoError(t, controlPlane.ClaimStep(migrationID, STEP_EXPORT_SCHEMA, neverLost(t)))
		got := attempts(t, migrationID, STEP_EXPORT_SCHEMA)
		require.Len(t, got, 2)
		assert.Equal(t, "failed", got[0].status)
		assert.Equal(t, "lease_expired", got[0].reason.String)
		assert.True(t, got[0].finished)
		assert.Equal(t, "running", got[1].status)
		require.NoError(t, controlPlane.CloseStep(nil))
	})

	t.Run("a live attempt makes a second claim fail", func(t *testing.T) {
		migrationID := uuid.New()
		withOperation(t, plan(t, migrationID, STEP_EXPORT_SCHEMA))
		first := newControlPlane(t)
		require.NoError(t, first.ClaimStep(migrationID, STEP_EXPORT_SCHEMA, neverLost(t)))

		second := newControlPlane(t)
		err := second.ClaimStep(migrationID, STEP_EXPORT_SCHEMA, neverLost(t))
		assert.ErrorContains(t, err, "already running")
		got := attempts(t, migrationID, STEP_EXPORT_SCHEMA)
		require.Len(t, got, 1)
		assert.Equal(t, "running", got[0].status)
		require.NoError(t, first.CloseStep(nil))
	})

	t.Run("an unplanned step or a foreign operation fails before any row", func(t *testing.T) {
		migrationID := uuid.New()
		withOperation(t, plan(t, migrationID, STEP_ASSESS_MIGRATION))
		controlPlane := newControlPlane(t)
		assert.ErrorContains(t, controlPlane.ClaimStep(migrationID, STEP_EXPORT_SCHEMA, neverLost(t)), "is not planned for operation")

		otherMigration := uuid.New()
		assert.ErrorContains(t, controlPlane.ClaimStep(otherMigration, STEP_ASSESS_MIGRATION, neverLost(t)), "belongs to migration")

		var count int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM voyager_core.step_execution WHERE migration_id IN ($1, $2)`,
			migrationID, otherMigration).Scan(&count))
		assert.Equal(t, 0, count)
	})
}
