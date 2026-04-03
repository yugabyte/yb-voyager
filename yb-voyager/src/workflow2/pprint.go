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
//
// Steps are classified as "chain" (sequential) or "floating" (parallel):
//   - Chain: has DependsOn OR is depended upon by another step
//   - Floating: no DependsOn AND nothing depends on it
//
// Chain steps render as vertical tree branches; multiple floating steps
// collapse onto a single horizontal line separated by " | ".
// Child workflows that are fully parallel render inline on one line;
// those with sequential steps expand as a sub-tree.

func defGraphToNode(g WorkflowDefinitionGraph) treeNode {
	return buildWorkflowNode(g, "Workflow: "+g.Definition.Name)
}

func buildWorkflowNode(g WorkflowDefinitionGraph, label string) treeNode {
	root := treeNode{label: label}

	chain, floating := classifySteps(g)
	sorted := topoSortSteps(chain)

	for _, sn := range sorted {
		root.children = append(root.children, buildDefStepNode(sn))
	}

	if len(floating) > 0 {
		if len(floating) == 1 {
			root.children = append(root.children, buildDefStepNode(floating[0]))
		} else {
			names := make([]string, len(floating))
			for i, f := range floating {
				names[i] = f.Step.Name
			}
			root.children = append(root.children, treeNode{label: strings.Join(names, " | ")})
		}
	}

	return root
}

func buildDefStepNode(sn StepDefinitionNode) treeNode {
	node := treeNode{label: sn.Step.Name}

	isOption := len(sn.ChildWorkflows) > 1
	for _, child := range sn.ChildWorkflows {
		if isOption {
			childNode := defGraphToNode(child)
			childNode.label = "(option) " + childNode.label
			node.children = append(node.children, childNode)
		} else if isAllParallel(child) && hasNoNestedChildren(child.Steps) {
			names := make([]string, len(child.Steps))
			for i, s := range child.Steps {
				names[i] = s.Step.Name
			}
			node.children = append(node.children, treeNode{
				label: child.Definition.Name + ": " + strings.Join(names, " | "),
			})
		} else {
			node.children = append(node.children, buildWorkflowNode(child, child.Definition.Name+":"))
		}
	}

	return node
}

// classifySteps splits steps into "chain" (part of the dependency graph)
// and "floating" (no deps and nothing depends on them).
func classifySteps(g WorkflowDefinitionGraph) (chain, floating []StepDefinitionNode) {
	dependedUpon := make(map[string]bool)
	for _, s := range g.Definition.Steps {
		for _, dep := range s.DependsOn {
			dependedUpon[dep] = true
		}
	}
	for _, sn := range g.Steps {
		if len(sn.Step.DependsOn) > 0 || dependedUpon[sn.Step.Name] {
			chain = append(chain, sn)
		} else {
			floating = append(floating, sn)
		}
	}
	return
}

// topoSortSteps returns chain steps in topological order, using
// definition order as a tiebreaker for determinism.
func topoSortSteps(steps []StepDefinitionNode) []StepDefinitionNode {
	if len(steps) == 0 {
		return nil
	}

	nameToNode := make(map[string]StepDefinitionNode, len(steps))
	inDegree := make(map[string]int, len(steps))
	adj := make(map[string][]string)
	stepNames := make(map[string]bool, len(steps))
	orderedNames := make([]string, 0, len(steps))

	for _, sn := range steps {
		nameToNode[sn.Step.Name] = sn
		stepNames[sn.Step.Name] = true
		orderedNames = append(orderedNames, sn.Step.Name)
		inDegree[sn.Step.Name] = 0
	}
	for _, sn := range steps {
		for _, dep := range sn.Step.DependsOn {
			if stepNames[dep] {
				adj[dep] = append(adj[dep], sn.Step.Name)
				inDegree[sn.Step.Name]++
			}
		}
	}

	sorted := make([]StepDefinitionNode, 0, len(steps))
	processed := make(map[string]bool, len(steps))
	for len(sorted) < len(steps) {
		for _, name := range orderedNames {
			if !processed[name] && inDegree[name] == 0 {
				processed[name] = true
				sorted = append(sorted, nameToNode[name])
				for _, next := range adj[name] {
					inDegree[next]--
				}
				break
			}
		}
	}
	return sorted
}

func isAllParallel(g WorkflowDefinitionGraph) bool {
	_, floating := classifySteps(g)
	return len(floating) == len(g.Steps)
}

func hasNoNestedChildren(steps []StepDefinitionNode) bool {
	for _, s := range steps {
		if len(s.ChildWorkflows) > 0 {
			return false
		}
	}
	return true
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
