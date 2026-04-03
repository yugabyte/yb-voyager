package workflow2

import "fmt"

type StepDefinition struct {
	Name           string
	DependsOn      []string
	ChildWorkflows []string // possible child workflows for this step
}

type WorkflowDefinition struct {
	Name  string
	Steps []StepDefinition
}

// StepOption is a functional option for AddStep.
type StepOption func(*StepDefinition)

func DependsOn(deps ...string) StepOption {
	return func(s *StepDefinition) {
		s.DependsOn = append(s.DependsOn, deps...)
	}
}

func ChildWorkflows(names ...string) StepOption {
	return func(s *StepDefinition) {
		s.ChildWorkflows = append(s.ChildWorkflows, names...)
	}
}

// Builder constructs a WorkflowDefinition incrementally.
type Builder struct {
	name  string
	steps []StepDefinition
	seen  map[string]bool
}

func NewBuilder(name string) *Builder {
	return &Builder{name: name, seen: make(map[string]bool)}
}

func (b *Builder) AddStep(name string, opts ...StepOption) *Builder {
	step := StepDefinition{Name: name}
	for _, opt := range opts {
		opt(&step)
	}
	b.steps = append(b.steps, step)
	b.seen[name] = true
	return b
}

func (b *Builder) Build() (WorkflowDefinition, error) {
	def := WorkflowDefinition{Name: b.name, Steps: b.steps}
	if err := validateDAG(def); err != nil {
		return WorkflowDefinition{}, err
	}
	return def, nil
}

func (b *Builder) MustBuild() WorkflowDefinition {
	def, err := b.Build()
	if err != nil {
		panic(err)
	}
	return def
}

func validateDAG(def WorkflowDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("workflow definition name cannot be empty")
	}
	names := make(map[string]bool, len(def.Steps))
	for _, s := range def.Steps {
		if s.Name == "" {
			return fmt.Errorf("step name cannot be empty in workflow %q", def.Name)
		}
		if names[s.Name] {
			return fmt.Errorf("duplicate step name %q in workflow %q", s.Name, def.Name)
		}
		names[s.Name] = true
	}
	for _, s := range def.Steps {
		for _, dep := range s.DependsOn {
			if !names[dep] {
				return fmt.Errorf("step %q in workflow %q depends on unknown step %q", s.Name, def.Name, dep)
			}
			if dep == s.Name {
				return fmt.Errorf("step %q in workflow %q depends on itself", s.Name, def.Name)
			}
		}
	}
	if err := detectCycle(def); err != nil {
		return err
	}
	return nil
}

func detectCycle(def WorkflowDefinition) error {
	adj := make(map[string][]string)
	for _, s := range def.Steps {
		adj[s.Name] = s.DependsOn
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)

	var visit func(string) error
	visit = func(name string) error {
		color[name] = gray
		for _, dep := range adj[name] {
			switch color[dep] {
			case gray:
				return fmt.Errorf("cycle detected in workflow %q involving step %q", def.Name, dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}

	for _, s := range def.Steps {
		if color[s.Name] == white {
			if err := visit(s.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// WorkflowDefinitionGraph is the recursive expansion of a WorkflowDefinition,
// resolving ChildWorkflows references into nested graphs.
type WorkflowDefinitionGraph struct {
	Definition WorkflowDefinition
	Steps      []StepDefinitionNode
}

type StepDefinitionNode struct {
	Step           StepDefinition
	ChildWorkflows []WorkflowDefinitionGraph
}

type workflowRegistry struct {
	definitions map[string]WorkflowDefinition
}

func newWorkflowRegistry() workflowRegistry {
	return workflowRegistry{definitions: make(map[string]WorkflowDefinition)}
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

func (r *workflowRegistry) getGraph(name string) (WorkflowDefinitionGraph, error) {
	def, err := r.get(name)
	if err != nil {
		return WorkflowDefinitionGraph{}, err
	}
	graph := WorkflowDefinitionGraph{Definition: def}
	for _, step := range def.Steps {
		node := StepDefinitionNode{Step: step}
		for _, childName := range step.ChildWorkflows {
			childGraph, err := r.getGraph(childName)
			if err != nil {
				return WorkflowDefinitionGraph{}, fmt.Errorf(
					"step %q references child workflow %q: %w", step.Name, childName, err)
			}
			node.ChildWorkflows = append(node.ChildWorkflows, childGraph)
		}
		graph.Steps = append(graph.Steps, node)
	}
	return graph, nil
}
