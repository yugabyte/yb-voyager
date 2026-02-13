/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// PhaseStatus represents the status of a high-level migration phase.
type PhaseStatus int

const (
	PhasePending    PhaseStatus = iota
	PhaseInProgress
	PhaseDone
)

// StepProgress holds the computed progress for a single step within a phase.
type StepProgress struct {
	DisplayName string
	Done        bool
}

// PhaseProgress holds the computed progress for a single high-level phase.
type PhaseProgress struct {
	Name           string         // e.g., "Assess", "Schema", "Data", "End"
	Status         PhaseStatus
	CompletedSteps int
	TotalSteps     int
	Steps          []StepProgress // individual steps within this phase
}

// CommandFooter captures all the data needed to render a post-command footer.
type CommandFooter struct {
	SectionTitle string          // section heading, e.g., "Assessment Summary"
	Title        string          // success message, e.g., "Migration assessment completed successfully."
	Artifacts    []string        // report/output file paths
	Summary      []string        // key-value stat lines (pre-formatted)
	NextSteps    []string        // instruction lines + styled command
	Phases       []PhaseProgress // from computePhaseStatuses
}

// computePhaseStatuses walks the workflow steps and groups them by phase to derive
// per-phase progress. The currentStepID marks the step that just completed, ensuring
// it (and all steps before it) are treated as done even if the MSR hasn't been updated yet.
func computePhaseStatuses(wf *Workflow, msr *metadb.MigrationStatusRecord, currentStepID string) []PhaseProgress {
	// Determine the index of the current step so we can treat everything up to
	// and including it as done.
	currentIdx := -1
	for i, step := range wf.Steps {
		if step.ID == currentStepID {
			currentIdx = i
			break
		}
	}

	// Build an ordered list of unique phases preserving workflow order.
	type phaseAccum struct {
		name      string
		completed int
		total     int
		steps     []StepProgress
	}
	var phaseOrder []string
	phases := make(map[string]*phaseAccum)

	for i, step := range wf.Steps {
		pa, exists := phases[step.Phase]
		if !exists {
			pa = &phaseAccum{name: step.Phase}
			phases[step.Phase] = pa
			phaseOrder = append(phaseOrder, step.Phase)
		}
		pa.total++

		// A step is considered done if:
		// 1. It is at or before the current step index, OR
		// 2. Its IsDone function returns true from MSR.
		done := i <= currentIdx || (msr != nil && step.IsDone(msr))
		if done {
			pa.completed++
		}
		pa.steps = append(pa.steps, StepProgress{
			DisplayName: step.DisplayName,
			Done:        done,
		})
	}

	// Convert to PhaseProgress slice.
	result := make([]PhaseProgress, 0, len(phaseOrder))
	for _, phaseName := range phaseOrder {
		pa := phases[phaseName]
		var status PhaseStatus
		switch {
		case pa.completed >= pa.total:
			status = PhaseDone
		case pa.completed > 0:
			status = PhaseInProgress
		default:
			status = PhasePending
		}
		result = append(result, PhaseProgress{
			Name:           pa.name,
			Status:         status,
			CompletedSteps: pa.completed,
			TotalSteps:     pa.total,
			Steps:          pa.steps,
		})
	}

	return result
}

// formatPhaseLines renders the phase progress as styled strings for display.
// Phases with more than one step get a drill-down showing individual step status.
func formatPhaseLines(phases []PhaseProgress) []string {
	lines := make([]string, 0, len(phases)*3)
	for _, p := range phases {
		var marker, label string
		switch p.Status {
		case PhaseDone:
			marker = phaseDoneMarker()
			label = dimStyle.Render("done")
		case PhaseInProgress:
			marker = phaseActiveMarker()
			label = "in progress"
		case PhasePending:
			marker = phasePendingMarker()
			label = dimStyle.Render("pending")
		}
		lines = append(lines, fmt.Sprintf("%s %-10s %s", marker, p.Name, label))

		// Show sub-step drill-down only for in-progress phases with multiple steps.
		if p.Status == PhaseInProgress && len(p.Steps) > 1 {
			for _, s := range p.Steps {
				var stepMarker, stepName string
				if s.Done {
					stepMarker = phaseDoneMarker()
					stepName = dimStyle.Render(s.DisplayName)
				} else {
					stepMarker = phasePendingMarker()
					stepName = s.DisplayName
				}
				lines = append(lines, fmt.Sprintf("    %s %s", stepMarker, stepName))
			}
		}
	}
	return lines
}

// printCommandFooter renders the full post-command footer.
// Section order: Summary (result + artifacts + stats), Migration Progress, What's Next.
func printCommandFooter(footer CommandFooter) {
	// ── Combined summary section ──
	var lines []string
	lines = append(lines, successLine(footer.Title))
	if len(footer.Artifacts) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Artifacts:")
		for _, a := range footer.Artifacts {
			lines = append(lines, "  "+dimStyle.Render(a))
		}
	}
	if len(footer.Summary) > 0 {
		lines = append(lines, "")
		lines = append(lines, footer.Summary...)
	}
	printSection(footer.SectionTitle, lines...)

	// ── Migration Progress (Status) section ──
	if len(footer.Phases) > 0 {
		phaseLines := formatPhaseLines(footer.Phases)
		printSection("Migration Progress", phaseLines...)
	}

	// ── What's Next section ──
	if len(footer.NextSteps) > 0 {
		lines := make([]string, len(footer.NextSteps))
		copy(lines, footer.NextSteps)

		// Add tip at the end.
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render(fmt.Sprintf("Tip: yb-voyager status --config-file %s", displayPath(cfgFile))))

		printSection("What's Next", lines...)
	}

	fmt.Println()
}

// nextStepCommand formats a "next step" instruction block with description and styled command.
func nextStepCommand(description, command string) []string {
	return []string{
		description,
		"",
		cmdStyle.Render("  " + command),
	}
}
