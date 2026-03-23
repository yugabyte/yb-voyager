package workflow

import "context"

type WorkflowStore interface {
	EnsureTables(ctx context.Context) error

	CreateInstance(ctx context.Context, inst *WorkflowInstance) error
	GetInstance(ctx context.Context, uuid string) (*WorkflowInstance, error)
	UpdateInstanceStatus(ctx context.Context, uuid string, status WorkflowStatus) error
	GetChildInstances(ctx context.Context, parentUUID string, stepName string) ([]*WorkflowInstance, error)

	SetStepState(ctx context.Context, state *StepState) error
	GetStepStates(ctx context.Context, workflowUUID string) ([]StepState, error)
	GetStepState(ctx context.Context, workflowUUID string, stepName string) (*StepState, error)
}
