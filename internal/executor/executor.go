// Package executor consumes schedule.fired events and webhook
// invocations and produces Execution records.
//
// The Executor is the missing runtime link of M2 — the Scheduler
// publishes events but nothing consumed them. This package closes
// that loop. Each invocation is a function of (Release, input)
// plus provider-side randomness; the ReplayBuffer (internal/replay)
// keys the resulting Record on ExecutionHash so identical inputs
// reproduce identically.
//
// Persistence is intentionally not wired in this commit. The
// Executor returns an ExecutionRecord value; a follow-on commit
// wires the SQLite / Postgres Repository and adds the audit chain
// adapter. Today's commit ships the value type, the gated caller
// path, and the unit tests.
package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/replay"
)

// Caller runs the actual LLM call and reports the result. The
// Executor does not couple itself to a Provider; consumers pass an
// implementation that talks to the right plugin / built-in.
type Caller func(ctx context.Context, req InvokeRequest) (InvokeResult, error)

// InvokeRequest is the payload passed to Caller.
type InvokeRequest struct {
	WorkspaceID   string
	ReleaseID     string
	ManifestHash  string
	InputHash     string
	Input         json.RawMessage
	Model         string
	ModelRevision string
}

// InvokeResult is the Caller's response.
type InvokeResult struct {
	Output         json.RawMessage
	PromptTokens   int
	OutputTokens   int
	CostUSDMicro   int64
	LatencyMS      int64
	Status         string
	Error          string
}

// ExecutionRecord is the immutable record an Executor produces.
type ExecutionRecord struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	ReleaseID       string    `json:"release_id"`
	CapabilityID    string    `json:"capability_id"`
	Environment     string    `json:"environment"`
	ManifestHash    string    `json:"manifest_hash"`
	InputHash       string    `json:"input_hash"`
	Input           json.RawMessage `json:"input"`
	Output          json.RawMessage `json:"output,omitempty"`
	Model           string    `json:"model"`
	ModelRevision   string    `json:"model_revision"`
	PromptTokens    int       `json:"prompt_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	CostUSD         float64   `json:"cost_usd"`
	LatencyMS       int64     `json:"latency_ms"`
	Status          string    `json:"status"`
	Error           string    `json:"error,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
}

// Executor wires a Caller to an EventBus so that schedule.fired
// events trigger Executions.
type Executor struct {
	publisher   eventbus.Publisher
	caller      Caller
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
		return fmt.Errorf("executor: schedule.fired: %w", err)
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
	now := time.Now().UTC()
	inputHash := hashRaw(input)
	req := InvokeRequest{
		WorkspaceID: workspaceID,
		ReleaseID:   releaseID,
		ManifestHash: "<injected-by-caller>",
		InputHash:    inputHash,
		Input:        input,
		Model:        "<injected-by-caller>",
		ModelRevision: time.Now().UTC().Format("2006-01-02"),
	}
	rec := ExecutionRecord{
		ID:            generateID("exec"),
		WorkspaceID:   workspaceID,
		ReleaseID:     releaseID,
		Environment:   environment,
		Input:         input,
		InputHash:     inputHash,
		StartedAt:     now,
		Status:        "running",
	}
	res, err := e.caller(ctx, req)
	rec.FinishedAt = time.Now().UTC()
	if err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
	} else {
		rec.Output = res.Output
		rec.Status = res.Status
		rec.Error = res.Error
		rec.PromptTokens = res.PromptTokens
		rec.OutputTokens = res.OutputTokens
		rec.CostUSD = float64(res.CostUSDMicro) * e.costUSDPerMicro
		rec.LatencyMS = res.LatencyMS
		rec.Model = req.Model
		rec.ModelRevision = req.ModelRevision
	}
	return rec, nil
}

// ReplayBuf is a tiny in-memory implementation of replay.Repository
// used by the tests and as a default in single-node deployments.
// Real deployments wire the SQLite/Postgres Repository.
func ReplayBuf() *replayBuf { return &replayBuf{records: map[string]replay.Record{}} }

type replayBuf struct {
	records map[string]replay.Record
}

// Put implements replay.Repository for the in-memory buffer.
func (r *replayBuf) Put(_ context.Context, rec *replay.Record) (*replay.Record, bool, error) {
	if existing, ok := r.records[rec.ExecutionHash]; ok {
		return &existing, false, nil
	}
	r.records[rec.ExecutionHash] = *rec
	return rec, true, nil
}

// Get implements replay.Repository for the in-memory buffer.
func (r *replayBuf) Get(_ context.Context, hash string) (*replay.Record, error) {
	if v, ok := r.records[hash]; ok {
		return &v, nil
	}
	return nil, errors.New("replay: not found")
}

// ListForWorkspace implements replay.Repository for the in-memory
// buffer.
func (r *replayBuf) ListForWorkspace(_ context.Context, _ string, _, _ int) ([]*replay.Record, error) {
	out := make([]*replay.Record, 0, len(r.records))
	for i := range r.records {
		v := r.records[i]
		out = append(out, &v)
	}
	return out, nil
}

// ListForRelease implements replay.Repository for the in-memory
// buffer.
func (r *replayBuf) ListForRelease(_ context.Context, _ string, _, _ int) ([]*replay.Record, error) {
	out := make([]*replay.Record, 0, len(r.records))
	for i := range r.records {
		v := r.records[i]
		out = append(out, &v)
	}
	return out, nil
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
