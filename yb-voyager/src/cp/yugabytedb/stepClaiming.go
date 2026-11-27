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
	"strconv"
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
// optional JSON object stored as-is on the step execution.
const OPERATION_ID_ENV_VAR = "VOYAGER_OPERATION_ID"
const EXECUTOR_REF_ENV_VAR = "EXECUTOR_REF"

// STEP_EXECUTION_ID_ENV_VAR is internal: Voyager sets it for its own child
// process, which joins the parent's step execution instead of claiming one.
const STEP_EXECUTION_ID_ENV_VAR = "VOYAGER_STEP_EXECUTION_ID"

type stepClaimingMode int

const (
	stepClaimingOff stepClaimingMode = iota
	stepClaimingClaim
	stepClaimingJoin
)

type stepClaimingEnv struct {
	mode        stepClaimingMode
	operationID uuid.UUID
	joinID      int64
	executorRef *string
}

const OPERATION_TABLE = "operation"
const OPERATION_STEP_TABLE = "operation_step"
const STEP_EXECUTION_TABLE = "step_execution"

const QUALIFIED_OPERATION_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + OPERATION_TABLE
const QUALIFIED_OPERATION_STEP_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + OPERATION_STEP_TABLE
const QUALIFIED_STEP_EXECUTION_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + STEP_EXECUTION_TABLE

const STEP_EXECUTION_ID_COLUMN = "step_execution_id"

const (
	defaultHeartbeatInterval       = 10 * time.Second
	defaultLeaseDuration           = 2 * time.Minute
	stepExecutionCloseTimeout      = 10 * time.Second
	uniqueViolationSQLState        = "23505"
	oneRunningPerStepConstraint    = "one_running_per_step"
	stepExecutionLeaseExpiredCause = "lease_expired"
)

var errStepExecutionNotRunning = goerrors.New("step attempt is no longer running")

type stepExecution struct {
	id   int64
	step string
	pool *pgxpool.Pool
	stop chan struct{}

	// mu guards closed and doneErr. It is held across the done transaction so
	// a close from the exit handler cannot interleave with it.
	mu      sync.Mutex
	closed  bool
	doneErr error

	// joined is a step execution held by the parent process: this process
	// stamps with its id but never heartbeats or closes it. pool is nil.
	joined bool
}

// releaseLocked ends the heartbeat and releases the step execution's pool. Callers hold a.mu.
func (a *stepExecution) releaseLocked() {
	a.closed = true
	close(a.stop)
	a.pool.Close()
}

// readStepClaimingEnv reads the step claiming env vars: off when neither
// VOYAGER_OPERATION_ID nor VOYAGER_STEP_EXECUTION_ID is set, claim for the
// first, join for the second. Both set is an error.
func readStepClaimingEnv() (stepClaimingEnv, error) {
	rawOperationID := os.Getenv(OPERATION_ID_ENV_VAR)
	rawJoinID := os.Getenv(STEP_EXECUTION_ID_ENV_VAR)
	if rawOperationID != "" && rawJoinID != "" {
		return stepClaimingEnv{}, goerrors.Errorf("%s and %s must not both be set", OPERATION_ID_ENV_VAR, STEP_EXECUTION_ID_ENV_VAR)
	}
	if rawJoinID != "" {
		joinID, err := strconv.ParseInt(rawJoinID, 10, 64)
		if err != nil || joinID <= 0 {
			return stepClaimingEnv{}, goerrors.Errorf("%s=%q is not a positive integer", STEP_EXECUTION_ID_ENV_VAR, rawJoinID)
		}
		return stepClaimingEnv{mode: stepClaimingJoin, joinID: joinID}, nil
	}
	if rawOperationID == "" {
		return stepClaimingEnv{}, nil
	}
	operationID, err := uuid.Parse(rawOperationID)
	if err != nil {
		return stepClaimingEnv{}, goerrors.Errorf("%s=%q is not a uuid: %w", OPERATION_ID_ENV_VAR, rawOperationID, err)
	}
	env := stepClaimingEnv{mode: stepClaimingClaim, operationID: operationID}
	rawExecutorRef := os.Getenv(EXECUTOR_REF_ENV_VAR)
	if rawExecutorRef == "" {
		return env, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(rawExecutorRef), &parsed); err != nil || parsed == nil {
		return stepClaimingEnv{}, goerrors.Errorf("%s must be a JSON object, got %q", EXECUTOR_REF_ENV_VAR, rawExecutorRef)
	}
	env.executorRef = &rawExecutorRef
	return env, nil
}

// ChildProcessEnv returns environ for a child Voyager process: both step
// claiming vars removed, and VOYAGER_STEP_EXECUTION_ID set to stepExecutionID
// when it is not nil, so the child joins that step execution.
func ChildProcessEnv(environ []string, stepExecutionID *int64) []string {
	env := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if strings.HasPrefix(entry, OPERATION_ID_ENV_VAR+"=") || strings.HasPrefix(entry, STEP_EXECUTION_ID_ENV_VAR+"=") {
			continue
		}
		env = append(env, entry)
	}
	if stepExecutionID != nil {
		env = append(env, fmt.Sprintf("%s=%d", STEP_EXECUTION_ID_ENV_VAR, *stepExecutionID))
	}
	return env
}

// StepClaimingEnabled reports whether step claiming is on at Init, in claim or join mode.
func (cp *YugabyteDB) StepClaimingEnabled() bool {
	return cp.claimingEnv.mode != stepClaimingOff
}

// HeldStepExecutionID is the id of the open step execution, for a child
// process to join. Nil when none is open.
func (cp *YugabyteDB) HeldStepExecutionID() *int64 {
	return cp.activeStepExecutionID()
}

// checkStepClaimingTables fails when a table or column step claiming uses is missing
// or lacks a privilege it needs.
func (cp *YugabyteDB) checkStepClaimingTables() error {
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

// newStepExecutionPool opens the step execution's own pool for the claim, heartbeat and close.
// It is separate from cp.connPool, which Finalize closes before the step execution is
// closed and whose writers hold connMutex while retrying; a heartbeat must not
// wait behind them. A pool, not one connection, because the heartbeat goroutine
// and the close can run at the same time.
func (cp *YugabyteDB) newStepExecutionPool() (*pgxpool.Pool, error) {
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

// ClaimStep claims step for the migration and starts the step execution's heartbeat.
// In join mode it instead joins the parent's step execution, with no heartbeat.
// onLost is called from the heartbeat goroutine when the step execution can no longer
// be held; the command must stop. A second call while a step execution is held is a no-op.
func (cp *YugabyteDB) ClaimStep(migrationID uuid.UUID, step string, onLost func(err error)) error {
	cp.stepExecutionMu.Lock()
	defer cp.stepExecutionMu.Unlock()
	if cp.stepExecution != nil {
		return nil
	}
	pool, err := cp.newStepExecutionPool()
	if err != nil {
		return err
	}
	if cp.claimingEnv.mode == stepClaimingJoin {
		defer pool.Close()
		execution, err := cp.joinStepExecution(pool, migrationID)
		if err != nil {
			return err
		}
		cp.stepExecution = execution
		log.Infof("Joined step execution %d (step %s) held by the parent process", execution.id, execution.step)
		return nil
	}
	id, err := cp.claimStepInTx(pool, migrationID, step)
	if err != nil {
		pool.Close()
		return err
	}
	execution := &stepExecution{id: id, step: step, pool: pool, stop: make(chan struct{})}
	cp.stepExecution = execution
	log.Infof("Claimed step %s of operation %s as step execution %d", step, cp.claimingEnv.operationID, id)
	go cp.heartbeatStepExecution(execution, onLost)
	return nil
}

// joinStepExecution checks that the step execution of VOYAGER_STEP_EXECUTION_ID is
// running and belongs to the migration, and returns it for stamping only.
func (cp *YugabyteDB) joinStepExecution(pool *pgxpool.Pool, migrationID uuid.UUID) (*stepExecution, error) {
	id := cp.claimingEnv.joinID
	ctx, cancel := context.WithTimeout(context.Background(), stepExecutionCloseTimeout)
	defer cancel()
	var step, status string
	var executionMigrationID uuid.UUID
	err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT step, status, migration_id FROM %s WHERE id = $1`,
		QUALIFIED_STEP_EXECUTION_TABLE_NAME), id).Scan(&step, &status, &executionMigrationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, goerrors.Errorf("step execution %d from %s does not exist", id, STEP_EXECUTION_ID_ENV_VAR)
	}
	if err != nil {
		return nil, fmt.Errorf("join step execution %d: %w", id, err)
	}
	if status != "running" {
		return nil, goerrors.Errorf("step execution %d is %s, not running", id, status)
	}
	if executionMigrationID != migrationID {
		return nil, goerrors.Errorf("step execution %d belongs to migration %s, not migration %s", id, executionMigrationID, migrationID)
	}
	return &stepExecution{id: id, step: step, joined: true}, nil
}

func (cp *YugabyteDB) claimStepInTx(pool *pgxpool.Pool, migrationID uuid.UUID, step string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stepExecutionCloseTimeout)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("claim step %s: begin transaction: %w", step, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	// 1. Find this command's step in the operation's plan, and check the
	// operation belongs to the migration this export dir is for.
	var operationStepID int64
	var operationMigrationID uuid.UUID
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT os.id, o.migration_id
		FROM %s os JOIN %s o ON o.id = os.operation_id
		WHERE os.operation_id = $1 AND os.step = $2`,
		QUALIFIED_OPERATION_STEP_TABLE_NAME, QUALIFIED_OPERATION_TABLE_NAME),
		cp.claimingEnv.operationID, step).Scan(&operationStepID, &operationMigrationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, goerrors.Errorf("step %s is not planned for operation %s", step, cp.claimingEnv.operationID)
	}
	if err != nil {
		return 0, fmt.Errorf("claim step %s: find planned step: %w", step, err)
	}
	if operationMigrationID != migrationID {
		return 0, goerrors.Errorf("operation %s belongs to migration %s, not migration %s",
			cp.claimingEnv.operationID, operationMigrationID, migrationID)
	}

	// 2. Close a previous step execution at this step only if its heartbeat is older
	// than the lease: its process is gone. A live step execution is left alone, and
	// the insert below then fails at once on one_running_per_step.
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

	// 3. The insert is the claim. one_running_per_step allows one running
	// step execution per (migration, step), so a second claimer gets a unique violation.
	var id int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (migration_id, operation_step_id, step, executor_ref)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id`, QUALIFIED_STEP_EXECUTION_TABLE_NAME),
		migrationID, operationStepID, step, cp.claimingEnv.executorRef).Scan(&id)
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

func (cp *YugabyteDB) heartbeatStepExecution(a *stepExecution, onLost func(err error)) {
	ticker := time.NewTicker(cp.heartbeatInterval)
	defer ticker.Stop()
	lastSuccess := time.Now()
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), cp.heartbeatInterval)
		tag, err := a.pool.Exec(ctx, fmt.Sprintf(
			`UPDATE %s SET last_heartbeat_at = now() WHERE id = $1 AND status = 'running'`,
			QUALIFIED_STEP_EXECUTION_TABLE_NAME), a.id)
		cancel()
		select {
		case <-a.stop:
			return
		default:
		}
		// No row updated: the step execution is no longer running, so another claimer
		// closed it as expired. Stop rather than run the step twice.
		if err == nil && tag.RowsAffected() == 0 {
			onLost(goerrors.Errorf("step %s (step execution %d) was closed by another process; stopping", a.step, a.id))
			return
		}
		if err == nil {
			lastSuccess = time.Now()
			continue
		}
		log.Warnf("Heartbeat for step execution %d failed: %v", a.id, err)
		// A failure is only seen when an UPDATE returns, up to one tick plus one
		// timeout late, so stop that much before the lease lets another pod claim.
		if failingFor := time.Since(lastSuccess); failingFor >= cp.leaseDuration-2*cp.heartbeatInterval {
			onLost(fmt.Errorf("heartbeat for step %s (step execution %d) failed for %s; stopping so no other pod runs the step twice: %w",
				a.step, a.id, failingFor.Round(time.Second), err))
			return
		}
	}
}

// CloseStep closes the held step execution: succeeded when runErr is nil, failed
// with runErr's text otherwise. It does nothing when no step execution is held or it
// is already closed, so both the normal and the exit path can call it.
func (cp *YugabyteDB) CloseStep(runErr error) error {
	cp.stepExecutionMu.Lock()
	a := cp.stepExecution
	cp.stepExecutionMu.Unlock()
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.joined {
		return nil
	}
	defer a.releaseLocked()

	// The command succeeded but writing its done event (which also closes the
	// step execution) failed: record the step execution as failed, with that error.
	if runErr == nil && a.doneErr != nil {
		runErr = fmt.Errorf("recording the step's completion in the control plane failed: %w", a.doneErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), stepExecutionCloseTimeout)
	defer cancel()
	var tag pgconn.CommandTag
	var err error
	if runErr == nil {
		tag, err = a.pool.Exec(ctx, closeStepExecutionSQL("'succeeded'", "NULL"), a.id)
	} else {
		tag, err = a.pool.Exec(ctx, closeStepExecutionSQL("'failed'", "$2"), a.id, runErr.Error())
	}
	if err != nil {
		return fmt.Errorf("close step execution %d: %w", a.id, err)
	}
	if tag.RowsAffected() == 0 {
		return goerrors.Errorf("close step execution %d: it is no longer running", a.id)
	}
	return nil
}

func closeStepExecutionSQL(status, errorMessage string) string {
	return fmt.Sprintf(`UPDATE %s SET status = %s, error_message = %s, finished_at = now()
		WHERE id = $1 AND status = 'running'`, QUALIFIED_STEP_EXECUTION_TABLE_NAME, status, errorMessage)
}

// activeStepExecutionID is the id to stamp on rows written now, nil when no
// step execution is open.
func (cp *YugabyteDB) activeStepExecutionID() *int64 {
	cp.stepExecutionMu.Lock()
	a := cp.stepExecution
	cp.stepExecutionMu.Unlock()
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

// stepExecutionDoneBy returns the open step execution when eventType is its own step's
// phase: that phase's done event closes it.
func (cp *YugabyteDB) stepExecutionDoneBy(eventType string) *stepExecution {
	cp.stepExecutionMu.Lock()
	a := cp.stepExecution
	cp.stepExecutionMu.Unlock()
	if a == nil || a.joined || stepForEventType[eventType] != a.step {
		return nil
	}
	return a
}

// execAndCloseStepExecution runs cmd and closes the step execution in one transaction. The
// close goes first: if the step execution is no longer running nothing is written.
func (cp *YugabyteDB) execAndCloseStepExecution(a *stepExecution, cmd string, args ...any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return errStepExecutionNotRunning
	}
	err := cp.withRetry(cmd, func(pool *pgxpool.Pool) error {
		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		tag, err := tx.Exec(ctx, closeStepExecutionSQL("'succeeded'", "NULL"), a.id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errStepExecutionNotRunning
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
	a.releaseLocked()
	return nil
}
