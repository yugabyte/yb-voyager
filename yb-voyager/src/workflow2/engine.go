package workflow2

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
	DependsOn    []string
	Status       StepStatus
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Error        string
	ChildReports []WorkflowReport
}

// WorkflowEngine is the sole public entry point for the workflow2 package.
// It is a state-tracker: callers own execution and call engine methods to
// record state transitions. The engine validates transitions, derives
// workflow-level status from step states, and builds recursive status reports.
//
// Key differences from workflow v1:
//   - Steps have DependsOn (advisory DAG) instead of sequential ordering
//   - GetReadySteps returns all steps whose dependencies are terminal
//   - StartChildWorkflow takes explicit (parentUUID, stepName, childWorkflowName)
//   - ChildWorkflows on StepDefinition is an annotation for tree display
type WorkflowEngine struct {
	registry workflowRegistry
	store    workflowStore
}

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

func (e *WorkflowEngine) RegisterWorkflow(def WorkflowDefinition) error {
	if err := validateDAG(def); err != nil {
		return err
	}
	return e.registry.register(def)
}

// GetWorkflowGraph returns the full recursive definition graph for a workflow.
func (e *WorkflowEngine) GetWorkflowGraph(workflowName string) (WorkflowDefinitionGraph, error) {
	return e.registry.getGraph(workflowName)
}

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

// StartChildWorkflow creates a child workflow linked to a specific parent step.
// The caller explicitly provides the step name (unlike v1 which auto-derived it).
func (e *WorkflowEngine) StartChildWorkflow(ctx context.Context, parentUUID, stepName, workflowName string) (string, error) {
	parentInst, err := e.store.GetInstance(ctx, parentUUID)
	if err != nil {
		return "", fmt.Errorf("parent workflow: %w", err)
	}
	parentDef, err := e.registry.get(parentInst.DefinitionName)
	if err != nil {
		return "", fmt.Errorf("parent workflow definition: %w", err)
	}

	if err := validateStepExists(parentDef, stepName); err != nil {
		return "", err
	}

	parentStepState, err := e.store.GetStepState(ctx, parentUUID, stepName)
	if err != nil {
		return "", fmt.Errorf("parent step state: %w", err)
	}
	if parentStepState.Status == StepStatusCompleted || parentStepState.Status == StepStatusSkipped {
		return "", fmt.Errorf("cannot start child workflow: parent step %q is %q", stepName, parentStepState.Status)
	}

	children, err := e.store.GetChildInstances(ctx, parentUUID, stepName)
	if err != nil {
		return "", fmt.Errorf("checking existing children: %w", err)
	}
	for _, child := range children {
		if child.Status == WorkflowStatusPending || child.Status == WorkflowStatusRunning {
			return "", fmt.Errorf("step %q already has an active child workflow %q", stepName, child.UUID)
		}
	}

	childDef, err := e.registry.get(workflowName)
	if err != nil {
		return "", err
	}

	inst := newChildWorkflowInstance(childDef.Name, parentUUID, stepName)
	if err := e.store.CreateInstance(ctx, inst); err != nil {
		return "", err
	}
	if err := e.initStepStates(ctx, inst.UUID, childDef); err != nil {
		return "", err
	}

	if parentStepState.Status == StepStatusPending || parentStepState.Status == StepStatusFailed {
		now := time.Now()
		parentStepState.Status = StepStatusRunning
		parentStepState.StartedAt = &now
		parentStepState.CompletedAt = nil
		parentStepState.Error = ""
		if err := e.store.SetStepState(ctx, parentStepState); err != nil {
			return "", err
		}
	}
	if parentInst.Status == WorkflowStatusPending || parentInst.Status == WorkflowStatusFailed {
		if err := e.store.UpdateInstanceStatus(ctx, parentUUID, WorkflowStatusRunning); err != nil {
			return "", err
		}
	}

	return inst.UUID, nil
}

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

	if err := e.store.UpdateInstanceStatus(ctx, workflowUUID, WorkflowStatusFailed); err != nil {
		return err
	}
	return e.propagateChildFailure(ctx, workflowUUID)
}

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
		sr := StepReport{
			StepName:  stepDef.Name,
			DependsOn: stepDef.DependsOn,
		}
		if ok {
			sr.Status = state.Status
			sr.StartedAt = state.StartedAt
			sr.CompletedAt = state.CompletedAt
			sr.Error = state.Error
		} else {
			sr.Status = StepStatusPending
		}

		if len(stepDef.ChildWorkflows) > 0 {
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
			// If no actual child instance exists yet and exactly 1 child
			// workflow is configured, project the definition as a pending
			// report so callers can see what comes next.
			if len(sr.ChildReports) == 0 && len(stepDef.ChildWorkflows) == 1 {
				projected, err := e.projectDefinition(stepDef.ChildWorkflows[0])
				if err == nil {
					sr.ChildReports = append(sr.ChildReports, projected)
				}
			}
		}

		report.Steps = append(report.Steps, sr)
	}

	return report, nil
}

// GetReadySteps returns all steps whose DependsOn dependencies are all terminal
// (completed or skipped) and whose own status is pending or failed.
// Steps with no DependsOn are always considered ready (if pending/failed).
func (e *WorkflowEngine) GetReadySteps(ctx context.Context, workflowUUID string) ([]string, error) {
	_, def, err := e.getInstanceAndDef(ctx, workflowUUID)
	if err != nil {
		return nil, err
	}

	stateMap, err := e.stepStateMap(ctx, workflowUUID)
	if err != nil {
		return nil, err
	}

	var ready []string
	for _, stepDef := range def.Steps {
		s, ok := stateMap[stepDef.Name]
		if ok && s.Status != StepStatusPending && s.Status != StepStatusFailed {
			continue
		}

		allDepsTerminal := true
		for _, dep := range stepDef.DependsOn {
			depState, ok := stateMap[dep]
			if !ok || !depState.Status.IsTerminal() {
				allDepsTerminal = false
				break
			}
		}
		if allDepsTerminal {
			ready = append(ready, stepDef.Name)
		}
	}
	return ready, nil
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
		if !s.Status.IsTerminal() {
			return nil
		}
	}
	if err := e.store.UpdateInstanceStatus(ctx, workflowUUID, WorkflowStatusCompleted); err != nil {
		return err
	}
	return e.propagateChildCompletion(ctx, workflowUUID)
}

func (e *WorkflowEngine) propagateChildCompletion(ctx context.Context, childUUID string) error {
	inst, err := e.store.GetInstance(ctx, childUUID)
	if err != nil {
		return err
	}
	if inst.ParentWorkflowUUID == "" {
		return nil
	}
	parentStep, err := e.store.GetStepState(ctx, inst.ParentWorkflowUUID, inst.ParentStepName)
	if err != nil {
		return err
	}
	now := time.Now()
	parentStep.Status = StepStatusCompleted
	parentStep.CompletedAt = &now
	if err := e.store.SetStepState(ctx, parentStep); err != nil {
		return err
	}
	return e.maybeCompleteWorkflow(ctx, inst.ParentWorkflowUUID)
}

func (e *WorkflowEngine) propagateChildFailure(ctx context.Context, workflowUUID string) error {
	inst, err := e.store.GetInstance(ctx, workflowUUID)
	if err != nil {
		return err
	}
	if inst.ParentWorkflowUUID == "" {
		return nil
	}
	parentStep, err := e.store.GetStepState(ctx, inst.ParentWorkflowUUID, inst.ParentStepName)
	if err != nil {
		return err
	}
	now := time.Now()
	parentStep.Status = StepStatusFailed
	parentStep.CompletedAt = &now
	parentStep.Error = "child workflow failed"
	if err := e.store.SetStepState(ctx, parentStep); err != nil {
		return err
	}
	if err := e.store.UpdateInstanceStatus(ctx, inst.ParentWorkflowUUID, WorkflowStatusFailed); err != nil {
		return err
	}
	return e.propagateChildFailure(ctx, inst.ParentWorkflowUUID)
}

// projectDefinition builds a synthetic WorkflowReport from a workflow
// definition, with all steps pending. This allows callers to see what
// a child workflow will contain before it is actually started.
func (e *WorkflowEngine) projectDefinition(workflowName string) (WorkflowReport, error) {
	def, err := e.registry.get(workflowName)
	if err != nil {
		return WorkflowReport{}, err
	}
	report := WorkflowReport{
		WorkflowName: def.Name,
		Status:       WorkflowStatusPending,
	}
	for _, stepDef := range def.Steps {
		sr := StepReport{
			StepName:  stepDef.Name,
			DependsOn: stepDef.DependsOn,
			Status:    StepStatusPending,
		}
		if len(stepDef.ChildWorkflows) == 1 {
			childProjected, err := e.projectDefinition(stepDef.ChildWorkflows[0])
			if err == nil {
				sr.ChildReports = append(sr.ChildReports, childProjected)
			}
		}
		report.Steps = append(report.Steps, sr)
	}
	return report, nil
}

func validateStepExists(def WorkflowDefinition, stepName string) error {
	for _, step := range def.Steps {
		if step.Name == stepName {
			return nil
		}
	}
	return fmt.Errorf("step %q not found in workflow %q", stepName, def.Name)
}
