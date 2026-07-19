// Package workflow: workflow Engine.Run.
package workflow

import (
	"context"
	"fmt"
	"time"
)

// Step is one node in a workflow DAG. The Engine runs steps in
// declaration order; future revisions can interpret DependsOn for
// parallel execution. For now the workflow is a strictly
// sequential list of (tool, input) pairs.
type Step struct {
	ID     string         `json:"id"`
	Tool   string         `json:"tool"`
	Input  map[string]any `json:"input"`
	Output string         `json:"output,omitempty"` // named output key, optional
}

// Definition is the static description of a workflow. The caller
// supplies it; the engine executes it.
type Definition struct {
	ID    string `json:"id"`
	Steps []Step `json:"steps"`
}

// Run executes the supplied workflow sequentially. Each step's
// output is recorded in the Result.Steps map; if Output is set
// the output is also merged into the next step's Input under
// that key (cross-step data flow). A step failure marks the
// workflow failed and short-circuits the remaining steps; the
// caller can inspect StepResult to find the failure.
//
// The engine is concurrency-safe and stateless: a new Engine
// instance can be created per request, or the same Engine can
// serve many concurrent requests.
func (e *Engine) Run(ctx context.Context, def Definition, initial map[string]any) (*Result, error) {
	if def.ID == "" {
		return nil, fmt.Errorf("workflow: definition id is required")
	}
	if len(def.Steps) == 0 {
		return nil, fmt.Errorf("workflow: definition has no steps")
	}
	res := &Result{
		WorkflowID: def.ID,
		Steps:      make(map[string]*StepResult, len(def.Steps)),
		Outputs:    copyMap(initial),
		StartedAt:  time.Now().UTC(),
	}
	scope := e.toolRegistry
	if scope == nil {
		scope = DefaultRegistry()
	}
	for _, step := range def.Steps {
		stepRes, err := e.runStep(ctx, scope, step, res.Outputs)
		res.Steps[step.ID] = stepRes
		if err != nil {
			res.Status = StatusFailed
			res.Error = err.Error()
			res.FinishedAt = time.Now().UTC()
			return res, err
		}
		if step.Output != "" && stepRes.Output != nil {
			res.Outputs[step.Output] = stepRes.Output
		}
	}
	res.Status = StatusCompleted
	res.FinishedAt = time.Now().UTC()
	return res, nil
}

// runStep executes one step and returns the structured result.
func (e *Engine) runStep(ctx context.Context, reg *Registry, step Step, inputs map[string]any) (*StepResult, error) {
	tool, ok := reg.Get(step.Tool)
	if !ok {
		return &StepResult{
			StepID: step.ID,
			Status: StatusFailed,
			Error:  fmt.Sprintf("tool %q is not registered", step.Tool),
		}, fmt.Errorf("workflow: step %s references unregistered tool %q", step.ID, step.Tool)
	}
	// Merge workflow outputs into the step input so a step can
	// reference the previous step's named output. The step's
	// own Input map takes precedence on conflict.
	merged := copyMap(inputs)
	for k, v := range step.Input {
		merged[k] = v
	}
	start := time.Now().UTC()
	out, err := tool.Execute(ctx, merged)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &StepResult{
			StepID:    step.ID,
			Status:    StatusFailed,
			Output:    out,
			LatencyMs: latency,
			Error:     err.Error(),
		}, err
	}
	return &StepResult{
		StepID:    step.ID,
		Status:    StatusCompleted,
		Output:    out,
		LatencyMs: latency,
	}, nil
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
