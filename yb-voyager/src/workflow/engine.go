package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// WorkflowReport is the recursive status view of a workflow instance.
type WorkflowReport struct {
	UUID         string
	WorkflowName string
	Status       WorkflowStatus
	Steps        []StepReport
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StepReport is the status of a single step within a workflow.
type StepReport struct {
	StepName     string
	Status       StepStatus
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Error        string
	ChildReports []WorkflowReport // all child instances for this step, ordered by creation time
}

// WorkflowEngine is the sole public entry point for the workflow package.
// It is a state-tracker: callers own execution and call engine methods to
// record state transitions. The engine validates transitions, derives
// workflow-level status from step states, and builds recursive status reports.
type WorkflowEngine struct {
	registry workflowRegistry
	store    workflowStore
}

// NewWorkflowEngine creates a WorkflowEngine backed by the given SQLite DB.
// It creates the required tables if they don't exist.
func NewWorkflowEngine(db *sql.DB) (*WorkflowEngine, error) {
	store := newSQLiteWorkflowStore(db)
	if err := store.EnsureTables(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize workflow store: %w", err)
	}
	return &WorkflowEngine{
		registry: newWorkflowRegistry(),
		store:    store,
	}, nil
}

// RegisterWorkflow registers a workflow definition so it can be used with
// StartWorkflow / StartChildWorkflow.
func (e *WorkflowEngine) RegisterWorkflow(def WorkflowDefinition) error {
	return e.registry.register(def)
}

// GetWorkflowTree returns the full recursive definition tree for a workflow,
// resolving all SubWorkflowName references.
func (e *WorkflowEngine) GetWorkflowTree(workflowName string) (WorkflowDefinitionTree, error) {
	return e.registry.getTree(workflowName)
}

// StartWorkflow creates a new workflow instance in pending status with all
// steps initialized to pending. Returns the workflow UUID.
func (e *WorkflowEngine) StartWorkflow(ctx context.Context, workflowName string) (string, error) {
	def, err := e.registry.get(workflowName)
	if err != nil {
		return "", err
	}
	inst := newWorkflowInstance(def.Name)
	if err := e.store.CreateInstance(ctx, inst); err != nil {
		return "", err
	}
	if err := e.initStepStates(ctx, inst.UUID, def); err != nil {
		return "", err
	}
	return inst.UUID, nil
}

// StartChildWorkflow creates a child workflow instance linked to a parent.
// The parent step is auto-derived by finding the step in the parent's
// definition whose SubWorkflowName matches workflowName.
// Multiple child instances per parent step are allowed (e.g., retries).
func (e *WorkflowEngine) StartChildWorkflow(ctx context.Context, parentUUID, workflowName string) (string, error) {
	parentInst, err := e.store.GetInstance(ctx, parentUUID)
	if err != nil {
		return "", fmt.Errorf("parent workflow: %w", err)
	}
	parentDef, err := e.registry.get(parentInst.DefinitionName)
	if err != nil {
		return "", fmt.Errorf("parent workflow definition: %w", err)
	}

	parentStepName := ""
	for _, step := range parentDef.Steps {
		if step.SubWorkflowName == workflowName {
			parentStepName = step.Name
			break
		}
	}
	if parentStepName == "" {
		return "", fmt.Errorf("no step in workflow %q has SubWorkflowName %q", parentDef.Name, workflowName)
	}

	childDef, err := e.registry.get(workflowName)
	if err != nil {
		return "", err
	}

	inst := newChildWorkflowInstance(childDef.Name, parentUUID, parentStepName)
	if err := e.store.CreateInstance(ctx, inst); err != nil {
		return "", err
	}
	if err := e.initStepStates(ctx, inst.UUID, childDef); err != nil {
		return "", err
	}
	return inst.UUID, nil
}

// StartStep marks a step as running. The step must be in pending or failed
// status (failed allows retry). Also transitions the workflow to running
// if it was pending or failed.
func (e *WorkflowEngine) StartStep(ctx context.Context, workflowUUID, stepName string) error {
	inst, def, err := e.getInstanceAndDef(ctx, workflowUUID)
	if err != nil {
		return err
	}
	if err := validateStepExists(def, stepName); err != nil {
		return err
	}

	state, err := e.store.GetStepState(ctx, workflowUUID, stepName)
	if err != nil {
		return err
	}
	if state.Status != StepStatusPending && state.Status != StepStatusFailed {
		return fmt.Errorf("cannot start step %q: current status is %q (must be %q or %q)",
			stepName, state.Status, StepStatusPending, StepStatusFailed)
	}

	now := time.Now()
	state.Status = StepStatusRunning
	state.StartedAt = &now
	state.CompletedAt = nil
	state.Error = ""
	if err := e.store.SetStepState(ctx, state); err != nil {
		return err
	}

	if inst.Status == WorkflowStatusPending || inst.Status == WorkflowStatusFailed {
		return e.store.UpdateInstanceStatus(ctx, workflowUUID, WorkflowStatusRunning)
	}
	return nil
}

// CompleteStep marks a step as completed. The step must be in running status.
// If all steps are now terminal (completed/skipped), the workflow is marked
// as completed.
func (e *WorkflowEngine) CompleteStep(ctx context.Context, workflowUUID, stepName string) error {
	_, def, err := e.getInstanceAndDef(ctx, workflowUUID)
	if err != nil {
		return err
	}
	if err := validateStepExists(def, stepName); err != nil {
		return err
	}

	state, err := e.store.GetStepState(ctx, workflowUUID, stepName)
	if err != nil {
		return err
	}
	if state.Status != StepStatusRunning {
		return fmt.Errorf("cannot complete step %q: current status is %q (must be %q)",
			stepName, state.Status, StepStatusRunning)
	}

	now := time.Now()
	state.Status = StepStatusCompleted
	state.CompletedAt = &now
	if err := e.store.SetStepState(ctx, state); err != nil {
		return err
	}

	return e.maybeCompleteWorkflow(ctx, workflowUUID)
}

// FailStep marks a step as failed with the given error. The step must be in
// running status. The workflow is marked as failed.
func (e *WorkflowEngine) FailStep(ctx context.Context, workflowUUID, stepName string, stepErr error) error {
	_, def, err := e.getInstanceAndDef(ctx, workflowUUID)
	if err != nil {
		return err
	}
	if err := validateStepExists(def, stepName); err != nil {
		return err
	}

	state, err := e.store.GetStepState(ctx, workflowUUID, stepName)
	if err != nil {
		return err
	}
	if state.Status != StepStatusRunning {
		return fmt.Errorf("cannot fail step %q: current status is %q (must be %q)",
			stepName, state.Status, StepStatusRunning)
	}

	now := time.Now()
	state.Status = StepStatusFailed
	state.CompletedAt = &now
	state.Error = stepErr.Error()
	if err := e.store.SetStepState(ctx, state); err != nil {
		return err
	}

	return e.store.UpdateInstanceStatus(ctx, workflowUUID, WorkflowStatusFailed)
}

// SkipStep marks a step as skipped. The step must be in pending status.
// If all steps are now terminal (completed/skipped), the workflow is marked
// as completed.
func (e *WorkflowEngine) SkipStep(ctx context.Context, workflowUUID, stepName string) error {
	_, def, err := e.getInstanceAndDef(ctx, workflowUUID)
	if err != nil {
		return err
	}
	if err := validateStepExists(def, stepName); err != nil {
		return err
	}

	state, err := e.store.GetStepState(ctx, workflowUUID, stepName)
	if err != nil {
		return err
	}
	if state.Status != StepStatusPending {
		return fmt.Errorf("cannot skip step %q: current status is %q (must be %q)",
			stepName, state.Status, StepStatusPending)
	}

	state.Status = StepStatusSkipped
	if err := e.store.SetStepState(ctx, state); err != nil {
		return err
	}

	return e.maybeCompleteWorkflow(ctx, workflowUUID)
}

// GetStatus builds a recursive status report for a workflow, including all
// child workflow instances for steps with SubWorkflowName.
func (e *WorkflowEngine) GetStatus(ctx context.Context, workflowUUID string) (WorkflowReport, error) {
	inst, def, err := e.getInstanceAndDef(ctx, workflowUUID)
	if err != nil {
		return WorkflowReport{}, err
	}

	stateMap, err := e.stepStateMap(ctx, workflowUUID)
	if err != nil {
		return WorkflowReport{}, err
	}

	report := WorkflowReport{
		UUID:         inst.UUID,
		WorkflowName: inst.DefinitionName,
		Status:       inst.Status,
		CreatedAt:    inst.CreatedAt,
		UpdatedAt:    inst.UpdatedAt,
	}

	for _, stepDef := range def.Steps {
		state, ok := stateMap[stepDef.Name]
		sr := StepReport{StepName: stepDef.Name}
		if ok {
			sr.Status = state.Status
			sr.StartedAt = state.StartedAt
			sr.CompletedAt = state.CompletedAt
			sr.Error = state.Error
		} else {
			sr.Status = StepStatusPending
		}

		if stepDef.SubWorkflowName != "" {
			children, err := e.store.GetChildInstances(ctx, workflowUUID, stepDef.Name)
			if err != nil {
				return WorkflowReport{}, err
			}
			for _, child := range children {
				childReport, err := e.GetStatus(ctx, child.UUID)
				if err != nil {
					return WorkflowReport{}, err
				}
				sr.ChildReports = append(sr.ChildReports, childReport)
			}
		}

		report.Steps = append(report.Steps, sr)
	}

	return report, nil
}

// GetNextPendingStep returns the first step in definition order whose status
// is pending or failed (skipping completed and skipped steps).
// Returns ("", false, nil) when all steps are terminal.
func (e *WorkflowEngine) GetNextPendingStep(ctx context.Context, workflowUUID string) (string, bool, error) {
	_, def, err := e.getInstanceAndDef(ctx, workflowUUID)
	if err != nil {
		return "", false, err
	}

	stateMap, err := e.stepStateMap(ctx, workflowUUID)
	if err != nil {
		return "", false, err
	}

	for _, stepDef := range def.Steps {
		s, ok := stateMap[stepDef.Name]
		if !ok || s.Status == StepStatusPending || s.Status == StepStatusFailed {
			return stepDef.Name, true, nil
		}
	}
	return "", false, nil
}

// --- internal helpers ---

func (e *WorkflowEngine) initStepStates(ctx context.Context, workflowUUID string, def WorkflowDefinition) error {
	for _, step := range def.Steps {
		state := stepState{
			WorkflowUUID: workflowUUID,
			StepName:     step.Name,
			Status:       StepStatusPending,
		}
		if err := e.store.SetStepState(ctx, state); err != nil {
			return fmt.Errorf("failed to initialize step %q: %w", step.Name, err)
		}
	}
	return nil
}

func (e *WorkflowEngine) getInstanceAndDef(ctx context.Context, workflowUUID string) (workflowInstance, WorkflowDefinition, error) {
	inst, err := e.store.GetInstance(ctx, workflowUUID)
	if err != nil {
		return workflowInstance{}, WorkflowDefinition{}, err
	}
	def, err := e.registry.get(inst.DefinitionName)
	if err != nil {
		return workflowInstance{}, WorkflowDefinition{}, err
	}
	return inst, def, nil
}

func (e *WorkflowEngine) stepStateMap(ctx context.Context, workflowUUID string) (map[string]stepState, error) {
	states, err := e.store.GetStepStates(ctx, workflowUUID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]stepState, len(states))
	for _, s := range states {
		m[s.StepName] = s
	}
	return m, nil
}

func (e *WorkflowEngine) maybeCompleteWorkflow(ctx context.Context, workflowUUID string) error {
	states, err := e.store.GetStepStates(ctx, workflowUUID)
	if err != nil {
		return err
	}
	for _, s := range states {
		if s.Status != StepStatusCompleted && s.Status != StepStatusSkipped {
			return nil
		}
	}
	return e.store.UpdateInstanceStatus(ctx, workflowUUID, WorkflowStatusCompleted)
}

func validateStepExists(def WorkflowDefinition, stepName string) error {
	for _, step := range def.Steps {
		if step.Name == stepName {
			return nil
		}
	}
	return fmt.Errorf("step %q not found in workflow %q", stepName, def.Name)
}
