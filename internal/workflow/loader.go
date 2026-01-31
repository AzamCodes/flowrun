package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load reads a workflow file from the given path, parses it, and validates it.
func Load(path string) (*Workflow, error) {
	// Clean the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path '%s': %w", path, err)
	}

	// Read file definitions
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workflow file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	// Parse YAML
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Validate
	if err := wf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid workflow definition: %w", err)
	}

	return &wf, nil
}
