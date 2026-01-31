package workflow

import (
	"errors"
	"fmt"
	"time"
)

// Workflow represents a sequence of steps to execute.
type Workflow struct {
	Name        string            `yaml:"name"`
	Env         map[string]string `yaml:"env,omitempty"`
	RequiredEnv []string          `yaml:"required_env,omitempty"`
	FailFast    *bool             `yaml:"fail_fast,omitempty"` // Default true
	Steps       []Step            `yaml:"steps"`
}

// Step represents a single command to execute.
type Step struct {
	Name     string            `yaml:"name"`
	Run      string            `yaml:"run"`
	Timeout  string            `yaml:"timeout,omitempty"`
	Retry    int               `yaml:"retry,omitempty"`
	Tags     []string          `yaml:"tags,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Needs    []string          `yaml:"needs,omitempty"`
	Parallel bool              `yaml:"parallel,omitempty"`
}

// IsFailFast returns true if execution should stop on first error.
func (w *Workflow) IsFailFast() bool {
	if w.FailFast == nil {
		return true // Default
	}
	return *w.FailFast
}

// Validate checks if the workflow definition is valid.
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return errors.New("workflow name is required")
	}

	if len(w.Steps) == 0 {
		return errors.New("workflow must have at least one step")
	}

	stepNames := make(map[string]bool)
	for _, s := range w.Steps {
		if stepNames[s.Name] {
			return fmt.Errorf("duplicate step name '%s'", s.Name)
		}
		stepNames[s.Name] = true
	}

	for i, step := range w.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("step %d ('%s') error: %w", i+1, step.Name, err)
		}

		// Validate dependencies
		for _, dep := range step.Needs {
			if !stepNames[dep] {
				return fmt.Errorf("step '%s' depends on undefined step '%s'", step.Name, dep)
			}
			if dep == step.Name {
				return fmt.Errorf("step '%s' cannot depend on itself", step.Name)
			}
		}
	}

	if err := w.detectCycles(); err != nil {
		return err
	}

	return nil
}

// detectCycles checks for circular dependencies.
func (w *Workflow) detectCycles() error {
	adj := make(map[string][]string)
	for _, s := range w.Steps {
		adj[s.Name] = s.Needs
	}

	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var visit func(string) error
	visit = func(node string) error {
		visited[node] = true
		recursionStack[node] = true

		for _, dep := range adj[node] {
			if !visited[dep] {
				if err := visit(dep); err != nil {
					return err
				}
			} else if recursionStack[dep] {
				return fmt.Errorf("circular dependency detected involving '%s'", node)
			}
		}

		recursionStack[node] = false
		return nil
	}

	for _, s := range w.Steps {
		if !visited[s.Name] {
			if err := visit(s.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate checks if the step definition is valid.
func (s *Step) Validate() error {
	if s.Name == "" {
		return errors.New("step name is required")
	}
	if s.Run == "" {
		return errors.New("step run command is required")
	}
	if s.Timeout != "" {
		if _, err := time.ParseDuration(s.Timeout); err != nil {
			return fmt.Errorf("invalid timeout '%s': %w", s.Timeout, err)
		}
	}
	if s.Retry < 0 {
		return errors.New("retry cannot be negative")
	}
	return nil
}
