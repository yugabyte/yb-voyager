package workflow

import "fmt"

type StepDefinition struct {
	Name               string
	SubWorkflowOptions []string // any of these child workflows may be spawned at runtime
}

type WorkflowDefinition struct {
	Name  string
	Steps []StepDefinition
}

// WorkflowDefinitionTree is the recursive expansion of a WorkflowDefinition,
// resolving SubWorkflowOptions references into nested trees.
type WorkflowDefinitionTree struct {
	Definition WorkflowDefinition
	Steps      []StepDefinitionNode
}

type StepDefinitionNode struct {
	Step           StepDefinition
	ChildWorkflows []WorkflowDefinitionTree // one per SubWorkflowOption
}

type workflowRegistry struct {
	definitions map[string]WorkflowDefinition
}

func newWorkflowRegistry() workflowRegistry {
	return workflowRegistry{
		definitions: make(map[string]WorkflowDefinition),
	}
}

func (r *workflowRegistry) register(def WorkflowDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("workflow definition name cannot be empty")
	}
	if _, exists := r.definitions[def.Name]; exists {
		return fmt.Errorf("workflow definition %q already registered", def.Name)
	}
	r.definitions[def.Name] = def
	return nil
}

func (r *workflowRegistry) get(name string) (WorkflowDefinition, error) {
	def, ok := r.definitions[name]
	if !ok {
		return WorkflowDefinition{}, fmt.Errorf("workflow definition %q not found", name)
	}
	return def, nil
}

func (r *workflowRegistry) getTree(name string) (WorkflowDefinitionTree, error) {
	def, err := r.get(name)
	if err != nil {
		return WorkflowDefinitionTree{}, err
	}
	tree := WorkflowDefinitionTree{Definition: def}
	for _, step := range def.Steps {
		node := StepDefinitionNode{Step: step}
		for _, opt := range step.SubWorkflowOptions {
			childTree, err := r.getTree(opt)
			if err != nil {
				return WorkflowDefinitionTree{}, fmt.Errorf(
					"step %q references sub-workflow %q: %w", step.Name, opt, err)
			}
			node.ChildWorkflows = append(node.ChildWorkflows, childTree)
		}
		tree.Steps = append(tree.Steps, node)
	}
	return tree, nil
}
