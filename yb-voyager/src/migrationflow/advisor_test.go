//go:build unit

package migrationflow

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/workflow2"
)

func setupEngine(t *testing.T) *workflow2.WorkflowEngine {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	engine, err := workflow2.NewWorkflowEngine(db)
	if err != nil {
		t.Fatalf("NewWorkflowEngine: %v", err)
	}
	return engine
}

func registerAll(t *testing.T, engine *workflow2.WorkflowEngine) {
	t.Helper()
	defs := []workflow2.WorkflowDefinition{
		workflow2.NewBuilder("schema-migrate").
			AddStep("export-schema").
			AddStep("analyze-schema", workflow2.DependsOn("export-schema")).
			AddStep("import-schema", workflow2.DependsOn("analyze-schema")).
			MustBuild(),
		workflow2.NewBuilder("data-offline").
			AddStep("export-data").
			AddStep("import-data", workflow2.DependsOn("export-data")).
			AddStep("finalize-schema", workflow2.DependsOn("import-data")).
			MustBuild(),
		workflow2.NewBuilder("data-live").
			AddStep("export-data").
			AddStep("import-data").
			AddStep("archive-changes").
			MustBuild(),
		workflow2.NewBuilder("data-live-ff").
			AddStep("export-data").
			AddStep("import-data-to-target").
			AddStep("import-data-to-source-replica").
			AddStep("archive-changes").
			MustBuild(),
		workflow2.NewBuilder("reverse-sync-flow").
			AddStep("export-data-from-target").
			AddStep("import-data-to-source").
			MustBuild(),
		workflow2.NewBuilder("forward-sync-flow").
			AddStep("export-data-from-target").
			AddStep("import-data-to-source-replica").
			MustBuild(),
		workflow2.NewBuilder("migrate-offline").
			AddStep("migrate-schema", workflow2.ChildWorkflows("schema-migrate")).
			AddStep("migrate-data", workflow2.DependsOn("migrate-schema"), workflow2.ChildWorkflows("data-offline")).
			AddStep("end-migration").
			MustBuild(),
		workflow2.NewBuilder("migrate-live").
			AddStep("migrate-schema", workflow2.ChildWorkflows("schema-migrate")).
			AddStep("migrate-data", workflow2.DependsOn("migrate-schema"), workflow2.ChildWorkflows("data-live")).
			AddStep("end-migration").
			MustBuild(),
		workflow2.NewBuilder("migrate-live-fb").
			AddStep("migrate-schema", workflow2.ChildWorkflows("schema-migrate")).
			AddStep("migrate-data", workflow2.DependsOn("migrate-schema"), workflow2.ChildWorkflows("data-live")).
			AddStep("reverse-sync", workflow2.DependsOn("migrate-data"), workflow2.ChildWorkflows("reverse-sync-flow")).
			AddStep("end-migration").
			MustBuild(),
		workflow2.NewBuilder("migrate-live-ff").
			AddStep("migrate-schema", workflow2.ChildWorkflows("schema-migrate")).
			AddStep("migrate-data", workflow2.DependsOn("migrate-schema"), workflow2.ChildWorkflows("data-live-ff")).
			AddStep("forward-sync", workflow2.DependsOn("migrate-data"), workflow2.ChildWorkflows("forward-sync-flow")).
			AddStep("end-migration").
			MustBuild(),
	}
	for _, def := range defs {
		if err := engine.RegisterWorkflow(def); err != nil {
			t.Fatalf("RegisterWorkflow(%s): %v", def.Name, err)
		}
	}
}

func mustStartComplete(t *testing.T, engine *workflow2.WorkflowEngine, ctx context.Context, uuid, step string) {
	t.Helper()
	if err := engine.StartStep(ctx, uuid, step); err != nil {
		t.Fatalf("StartStep %q: %v", step, err)
	}
	if err := engine.CompleteStep(ctx, uuid, step); err != nil {
		t.Fatalf("CompleteStep %q: %v", step, err)
	}
}

// ==========================================
// Offline migration scenarios
// ==========================================

func TestOffline_SchemaDone_DataNotStarted(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-offline")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Offline: schema done, data not started ===")
	printCommands(cmds)

	// migrate-data is composite with 1 child (data-offline), so it projects
	// into the child and resolves to the first leaf: export-data.
	// end-migration is filtered out.
	assertHasCommand(t, cmds, "yb-voyager export data")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d: %v", len(cmds), cmds)
	}
}

func TestOffline_DataExportRunning(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-offline")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-data", "data-offline")
	if err := engine.StartStep(ctx, dataUUID, "export-data"); err != nil {
		t.Fatal(err)
	}

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Offline: export-data running ===")
	printCommands(cmds)

	// No cutover triggers for offline. end-migration filtered. Nothing to suggest.
	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands (wait), got %d: %v", len(cmds), cmds)
	}
}

func TestOffline_ImportDataFailed(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-offline")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-data", "data-offline")
	mustStartComplete(t, engine, ctx, dataUUID, "export-data")

	if err := engine.StartStep(ctx, dataUUID, "import-data"); err != nil {
		t.Fatal(err)
	}
	if err := engine.FailStep(ctx, dataUUID, "import-data", fmt.Errorf("connection timeout")); err != nil {
		t.Fatal(err)
	}

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Offline: import-data failed ===")
	printCommands(cmds)

	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (retry), got %d: %v", len(cmds), cmds)
	}
	assertHasCommand(t, cmds, "yb-voyager import data")
	if !cmds[0].IsRetry {
		t.Fatal("expected IsRetry=true")
	}
}

// ==========================================
// Live fall-back scenarios
// ==========================================

func TestLiveFB_DataStreaming(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-live-fb")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-data", "data-live")
	for _, step := range []string{"export-data", "import-data", "archive-changes"} {
		if err := engine.StartStep(ctx, dataUUID, step); err != nil {
			t.Fatal(err)
		}
	}

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Live-FB: data streaming ===")
	printCommands(cmds)

	assertHasCommand(t, cmds, "yb-voyager initiate cutover to target")
	assertNotHasCommand(t, cmds, "yb-voyager end migration")
	assertNotHasCommand(t, cmds, "yb-voyager initiate cutover to source")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (cutover), got %d: %v", len(cmds), cmds)
	}
}

func TestLiveFB_ReverseSyncRunning(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-live-fb")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-data", "data-live")
	for _, step := range []string{"export-data", "import-data", "archive-changes"} {
		mustStartComplete(t, engine, ctx, dataUUID, step)
	}

	reverseUUID, _ := engine.StartChildWorkflow(ctx, uuid, "reverse-sync", "reverse-sync-flow")
	for _, step := range []string{"export-data-from-target", "import-data-to-source"} {
		if err := engine.StartStep(ctx, reverseUUID, step); err != nil {
			t.Fatal(err)
		}
	}

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Live-FB: reverse sync running ===")
	printCommands(cmds)

	assertHasCommand(t, cmds, "yb-voyager initiate cutover to source")
	assertNotHasCommand(t, cmds, "yb-voyager end migration")
	assertNotHasCommand(t, cmds, "yb-voyager initiate cutover to target")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (cutover), got %d: %v", len(cmds), cmds)
	}
}

// ==========================================
// Live fall-forward scenarios
// ==========================================

func TestLiveFF_DataStreaming(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-live-ff")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-data", "data-live-ff")
	for _, step := range []string{"export-data", "import-data-to-target", "import-data-to-source-replica", "archive-changes"} {
		if err := engine.StartStep(ctx, dataUUID, step); err != nil {
			t.Fatal(err)
		}
	}

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Live-FF: data streaming ===")
	printCommands(cmds)

	assertHasCommand(t, cmds, "yb-voyager initiate cutover to target")
	assertNotHasCommand(t, cmds, "yb-voyager end migration")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (cutover), got %d: %v", len(cmds), cmds)
	}
}

func TestLiveFF_ForwardSyncRunning(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-live-ff")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}

	dataUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-data", "data-live-ff")
	for _, step := range []string{"export-data", "import-data-to-target", "import-data-to-source-replica", "archive-changes"} {
		mustStartComplete(t, engine, ctx, dataUUID, step)
	}

	fwdUUID, _ := engine.StartChildWorkflow(ctx, uuid, "forward-sync", "forward-sync-flow")
	for _, step := range []string{"export-data-from-target", "import-data-to-source-replica"} {
		if err := engine.StartStep(ctx, fwdUUID, step); err != nil {
			t.Fatal(err)
		}
	}

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Live-FF: forward sync running ===")
	printCommands(cmds)

	assertHasCommand(t, cmds, "yb-voyager initiate cutover to source-replica")
	assertNotHasCommand(t, cmds, "yb-voyager end migration")
	assertNotHasCommand(t, cmds, "yb-voyager initiate cutover to target")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (cutover), got %d: %v", len(cmds), cmds)
	}
}

// ==========================================
// Completed workflow
// ==========================================

func TestCompleted_NothingToSuggest(t *testing.T) {
	engine := setupEngine(t)
	registerAll(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migrate-offline")
	schemaUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-schema", "schema-migrate")
	for _, step := range []string{"export-schema", "analyze-schema", "import-schema"} {
		mustStartComplete(t, engine, ctx, schemaUUID, step)
	}
	dataUUID, _ := engine.StartChildWorkflow(ctx, uuid, "migrate-data", "data-offline")
	for _, step := range []string{"export-data", "import-data", "finalize-schema"} {
		mustStartComplete(t, engine, ctx, dataUUID, step)
	}
	mustStartComplete(t, engine, ctx, uuid, "end-migration")

	report, _ := engine.GetStatus(ctx, uuid)
	cmds := GetNextCommands(report)

	fmt.Println("=== Completed: nothing to suggest ===")
	printCommands(cmds)

	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands for completed workflow, got %d: %v", len(cmds), cmds)
	}
}

// --- helpers ---

func printCommands(cmds []SuggestedCommand) {
	if len(cmds) == 0 {
		fmt.Println("  (nothing — wait for current steps to complete)")
	}
	for i, c := range cmds {
		retry := ""
		if c.IsRetry {
			retry = " (retry)"
		}
		fmt.Printf("  %d. %s%s\n", i+1, c.Command, retry)
	}
	fmt.Println()
}

func assertHasCommand(t *testing.T, cmds []SuggestedCommand, command string) {
	t.Helper()
	for _, c := range cmds {
		if c.Command == command {
			return
		}
	}
	t.Errorf("expected command %q in %v", command, cmds)
}

func assertNotHasCommand(t *testing.T, cmds []SuggestedCommand, command string) {
	t.Helper()
	for _, c := range cmds {
		if c.Command == command {
			t.Errorf("did NOT expect command %q in %v", command, cmds)
			return
		}
	}
}

func assertNoRetries(t *testing.T, cmds []SuggestedCommand) {
	t.Helper()
	for _, c := range cmds {
		if c.IsRetry {
			t.Errorf("did not expect any retry commands, found %q", c.Command)
		}
	}
}
