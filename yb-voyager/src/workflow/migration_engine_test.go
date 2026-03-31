//go:build unit

package workflow

import (
	"context"
	"fmt"
	"testing"
)

// Top level is constant (assess → migrate). The migrate step spawns a pluggable sub-workflow
// (e.g. migrate-live-basic, migrate-live-fall-back — see registerLiveFallBackMigrationWorkflows).

const (
	wfTopLevelMigration   = "top-level-migration"
	wfMigrateLiveBasic    = "migrate-live-basic"
	wfSchemaMigrate       = "schema-migrate-flow"
	wfDataLiveBasic       = "data-live-basic"
	wfCutoverToTarget     = "cutover-to-target-flow"
	wfTopLevelMigrationFB = "top-level-migration-fallback"
	wfMigrateLiveFallBack = "migrate-live-fall-back"
	wfDataFallBackReverse = "data-fall-back-reverse-sync"
	wfCutoverToSource     = "cutover-to-source-flow"
)

// registerLiveMigrationWorkflows registers:
//   - top-level-migration: assess → migrate (SubWorkflowOptions: migrate-live-basic, …)
//   - migrate-live-basic: schema → data → cutover → validate → end (basic live path)
//   - schema-migrate-flow, data-live-basic, cutover-to-target-flow: unchanged leaves
func registerLiveMigrationWorkflows(t *testing.T, engine *WorkflowEngine) {
	t.Helper()

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfTopLevelMigration,
		Steps: []StepDefinition{
			{Name: "assess"},
			{Name: "migrate", SubWorkflowOptions: []string{wfMigrateLiveBasic}},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfTopLevelMigration, err)
	}

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfMigrateLiveBasic,
		Steps: []StepDefinition{
			{Name: "migrate-schema", SubWorkflowOptions: []string{wfSchemaMigrate}},
			{Name: "migrate-data", SubWorkflowOptions: []string{wfDataLiveBasic}},
			{Name: "cutover-to-target", SubWorkflowOptions: []string{wfCutoverToTarget}},
			{Name: "validate-migration"},
			{Name: "end-migration"},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfMigrateLiveBasic, err)
	}

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfCutoverToTarget,
		Steps: []StepDefinition{
			{Name: "initiate-cutover-to-target"},
			{Name: "wait-cutover-complete"},
			{Name: "finalize-schema-post-data-import"},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfCutoverToTarget, err)
	}

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfSchemaMigrate,
		Steps: []StepDefinition{
			{Name: "export-schema"},
			{Name: "analyze-schema"},
			{Name: "import-schema"},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfSchemaMigrate, err)
	}

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfDataLiveBasic,
		Steps: []StepDefinition{
			{Name: "export-data-from-source"},
			{Name: "import-data-to-target"},
			{Name: "archive-changes"},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfDataLiveBasic, err)
	}
}

// registerLiveFallBackMigrationWorkflows registers a live migration with fall-back path
// (after cutover to target: export from target + import to source; optional cutover back to source).
func registerLiveFallBackMigrationWorkflows(t *testing.T, engine *WorkflowEngine) {
	t.Helper()

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfTopLevelMigrationFB,
		Steps: []StepDefinition{
			{Name: "assess"},
			{Name: "migrate", SubWorkflowOptions: []string{wfMigrateLiveFallBack}},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfTopLevelMigrationFB, err)
	}

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfMigrateLiveFallBack,
		Steps: []StepDefinition{
			{Name: "migrate-schema", SubWorkflowOptions: []string{wfSchemaMigrate}},
			{Name: "migrate-data", SubWorkflowOptions: []string{wfDataLiveBasic}},
			{Name: "cutover-to-target", SubWorkflowOptions: []string{wfCutoverToTarget}},
			{Name: "fall-back-reverse-sync", SubWorkflowOptions: []string{wfDataFallBackReverse}},
			{Name: "cutover-to-source", SubWorkflowOptions: []string{wfCutoverToSource}},
			{Name: "validate-migration"},
			{Name: "end-migration"},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfMigrateLiveFallBack, err)
	}

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfDataFallBackReverse,
		Steps: []StepDefinition{
			{Name: "export-data-from-target"},
			{Name: "import-data-to-source"},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfDataFallBackReverse, err)
	}

	if err := engine.RegisterWorkflow(WorkflowDefinition{
		Name: wfCutoverToSource,
		Steps: []StepDefinition{
			{Name: "initiate-cutover-to-source"},
			{Name: "wait-cutover-to-source-complete"},
		},
	}); err != nil {
		t.Fatalf("RegisterWorkflow (%s): %v", wfCutoverToSource, err)
	}

	// Shared leaf workflows (same names as basic live)
	for _, reg := range []struct {
		name string
		def  WorkflowDefinition
	}{
		{wfSchemaMigrate, WorkflowDefinition{Name: wfSchemaMigrate, Steps: []StepDefinition{
			{Name: "export-schema"}, {Name: "analyze-schema"}, {Name: "import-schema"},
		}}},
		{wfDataLiveBasic, WorkflowDefinition{Name: wfDataLiveBasic, Steps: []StepDefinition{
			{Name: "export-data-from-source"}, {Name: "import-data-to-target"}, {Name: "archive-changes"},
		}}},
		{wfCutoverToTarget, WorkflowDefinition{Name: wfCutoverToTarget, Steps: []StepDefinition{
			{Name: "initiate-cutover-to-target"}, {Name: "wait-cutover-complete"}, {Name: "finalize-schema-post-data-import"},
		}}},
	} {
		if err := engine.RegisterWorkflow(reg.def); err != nil {
			t.Fatalf("RegisterWorkflow (%s): %v", reg.name, err)
		}
	}
}

func TestLiveMigrationWorkflowDefinitionTree(t *testing.T) {
	engine := setupTestEngine(t)
	registerLiveMigrationWorkflows(t, engine)

	tree, err := engine.GetWorkflowTree(wfTopLevelMigration)
	if err != nil {
		t.Fatalf("GetWorkflowTree: %v", err)
	}
	fmt.Println(tree.PPrint())
	if tree.Definition.Name != wfTopLevelMigration {
		t.Fatalf("root name: got %q", tree.Definition.Name)
	}
	if len(tree.Steps) != 2 {
		t.Fatalf("expected 2 top-level steps, got %d", len(tree.Steps))
	}
	if len(tree.Steps[0].ChildWorkflows) != 0 {
		t.Fatalf("assess should have no child workflows")
	}
	if len(tree.Steps[1].ChildWorkflows) != 1 || tree.Steps[1].ChildWorkflows[0].Definition.Name != wfMigrateLiveBasic {
		t.Fatalf("migrate child: %+v", tree.Steps[1].ChildWorkflows)
	}
	migrateFlow := tree.Steps[1].ChildWorkflows[0]
	if len(migrateFlow.Steps) != 5 {
		t.Fatalf("migrate-live-basic should have 5 steps, got %d", len(migrateFlow.Steps))
	}
	if len(migrateFlow.Steps[0].ChildWorkflows) != 1 || migrateFlow.Steps[0].ChildWorkflows[0].Definition.Name != wfSchemaMigrate {
		t.Fatalf("migrate-schema child: %+v", migrateFlow.Steps[0].ChildWorkflows)
	}
	if len(migrateFlow.Steps[1].ChildWorkflows) != 1 || migrateFlow.Steps[1].ChildWorkflows[0].Definition.Name != wfDataLiveBasic {
		t.Fatalf("migrate-data child: %+v", migrateFlow.Steps[1].ChildWorkflows)
	}
	if len(migrateFlow.Steps[2].ChildWorkflows) != 1 || migrateFlow.Steps[2].ChildWorkflows[0].Definition.Name != wfCutoverToTarget {
		t.Fatalf("cutover-to-target child: %+v", migrateFlow.Steps[2].ChildWorkflows)
	}
	cutover := migrateFlow.Steps[2].ChildWorkflows[0]
	if len(cutover.Steps) != 3 {
		t.Fatalf("cutover-to-target-flow should have 3 steps, got %d", len(cutover.Steps))
	}
	live := migrateFlow.Steps[1].ChildWorkflows[0]
	if len(live.Steps) != 3 {
		t.Fatalf("data-live-basic should have 3 steps, got %d", len(live.Steps))
	}
}

// TestLiveMigrationWorkflowHappyPath runs assess on the root, then the full basic-live path inside migrate-live-basic.
func TestLiveMigrationWorkflowHappyPath(t *testing.T) {
	engine := setupTestEngine(t)
	registerLiveMigrationWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, err := engine.StartWorkflow(ctx, wfTopLevelMigration)
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}

	mustStartComplete(t, engine, ctx, rootUUID, "assess")

	migrateUUID, err := engine.StartChildWorkflow(ctx, rootUUID, wfMigrateLiveBasic)
	if err != nil {
		t.Fatalf("StartChildWorkflow migrate-live-basic: %v", err)
	}

	runSchemaAndForwardLiveDataPhase(t, engine, ctx, migrateUUID)

	cutoverChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfCutoverToTarget)
	if err != nil {
		t.Fatalf("StartChildWorkflow cutover: %v", err)
	}
	for _, step := range []string{
		"initiate-cutover-to-target",
		"wait-cutover-complete",
		"finalize-schema-post-data-import",
	} {
		mustStartComplete(t, engine, ctx, cutoverChild, step)
	}

	for _, step := range []string{"validate-migration", "end-migration"} {
		mustStartComplete(t, engine, ctx, migrateUUID, step)
	}

	migrateReport, err := engine.GetStatus(ctx, migrateUUID)
	if err != nil {
		t.Fatalf("GetStatus migrate: %v", err)
	}
	if migrateReport.Status != WorkflowStatusCompleted {
		t.Fatalf("migrate sub-workflow status = %q, want completed", migrateReport.Status)
	}

	report, err := engine.GetStatus(ctx, rootUUID)
	if err != nil {
		t.Fatalf("GetStatus root: %v", err)
	}
	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root workflow status = %q, want completed", report.Status)
	}
	for _, sr := range report.Steps {
		if sr.Status != StepStatusCompleted {
			t.Errorf("root step %q status = %q, want completed", sr.StepName, sr.Status)
		}
	}
}

func TestLiveMigrationWorkflowSkipOptionalArchive(t *testing.T) {
	engine := setupTestEngine(t)
	registerLiveMigrationWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, _ := engine.StartWorkflow(ctx, wfTopLevelMigration)
	mustStartComplete(t, engine, ctx, rootUUID, "assess")

	migrateUUID, _ := engine.StartChildWorkflow(ctx, rootUUID, wfMigrateLiveBasic)

	runSchemaAndForwardLiveDataPhaseSkipArchive(t, engine, ctx, migrateUUID)

	cutoverChild, _ := engine.StartChildWorkflow(ctx, migrateUUID, wfCutoverToTarget)
	for _, step := range []string{
		"initiate-cutover-to-target",
		"wait-cutover-complete",
		"finalize-schema-post-data-import",
	} {
		mustStartComplete(t, engine, ctx, cutoverChild, step)
	}

	for _, step := range []string{"validate-migration", "end-migration"} {
		mustStartComplete(t, engine, ctx, migrateUUID, step)
	}

	report, _ := engine.GetStatus(ctx, rootUUID)
	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q", report.Status)
	}
}

func TestLiveFallBackMigrationDefinitionTree(t *testing.T) {
	engine := setupTestEngine(t)
	registerLiveFallBackMigrationWorkflows(t, engine)

	tree, err := engine.GetWorkflowTree(wfTopLevelMigrationFB)
	if err != nil {
		t.Fatalf("GetWorkflowTree: %v", err)
	}
	fmt.Println(tree.PPrint())
	if len(tree.Steps) != 2 {
		t.Fatalf("expected 2 top-level steps, got %d", len(tree.Steps))
	}
	migrateFB := tree.Steps[1].ChildWorkflows[0]
	if migrateFB.Definition.Name != wfMigrateLiveFallBack {
		t.Fatalf("migrate child name = %q", migrateFB.Definition.Name)
	}
	if len(migrateFB.Steps) != 7 {
		t.Fatalf("migrate-live-fall-back should have 7 steps, got %d", len(migrateFB.Steps))
	}
	if migrateFB.Steps[3].ChildWorkflows[0].Definition.Name != wfDataFallBackReverse {
		t.Fatalf("fall-back-reverse-sync child: %+v", migrateFB.Steps[3].ChildWorkflows)
	}
	rev := migrateFB.Steps[3].ChildWorkflows[0]
	if len(rev.Steps) != 2 {
		t.Fatalf("reverse sync should have 2 steps, got %d", len(rev.Steps))
	}
	if migrateFB.Steps[4].ChildWorkflows[0].Definition.Name != wfCutoverToSource {
		t.Fatalf("cutover-to-source child: %+v", migrateFB.Steps[4].ChildWorkflows)
	}
}

// TestLiveFallBackMigrationHappyPathSkipCutoverToSource runs fall-back through reverse sync, skips optional cutover to source.
func TestLiveFallBackMigrationHappyPathSkipCutoverToSource(t *testing.T) {
	engine := setupTestEngine(t)
	registerLiveFallBackMigrationWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, err := engine.StartWorkflow(ctx, wfTopLevelMigrationFB)
	if err != nil {
		t.Fatalf("StartWorkflow: %v", err)
	}
	mustStartComplete(t, engine, ctx, rootUUID, "assess")

	migrateUUID, err := engine.StartChildWorkflow(ctx, rootUUID, wfMigrateLiveFallBack)
	if err != nil {
		t.Fatalf("StartChildWorkflow migrate-live-fall-back: %v", err)
	}

	runSchemaAndForwardLiveDataPhase(t, engine, ctx, migrateUUID)

	cutoverChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfCutoverToTarget)
	if err != nil {
		t.Fatalf("StartChildWorkflow cutover-to-target: %v", err)
	}
	for _, step := range []string{
		"initiate-cutover-to-target",
		"wait-cutover-complete",
		"finalize-schema-post-data-import",
	} {
		mustStartComplete(t, engine, ctx, cutoverChild, step)
	}

	reverseChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfDataFallBackReverse)
	if err != nil {
		t.Fatalf("StartChildWorkflow fall-back-reverse-sync: %v", err)
	}
	for _, step := range []string{"export-data-from-target", "import-data-to-source"} {
		if err := engine.StartStep(ctx, reverseChild, step); err != nil {
			t.Fatalf("StartStep %s: %v", step, err)
		}
	}
	for _, step := range []string{"export-data-from-target", "import-data-to-source"} {
		if err := engine.CompleteStep(ctx, reverseChild, step); err != nil {
			t.Fatalf("CompleteStep %s: %v", step, err)
		}
	}

	if err := engine.SkipStep(ctx, migrateUUID, "cutover-to-source"); err != nil {
		t.Fatalf("SkipStep cutover-to-source: %v", err)
	}

	for _, step := range []string{"validate-migration", "end-migration"} {
		mustStartComplete(t, engine, ctx, migrateUUID, step)
	}

	report, err := engine.GetStatus(ctx, rootUUID)
	if err != nil {
		t.Fatalf("GetStatus root: %v", err)
	}
	if report.Status != WorkflowStatusCompleted {
		t.Fatalf("root workflow status = %q, want completed", report.Status)
	}
}

// TestLiveFallBackMigrationHappyPathWithCutoverToSource completes optional cutover back to source.
func TestLiveFallBackMigrationHappyPathWithCutoverToSource(t *testing.T) {
	engine := setupTestEngine(t)
	registerLiveFallBackMigrationWorkflows(t, engine)
	ctx := context.Background()

	rootUUID, _ := engine.StartWorkflow(ctx, wfTopLevelMigrationFB)
	mustStartComplete(t, engine, ctx, rootUUID, "assess")

	migrateUUID, _ := engine.StartChildWorkflow(ctx, rootUUID, wfMigrateLiveFallBack)
	runSchemaAndForwardLiveDataPhase(t, engine, ctx, migrateUUID)

	cutoverTgt, _ := engine.StartChildWorkflow(ctx, migrateUUID, wfCutoverToTarget)
	for _, step := range []string{
		"initiate-cutover-to-target",
		"wait-cutover-complete",
		"finalize-schema-post-data-import",
	} {
		mustStartComplete(t, engine, ctx, cutoverTgt, step)
	}

	reverseChild, _ := engine.StartChildWorkflow(ctx, migrateUUID, wfDataFallBackReverse)
	mustStartComplete(t, engine, ctx, reverseChild, "export-data-from-target")
	mustStartComplete(t, engine, ctx, reverseChild, "import-data-to-source")

	srcChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfCutoverToSource)
	if err != nil {
		t.Fatalf("StartChildWorkflow cutover-to-source: %v", err)
	}
	mustStartComplete(t, engine, ctx, srcChild, "initiate-cutover-to-source")
	mustStartComplete(t, engine, ctx, srcChild, "wait-cutover-to-source-complete")

	for _, step := range []string{"validate-migration", "end-migration"} {
		mustStartComplete(t, engine, ctx, migrateUUID, step)
	}

	if report, _ := engine.GetStatus(ctx, rootUUID); report.Status != WorkflowStatusCompleted {
		t.Fatalf("root status = %q", report.Status)
	}
}

// runSchemaAndForwardLiveDataPhase runs shared schema + forward CDC (source → target) used by live and live fall-back.
func runSchemaAndForwardLiveDataPhase(t *testing.T, engine *WorkflowEngine, ctx context.Context, migrateUUID string) {
	t.Helper()
	schemaChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfSchemaMigrate)
	if err != nil {
		t.Fatalf("StartChildWorkflow schema: %v", err)
	}
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaChild, step)
	}

	dataChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfDataLiveBasic)
	if err != nil {
		t.Fatalf("StartChildWorkflow data: %v", err)
	}
	for _, step := range []string{"export-data-from-source", "import-data-to-target", "archive-changes"} {
		if err := engine.StartStep(ctx, dataChild, step); err != nil {
			t.Fatalf("StartStep %s: %v", step, err)
		}
	}
	for _, step := range []string{"export-data-from-source", "import-data-to-target", "archive-changes"} {
		if err := engine.CompleteStep(ctx, dataChild, step); err != nil {
			t.Fatalf("CompleteStep %s: %v", step, err)
		}
	}
}

func runSchemaAndForwardLiveDataPhaseSkipArchive(t *testing.T, engine *WorkflowEngine, ctx context.Context, migrateUUID string) {
	t.Helper()
	schemaChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfSchemaMigrate)
	if err != nil {
		t.Fatalf("StartChildWorkflow schema: %v", err)
	}
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaChild, step)
	}

	dataChild, err := engine.StartChildWorkflow(ctx, migrateUUID, wfDataLiveBasic)
	if err != nil {
		t.Fatalf("StartChildWorkflow data: %v", err)
	}
	mustStartComplete(t, engine, ctx, dataChild, "export-data-from-source")
	mustStartComplete(t, engine, ctx, dataChild, "import-data-to-target")
	if err := engine.SkipStep(ctx, dataChild, "archive-changes"); err != nil {
		t.Fatalf("SkipStep archive-changes: %v", err)
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
