//go:build unit

package workflow

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestEngine(t *testing.T) *WorkflowEngine {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	engine, err := NewWorkflowEngine(db)
	if err != nil {
		t.Fatalf("NewWorkflowEngine failed: %v", err)
	}
	return engine
}

func registerMigrationWorkflows(t *testing.T, engine *WorkflowEngine) {
	t.Helper()
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: "migration",
		Steps: []StepDefinition{
			{Name: "assess"},
			{Name: "schema-migrate", SubWorkflowName: "schema-migrate-flow"},
			{Name: "data-migrate"},
		},
	})
	if err != nil {
		t.Fatalf("RegisterWorkflow (migration) failed: %v", err)
	}

	err = engine.RegisterWorkflow(WorkflowDefinition{
		Name: "schema-migrate-flow",
		Steps: []StepDefinition{
			{Name: "export-schema"},
			{Name: "analyze-schema"},
			{Name: "import-schema"},
		},
	})
	if err != nil {
		t.Fatalf("RegisterWorkflow (schema-migrate-flow) failed: %v", err)
	}
}

// --- Registration ---

func TestRegisterWorkflow(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "test-wf",
		Steps: []StepDefinition{{Name: "step-a"}},
	})
	if err != nil {
		t.Fatalf("RegisterWorkflow failed: %v", err)
	}
}

func TestRegisterWorkflowDuplicate(t *testing.T) {
	engine := setupTestEngine(t)
	def := WorkflowDefinition{Name: "dup", Steps: []StepDefinition{{Name: "a"}}}
	if err := engine.RegisterWorkflow(def); err != nil {
		t.Fatal(err)
	}
	if err := engine.RegisterWorkflow(def); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegisterWorkflowEmptyName(t *testing.T) {
	engine := setupTestEngine(t)
	if err := engine.RegisterWorkflow(WorkflowDefinition{Steps: []StepDefinition{{Name: "a"}}}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

// --- GetWorkflowTree ---

func TestGetWorkflowTree(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)

	tree, err := engine.GetWorkflowTree("migration")
	if err != nil {
		t.Fatalf("GetWorkflowTree failed: %v", err)
	}
	if tree.Definition.Name != "migration" {
		t.Errorf("expected root name 'migration', got %q", tree.Definition.Name)
	}
	if len(tree.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(tree.Steps))
	}
	if tree.Steps[0].ChildWorkflow != nil {
		t.Error("assess should not have a child workflow")
	}
	if tree.Steps[1].ChildWorkflow == nil {
		t.Fatal("schema-migrate should have a child workflow")
	}
	if tree.Steps[1].ChildWorkflow.Definition.Name != "schema-migrate-flow" {
		t.Errorf("child workflow name mismatch: got %q", tree.Steps[1].ChildWorkflow.Definition.Name)
	}
	if len(tree.Steps[1].ChildWorkflow.Steps) != 3 {
		t.Errorf("expected 3 child steps, got %d", len(tree.Steps[1].ChildWorkflow.Steps))
	}
	if tree.Steps[2].ChildWorkflow != nil {
		t.Error("data-migrate should not have a child workflow")
	}
}

func TestGetWorkflowTreeUnregisteredChild(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: "broken",
		Steps: []StepDefinition{
			{Name: "step-a", SubWorkflowName: "nonexistent"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.GetWorkflowTree("broken")
	if err == nil {
		t.Fatal("expected error for unregistered child workflow")
	}
}

// --- StartWorkflow ---

func TestStartWorkflow(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	uuid, err := engine.StartWorkflow(ctx, "migration")
	if err != nil {
		t.Fatalf("StartWorkflow failed: %v", err)
	}
	if uuid == "" {
		t.Fatal("expected non-empty UUID")
	}

	report, err := engine.GetStatus(ctx, uuid)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if report.Status != WorkflowStatusPending {
		t.Errorf("expected pending, got %q", report.Status)
	}
	if len(report.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(report.Steps))
	}
	for _, s := range report.Steps {
		if s.Status != StepStatusPending {
			t.Errorf("step %q should be pending, got %q", s.StepName, s.Status)
		}
	}
}

func TestStartWorkflowUnregistered(t *testing.T) {
	engine := setupTestEngine(t)
	ctx := context.Background()

	_, err := engine.StartWorkflow(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unregistered workflow")
	}
}

// --- Step Transitions ---

func TestStepLifecycle(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migration")

	if err := engine.StartStep(ctx, uuid, "assess"); err != nil {
		t.Fatalf("StartStep failed: %v", err)
	}
	report, _ := engine.GetStatus(ctx, uuid)
	if report.Status != WorkflowStatusRunning {
		t.Errorf("workflow should be running, got %q", report.Status)
	}
	if report.Steps[0].Status != StepStatusRunning {
		t.Errorf("assess should be running, got %q", report.Steps[0].Status)
	}
	if report.Steps[0].StartedAt == nil {
		t.Error("assess StartedAt should be set")
	}

	if err := engine.CompleteStep(ctx, uuid, "assess"); err != nil {
		t.Fatalf("CompleteStep failed: %v", err)
	}
	report, _ = engine.GetStatus(ctx, uuid)
	if report.Steps[0].Status != StepStatusCompleted {
		t.Errorf("assess should be completed, got %q", report.Steps[0].Status)
	}
	if report.Steps[0].CompletedAt == nil {
		t.Error("assess CompletedAt should be set")
	}
	if report.Status != WorkflowStatusRunning {
		t.Errorf("workflow should still be running (other steps pending), got %q", report.Status)
	}
}

func TestAllStepsCompleteMarksWorkflowCompleted(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "simple",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "simple")

	engine.StartStep(ctx, uuid, "a")
	engine.CompleteStep(ctx, uuid, "a")
	engine.StartStep(ctx, uuid, "b")
	engine.CompleteStep(ctx, uuid, "b")

	report, _ := engine.GetStatus(ctx, uuid)
	if report.Status != WorkflowStatusCompleted {
		t.Errorf("workflow should be completed, got %q", report.Status)
	}
}

func TestSkipAndCompleteMarksWorkflowCompleted(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "skip-test",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "skip-test")

	engine.SkipStep(ctx, uuid, "a")
	engine.StartStep(ctx, uuid, "b")
	engine.CompleteStep(ctx, uuid, "b")
	engine.SkipStep(ctx, uuid, "c")

	report, _ := engine.GetStatus(ctx, uuid)
	if report.Status != WorkflowStatusCompleted {
		t.Errorf("workflow should be completed, got %q", report.Status)
	}
}

func TestFailStepMarksWorkflowFailed(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "fail-test",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "fail-test")

	engine.StartStep(ctx, uuid, "a")
	engine.FailStep(ctx, uuid, "a", errors.New("boom"))

	report, _ := engine.GetStatus(ctx, uuid)
	if report.Status != WorkflowStatusFailed {
		t.Errorf("workflow should be failed, got %q", report.Status)
	}
	if report.Steps[0].Status != StepStatusFailed {
		t.Errorf("step a should be failed, got %q", report.Steps[0].Status)
	}
	if report.Steps[0].Error != "boom" {
		t.Errorf("step a error should be 'boom', got %q", report.Steps[0].Error)
	}
	if report.Steps[0].CompletedAt == nil {
		t.Error("failed step should have CompletedAt set")
	}
}

func TestRetryAfterFailure(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "retry-test",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "retry-test")

	engine.StartStep(ctx, uuid, "a")
	engine.FailStep(ctx, uuid, "a", errors.New("first attempt"))

	// Retry: start the failed step again
	if err := engine.StartStep(ctx, uuid, "a"); err != nil {
		t.Fatalf("retry StartStep failed: %v", err)
	}
	report, _ := engine.GetStatus(ctx, uuid)
	if report.Status != WorkflowStatusRunning {
		t.Errorf("workflow should be running after retry, got %q", report.Status)
	}
	if report.Steps[0].Status != StepStatusRunning {
		t.Errorf("step a should be running after retry, got %q", report.Steps[0].Status)
	}
	if report.Steps[0].Error != "" {
		t.Errorf("error should be cleared after retry, got %q", report.Steps[0].Error)
	}

	engine.CompleteStep(ctx, uuid, "a")
	engine.StartStep(ctx, uuid, "b")
	engine.CompleteStep(ctx, uuid, "b")

	report, _ = engine.GetStatus(ctx, uuid)
	if report.Status != WorkflowStatusCompleted {
		t.Errorf("workflow should be completed after retry, got %q", report.Status)
	}
}

// --- Non-Sequential Execution ---

func TestNonSequentialStepStart(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "non-seq",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "non-seq")

	// Skip a, start c directly (b stays pending)
	engine.SkipStep(ctx, uuid, "a")
	if err := engine.StartStep(ctx, uuid, "c"); err != nil {
		t.Fatalf("starting step c out of order should succeed: %v", err)
	}

	report, _ := engine.GetStatus(ctx, uuid)
	if report.Steps[0].Status != StepStatusSkipped {
		t.Errorf("a should be skipped, got %q", report.Steps[0].Status)
	}
	if report.Steps[1].Status != StepStatusPending {
		t.Errorf("b should still be pending, got %q", report.Steps[1].Status)
	}
	if report.Steps[2].Status != StepStatusRunning {
		t.Errorf("c should be running, got %q", report.Steps[2].Status)
	}
}

// --- Invalid Transitions ---

func TestStartStepInvalidStatus(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "inv",
		Steps: []StepDefinition{{Name: "a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "inv")

	engine.StartStep(ctx, uuid, "a")
	// Starting an already running step should fail
	if err := engine.StartStep(ctx, uuid, "a"); err == nil {
		t.Fatal("starting a running step should fail")
	}
}

func TestCompleteStepNotRunning(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "inv",
		Steps: []StepDefinition{{Name: "a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "inv")

	// Completing a pending step should fail
	if err := engine.CompleteStep(ctx, uuid, "a"); err == nil {
		t.Fatal("completing a pending step should fail")
	}
}

func TestFailStepNotRunning(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "inv",
		Steps: []StepDefinition{{Name: "a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "inv")

	if err := engine.FailStep(ctx, uuid, "a", errors.New("nope")); err == nil {
		t.Fatal("failing a pending step should fail")
	}
}

func TestSkipStepNotPending(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "inv",
		Steps: []StepDefinition{{Name: "a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "inv")

	engine.StartStep(ctx, uuid, "a")
	if err := engine.SkipStep(ctx, uuid, "a"); err == nil {
		t.Fatal("skipping a running step should fail")
	}
}

func TestInvalidStepName(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "inv",
		Steps: []StepDefinition{{Name: "a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "inv")

	if err := engine.StartStep(ctx, uuid, "nonexistent"); err == nil {
		t.Fatal("expected error for invalid step name")
	}
}

// --- Child Workflows ---

func TestChildWorkflow(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	parentUUID, _ := engine.StartWorkflow(ctx, "migration")
	engine.StartStep(ctx, parentUUID, "schema-migrate")

	childUUID, err := engine.StartChildWorkflow(ctx, parentUUID, "schema-migrate-flow")
	if err != nil {
		t.Fatalf("StartChildWorkflow failed: %v", err)
	}
	if childUUID == "" {
		t.Fatal("expected non-empty child UUID")
	}

	// Complete all child steps
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		engine.StartStep(ctx, childUUID, step)
		engine.CompleteStep(ctx, childUUID, step)
	}

	// Child workflow should be completed
	childReport, _ := engine.GetStatus(ctx, childUUID)
	if childReport.Status != WorkflowStatusCompleted {
		t.Errorf("child workflow should be completed, got %q", childReport.Status)
	}

	// Parent step is still running; complete it
	engine.CompleteStep(ctx, parentUUID, "schema-migrate")

	parentReport, _ := engine.GetStatus(ctx, parentUUID)
	if parentReport.Steps[1].Status != StepStatusCompleted {
		t.Errorf("schema-migrate should be completed, got %q", parentReport.Steps[1].Status)
	}
}

func TestChildWorkflowInvalidParent(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	_, err := engine.StartChildWorkflow(ctx, "nonexistent-uuid", "schema-migrate-flow")
	if err == nil {
		t.Fatal("expected error for nonexistent parent UUID")
	}
}

func TestChildWorkflowNoMatchingStep(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migration")
	// "schema-migrate-flow" is a valid workflow but "migration" has no step
	// with SubWorkflowName="migration" itself
	_, err := engine.StartChildWorkflow(ctx, uuid, "migration")
	if err == nil {
		t.Fatal("expected error for no matching SubWorkflowName")
	}
}

func TestMultipleChildInstances(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	parentUUID, _ := engine.StartWorkflow(ctx, "migration")
	engine.StartStep(ctx, parentUUID, "schema-migrate")

	// First child attempt: fails
	child1UUID, _ := engine.StartChildWorkflow(ctx, parentUUID, "schema-migrate-flow")
	engine.StartStep(ctx, child1UUID, "export-schema")
	engine.FailStep(ctx, child1UUID, "export-schema", errors.New("network error"))

	// Second child attempt: succeeds
	child2UUID, _ := engine.StartChildWorkflow(ctx, parentUUID, "schema-migrate-flow")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		engine.StartStep(ctx, child2UUID, step)
		engine.CompleteStep(ctx, child2UUID, step)
	}

	// GetStatus should show both child instances
	report, err := engine.GetStatus(ctx, parentUUID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	schemaMigrateStep := report.Steps[1]
	if len(schemaMigrateStep.ChildReports) != 2 {
		t.Fatalf("expected 2 child reports, got %d", len(schemaMigrateStep.ChildReports))
	}
	if schemaMigrateStep.ChildReports[0].Status != WorkflowStatusFailed {
		t.Errorf("first child should be failed, got %q", schemaMigrateStep.ChildReports[0].Status)
	}
	if schemaMigrateStep.ChildReports[1].Status != WorkflowStatusCompleted {
		t.Errorf("second child should be completed, got %q", schemaMigrateStep.ChildReports[1].Status)
	}
}

// --- Recursive Status ---

func TestRecursiveStatus(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	parentUUID, _ := engine.StartWorkflow(ctx, "migration")

	// Complete assess
	engine.StartStep(ctx, parentUUID, "assess")
	engine.CompleteStep(ctx, parentUUID, "assess")

	// Start schema-migrate with a child workflow
	engine.StartStep(ctx, parentUUID, "schema-migrate")
	childUUID, _ := engine.StartChildWorkflow(ctx, parentUUID, "schema-migrate-flow")
	engine.StartStep(ctx, childUUID, "export-schema")
	engine.CompleteStep(ctx, childUUID, "export-schema")
	engine.StartStep(ctx, childUUID, "analyze-schema")

	report, err := engine.GetStatus(ctx, parentUUID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if report.WorkflowName != "migration" {
		t.Errorf("expected workflow name 'migration', got %q", report.WorkflowName)
	}

	// assess: completed
	if report.Steps[0].Status != StepStatusCompleted {
		t.Errorf("assess should be completed")
	}

	// schema-migrate: running, with one child
	if report.Steps[1].Status != StepStatusRunning {
		t.Errorf("schema-migrate should be running")
	}
	if len(report.Steps[1].ChildReports) != 1 {
		t.Fatalf("expected 1 child report, got %d", len(report.Steps[1].ChildReports))
	}

	childReport := report.Steps[1].ChildReports[0]
	if childReport.WorkflowName != "schema-migrate-flow" {
		t.Errorf("child workflow name mismatch: %q", childReport.WorkflowName)
	}
	if childReport.Steps[0].Status != StepStatusCompleted {
		t.Errorf("export-schema should be completed")
	}
	if childReport.Steps[1].Status != StepStatusRunning {
		t.Errorf("analyze-schema should be running")
	}
	if childReport.Steps[2].Status != StepStatusPending {
		t.Errorf("import-schema should be pending")
	}

	// data-migrate: pending, no children
	if report.Steps[2].Status != StepStatusPending {
		t.Errorf("data-migrate should be pending")
	}
	if len(report.Steps[2].ChildReports) != 0 {
		t.Errorf("data-migrate should have no child reports")
	}
}

// --- GetNextPendingStep ---

func TestGetNextPendingStep(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "next-test",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "next-test")

	// All pending: first step
	next, ok, err := engine.GetNextPendingStep(ctx, uuid)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || next != "a" {
		t.Errorf("expected 'a', got %q (ok=%v)", next, ok)
	}

	// Complete a, skip b: next should be c
	engine.StartStep(ctx, uuid, "a")
	engine.CompleteStep(ctx, uuid, "a")
	engine.SkipStep(ctx, uuid, "b")
	next, ok, _ = engine.GetNextPendingStep(ctx, uuid)
	if !ok || next != "c" {
		t.Errorf("expected 'c', got %q (ok=%v)", next, ok)
	}

	// Complete c and d: no more pending
	engine.StartStep(ctx, uuid, "c")
	engine.CompleteStep(ctx, uuid, "c")
	engine.StartStep(ctx, uuid, "d")
	engine.CompleteStep(ctx, uuid, "d")
	next, ok, _ = engine.GetNextPendingStep(ctx, uuid)
	if ok {
		t.Errorf("expected no next step, got %q", next)
	}
}

func TestGetNextPendingStepReturnsFailed(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "failed-next",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "failed-next")

	engine.StartStep(ctx, uuid, "a")
	engine.FailStep(ctx, uuid, "a", errors.New("oops"))

	next, ok, _ := engine.GetNextPendingStep(ctx, uuid)
	if !ok || next != "a" {
		t.Errorf("expected failed step 'a' as next, got %q (ok=%v)", next, ok)
	}
}
