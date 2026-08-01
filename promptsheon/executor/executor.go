// Package executor consumes schedule.fired events and webhook
// invocations and produces Execution records.
//
// Each invocation is a function of (Release, input) plus
// provider-side randomness; the ReplayBuffer (internal/replay)
// keys the resulting Record on ExecutionHash so identical inputs
// reproduce identically.
//
// Persistence is not wired here. The Executor returns an
// ExecutionRecord value; a follow-on commit wires the Repository
// and adds the audit chain adapter. This package ships the value
// type, the gated caller path, and the unit tests.
package executor

import (
	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/eventbus"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

)

// Caller runs the actual LLM call and reports the result. The
// Executor does not couple itself to a Provider; consumers pass an
// implementation that talks to the right plugin / built-in.
type Caller func(ctx context.Context, req InvokeRequest) (InvokeResult, error)

// errs.ErrProviderMissing is returned by a Caller when the request names
// a model whose provider is not registered, or names no provider at
// all. The handler maps this to a 502 Bad Gateway so operators can
// distinguish "no provider configured" from "provider failed"
// without reading the daemon log. RunRequest propagates it as a Go
// error so the handler's errors.Is check fires (BUG-19).

// InvokeRequest is the payload passed to Caller.
type InvokeRequest struct {
	WorkspaceID   string
	ReleaseID     string
	ManifestHash  string
	InputHash     string
	Input         json.RawMessage
	Model         string
	ModelRevision string
	Provider      string
	// SystemPrompt is the resolved manifest's prompt artifact bytes.
	// The caller MUST prepend it as a system-role message before the
	// user message; the harness/engine depend on the model seeing it.
	// ponytail: previously dropped — the eval ran with no instruction
	// and the model answered generically, so the scorer treated real
	// answers as fails.
	SystemPrompt json.RawMessage
}

// InvokeResult is the Caller's response.
type InvokeResult struct {
	Output       json.RawMessage
	PromptTokens int
	OutputTokens int
	CostUSDMicro int64
	LatencyMS    int64
	Status       string
	Error        string
}

// ExecutionRecord is the immutable record an Executor produces.
type ExecutionRecord struct {
	ID            string          `json:"id"`
	WorkspaceID   string          `json:"workspace_id"`
	ReleaseID     string          `json:"release_id"`
	CapabilityID  string          `json:"capability_id"`
	Environment   string          `json:"environment"`
	ManifestHash  string          `json:"manifest_hash"`
	InputHash     string          `json:"input_hash"`
	Input         json.RawMessage `json:"input"`
	Output        json.RawMessage `json:"output,omitempty"`
	Model         string          `json:"model"`
	ModelRevision string          `json:"model_revision"`
	PromptTokens  int             `json:"prompt_tokens"`
	OutputTokens  int             `json:"output_tokens"`
	CostUSD       float64         `json:"cost_usd"`
	LatencyMS     int64           `json:"latency_ms"`
	Status        string          `json:"status"`
	Error         string          `json:"error,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    time.Time       `json:"finished_at"`
}

// Executor wires a Caller to an EventBus so that schedule.fired
// events trigger Executions.
type Executor struct {
	publisher       eventbus.Publisher
	caller          Caller
	costUSDPerMicro float64
}

// New constructs an Executor. costUSDPerMicro converts micro-USD
// from Caller.CostUSDMicro into float-dollar for the
// ExecutionRecord; the default 1e-6 is correct (the value is
// already in micro-USD).
func New(p eventbus.Publisher, caller Caller) *Executor {
	return &Executor{publisher: p, caller: caller, costUSDPerMicro: 1e-6}
}

// HandleScheduleEvent is the EventBus subscriber body that turns
// one schedule.fired event into one Execution. It is exposed so
// tests can drive the executor without the EventBus indirection.
//
// In production the Executor subscribes to the EventBus at boot
// and the bus dispatches directly.
func (e *Executor) HandleScheduleEvent(ctx context.Context, ev capability.Event) error {
	workspaceID, _ := ev.Data["workspace_id"].(string)
	releaseID, _ := ev.Data["release_id"].(string)
	if workspaceID == "" || releaseID == "" {
		return errors.New("executor: schedule.fired missing workspace_id or release_id")
	}
	rec, err := e.Run(ctx, workspaceID, releaseID, "", json.RawMessage(`{}`))
	if err != nil {
		return errf.Errorf("executor: schedule.fired: %w", err)
	}
	return e.publisher.Publish(capability.Event{
		Type:          capability.EventExecutionFinished,
		AggregateID:   rec.ID,
		AggregateType: "execution",
		Data: map[string]any{
			"workspace_id": rec.WorkspaceID,
			"release_id":   rec.ReleaseID,
			"status":       rec.Status,
			"cost_usd":     rec.CostUSD,
			"latency_ms":   rec.LatencyMS,
		},
	})
}

// Run executes one invocation against the supplied Release context.
// Inputs are the request body (Raw JSON); the model and revision
// come from the supplied arguments.
//
// In the production wiring the caller looks up the active Release
// for the (Workspace, Capability, Environment) tuple; this method
// keeps that lookup out of the test path.
func (e *Executor) Run(ctx context.Context, workspaceID, releaseID, environment string, input json.RawMessage) (ExecutionRecord, error) {
	return e.RunRequest(ctx, InvokeRequest{
		WorkspaceID:   workspaceID,
		ReleaseID:     releaseID,
		ManifestHash:  deriveManifestHash(workspaceID, releaseID),
		InputHash:     hashRaw(input),
		Input:         input,
		Model:         "<unspecified>",
		ModelRevision: time.Now().UTC().Format("2006-01-02"),
	}, environment)
}

// RunRequest executes an invocation using the supplied InvokeRequest.
// The caller is responsible for filling ManifestHash, Model, and
// ModelRevision from the production lookup; this method does not
// second-guess them.
func (e *Executor) RunRequest(ctx context.Context, req InvokeRequest, environment string) (ExecutionRecord, error) {
	now := time.Now().UTC()
	if req.InputHash == "" {
		req.InputHash = hashRaw(req.Input)
	}
	rec := ExecutionRecord{
		ID:            generateID("exec"),
		WorkspaceID:   req.WorkspaceID,
		ReleaseID:     req.ReleaseID,
		Environment:   environment,
		ManifestHash:  req.ManifestHash,
		InputHash:     req.InputHash,
		Input:         req.Input,
		Model:         req.Model,
		ModelRevision: req.ModelRevision,
		StartedAt:     now,
		Status:        "running",
	}
	res, err := e.caller(ctx, req)
	rec.FinishedAt = time.Now().UTC()
	if err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		// ponytail: BUG-19 used to swallow non-errs.ErrProviderMissing
		// errors (return nil) so the API path could 201 with the
		// error in the body. That hid provider-side errors from
		// the eval harness, which only sees Go-errors. Now we
		// surface every error; the API handler decides what status
		// to return based on rec.Status and the error.
		return rec, err
	}
	rec.Output = res.Output
	rec.Status = res.Status
	rec.Error = res.Error
	rec.PromptTokens = res.PromptTokens
	rec.OutputTokens = res.OutputTokens
	rec.CostUSD = float64(res.CostUSDMicro) * e.costUSDPerMicro
	rec.LatencyMS = res.LatencyMS
	return rec, nil
}

// deriveManifestHash returns a deterministic placeholder when no
// manifest hash is supplied. Real production paths fill it from
// the active Release lookup.
func deriveManifestHash(workspaceID, releaseID string) string {
	h := sha256.New()
	h.Write([]byte(workspaceID))
	h.Write([]byte{0x1f})
	h.Write([]byte(releaseID))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// hashRaw returns the SHA-256 hex of a JSON RawMessage. Empty
// inputs hash to the empty-string hex.
func hashRaw(r json.RawMessage) string {
	if len(r) == 0 {
		return ""
	}
	sum := sha256.Sum256(r)
	return hex.EncodeToString(sum[:])
}

func generateID(prefix string) string {
	b := make([]byte, 12)
	now := time.Now().UTC().UnixNano()
	for i := range b {
		b[i] = byte((now >> (i % 8)) & 0xff)
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}
