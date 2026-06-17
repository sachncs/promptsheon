// Package workflow implements a DAG-based execution engine for multi-step
// agent workflows with tool integration and dependency resolution.
package workflow

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"promptsheon/internal/models"
)

// Status represents the state of a workflow or step.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusSkipped   Status = "skipped"
)

// StepResult holds the output of a single step execution.
type StepResult struct {
	StepID    string         `json:"step_id"`
	Status    Status         `json:"status"`
	Output    map[string]any `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	ToolCalls []models.ToolCall `json:"tool_calls,omitempty"`
	LatencyMs int64          `json:"latency_ms"`
}

// WorkflowResult holds the full output of a workflow execution.
type WorkflowResult struct {
	WorkflowID string                 `json:"workflow_id"`
	Status     Status                 `json:"status"`
	Steps      map[string]*StepResult `json:"steps"`
	Outputs    map[string]any         `json:"outputs"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt time.Time              `json:"finished_at"`
	Error      string                 `json:"error,omitempty"`
}

// Engine resolves dependencies, schedules steps, and executes them.
type Engine struct {
	toolRegistry *Registry
	llmProvider  LLMProvider
}

// LLMProvider is the subset of llm.Provider needed by the workflow engine.
type LLMProvider interface {
	Complete(ctx context.Context, req interface{}) (interface{}, error)
}

// NewEngine creates a workflow engine with the given tool registry.
func NewEngine(registry *Registry) *Engine {
	return &Engine{toolRegistry: registry}
}

// Execute runs a workflow defined by an Agent, resolving dependencies and
// executing steps in topological order with parallelism for independent steps.
func (e *Engine) Execute(ctx context.Context, agent *models.Agent, input map[string]any) (*WorkflowResult, error) {
	if len(agent.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}

	// Validate DAG (detect cycles)
	if err := validateDAG(agent.Steps); err != nil {
		return nil, fmt.Errorf("invalid workflow DAG: %w", err)
	}

	result := &WorkflowResult{
		WorkflowID: agent.ID,
		Status:     StatusRunning,
		Steps:      make(map[string]*StepResult),
		Outputs:    make(map[string]any),
		StartedAt:  time.Now(),
	}

	// Copy initial input to outputs
	for k, v := range input {
		result.Outputs[k] = v
	}

	// Compute execution levels (topological sort)
	levels := topologicalLevels(agent.Steps)

	// Execute level by level
	for _, level := range levels {
		if ctx.Err() != nil {
			result.Status = StatusCancelled
			result.Error = "workflow cancelled"
			result.FinishedAt = time.Now()
			return result, ctx.Err()
		}

		// Execute all steps in this level concurrently
		if err := e.executeLevel(ctx, agent, level, result); err != nil {
			result.Status = StatusFailed
			result.Error = err.Error()
			result.FinishedAt = time.Now()
			return result, nil
		}
	}

	result.Status = StatusCompleted
	result.FinishedAt = time.Now()
	return result, nil
}

func (e *Engine) executeLevel(ctx context.Context, agent *models.Agent, level []string, result *WorkflowResult) error {
	// Build step lookup
	stepMap := make(map[string]models.AgentStep)
	for _, s := range agent.Steps {
		stepMap[s.ID] = s
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, stepID := range level {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			step := stepMap[id]
			stepResult := e.executeStep(ctx, agent, &step, result.Outputs)

			mu.Lock()
			defer mu.Unlock()

			if stepResult.Status == StatusFailed {
				if firstErr == nil {
					firstErr = fmt.Errorf("step %s failed: %s", id, stepResult.Error)
				}
			}

			// Store step output
			if stepResult.Output != nil {
				for k, v := range stepResult.Output {
					result.Outputs[k] = v
				}
			}

			// Mark dependent steps as skipped if this step failed
			if stepResult.Status == StatusFailed {
				for _, dependent := range agent.Steps {
					for _, dep := range dependent.DependsOn {
						if dep == id {
							result.Steps[dependent.ID] = &StepResult{
								StepID: dependent.ID,
								Status: StatusSkipped,
								Error:  fmt.Sprintf("dependency %s failed", id),
							}
						}
					}
				}
			}

			result.Steps[id] = stepResult
		}(stepID)
	}

	wg.Wait()
	return firstErr
}

func (e *Engine) executeStep(ctx context.Context, agent *models.Agent, step *models.AgentStep, outputs map[string]any) *StepResult {
	start := time.Now()
	result := &StepResult{
		StepID: step.ID,
		Status: StatusRunning,
	}

	// Evaluate branching condition (if set)
	if step.Condition != nil {
		if !evaluateCondition(step.Condition, outputs) {
			result.Status = StatusSkipped
			result.Error = "condition not met"
			result.LatencyMs = time.Since(start).Milliseconds()
			return result
		}
	}

	// Gather inputs from dependencies
	stepInput := make(map[string]any)
	for _, depID := range step.DependsOn {
		if out, ok := outputs[depID]; ok {
			stepInput[depID] = out
		}
	}

	// Execute tool calls
	for _, tc := range step.ToolCalls {
		tool, ok := e.toolRegistry.Get(tc.Tool)
		if !ok {
			result.Status = StatusFailed
			result.Error = fmt.Sprintf("tool not found: %s", tc.Tool)
			result.LatencyMs = time.Since(start).Milliseconds()
			return result
		}

		// Merge step input with tool call input
		input := make(map[string]any)
		for k, v := range stepInput {
			input[k] = v
		}
		for k, v := range tc.Input {
			input[k] = v
		}

		toolStart := time.Now()
		output, err := tool.Execute(ctx, input)
		toolLatency := time.Since(toolStart).Milliseconds()

		recorded := models.ToolCall{
			Tool:      tc.Tool,
			Input:     tc.Input,
			Output:    output,
			LatencyMs: toolLatency,
		}
		if err != nil {
			recorded.Error = err.Error()
			result.ToolCalls = append(result.ToolCalls, recorded)
			result.Status = StatusFailed
			result.Error = fmt.Errorf("tool %s: %w", tc.Tool, err).Error()
			result.LatencyMs = time.Since(start).Milliseconds()
			return result
		}
		result.ToolCalls = append(result.ToolCalls, recorded)

		// Store output under the tool name
		if step.OutputKey != "" {
			stepInput[step.OutputKey] = output
		} else {
			stepInput[tc.Tool] = output
		}
	}

	result.Output = stepInput
	result.Status = StatusCompleted
	result.LatencyMs = time.Since(start).Milliseconds()
	return result
}

// topologicalLevels returns steps grouped by dependency level.
// Steps in the same level have no dependencies on each other and can
// run in parallel.
func topologicalLevels(steps []models.AgentStep) [][]string {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for _, s := range steps {
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			inDegree[s.ID]++
			dependents[dep] = append(dependents[dep], s.ID)
		}
	}

	var levels [][]string
	for {
		// Find all steps with in-degree 0
		var currentLevel []string
		for id, deg := range inDegree {
			if deg == 0 {
				currentLevel = append(currentLevel, id)
			}
		}
		if len(currentLevel) == 0 {
			break
		}

		// Sort for determinism
		sort.Strings(currentLevel)
		levels = append(levels, currentLevel)

		// Remove current level from graph
		for _, id := range currentLevel {
			delete(inDegree, id)
			for _, dep := range dependents[id] {
				inDegree[dep]--
			}
		}
	}

	return levels
}

// validateDAG checks for cycles in the step dependency graph.
func validateDAG(steps []models.AgentStep) error {
	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done

	var visit func(id string) error
	visit = func(id string) error {
		if visited[id] == 2 {
			return nil
		}
		if visited[id] == 1 {
			return fmt.Errorf("cycle detected at step %s", id)
		}
		visited[id] = 1

		for _, s := range steps {
			if s.ID == id {
				for _, dep := range s.DependsOn {
					if err := visit(dep); err != nil {
						return err
					}
				}
			}
		}
		visited[id] = 2
		return nil
	}

	for _, s := range steps {
		if err := visit(s.ID); err != nil {
			return err
		}
	}
	return nil
}

// evaluateCondition checks if a condition is satisfied against the current outputs.
func evaluateCondition(cond *models.Condition, outputs map[string]any) bool {
	val, exists := outputs[cond.Field]
	valStr := toString(val)

	switch cond.Operator {
	case "eq":
		return exists && valStr == cond.Value
	case "neq":
		return !exists || valStr != cond.Value
	case "contains":
		return exists && strings.Contains(valStr, cond.Value)
	case "gt":
		if !exists {
			return false
		}
		fv, err1 := strconv.ParseFloat(valStr, 64)
		cv, err2 := strconv.ParseFloat(cond.Value, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		return fv > cv
	case "lt":
		if !exists {
			return false
		}
		fv, err1 := strconv.ParseFloat(valStr, 64)
		cv, err2 := strconv.ParseFloat(cond.Value, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		return fv < cv
	case "exists":
		return exists
	default:
		return true
	}
}
