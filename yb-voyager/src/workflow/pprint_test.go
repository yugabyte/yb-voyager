//go:build unit

package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// --- WorkflowDefinitionTree.PPrint ---

func TestPPrintDefinitionFlat(t *testing.T) {
	tree := WorkflowDefinitionTree{
		Definition: WorkflowDefinition{Name: "simple"},
		Steps: []StepDefinitionNode{
			{Step: StepDefinition{Name: "step-a"}},
			{Step: StepDefinition{Name: "step-b"}},
			{Step: StepDefinition{Name: "step-c"}},
		},
	}
	expected := strings.Join([]string{
		"Workflow: simple",
		"├── step-a",
		"├── step-b",
		"└── step-c",
	}, "\n")
	got := tree.PPrint()
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}

func TestPPrintDefinitionNested(t *testing.T) {
	tree := WorkflowDefinitionTree{
		Definition: WorkflowDefinition{Name: "migration"},
		Steps: []StepDefinitionNode{
			{Step: StepDefinition{Name: "assess"}},
			{
				Step: StepDefinition{Name: "schema-migrate", SubWorkflowName: "schema-migrate-flow"},
				ChildWorkflow: &WorkflowDefinitionTree{
					Definition: WorkflowDefinition{Name: "schema-migrate-flow"},
					Steps: []StepDefinitionNode{
						{Step: StepDefinition{Name: "export-schema"}},
						{Step: StepDefinition{Name: "analyze-schema"}},
						{Step: StepDefinition{Name: "import-schema"}},
					},
				},
			},
			{Step: StepDefinition{Name: "data-migrate"}},
		},
	}
	expected := strings.Join([]string{
		"Workflow: migration",
		"├── assess",
		"├── schema-migrate",
		"│   └── Workflow: schema-migrate-flow",
		"│       ├── export-schema",
		"│       ├── analyze-schema",
		"│       └── import-schema",
		"└── data-migrate",
	}, "\n")
	got := tree.PPrint()
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}

func TestPPrintDefinitionViaEngine(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)

	tree, err := engine.GetWorkflowTree("migration")
	if err != nil {
		t.Fatal(err)
	}
	got := tree.PPrint()
	if !strings.Contains(got, "Workflow: migration") {
		t.Error("should contain root workflow name")
	}
	if !strings.Contains(got, "Workflow: schema-migrate-flow") {
		t.Error("should contain child workflow name")
	}
	if !strings.Contains(got, "export-schema") {
		t.Error("should contain child steps")
	}
}

// --- WorkflowReport.PPrint ---

func TestPPrintReportAllPending(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migration")
	report, _ := engine.GetStatus(ctx, uuid)
	got := report.PPrint()

	expected := strings.Join([]string{
		"Workflow: migration [pending]",
		"├── [·] assess",
		"├── [·] schema-migrate",
		"└── [·] data-migrate",
	}, "\n")
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}

func TestPPrintReportMixedStates(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	uuid, _ := engine.StartWorkflow(ctx, "migration")
	engine.StartStep(ctx, uuid, "assess")
	engine.CompleteStep(ctx, uuid, "assess")
	engine.StartStep(ctx, uuid, "schema-migrate")

	report, _ := engine.GetStatus(ctx, uuid)
	got := report.PPrint()

	if !strings.Contains(got, "Workflow: migration [running]") {
		t.Errorf("should show running workflow:\n%s", got)
	}
	if !strings.Contains(got, "[✓] assess") {
		t.Errorf("should show completed assess:\n%s", got)
	}
	if !strings.Contains(got, "[▶] schema-migrate") {
		t.Errorf("should show running schema-migrate:\n%s", got)
	}
	if !strings.Contains(got, "[·] data-migrate") {
		t.Errorf("should show pending data-migrate:\n%s", got)
	}
}

func TestPPrintReportWithFailedStep(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "fail-wf",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "fail-wf")
	engine.StartStep(ctx, uuid, "a")
	engine.FailStep(ctx, uuid, "a", errors.New("connection refused"))

	report, _ := engine.GetStatus(ctx, uuid)
	got := report.PPrint()

	expected := strings.Join([]string{
		"Workflow: fail-wf [failed]",
		"├── [✗] a — connection refused",
		"└── [·] b",
	}, "\n")
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}

func TestPPrintReportWithSkippedSteps(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "skip-wf",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "skip-wf")
	engine.SkipStep(ctx, uuid, "a")
	engine.StartStep(ctx, uuid, "b")
	engine.CompleteStep(ctx, uuid, "b")
	engine.SkipStep(ctx, uuid, "c")

	report, _ := engine.GetStatus(ctx, uuid)
	got := report.PPrint()

	expected := strings.Join([]string{
		"Workflow: skip-wf [completed]",
		"├── [−] a",
		"├── [✓] b",
		"└── [−] c",
	}, "\n")
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}

func TestPPrintReportImplicitlySkippedSteps(t *testing.T) {
	engine := setupTestEngine(t)
	err := engine.RegisterWorkflow(WorkflowDefinition{
		Name:  "implicit-skip",
		Steps: []StepDefinition{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uuid, _ := engine.StartWorkflow(ctx, "implicit-skip")

	// Start b and d directly, never touch a or c
	engine.StartStep(ctx, uuid, "b")
	engine.CompleteStep(ctx, uuid, "b")
	engine.StartStep(ctx, uuid, "d")
	engine.CompleteStep(ctx, uuid, "d")

	report, _ := engine.GetStatus(ctx, uuid)
	got := report.PPrint()

	expected := strings.Join([]string{
		"Workflow: implicit-skip [running]",
		"├── [·] a",
		"├── [✓] b",
		"├── [·] c",
		"└── [✓] d",
	}, "\n")
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}

func TestPPrintReportWithChildWorkflow(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	parentUUID, _ := engine.StartWorkflow(ctx, "migration")
	engine.StartStep(ctx, parentUUID, "assess")
	engine.CompleteStep(ctx, parentUUID, "assess")
	engine.StartStep(ctx, parentUUID, "schema-migrate")

	childUUID, _ := engine.StartChildWorkflow(ctx, parentUUID, "schema-migrate-flow")
	engine.StartStep(ctx, childUUID, "export-schema")
	engine.CompleteStep(ctx, childUUID, "export-schema")
	engine.StartStep(ctx, childUUID, "analyze-schema")

	report, _ := engine.GetStatus(ctx, parentUUID)
	got := report.PPrint()

	expected := strings.Join([]string{
		"Workflow: migration [running]",
		"├── [✓] assess",
		"├── [▶] schema-migrate",
		"│   └── Workflow: schema-migrate-flow [running]",
		"│       ├── [✓] export-schema",
		"│       ├── [▶] analyze-schema",
		"│       └── [·] import-schema",
		"└── [·] data-migrate",
	}, "\n")
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}

func TestPPrintReportWithMultipleChildInstances(t *testing.T) {
	engine := setupTestEngine(t)
	registerMigrationWorkflows(t, engine)
	ctx := context.Background()

	parentUUID, _ := engine.StartWorkflow(ctx, "migration")
	engine.StartStep(ctx, parentUUID, "assess")
	engine.CompleteStep(ctx, parentUUID, "assess")
	engine.StartStep(ctx, parentUUID, "schema-migrate")

	// First attempt: fails
	child1, _ := engine.StartChildWorkflow(ctx, parentUUID, "schema-migrate-flow")
	engine.StartStep(ctx, child1, "export-schema")
	engine.FailStep(ctx, child1, "export-schema", errors.New("network error"))

	// Second attempt: succeeds
	child2, _ := engine.StartChildWorkflow(ctx, parentUUID, "schema-migrate-flow")
	for _, s := range []string{"export-schema", "analyze-schema", "import-schema"} {
		engine.StartStep(ctx, child2, s)
		engine.CompleteStep(ctx, child2, s)
	}
	engine.CompleteStep(ctx, parentUUID, "schema-migrate")

	report, _ := engine.GetStatus(ctx, parentUUID)
	got := report.PPrint()

	expected := strings.Join([]string{
		"Workflow: migration [running]",
		"├── [✓] assess",
		"├── [✓] schema-migrate",
		"│   ├── Workflow: schema-migrate-flow [failed] (attempt #1)",
		"│   │   ├── [✗] export-schema — network error",
		"│   │   ├── [·] analyze-schema",
		"│   │   └── [·] import-schema",
		"│   └── Workflow: schema-migrate-flow [completed] (attempt #2)",
		"│       ├── [✓] export-schema",
		"│       ├── [✓] analyze-schema",
		"│       └── [✓] import-schema",
		"└── [·] data-migrate",
	}, "\n")
	if got != expected {
		t.Errorf("PPrint mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}
}
