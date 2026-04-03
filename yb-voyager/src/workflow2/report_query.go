package workflow2

// LeafStep identifies a step at the deepest active level of a workflow tree.
type LeafStep struct {
	StepName     string
	WorkflowName string
	WorkflowUUID string
}

// RunningSteps walks the report tree and returns leaf-level running steps.
// A running step is "leaf" if it has no active (running/pending) child workflow.
func (r WorkflowReport) RunningSteps() []LeafStep {
	var result []LeafStep
	for _, step := range r.Steps {
		if step.Status != StepStatusRunning {
			continue
		}
		if child, ok := activeChildReport(step); ok {
			result = append(result, child.RunningSteps()...)
			continue
		}
		result = append(result, LeafStep{
			StepName:     step.StepName,
			WorkflowName: r.WorkflowName,
			WorkflowUUID: r.UUID,
		})
	}
	return result
}

// FailedSteps walks the report tree and returns leaf-level failed steps.
// These represent the deepest point of failure, suitable for retry suggestions.
func (r WorkflowReport) FailedSteps() []LeafStep {
	var result []LeafStep
	for _, step := range r.Steps {
		if step.Status != StepStatusFailed {
			continue
		}
		if child, ok := failedChildReport(step); ok {
			result = append(result, child.FailedSteps()...)
			continue
		}
		result = append(result, LeafStep{
			StepName:     step.StepName,
			WorkflowName: r.WorkflowName,
			WorkflowUUID: r.UUID,
		})
	}
	return result
}

// NextPotentialSteps returns pending steps whose DependsOn are all terminal
// (completed or skipped), recursing into active child workflows for their
// ready steps. Does not include failed steps.
func (r WorkflowReport) NextPotentialSteps() []LeafStep {
	statusMap := make(map[string]StepStatus, len(r.Steps))
	for _, step := range r.Steps {
		statusMap[step.StepName] = step.Status
	}

	var result []LeafStep
	for _, step := range r.Steps {
		if step.Status == StepStatusRunning {
			if child, ok := activeChildReport(step); ok {
				result = append(result, child.NextPotentialSteps()...)
			}
			continue
		}

		if step.Status != StepStatusPending {
			continue
		}

		allDepsTerminal := true
		for _, dep := range step.DependsOn {
			depStatus, ok := statusMap[dep]
			if !ok || !depStatus.IsTerminal() {
				allDepsTerminal = false
				break
			}
		}
		if allDepsTerminal {
			if len(step.ChildReports) > 0 {
				result = append(result, step.ChildReports[0].NextPotentialSteps()...)
			} else {
				result = append(result, LeafStep{
					StepName:     step.StepName,
					WorkflowName: r.WorkflowName,
					WorkflowUUID: r.UUID,
				})
			}
		}
	}
	return result
}

func activeChildReport(step StepReport) (WorkflowReport, bool) {
	for i := len(step.ChildReports) - 1; i >= 0; i-- {
		child := step.ChildReports[i]
		if child.Status == WorkflowStatusRunning || child.Status == WorkflowStatusPending {
			return child, true
		}
	}
	return WorkflowReport{}, false
}

func failedChildReport(step StepReport) (WorkflowReport, bool) {
	for i := len(step.ChildReports) - 1; i >= 0; i-- {
		child := step.ChildReports[i]
		if child.Status == WorkflowStatusFailed {
			return child, true
		}
	}
	return WorkflowReport{}, false
}
