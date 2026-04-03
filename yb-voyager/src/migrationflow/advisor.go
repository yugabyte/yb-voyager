package migrationflow

import (
	"github.com/yugabyte/yb-voyager/yb-voyager/src/workflow2"
)

// SuggestedCommand represents a CLI command the user can run to advance the workflow.
type SuggestedCommand struct {
	Command string
	IsRetry bool
}

var stepCommandMap = map[string]string{
	"assess-migration":             "yb-voyager assess-migration",
	"export-schema":                "yb-voyager export schema",
	"analyze-schema":               "yb-voyager analyze-schema",
	"import-schema":                "yb-voyager import schema",
	"export-data":                  "yb-voyager export data",
	"import-data":                  "yb-voyager import data",
	"import-data-to-target":        "yb-voyager import data to target",
	"import-data-to-source-replica": "yb-voyager import data to source-replica",
	"archive-changes":              "yb-voyager archive changes",
	"finalize-schema":              "yb-voyager finalize-schema-post-data-import",
	"export-data-from-target":      "yb-voyager export data from target",
	"import-data-to-source":        "yb-voyager import data to source",
	"end-migration":                "yb-voyager end migration",
}

var cutoverTriggerMap = map[string]string{
	"data-live":         "yb-voyager initiate cutover to target",
	"data-live-ff":      "yb-voyager initiate cutover to target",
	"reverse-sync-flow": "yb-voyager initiate cutover to source",
	"forward-sync-flow": "yb-voyager initiate cutover to source-replica",
}

// GetNextCommands analyzes a workflow report and returns CLI commands the user
// can run to advance the migration. Returns nil when the user should wait for
// currently running steps to complete.
func GetNextCommands(report workflow2.WorkflowReport) []SuggestedCommand {
	failed := report.FailedSteps()
	if len(failed) > 0 {
		var commands []SuggestedCommand
		for _, step := range failed {
			if cmd, ok := stepCommandMap[step.StepName]; ok {
				commands = append(commands, SuggestedCommand{Command: cmd, IsRetry: true})
			}
		}
		return commands
	}

	var commands []SuggestedCommand

	running := report.RunningSteps()
	commands = append(commands, checkCutoverTriggers(running)...)

	ready := report.NextPotentialSteps()
	for _, step := range ready {
		if step.StepName == "end-migration" {
			continue
		}
		if cmd, ok := stepCommandMap[step.StepName]; ok {
			commands = append(commands, SuggestedCommand{Command: cmd})
		}
	}

	return commands
}

func checkCutoverTriggers(running []workflow2.LeafStep) []SuggestedCommand {
	seen := make(map[string]bool)
	var commands []SuggestedCommand
	for _, step := range running {
		if seen[step.WorkflowName] {
			continue
		}
		seen[step.WorkflowName] = true
		if cmd, ok := cutoverTriggerMap[step.WorkflowName]; ok {
			commands = append(commands, SuggestedCommand{Command: cmd})
		}
	}
	return commands
}
