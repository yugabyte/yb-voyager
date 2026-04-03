package workflow2

import (
	"fmt"
	"strings"
)

// PPrint returns a human-readable, indented representation of the
// workflow definition graph, showing steps, their dependencies, and
// recursively expanding child workflows.
func (g WorkflowDefinitionGraph) PPrint() string {
	return renderTree(defGraphToNode(g))
}

// PPrint returns a human-readable, indented representation of the
// workflow status report with status indicators and dependencies.
//
// Status indicators:
//
//	✓  completed
//	▶  running
//	✗  failed (with error message)
//	·  pending
//	−  skipped
func (r WorkflowReport) PPrint() string {
	return renderTree(reportToNode(r))
}

// --- internal tree rendering ---

type treeNode struct {
	label    string
	children []treeNode
}

func renderTree(root treeNode) string {
	var b strings.Builder
	b.WriteString(root.label)
	for i, child := range root.children {
		b.WriteByte('\n')
		writeTreeNode(&b, child, "", i == len(root.children)-1)
	}
	return b.String()
}

func writeTreeNode(b *strings.Builder, node treeNode, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	b.WriteString(prefix)
	b.WriteString(connector)
	b.WriteString(node.label)

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}
	for i, child := range node.children {
		b.WriteByte('\n')
		writeTreeNode(b, child, childPrefix, i == len(node.children)-1)
	}
}

// --- definition graph → treeNode ---

func defGraphToNode(g WorkflowDefinitionGraph) treeNode {
	root := treeNode{label: "Workflow: " + g.Definition.Name}
	for _, step := range g.Steps {
		label := step.Step.Name
		if len(step.Step.DependsOn) > 0 {
			label += " (after: " + strings.Join(step.Step.DependsOn, ", ") + ")"
		}
		stepNode := treeNode{label: label}
		isOption := len(step.ChildWorkflows) > 1
		for _, child := range step.ChildWorkflows {
			childNode := defGraphToNode(child)
			if isOption {
				childNode.label = "(option) " + childNode.label
			}
			stepNode.children = append(stepNode.children, childNode)
		}
		root.children = append(root.children, stepNode)
	}
	return root
}

// --- workflow report → treeNode ---

func reportToNode(r WorkflowReport) treeNode {
	root := treeNode{label: fmt.Sprintf("Workflow: %s [%s]", r.WorkflowName, r.Status)}
	for _, step := range r.Steps {
		label := fmt.Sprintf("[%s] %s", stepStatusIndicator(step.Status), step.StepName)
		if step.Error != "" {
			label += " — " + step.Error
		}
		stepNode := treeNode{label: label}
		for i, child := range step.ChildReports {
			childNode := reportToNode(child)
			if len(step.ChildReports) > 1 {
				childNode.label += fmt.Sprintf(" (attempt #%d)", i+1)
			}
			stepNode.children = append(stepNode.children, childNode)
		}
		root.children = append(root.children, stepNode)
	}
	return root
}

func stepStatusIndicator(status StepStatus) string {
	switch status {
	case StepStatusCompleted:
		return "✓"
	case StepStatusRunning:
		return "▶"
	case StepStatusFailed:
		return "✗"
	case StepStatusPending:
		return "·"
	case StepStatusSkipped:
		return "−"
	default:
		return "?"
	}
}
