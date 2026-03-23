package workflow

import "fmt"

type StepDefinition struct {
	Name            string
	SubWorkflowName string // non-empty means this step spawns a child workflow
	// DependsOn    []string // reserved for future DAG support
}

type WorkflowDefinition struct {
	Name  string
	Steps []StepDefinition
}

type WorkflowRegistry struct {
	definitions map[string]*WorkflowDefinition
}

func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		definitions: make(map[string]*WorkflowDefinition),
	}
}

func (r *WorkflowRegistry) Register(def *WorkflowDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("workflow definition name cannot be empty")
	}
	if _, exists := r.definitions[def.Name]; exists {
		return fmt.Errorf("workflow definition %q already registered", def.Name)
	}
	r.definitions[def.Name] = def
	return nil
}

func (r *WorkflowRegistry) Get(name string) (*WorkflowDefinition, error) {
	def, ok := r.definitions[name]
	if !ok {
		return nil, fmt.Errorf("workflow definition %q not found", name)
	}
	return def, nil
}

func (r *WorkflowRegistry) List() []*WorkflowDefinition {
	result := make([]*WorkflowDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		result = append(result, def)
	}
	return result
}
