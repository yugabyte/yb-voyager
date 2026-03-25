package workflow

import "context"

type workflowStore interface {
	EnsureTables(ctx context.Context) error

	CreateInstance(ctx context.Context, inst workflowInstance) error
	GetInstance(ctx context.Context, uuid string) (workflowInstance, error)
	UpdateInstanceStatus(ctx context.Context, uuid string, status WorkflowStatus) error
	GetChildInstances(ctx context.Context, parentUUID string, stepName string) ([]workflowInstance, error)

	SetStepState(ctx context.Context, state stepState) error
	GetStepStates(ctx context.Context, workflowUUID string) ([]stepState, error)
	GetStepState(ctx context.Context, workflowUUID string, stepName string) (stepState, error)
}
