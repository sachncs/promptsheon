package workflow

import (
	"fmt"
	"strings"

	"promptsheon/internal/models"
)

// ValidateSteps checks a set of agent steps for DAG validity.
// Exported for use by API handlers at save time.
func ValidateSteps(steps []models.AgentStep) []ValidationError {
	var errors []ValidationError

	if len(steps) == 0 {
		return errors
	}

	// Build step ID set
	stepIDs := make(map[string]bool)
	for _, s := range steps {
		if s.ID == "" {
			errors = append(errors, ValidationError{
				Field:   "steps",
				Message: "step ID cannot be empty",
			})
			continue
		}
		if stepIDs[s.ID] {
			errors = append(errors, ValidationError{
				Field:   "steps",
				Message: fmt.Sprintf("duplicate step ID: %s", s.ID),
			})
		}
		stepIDs[s.ID] = true
	}

	// Check dependencies exist
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if !stepIDs[dep] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("step[%s].depends_on", s.ID),
					Message: fmt.Sprintf("dependency %q does not exist", dep),
				})
			}
		}
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var hasCycle bool

	var dfs func(id string)
	dfs = func(id string) {
		if inStack[id] {
			hasCycle = true
			return
		}
		if visited[id] {
			return
		}
		visited[id] = true
		inStack[id] = true

		for _, s := range steps {
			if s.ID == id {
				for _, dep := range s.DependsOn {
					dfs(dep)
				}
				break
			}
		}
		inStack[id] = false
	}

	for _, s := range steps {
		dfs(s.ID)
		if hasCycle {
			errors = append(errors, ValidationError{
				Field:   "steps",
				Message: "circular dependency detected in step graph",
			})
			break
		}
	}

	return errors
}

// ValidationError represents a validation issue.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ParseYAML parses a simple YAML-like workflow configuration.
// Supports basic YAML syntax: key-value pairs, lists, nested objects.
func ParseYAML(data []byte) (*models.Agent, error) {
	lines := strings.Split(string(data), "\n")
	agent := &models.Agent{
		Steps: []models.AgentStep{},
		Tools: []models.ToolRef{},
	}

	var currentStep *models.AgentStep
	indent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Parse key-value
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		currentIndent := len(line) - len(strings.TrimLeft(line, " "))

		switch {
		case key == "name" && currentIndent == 0:
			agent.Name = value
		case key == "description" && currentIndent == 0:
			agent.Description = value
		case key == "steps" && currentIndent == 0:
			indent = currentIndent
		case key == "id" && currentIndent > indent:
			if currentStep == nil {
				currentStep = &models.AgentStep{}
				agent.Steps = append(agent.Steps, *currentStep)
			}
			agent.Steps[len(agent.Steps)-1].ID = value
		case key == "prompt_id" && currentStep != nil:
			agent.Steps[len(agent.Steps)-1].PromptID = value
		case key == "depends_on" && currentStep != nil && value != "[]":
			deps := strings.Split(value, ",")
			for j := range deps {
				deps[j] = strings.TrimSpace(deps[j])
			}
			agent.Steps[len(agent.Steps)-1].DependsOn = deps
		case key == "output_key" && currentStep != nil:
			agent.Steps[len(agent.Steps)-1].OutputKey = value
		}

		_ = i
	}

	if agent.Name == "" {
		return nil, fmt.Errorf("workflow must have a name")
	}

	return agent, nil
}

// ExportYAML exports an agent to a simple YAML format.
func ExportYAML(a *models.Agent) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("name: %s\n", a.Name))
	if a.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", a.Description))
	}

	if len(a.Steps) > 0 {
		sb.WriteString("steps:\n")
		for _, s := range a.Steps {
			sb.WriteString(fmt.Sprintf("  - id: %s\n", s.ID))
			if s.PromptID != "" {
				sb.WriteString(fmt.Sprintf("    prompt_id: %s\n", s.PromptID))
			}
			if len(s.DependsOn) > 0 {
				sb.WriteString(fmt.Sprintf("    depends_on: %s\n", strings.Join(s.DependsOn, ", ")))
			}
			if s.Condition != nil {
				sb.WriteString("    condition:\n")
				sb.WriteString(fmt.Sprintf("      field: %s\n", s.Condition.Field))
				sb.WriteString(fmt.Sprintf("      operator: %s\n", s.Condition.Operator))
				sb.WriteString(fmt.Sprintf("      value: %s\n", s.Condition.Value))
			}
		}
	}

	return []byte(sb.String()), nil
}
