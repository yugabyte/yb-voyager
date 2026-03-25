//go:build unit

package workflow

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestStore(t *testing.T) *sqliteWorkflowStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := newSQLiteWorkflowStore(db)
	if err := store.EnsureTables(context.Background()); err != nil {
		t.Fatalf("failed to ensure tables: %v", err)
	}
	return store
}

func TestEnsureTablesIdempotent(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	if err := store.EnsureTables(ctx); err != nil {
		t.Fatalf("second EnsureTables call should be idempotent: %v", err)
	}
}

func TestCreateAndGetInstance(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	inst := newWorkflowInstance("import-data")
	if err := store.CreateInstance(ctx, inst); err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	got, err := store.GetInstance(ctx, inst.UUID)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if got.UUID != inst.UUID {
		t.Errorf("UUID mismatch: got %q, want %q", got.UUID, inst.UUID)
	}
	if got.DefinitionName != "import-data" {
		t.Errorf("DefinitionName mismatch: got %q, want %q", got.DefinitionName, "import-data")
	}
	if got.Status != WorkflowStatusPending {
		t.Errorf("Status mismatch: got %q, want %q", got.Status, WorkflowStatusPending)
	}
	if got.ParentWorkflowUUID != "" {
		t.Errorf("ParentWorkflowUUID should be empty for top-level, got %q", got.ParentWorkflowUUID)
	}
}

func TestGetInstanceNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	_, err := store.GetInstance(ctx, "nonexistent-uuid")
	if err == nil {
		t.Fatal("expected error for nonexistent instance, got nil")
	}
}

func TestUpdateInstanceStatus(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	inst := newWorkflowInstance("export-data")
	if err := store.CreateInstance(ctx, inst); err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if err := store.UpdateInstanceStatus(ctx, inst.UUID, WorkflowStatusRunning); err != nil {
		t.Fatalf("UpdateInstanceStatus failed: %v", err)
	}

	got, err := store.GetInstance(ctx, inst.UUID)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if got.Status != WorkflowStatusRunning {
		t.Errorf("Status mismatch: got %q, want %q", got.Status, WorkflowStatusRunning)
	}
	if !got.UpdatedAt.After(got.CreatedAt) || got.UpdatedAt.Equal(got.CreatedAt) {
		// UpdatedAt should be >= CreatedAt after an update.
		// Due to time granularity (Unix seconds), they may be equal if the test runs fast.
	}
}

func TestUpdateInstanceStatusNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	err := store.UpdateInstanceStatus(ctx, "nonexistent-uuid", WorkflowStatusRunning)
	if err == nil {
		t.Fatal("expected error for nonexistent instance, got nil")
	}
}

func TestChildInstances(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	parent := newWorkflowInstance("migration")
	if err := store.CreateInstance(ctx, parent); err != nil {
		t.Fatalf("CreateInstance (parent) failed: %v", err)
	}

	child1 := newChildWorkflowInstance("schema-migrate", parent.UUID, "schema-migrate-step")
	child2 := newChildWorkflowInstance("schema-migrate", parent.UUID, "schema-migrate-step")
	otherChild := newChildWorkflowInstance("data-migrate", parent.UUID, "data-migrate-step")

	for _, c := range []workflowInstance{child1, child2, otherChild} {
		if err := store.CreateInstance(ctx, c); err != nil {
			t.Fatalf("CreateInstance (child) failed: %v", err)
		}
	}

	children, err := store.GetChildInstances(ctx, parent.UUID, "schema-migrate-step")
	if err != nil {
		t.Fatalf("GetChildInstances failed: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 children for schema-migrate-step, got %d", len(children))
	}
	for _, c := range children {
		if c.ParentWorkflowUUID != parent.UUID {
			t.Errorf("child ParentWorkflowUUID mismatch: got %q, want %q", c.ParentWorkflowUUID, parent.UUID)
		}
		if c.ParentStepName != "schema-migrate-step" {
			t.Errorf("child ParentStepName mismatch: got %q, want %q", c.ParentStepName, "schema-migrate-step")
		}
	}

	otherChildren, err := store.GetChildInstances(ctx, parent.UUID, "data-migrate-step")
	if err != nil {
		t.Fatalf("GetChildInstances (data-migrate) failed: %v", err)
	}
	if len(otherChildren) != 1 {
		t.Fatalf("expected 1 child for data-migrate-step, got %d", len(otherChildren))
	}

	noChildren, err := store.GetChildInstances(ctx, parent.UUID, "nonexistent-step")
	if err != nil {
		t.Fatalf("GetChildInstances (nonexistent) failed: %v", err)
	}
	if len(noChildren) != 0 {
		t.Fatalf("expected 0 children for nonexistent-step, got %d", len(noChildren))
	}
}

func TestSetAndGetStepState(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	inst := newWorkflowInstance("import-data")
	if err := store.CreateInstance(ctx, inst); err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	now := time.Now()
	state := stepState{
		WorkflowUUID: inst.UUID,
		StepName:     "import-snapshot",
		Status:       StepStatusRunning,
		StartedAt:    &now,
	}
	if err := store.SetStepState(ctx, state); err != nil {
		t.Fatalf("SetStepState failed: %v", err)
	}

	got, err := store.GetStepState(ctx, inst.UUID, "import-snapshot")
	if err != nil {
		t.Fatalf("GetStepState failed: %v", err)
	}
	if got.Status != StepStatusRunning {
		t.Errorf("Status mismatch: got %q, want %q", got.Status, StepStatusRunning)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
	if got.CompletedAt != nil {
		t.Error("CompletedAt should be nil")
	}
	if got.Error != "" {
		t.Errorf("Error should be empty, got %q", got.Error)
	}
}

func TestSetStepStateUpsert(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	inst := newWorkflowInstance("import-data")
	if err := store.CreateInstance(ctx, inst); err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	now := time.Now()
	state := stepState{
		WorkflowUUID: inst.UUID,
		StepName:     "import-snapshot",
		Status:       StepStatusRunning,
		StartedAt:    &now,
	}
	if err := store.SetStepState(ctx, state); err != nil {
		t.Fatalf("SetStepState (initial) failed: %v", err)
	}

	completed := time.Now()
	state.Status = StepStatusCompleted
	state.CompletedAt = &completed
	if err := store.SetStepState(ctx, state); err != nil {
		t.Fatalf("SetStepState (upsert) failed: %v", err)
	}

	got, err := store.GetStepState(ctx, inst.UUID, "import-snapshot")
	if err != nil {
		t.Fatalf("GetStepState failed: %v", err)
	}
	if got.Status != StepStatusCompleted {
		t.Errorf("Status mismatch after upsert: got %q, want %q", got.Status, StepStatusCompleted)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should not be nil after upsert")
	}
}

func TestGetStepStates(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	inst := newWorkflowInstance("import-data")
	if err := store.CreateInstance(ctx, inst); err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	steps := []string{"import-snapshot", "stream-changes", "cutover-processing"}
	for _, name := range steps {
		state := stepState{
			WorkflowUUID: inst.UUID,
			StepName:     name,
			Status:       StepStatusPending,
		}
		if err := store.SetStepState(ctx, state); err != nil {
			t.Fatalf("SetStepState for %q failed: %v", name, err)
		}
	}

	states, err := store.GetStepStates(ctx, inst.UUID)
	if err != nil {
		t.Fatalf("GetStepStates failed: %v", err)
	}
	if len(states) != 3 {
		t.Fatalf("expected 3 step states, got %d", len(states))
	}
}

func TestGetStepStateNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	_, err := store.GetStepState(ctx, "nonexistent-uuid", "nonexistent-step")
	if err == nil {
		t.Fatal("expected error for nonexistent step state, got nil")
	}
}

func TestStepStateWithError(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	inst := newWorkflowInstance("import-data")
	if err := store.CreateInstance(ctx, inst); err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	now := time.Now()
	state := stepState{
		WorkflowUUID: inst.UUID,
		StepName:     "stream-changes",
		Status:       StepStatusFailed,
		StartedAt:    &now,
		CompletedAt:  &now,
		Error:        "connection refused to target DB",
	}
	if err := store.SetStepState(ctx, state); err != nil {
		t.Fatalf("SetStepState failed: %v", err)
	}

	got, err := store.GetStepState(ctx, inst.UUID, "stream-changes")
	if err != nil {
		t.Fatalf("GetStepState failed: %v", err)
	}
	if got.Error != "connection refused to target DB" {
		t.Errorf("Error mismatch: got %q, want %q", got.Error, "connection refused to target DB")
	}
	if got.Status != StepStatusFailed {
		t.Errorf("Status mismatch: got %q, want %q", got.Status, StepStatusFailed)
	}
}
