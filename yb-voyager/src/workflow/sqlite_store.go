package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ WorkflowStore = (*SQLiteWorkflowStore)(nil)

type SQLiteWorkflowStore struct {
	db *sql.DB
}

func NewSQLiteWorkflowStore(db *sql.DB) *SQLiteWorkflowStore {
	return &SQLiteWorkflowStore{db: db}
}

func (s *SQLiteWorkflowStore) EnsureTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS workflow_instances (
			uuid                  TEXT PRIMARY KEY,
			definition_name       TEXT NOT NULL,
			status                TEXT NOT NULL DEFAULT 'pending',
			parent_workflow_uuid  TEXT,
			parent_step_name      TEXT,
			created_at            INTEGER NOT NULL,
			updated_at            INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workflow_step_states (
			workflow_uuid  TEXT NOT NULL,
			step_name      TEXT NOT NULL,
			status         TEXT NOT NULL DEFAULT 'pending',
			started_at     INTEGER,
			completed_at   INTEGER,
			error          TEXT,
			PRIMARY KEY (workflow_uuid, step_name)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create workflow table: %w", err)
		}
	}
	return nil
}

func (s *SQLiteWorkflowStore) CreateInstance(ctx context.Context, inst *WorkflowInstance) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflow_instances
			(uuid, definition_name, status, parent_workflow_uuid, parent_step_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		inst.UUID,
		inst.DefinitionName,
		string(inst.Status),
		nullableString(inst.ParentWorkflowUUID),
		nullableString(inst.ParentStepName),
		inst.CreatedAt.Unix(),
		inst.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to create workflow instance: %w", err)
	}
	return nil
}

func (s *SQLiteWorkflowStore) GetInstance(ctx context.Context, uuid string) (*WorkflowInstance, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT uuid, definition_name, status, parent_workflow_uuid, parent_step_name, created_at, updated_at
		FROM workflow_instances WHERE uuid = ?`, uuid)

	inst, err := scanWorkflowInstance(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow instance %q not found", uuid)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow instance: %w", err)
	}
	return inst, nil
}

func (s *SQLiteWorkflowStore) UpdateInstanceStatus(ctx context.Context, uuid string, status WorkflowStatus) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE workflow_instances SET status = ?, updated_at = ? WHERE uuid = ?`,
		string(status), time.Now().Unix(), uuid)
	if err != nil {
		return fmt.Errorf("failed to update workflow instance status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("workflow instance %q not found", uuid)
	}
	return nil
}

func (s *SQLiteWorkflowStore) GetChildInstances(ctx context.Context, parentUUID string, stepName string) ([]*WorkflowInstance, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT uuid, definition_name, status, parent_workflow_uuid, parent_step_name, created_at, updated_at
		FROM workflow_instances
		WHERE parent_workflow_uuid = ? AND parent_step_name = ?
		ORDER BY created_at`, parentUUID, stepName)
	if err != nil {
		return nil, fmt.Errorf("failed to query child workflow instances: %w", err)
	}
	defer rows.Close()

	var instances []*WorkflowInstance
	for rows.Next() {
		inst, err := scanWorkflowInstanceFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan child workflow instance: %w", err)
		}
		instances = append(instances, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating child workflow instances: %w", err)
	}
	return instances, nil
}

func (s *SQLiteWorkflowStore) SetStepState(ctx context.Context, state *StepState) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO workflow_step_states
			(workflow_uuid, step_name, status, started_at, completed_at, error)
		VALUES (?, ?, ?, ?, ?, ?)`,
		state.WorkflowUUID,
		state.StepName,
		string(state.Status),
		nullableTime(state.StartedAt),
		nullableTime(state.CompletedAt),
		nullableString(state.Error),
	)
	if err != nil {
		return fmt.Errorf("failed to set step state: %w", err)
	}
	return nil
}

func (s *SQLiteWorkflowStore) GetStepStates(ctx context.Context, workflowUUID string) ([]StepState, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT workflow_uuid, step_name, status, started_at, completed_at, error
		FROM workflow_step_states WHERE workflow_uuid = ?`, workflowUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query step states: %w", err)
	}
	defer rows.Close()

	var states []StepState
	for rows.Next() {
		state, err := scanStepState(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan step state: %w", err)
		}
		states = append(states, *state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating step states: %w", err)
	}
	return states, nil
}

func (s *SQLiteWorkflowStore) GetStepState(ctx context.Context, workflowUUID string, stepName string) (*StepState, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT workflow_uuid, step_name, status, started_at, completed_at, error
		FROM workflow_step_states WHERE workflow_uuid = ? AND step_name = ?`,
		workflowUUID, stepName)

	var state StepState
	var startedAt, completedAt sql.NullInt64
	var errStr sql.NullString
	err := row.Scan(&state.WorkflowUUID, &state.StepName, &state.Status, &startedAt, &completedAt, &errStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("step state for workflow %q step %q not found", workflowUUID, stepName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get step state: %w", err)
	}
	state.StartedAt = timeFromNullInt(startedAt)
	state.CompletedAt = timeFromNullInt(completedAt)
	if errStr.Valid {
		state.Error = errStr.String
	}
	return &state, nil
}

// --- helpers ---

func scanWorkflowInstance(row *sql.Row) (*WorkflowInstance, error) {
	var inst WorkflowInstance
	var parentUUID, parentStep sql.NullString
	var createdAt, updatedAt int64
	err := row.Scan(&inst.UUID, &inst.DefinitionName, &inst.Status,
		&parentUUID, &parentStep, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if parentUUID.Valid {
		inst.ParentWorkflowUUID = parentUUID.String
	}
	if parentStep.Valid {
		inst.ParentStepName = parentStep.String
	}
	inst.CreatedAt = time.Unix(createdAt, 0)
	inst.UpdatedAt = time.Unix(updatedAt, 0)
	return &inst, nil
}

func scanWorkflowInstanceFromRows(rows *sql.Rows) (*WorkflowInstance, error) {
	var inst WorkflowInstance
	var parentUUID, parentStep sql.NullString
	var createdAt, updatedAt int64
	err := rows.Scan(&inst.UUID, &inst.DefinitionName, &inst.Status,
		&parentUUID, &parentStep, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if parentUUID.Valid {
		inst.ParentWorkflowUUID = parentUUID.String
	}
	if parentStep.Valid {
		inst.ParentStepName = parentStep.String
	}
	inst.CreatedAt = time.Unix(createdAt, 0)
	inst.UpdatedAt = time.Unix(updatedAt, 0)
	return &inst, nil
}

func scanStepState(rows *sql.Rows) (*StepState, error) {
	var state StepState
	var startedAt, completedAt sql.NullInt64
	var errStr sql.NullString
	err := rows.Scan(&state.WorkflowUUID, &state.StepName, &state.Status,
		&startedAt, &completedAt, &errStr)
	if err != nil {
		return nil, err
	}
	state.StartedAt = timeFromNullInt(startedAt)
	state.CompletedAt = timeFromNullInt(completedAt)
	if errStr.Valid {
		state.Error = errStr.String
	}
	return &state, nil
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullableTime(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

func timeFromNullInt(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := time.Unix(n.Int64, 0)
	return &t
}
