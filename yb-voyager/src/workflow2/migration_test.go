//go:build unit

package workflow2

import (
	"context"
	"database/sql"
	"fmt"
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

// --- Shared workflow definitions ---

func buildSchemaMigrate() WorkflowDefinition {
	return NewBuilder("schema-migrate").
		AddStep("export-schema").
		AddStep("analyze-schema", DependsOn("export-schema")).
		AddStep("import-schema", DependsOn("analyze-schema")).
		MustBuild()
}

func buildDataOffline() WorkflowDefinition {
	return NewBuilder("data-offline").
		AddStep("export-data").
		AddStep("import-data", DependsOn("export-data")).
		AddStep("finalize-schema", DependsOn("import-data")).
		MustBuild()
}

func buildDataLive() WorkflowDefinition {
	return NewBuilder("data-live").
		AddStep("export-data").
		AddStep("import-data").
		AddStep("archive-changes").
		MustBuild()
}

func buildDataLiveFF() WorkflowDefinition {
	return NewBuilder("data-live-ff").
		AddStep("export-data").
		AddStep("import-data-to-target").
		AddStep("import-data-to-source-replica").
		AddStep("archive-changes").
		MustBuild()
}

func buildReverseSyncFlow() WorkflowDefinition {
	return NewBuilder("reverse-sync-flow").
		AddStep("export-data-from-target").
		AddStep("import-data-to-source").
		MustBuild()
}

func buildForwardSyncFlow() WorkflowDefinition {
	return NewBuilder("forward-sync-flow").
		AddStep("export-data-from-target").
		AddStep("import-data-to-source-replica").
		MustBuild()
}

func buildTopLevel() WorkflowDefinition {
	return NewBuilder("top-level-migration").
		AddStep("assess-migration").
		AddStep("migrate", DependsOn("assess-migration"),
			ChildWorkflows("migrate-offline", "migrate-live", "migrate-live-fb", "migrate-live-ff")).
		MustBuild()
}

func buildMigrateOffline() WorkflowDefinition {
	return NewBuilder("migrate-offline").
		AddStep("migrate-schema", ChildWorkflows("schema-migrate")).
		AddStep("migrate-data", DependsOn("migrate-schema"), ChildWorkflows("data-offline")).
		AddStep("validate").
		AddStep("end-migration").
		MustBuild()
}

func buildMigrateLive() WorkflowDefinition {
	return NewBuilder("migrate-live").
		AddStep("migrate-schema", ChildWorkflows("schema-migrate")).
		AddStep("migrate-data", DependsOn("migrate-schema"), ChildWorkflows("data-live")).
		AddStep("validate").
		AddStep("end-migration").
		MustBuild()
}

func buildMigrateLiveFB() WorkflowDefinition {
	return NewBuilder("migrate-live-fb").
		AddStep("migrate-schema", ChildWorkflows("schema-migrate")).
		AddStep("migrate-data", DependsOn("migrate-schema"), ChildWorkflows("data-live")).
		AddStep("reverse-sync", DependsOn("migrate-data"), ChildWorkflows("reverse-sync-flow")).
		AddStep("validate").
		AddStep("end-migration").
		MustBuild()
}

func buildMigrateLiveFF() WorkflowDefinition {
	return NewBuilder("migrate-live-ff").
		AddStep("migrate-schema", ChildWorkflows("schema-migrate")).
		AddStep("migrate-data", DependsOn("migrate-schema"), ChildWorkflows("data-live-ff")).
		AddStep("forward-sync", DependsOn("migrate-data"), ChildWorkflows("forward-sync-flow")).
		AddStep("validate").
		AddStep("end-migration").
		MustBuild()
}

func registerAllWorkflows(t *testing.T, engine *WorkflowEngine) {
	t.Helper()
	defs := []WorkflowDefinition{
		buildSchemaMigrate(),
		buildDataOffline(),
		buildDataLive(),
		buildDataLiveFF(),
		buildReverseSyncFlow(),
		buildForwardSyncFlow(),
		buildTopLevel(),
		buildMigrateOffline(),
		buildMigrateLive(),
		buildMigrateLiveFB(),
		buildMigrateLiveFF(),
	}
	for _, def := range defs {
		if err := engine.RegisterWorkflow(def); err != nil {
			t.Fatalf("RegisterWorkflow (%s): %v", def.Name, err)
		}
	}
}

func mustStartComplete(t *testing.T, engine *WorkflowEngine, ctx context.Context, workflowUUID, step string) {
	t.Helper()
	if err := engine.StartStep(ctx, workflowUUID, step); err != nil {
		t.Fatalf("StartStep %q: %v", step, err)
	}
	if err := engine.CompleteStep(ctx, workflowUUID, step); err != nil {
		t.Fatalf("CompleteStep %q: %v", step, err)
	}
}

// ==========================================
// Offline migration
// ==========================================

func TestOfflineMigrationDefinitionGraph(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)

	graph, err := engine.GetWorkflowGraph("migrate-offline")
	if err != nil {
		t.Fatalf("GetWorkflowGraph: %v", err)
	}
	fmt.Println("=== Offline Migration Definition ===")
	fmt.Println(graph.PPrint())
	fmt.Println()

	if graph.Definition.Name != "migrate-offline" {
		t.Fatalf("root name: got %q", graph.Definition.Name)
	}
	if len(graph.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(graph.Steps))
	}
	if len(graph.Steps[0].ChildWorkflows) != 1 {
		t.Fatalf("migrate-schema should have 1 child workflow")
	}
	if graph.Steps[0].ChildWorkflows[0].Definition.Name != "schema-migrate" {
		t.Fatalf("migrate-schema child: got %q", graph.Steps[0].ChildWorkflows[0].Definition.Name)
	}
	if len(graph.Steps[1].ChildWorkflows) != 1 {
		t.Fatalf("migrate-data should have 1 child workflow")
	}
	if graph.Steps[1].ChildWorkflows[0].Definition.Name != "data-offline" {
		t.Fatalf("migrate-data child: got %q", graph.Steps[1].ChildWorkflows[0].Definition.Name)
	}
}

func TestOfflineMigrationHappyPath(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)
	ctx := context.Background()

	// Start top-level
	rootUUID, err := engine.StartWorkflow(ctx, "top-level-migration")
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}
	mustStartComplete(t, engine, ctx, rootUUID, "assess-migration")

	// Choose offline migration
	offlineUUID, err := engine.StartChildWorkflow(ctx, rootUUID, "migrate", "migrate-offline")
	if err != nil {
		t.Fatalf("StartChildWorkflow migrate-offline: %v", err)
	}

	// Schema migration sub-workflow
	schemaUUID, err := engine.StartChildWorkflow(ctx, offlineUUID, "migrate-schema", "schema-migrate")
	if err != nil {
		t.Fatalf("StartChildWorkflow schema-migrate: %v", err)
	}
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	// Check that GetReadySteps now shows migrate-data (deps met)
	ready, err := engine.GetReadySteps(ctx, offlineUUID)
	if err != nil {
		t.Fatalf("GetReadySteps: %v", err)
	}
	assertContains(t, ready, "migrate-data")
	assertContains(t, ready, "validate")
	assertContains(t, ready, "end-migration")

	// Data migration sub-workflow
	dataUUID, err := engine.StartChildWorkflow(ctx, offlineUUID, "migrate-data", "data-offline")
	if err != nil {
		t.Fatalf("StartChildWorkflow data-offline: %v", err)
	}
	mustStartComplete(t, engine, ctx, dataUUID, "export-data")
	mustStartComplete(t, engine, ctx, dataUUID, "import-data")
	mustStartComplete(t, engine, ctx, dataUUID, "finalize-schema")

	// Validate and end
	mustStartComplete(t, engine, ctx, offlineUUID, "validate")
	mustStartComplete(t, engine, ctx, offlineUUID, "end-migration")

	// Print final status
	report, err := engine.GetStatus(ctx, rootUUID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	fmt.Println("=== Offline Migration Final Status ===")
	fmt.Println(report.PPrint())
	fmt.Println()

	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q, want completed", report.Status)
	}
}

// ==========================================
// Live migration
// ==========================================

func TestLiveMigrationDefinitionGraph(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)

	graph, err := engine.GetWorkflowGraph("migrate-live")
	if err != nil {
		t.Fatalf("GetWorkflowGraph: %v", err)
	}
	fmt.Println("=== Live Migration Definition ===")
	fmt.Println(graph.PPrint())
	fmt.Println()

	if len(graph.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(graph.Steps))
	}
	dataLive := graph.Steps[1].ChildWorkflows[0]
	if len(dataLive.Steps) != 3 {
		t.Fatalf("data-live should have 3 steps, got %d", len(dataLive.Steps))
	}
	// All three data-live steps should have no DependsOn (parallel)
	for _, s := range dataLive.Steps {
		if len(s.Step.DependsOn) != 0 {
			t.Errorf("data-live step %q should have no deps, got %v", s.Step.Name, s.Step.DependsOn)
		}
	}
}

func TestLiveMigrationHappyPath(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, _ := engine.StartWorkflow(ctx, "top-level-migration")
	mustStartComplete(t, engine, ctx, rootUUID, "assess-migration")

	liveUUID, err := engine.StartChildWorkflow(ctx, rootUUID, "migrate", "migrate-live")
	if err != nil {
		t.Fatalf("StartChildWorkflow migrate-live: %v", err)
	}

	// Schema
	schemaUUID, _ := engine.StartChildWorkflow(ctx, liveUUID, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	// Data live — all three start in parallel
	dataUUID, _ := engine.StartChildWorkflow(ctx, liveUUID, "migrate-data", "data-live")

	// GetReadySteps on the data sub-workflow: all three should be ready (no deps)
	ready, _ := engine.GetReadySteps(ctx, dataUUID)
	assertContains(t, ready, "export-data")
	assertContains(t, ready, "import-data")
	assertContains(t, ready, "archive-changes")

	// Start all three (parallel)
	for _, step := range []string{"export-data", "import-data", "archive-changes"} {
		if err := engine.StartStep(ctx, dataUUID, step); err != nil {
			t.Fatalf("StartStep %s: %v", step, err)
		}
	}

	// Print mid-migration status
	midReport, _ := engine.GetStatus(ctx, liveUUID)
	fmt.Println("=== Live Migration Mid-Status (data streaming) ===")
	fmt.Println(midReport.PPrint())
	fmt.Println()

	// Simulate cutover: all three complete
	for _, step := range []string{"export-data", "import-data", "archive-changes"} {
		if err := engine.CompleteStep(ctx, dataUUID, step); err != nil {
			t.Fatalf("CompleteStep %s: %v", step, err)
		}
	}

	// migrate-data should auto-complete via child propagation
	offlineReport, _ := engine.GetStatus(ctx, liveUUID)
	if offlineReport.Steps[1].Status != StepStatusCompleted {
		t.Fatalf("migrate-data should be completed, got %q", offlineReport.Steps[1].Status)
	}

	mustStartComplete(t, engine, ctx, liveUUID, "validate")
	mustStartComplete(t, engine, ctx, liveUUID, "end-migration")

	report, _ := engine.GetStatus(ctx, rootUUID)
	fmt.Println("=== Live Migration Final Status ===")
	fmt.Println(report.PPrint())
	fmt.Println()

	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q, want completed", report.Status)
	}
}

func TestLiveMigrationSkipArchive(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, _ := engine.StartWorkflow(ctx, "top-level-migration")
	mustStartComplete(t, engine, ctx, rootUUID, "assess-migration")

	liveUUID, _ := engine.StartChildWorkflow(ctx, rootUUID, "migrate", "migrate-live")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, liveUUID, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, liveUUID, "migrate-data", "data-live")
	mustStartComplete(t, engine, ctx, dataUUID, "export-data")
	mustStartComplete(t, engine, ctx, dataUUID, "import-data")
	if err := engine.SkipStep(ctx, dataUUID, "archive-changes"); err != nil {
		t.Fatalf("SkipStep archive-changes: %v", err)
	}

	mustStartComplete(t, engine, ctx, liveUUID, "validate")
	mustStartComplete(t, engine, ctx, liveUUID, "end-migration")

	report, _ := engine.GetStatus(ctx, rootUUID)
	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q, want completed", report.Status)
	}
}

// ==========================================
// Live migration with fall-back
// ==========================================

func TestLiveFBMigrationDefinitionGraph(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)

	graph, err := engine.GetWorkflowGraph("migrate-live-fb")
	if err != nil {
		t.Fatalf("GetWorkflowGraph: %v", err)
	}
	fmt.Println("=== Live Fall-Back Migration Definition ===")
	fmt.Println(graph.PPrint())
	fmt.Println()

	if len(graph.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(graph.Steps))
	}
	if graph.Steps[2].Step.Name != "reverse-sync" {
		t.Fatalf("expected reverse-sync as 3rd step, got %q", graph.Steps[2].Step.Name)
	}
	if len(graph.Steps[2].Step.DependsOn) != 1 || graph.Steps[2].Step.DependsOn[0] != "migrate-data" {
		t.Fatalf("reverse-sync should depend on migrate-data, got %v", graph.Steps[2].Step.DependsOn)
	}
}

func TestLiveFBMigrationHappyPath(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, _ := engine.StartWorkflow(ctx, "top-level-migration")
	mustStartComplete(t, engine, ctx, rootUUID, "assess-migration")

	fbUUID, _ := engine.StartChildWorkflow(ctx, rootUUID, "migrate", "migrate-live-fb")

	// Schema
	schemaUUID, _ := engine.StartChildWorkflow(ctx, fbUUID, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	// Forward data (source → target)
	dataUUID, _ := engine.StartChildWorkflow(ctx, fbUUID, "migrate-data", "data-live")
	for _, step := range []string{"export-data", "import-data", "archive-changes"} {
		if err := engine.StartStep(ctx, dataUUID, step); err != nil {
			t.Fatalf("StartStep %s: %v", step, err)
		}
	}
	for _, step := range []string{"export-data", "import-data", "archive-changes"} {
		if err := engine.CompleteStep(ctx, dataUUID, step); err != nil {
			t.Fatalf("CompleteStep %s: %v", step, err)
		}
	}

	// After cutover-to-target: reverse sync starts
	reverseUUID, err := engine.StartChildWorkflow(ctx, fbUUID, "reverse-sync", "reverse-sync-flow")
	if err != nil {
		t.Fatalf("StartChildWorkflow reverse-sync: %v", err)
	}

	// Both reverse sync steps run in parallel
	for _, step := range []string{"export-data-from-target", "import-data-to-source"} {
		if err := engine.StartStep(ctx, reverseUUID, step); err != nil {
			t.Fatalf("StartStep %s: %v", step, err)
		}
	}

	// Print mid-status (reverse sync running)
	midReport, _ := engine.GetStatus(ctx, fbUUID)
	fmt.Println("=== Live Fall-Back Mid-Status (reverse sync running) ===")
	fmt.Println(midReport.PPrint())
	fmt.Println()

	// Cutover to source: complete reverse sync
	for _, step := range []string{"export-data-from-target", "import-data-to-source"} {
		if err := engine.CompleteStep(ctx, reverseUUID, step); err != nil {
			t.Fatalf("CompleteStep %s: %v", step, err)
		}
	}

	mustStartComplete(t, engine, ctx, fbUUID, "validate")
	mustStartComplete(t, engine, ctx, fbUUID, "end-migration")

	report, _ := engine.GetStatus(ctx, rootUUID)
	fmt.Println("=== Live Fall-Back Final Status ===")
	fmt.Println(report.PPrint())
	fmt.Println()

	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q, want completed", report.Status)
	}
}

func TestLiveFBMigrationSkipCutoverToSource(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, _ := engine.StartWorkflow(ctx, "top-level-migration")
	mustStartComplete(t, engine, ctx, rootUUID, "assess-migration")

	fbUUID, _ := engine.StartChildWorkflow(ctx, rootUUID, "migrate", "migrate-live-fb")

	schemaUUID, _ := engine.StartChildWorkflow(ctx, fbUUID, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, fbUUID, "migrate-data", "data-live")
	for _, step := range []string{"export-data", "import-data", "archive-changes"} {
		mustStartComplete(t, engine, ctx, dataUUID, step)
	}

	// Skip reverse-sync entirely (user decided YB is fine)
	if err := engine.SkipStep(ctx, fbUUID, "reverse-sync"); err != nil {
		t.Fatalf("SkipStep reverse-sync: %v", err)
	}

	mustStartComplete(t, engine, ctx, fbUUID, "validate")
	mustStartComplete(t, engine, ctx, fbUUID, "end-migration")

	report, _ := engine.GetStatus(ctx, rootUUID)
	fmt.Println("=== Live Fall-Back (skip reverse-sync) Final Status ===")
	fmt.Println(report.PPrint())
	fmt.Println()

	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q, want completed", report.Status)
	}
}

// ==========================================
// Live migration with fall-forward
// ==========================================

func TestLiveFFMigrationDefinitionGraph(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)

	graph, err := engine.GetWorkflowGraph("migrate-live-ff")
	if err != nil {
		t.Fatalf("GetWorkflowGraph: %v", err)
	}
	fmt.Println("=== Live Fall-Forward Migration Definition ===")
	fmt.Println(graph.PPrint())
	fmt.Println()

	if len(graph.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(graph.Steps))
	}
	dataFF := graph.Steps[1].ChildWorkflows[0]
	if len(dataFF.Steps) != 4 {
		t.Fatalf("data-live-ff should have 4 steps, got %d", len(dataFF.Steps))
	}
	fwdSync := graph.Steps[2].ChildWorkflows[0]
	if len(fwdSync.Steps) != 2 {
		t.Fatalf("forward-sync-flow should have 2 steps, got %d", len(fwdSync.Steps))
	}
}

func TestLiveFFMigrationHappyPath(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, _ := engine.StartWorkflow(ctx, "top-level-migration")
	mustStartComplete(t, engine, ctx, rootUUID, "assess-migration")

	ffUUID, _ := engine.StartChildWorkflow(ctx, rootUUID, "migrate", "migrate-live-ff")

	// Schema
	schemaUUID, _ := engine.StartChildWorkflow(ctx, ffUUID, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	// Data live FF — 4 parallel steps
	dataUUID, _ := engine.StartChildWorkflow(ctx, ffUUID, "migrate-data", "data-live-ff")
	ready, _ := engine.GetReadySteps(ctx, dataUUID)
	if len(ready) != 4 {
		t.Fatalf("expected 4 ready steps in data-live-ff, got %d: %v", len(ready), ready)
	}

	for _, step := range []string{"export-data", "import-data-to-target", "import-data-to-source-replica", "archive-changes"} {
		if err := engine.StartStep(ctx, dataUUID, step); err != nil {
			t.Fatalf("StartStep %s: %v", step, err)
		}
	}

	// Print mid-status
	midReport, _ := engine.GetStatus(ctx, ffUUID)
	fmt.Println("=== Live Fall-Forward Mid-Status (data streaming) ===")
	fmt.Println(midReport.PPrint())
	fmt.Println()

	// Cutover to target: data steps drain
	for _, step := range []string{"export-data", "import-data-to-target", "import-data-to-source-replica", "archive-changes"} {
		if err := engine.CompleteStep(ctx, dataUUID, step); err != nil {
			t.Fatalf("CompleteStep %s: %v", step, err)
		}
	}

	// Forward sync: target → source-replica
	fwdUUID, err := engine.StartChildWorkflow(ctx, ffUUID, "forward-sync", "forward-sync-flow")
	if err != nil {
		t.Fatalf("StartChildWorkflow forward-sync: %v", err)
	}
	for _, step := range []string{"export-data-from-target", "import-data-to-source-replica"} {
		if err := engine.StartStep(ctx, fwdUUID, step); err != nil {
			t.Fatalf("StartStep %s: %v", step, err)
		}
	}
	for _, step := range []string{"export-data-from-target", "import-data-to-source-replica"} {
		if err := engine.CompleteStep(ctx, fwdUUID, step); err != nil {
			t.Fatalf("CompleteStep %s: %v", step, err)
		}
	}

	mustStartComplete(t, engine, ctx, ffUUID, "validate")
	mustStartComplete(t, engine, ctx, ffUUID, "end-migration")

	report, _ := engine.GetStatus(ctx, rootUUID)
	fmt.Println("=== Live Fall-Forward Final Status ===")
	fmt.Println(report.PPrint())
	fmt.Println()

	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q, want completed", report.Status)
	}
}

// ==========================================
// Top-level definition graph (all options)
// ==========================================

func TestTopLevelDefinitionGraph(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)

	graph, err := engine.GetWorkflowGraph("top-level-migration")
	if err != nil {
		t.Fatalf("GetWorkflowGraph: %v", err)
	}
	fmt.Println("=== Top-Level Migration Definition (all options) ===")
	fmt.Println(graph.PPrint())
	fmt.Println()

	if len(graph.Steps) != 2 {
		t.Fatalf("expected 2 top-level steps, got %d", len(graph.Steps))
	}
	migrateStep := graph.Steps[1]
	if len(migrateStep.ChildWorkflows) != 4 {
		t.Fatalf("migrate should have 4 child workflow options, got %d", len(migrateStep.ChildWorkflows))
	}
}

// ==========================================
// GetReadySteps tests
// ==========================================

func TestGetReadyStepsDAG(t *testing.T) {
	engine := setupTestEngine(t)
	registerAllWorkflows(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-live-fb")

	// Initially: migrate-schema (no deps), validate (no deps), end-migration (no deps)
	// reverse-sync depends on migrate-data (pending), migrate-data depends on migrate-schema (pending)
	ready, _ := engine.GetReadySteps(ctx, uuid)
	assertContains(t, ready, "migrate-schema")
	assertContains(t, ready, "validate")
	assertContains(t, ready, "end-migration")
	assertNotContains(t, ready, "migrate-data")
	assertNotContains(t, ready, "reverse-sync")

	// Complete migrate-schema: migrate-data becomes ready
	mustStartComplete(t, engine, ctx, uuid, "migrate-schema")
	ready, _ = engine.GetReadySteps(ctx, uuid)
	assertContains(t, ready, "migrate-data")
	assertNotContains(t, ready, "reverse-sync")

	// Complete migrate-data: reverse-sync becomes ready
	mustStartComplete(t, engine, ctx, uuid, "migrate-data")
	ready, _ = engine.GetReadySteps(ctx, uuid)
	assertContains(t, ready, "reverse-sync")
}

// ==========================================
// DAG validation tests
// ==========================================

func TestBuildCycleDetection(t *testing.T) {
	_, err := NewBuilder("cycle").
		AddStep("a", DependsOn("b")).
		AddStep("b", DependsOn("a")).
		Build()
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestBuildSelfDependency(t *testing.T) {
	_, err := NewBuilder("self-dep").
		AddStep("a", DependsOn("a")).
		Build()
	if err == nil {
		t.Fatal("expected self-dependency error")
	}
}

func TestBuildUnknownDependency(t *testing.T) {
	_, err := NewBuilder("bad-dep").
		AddStep("a", DependsOn("nonexistent")).
		Build()
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
}

func TestBuildDuplicateStep(t *testing.T) {
	_, err := NewBuilder("dup").
		AddStep("a").
		AddStep("a").
		Build()
	if err == nil {
		t.Fatal("expected duplicate step error")
	}
}

// --- test helpers ---

func assertContains(t *testing.T, slice []string, item string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			return
		}
	}
	t.Errorf("expected %v to contain %q", slice, item)
}

func assertNotContains(t *testing.T, slice []string, item string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			t.Errorf("expected %v to NOT contain %q", slice, item)
			return
		}
	}
}
