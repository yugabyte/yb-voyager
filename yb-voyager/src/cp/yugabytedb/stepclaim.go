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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

// See design-specs/cp-step-claiming.md.

// OPERATION_ID_ENV_VAR turns step claiming on. EXECUTOR_REF_ENV_VAR is an
// optional JSON object stored as-is on the attempt.
const OPERATION_ID_ENV_VAR = "VOYAGER_OPERATION_ID"
const EXECUTOR_REF_ENV_VAR = "EXECUTOR_REF"

const OPERATION_TABLE = "operation"
const OPERATION_STEP_TABLE = "operation_step"
const STEP_EXECUTION_TABLE = "step_execution"

const QUALIFIED_OPERATION_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + OPERATION_TABLE
const QUALIFIED_OPERATION_STEP_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + OPERATION_STEP_TABLE
const QUALIFIED_STEP_EXECUTION_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + STEP_EXECUTION_TABLE

const STEP_EXECUTION_ID_COLUMN = "step_execution_id"

const (
	STEP_ASSESS_MIGRATION        = "assess-migration"
	STEP_EXPORT_SCHEMA           = "export-schema"
	STEP_ANALYZE_SCHEMA          = "analyze-schema"
	STEP_IMPORT_SCHEMA           = "import-schema"
	STEP_EXPORT_DATA_FROM_SOURCE = "export-data-from-source"
	STEP_IMPORT_DATA_TO_TARGET   = "import-data-to-target"
	STEP_FINALIZE_SCHEMA         = "finalize-schema"
	STEP_END_MIGRATION           = "end-migration"
)

const (
	defaultHeartbeatInterval       = 10 * time.Second
	defaultLeaseDuration           = 2 * time.Minute
	stepAttemptCloseTimeout        = 10 * time.Second
	uniqueViolationSQLState        = "23505"
	oneRunningPerStepConstraint    = "one_running_per_step"
	stepExecutionLeaseExpiredCause = "lease_expired"
)

// The step each migration event type reports. Its done event closes the attempt.
var phaseSteps = map[string]string{
	"ASSESS MIGRATION": STEP_ASSESS_MIGRATION,
	"EXPORT SCHEMA":    STEP_EXPORT_SCHEMA,
	"ANALYZE SCHEMA":   STEP_ANALYZE_SCHEMA,
	"EXPORT DATA":      STEP_EXPORT_DATA_FROM_SOURCE,
	"IMPORT SCHEMA":    STEP_IMPORT_SCHEMA,
	"IMPORT DATA":      STEP_IMPORT_DATA_TO_TARGET,
}

var errAttemptNotRunning = goerrors.New("step attempt is no longer running")

type stepAttempt struct {
	id   int64
	step string
	pool *pgxpool.Pool
	stop chan struct{}

	// mu guards closed and doneErr. It is held across the done transaction so
	// a close from the exit handler cannot interleave with it.
	mu      sync.Mutex
	closed  bool
	doneErr error
}

// finishLocked ends the heartbeat and releases the attempt's pool. Callers hold a.mu.
func (a *stepAttempt) finishLocked() {
	a.closed = true
	close(a.stop)
	a.pool.Close()
}

// parseStepClaimEnv reads the operation env vars. A zero uuid means step
// claiming is off.
func parseStepClaimEnv() (operationID uuid.UUID, executorRef *string, err error) {
	rawOperationID := os.Getenv(OPERATION_ID_ENV_VAR)
	if rawOperationID == "" {
		return uuid.Nil, nil, nil
	}
	operationID, err = uuid.Parse(rawOperationID)
	if err != nil {
		return uuid.Nil, nil, goerrors.Errorf("%s=%q is not a uuid: %w", OPERATION_ID_ENV_VAR, rawOperationID, err)
	}
	rawExecutorRef := os.Getenv(EXECUTOR_REF_ENV_VAR)
	if rawExecutorRef == "" {
		return operationID, nil, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(rawExecutorRef), &parsed); err != nil || parsed == nil {
		return uuid.Nil, nil, goerrors.Errorf("%s must be a JSON object, got %q", EXECUTOR_REF_ENV_VAR, rawExecutorRef)
	}
	return operationID, &rawExecutorRef, nil
}

// StepClaimingEnabled reports whether VOYAGER_OPERATION_ID was set at Init.
func (cp *YugabyteDB) StepClaimingEnabled() bool {
	return cp.operationID != uuid.Nil
}

// checkStepTables fails when a table or column step claiming uses is missing
// or lacks a privilege it needs.
func (cp *YugabyteDB) checkStepTables() error {
	for _, check := range []struct {
		table      string
		privileges []string
	}{
		{QUALIFIED_OPERATION_TABLE_NAME, []string{"SELECT"}},
		{QUALIFIED_OPERATION_STEP_TABLE_NAME, []string{"SELECT"}},
		{QUALIFIED_STEP_EXECUTION_TABLE_NAME, []string{"INSERT", "SELECT", "UPDATE"}},
	} {
		var oid *uint32
		err := cp.connPool.QueryRow(context.Background(), `SELECT to_regclass($1)::oid`, check.table).Scan(&oid)
		if err != nil {
			return fmt.Errorf("check table %s on yugabytedb control plane db: %w", check.table, err)
		}
		if oid == nil {
			return goerrors.Errorf("table %s does not exist on the yugabytedb control plane db", check.table)
		}
		for _, privilege := range check.privileges {
			var allowed bool
			err := cp.connPool.QueryRow(context.Background(),
				`SELECT has_table_privilege($1::text::regclass, $2)`, check.table, privilege).Scan(&allowed)
			if err != nil {
				return fmt.Errorf("check %s privilege on table %s of yugabytedb control plane db: %w", privilege, check.table, err)
			}
			if !allowed {
				return goerrors.Errorf("%s privilege is needed on table %s of the yugabytedb control plane db", privilege, check.table)
			}
		}
	}
	for _, table := range []string{QUALIFIED_MIGRATION_STATE_TABLE_NAME, QUALIFIED_METADATA_TABLE_NAME} {
		var present bool
		err := cp.connPool.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_attribute
				WHERE attrelid = to_regclass($1) AND attname = $2 AND NOT attisdropped)`,
			table, STEP_EXECUTION_ID_COLUMN).Scan(&present)
		if err != nil {
			return fmt.Errorf("check column %s of table %s on yugabytedb control plane db: %w", STEP_EXECUTION_ID_COLUMN, table, err)
		}
		if !present {
			return goerrors.Errorf("column %s of table %s does not exist on the yugabytedb control plane db", STEP_EXECUTION_ID_COLUMN, table)
		}
	}
	return nil
}

func (cp *YugabyteDB) newAttemptPool() (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), os.Getenv(DB_CONN_STRING_ENV_VAR))
	if err != nil {
		return nil, fmt.Errorf("connect to yugabytedb control plane db for the step attempt: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to yugabytedb control plane db for the step attempt: %w", err)
	}
	return pool, nil
}

// ClaimStep claims step for the migration and starts the attempt's heartbeat.
// onLost is called from the heartbeat goroutine when the attempt can no longer
// be held; the command must stop. A second call while an attempt is held is a no-op.
func (cp *YugabyteDB) ClaimStep(migrationID uuid.UUID, step string, onLost func(err error)) error {
	cp.attemptMu.Lock()
	defer cp.attemptMu.Unlock()
	if cp.attempt != nil {
		return nil
	}
	pool, err := cp.newAttemptPool()
	if err != nil {
		return err
	}
	id, err := cp.claimStepTx(pool, migrationID, step)
	if err != nil {
		pool.Close()
		return err
	}
	attempt := &stepAttempt{id: id, step: step, pool: pool, stop: make(chan struct{})}
	cp.attempt = attempt
	log.Infof("Claimed step %s of operation %s as step execution %d", step, cp.operationID, id)
	go cp.heartbeatLoop(attempt, onLost)
	return nil
}

func (cp *YugabyteDB) claimStepTx(pool *pgxpool.Pool, migrationID uuid.UUID, step string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stepAttemptCloseTimeout)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("claim step %s: begin transaction: %w", step, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var operationStepID int64
	var operationMigrationID uuid.UUID
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT os.id, o.migration_id
		FROM %s os JOIN %s o ON o.id = os.operation_id
		WHERE os.operation_id = $1 AND os.step = $2`,
		QUALIFIED_OPERATION_STEP_TABLE_NAME, QUALIFIED_OPERATION_TABLE_NAME),
		cp.operationID, step).Scan(&operationStepID, &operationMigrationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, goerrors.Errorf("step %s is not planned for operation %s", step, cp.operationID)
	}
	if err != nil {
		return 0, fmt.Errorf("claim step %s: find planned step: %w", step, err)
	}
	if operationMigrationID != migrationID {
		return 0, goerrors.Errorf("operation %s belongs to migration %s, not migration %s",
			cp.operationID, operationMigrationID, migrationID)
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = 'failed', reason = $3, finished_at = now()
		WHERE migration_id = $1 AND step = $2 AND status = 'running'
		  AND last_heartbeat_at < now() - make_interval(secs => $4::float8)`,
		QUALIFIED_STEP_EXECUTION_TABLE_NAME),
		migrationID, step, stepExecutionLeaseExpiredCause, cp.leaseDuration.Seconds())
	if err != nil {
		return 0, fmt.Errorf("claim step %s: close expired attempts: %w", step, err)
	}

	var id int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (migration_id, operation_step_id, step, executor_ref)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id`, QUALIFIED_STEP_EXECUTION_TABLE_NAME),
		migrationID, operationStepID, step, cp.executorRef).Scan(&id)
	// YugabyteDB leaves ConstraintName empty and names the index only in Message.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationSQLState &&
		(pgErr.ConstraintName == oneRunningPerStepConstraint || strings.Contains(pgErr.Message, oneRunningPerStepConstraint)) {
		return 0, goerrors.Errorf("step %s of migration %s is already running", step, migrationID)
	}
	if err != nil {
		return 0, fmt.Errorf("claim step %s: insert attempt: %w", step, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("claim step %s: commit: %w", step, err)
	}
	return id, nil
}

func (cp *YugabyteDB) heartbeatLoop(a *stepAttempt, onLost func(err error)) {
	ticker := time.NewTicker(cp.heartbeatInterval)
	defer ticker.Stop()
	lastSuccess := time.Now()
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), cp.heartbeatInterval+stepAttemptCloseTimeout)
		tag, err := a.pool.Exec(ctx, fmt.Sprintf(
			`UPDATE %s SET last_heartbeat_at = now() WHERE id = $1 AND status = 'running'`,
			QUALIFIED_STEP_EXECUTION_TABLE_NAME), a.id)
		cancel()
		select {
		case <-a.stop:
			return
		default:
		}
		if err == nil && tag.RowsAffected() == 0 {
			onLost(goerrors.Errorf("step %s (step execution %d) was closed by another process; stopping", a.step, a.id))
			return
		}
		if err == nil {
			lastSuccess = time.Now()
			continue
		}
		log.Warnf("Heartbeat for step execution %d failed: %v", a.id, err)
		if time.Since(lastSuccess) >= cp.leaseDuration {
			onLost(fmt.Errorf("heartbeat for step %s (step execution %d) failed for %s; stopping so no other pod runs the step twice: %w",
				a.step, a.id, cp.leaseDuration, err))
			return
		}
	}
}

// CloseStep closes the held attempt: succeeded when runErr is nil, failed
// with runErr's text otherwise. It does nothing when no attempt is held or it
// is already closed, so both the normal and the exit path can call it.
func (cp *YugabyteDB) CloseStep(runErr error) error {
	cp.attemptMu.Lock()
	a := cp.attempt
	cp.attemptMu.Unlock()
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	defer a.finishLocked()

	if runErr == nil && a.doneErr != nil {
		runErr = fmt.Errorf("recording the step's completion in the control plane failed: %w", a.doneErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), stepAttemptCloseTimeout)
	defer cancel()
	var tag pgconn.CommandTag
	var err error
	if runErr == nil {
		tag, err = a.pool.Exec(ctx, closeAttemptSQL("'succeeded'", "NULL"), a.id)
	} else {
		tag, err = a.pool.Exec(ctx, closeAttemptSQL("'failed'", "$2"), a.id, runErr.Error())
	}
	if err != nil {
		return fmt.Errorf("close step execution %d: %w", a.id, err)
	}
	if tag.RowsAffected() == 0 {
		return goerrors.Errorf("close step execution %d: it is no longer running", a.id)
	}
	return nil
}

func closeAttemptSQL(status, errorMessage string) string {
	return fmt.Sprintf(`UPDATE %s SET status = %s, error_message = %s, finished_at = now()
		WHERE id = $1 AND status = 'running'`, QUALIFIED_STEP_EXECUTION_TABLE_NAME, status, errorMessage)
}

// activeStepExecutionID is the id to stamp on rows written now, nil when no
// attempt is open.
func (cp *YugabyteDB) activeStepExecutionID() *int64 {
	cp.attemptMu.Lock()
	a := cp.attempt
	cp.attemptMu.Unlock()
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	id := a.id
	return &id
}

// attemptClosedBy returns the open attempt when eventType is its own step's
// phase: that phase's done event closes it.
func (cp *YugabyteDB) attemptClosedBy(eventType string) *stepAttempt {
	cp.attemptMu.Lock()
	a := cp.attempt
	cp.attemptMu.Unlock()
	if a == nil || phaseSteps[eventType] != a.step {
		return nil
	}
	return a
}

// execClosingAttempt runs cmd and closes the attempt in one transaction. The
// close goes first: if the attempt is no longer running nothing is written.
func (cp *YugabyteDB) execClosingAttempt(a *stepAttempt, cmd string, args ...any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return errAttemptNotRunning
	}
	err := cp.withPool(cmd, func(pool *pgxpool.Pool) error {
		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		tag, err := tx.Exec(ctx, closeAttemptSQL("'succeeded'", "NULL"), a.id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errAttemptNotRunning
		}
		if _, err := tx.Exec(ctx, cmd, args...); err != nil {
			return err
		}
		return tx.Commit(ctx)
	})
	if err != nil {
		a.doneErr = err
		return err
	}
	a.finishLocked()
	return nil
}
