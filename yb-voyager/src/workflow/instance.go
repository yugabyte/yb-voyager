package workflow

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
)

type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

type WorkflowInstance struct {
	UUID               string
	DefinitionName     string
	Status             WorkflowStatus
	ParentWorkflowUUID string // empty for top-level workflows
	ParentStepName     string // which step in the parent spawned this
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type StepState struct {
	WorkflowUUID string
	StepName     string
	Status       StepStatus
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Error        string
}

func NewWorkflowInstance(definitionName string) *WorkflowInstance {
	now := time.Now()
	return &WorkflowInstance{
		UUID:           uuid.New().String(),
		DefinitionName: definitionName,
		Status:         WorkflowStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func NewChildWorkflowInstance(definitionName, parentWorkflowUUID, parentStepName string) *WorkflowInstance {
	inst := NewWorkflowInstance(definitionName)
	inst.ParentWorkflowUUID = parentWorkflowUUID
	inst.ParentStepName = parentStepName
	return inst
}
