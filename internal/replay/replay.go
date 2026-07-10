// Package replay owns the write-once replay buffer and the API path
// for deterministic Execution re-runs.
//
// An Execution is captured into the replay buffer at the moment a
// model response is observed. Each record carries a SHA-256
// ExecutionHash that fixes the (manifest_hash, input_hash,
// model_revision) tuple; identical inputs to the same Manifest on
// the same model revision produce the same ExecutionHash, and a
// replay is the canonical way to debug a regression or audit a
// decision.
//
// The replay buffer is content-addressed. A second observation of
// the same ExecutionHash returns the original record; we dedupe
// rather than append so the buffer cannot drift into multiple
// contradictory copies of the same execution.
package replay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// Record is one immutable observation of one execution.
type Record struct {
	ExecutionHash  string          `json:"execution_hash"`
	WorkspaceID    string          `json:"workspace_id"`
	ReleaseID      string          `json:"release_id"`
	ManifestHash   string          `json:"manifest_hash"`
	InputHash      string          `json:"input_hash"`
	Model          string          `json:"model"`
	ModelRevision  string          `json:"model_revision"`
	Input          json.RawMessage `json:"input"`
	Output         json.RawMessage `json:"output"`
	PromptTokens   int             `json:"prompt_tokens"`
	OutputTokens   int             `json:"output_tokens"`
	CostUSD        float64         `json:"cost_usd"`
	LatencyMS      int64           `json:"latency_ms"`
	Status         string          `json:"status"`
	Error          string          `json:"error,omitempty"`
	CapturedAt     time.Time       `json:"captured_at"`
}

// HashInput is the bundle of fields used to compute the ExecutionHash.
// Adding a field here changes every existing hash, so the field set
// is frozen by the package documentation.
type HashInput struct {
	WorkspaceID   string
	ReleaseID     string
	ManifestHash  string
	InputHash     string
	Model         string
	ModelRevision string
}

// Hash returns the content-addressed hash for one execution. Inputs
// are canonicalised through JSON so callers cannot construct two
// hashes from semantically identical records.
func Hash(in HashInput) (string, error) {
	if in.WorkspaceID == "" || in.ReleaseID == "" || in.ManifestHash == "" || in.InputHash == "" || in.Model == "" {
		return "", errors.New("replay: hash input missing required fields")
	}
	b, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// RecordFor builds a Record from inputs and outputs, computing the
// ExecutionHash and timestamp.
func RecordFor(
	workspaceID, releaseID, manifestHash, inputHash, model, modelRevision string,
	input, output json.RawMessage,
	promptTokens, outputTokens int, costUSD float64, latencyMS int64,
	status, errorMsg string,
	at time.Time,
) (Record, error) {
	h, err := Hash(HashInput{
		WorkspaceID:   workspaceID,
		ReleaseID:     releaseID,
		ManifestHash:  manifestHash,
		InputHash:     inputHash,
		Model:         model,
		ModelRevision: modelRevision,
	})
	if err != nil {
		return Record{}, err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return Record{
		ExecutionHash: h,
		WorkspaceID:   workspaceID,
		ReleaseID:     releaseID,
		ManifestHash:  manifestHash,
		InputHash:     inputHash,
		Model:         model,
		ModelRevision: modelRevision,
		Input:         input,
		Output:        output,
		PromptTokens:  promptTokens,
		OutputTokens:  outputTokens,
		CostUSD:       costUSD,
		LatencyMS:     latencyMS,
		Status:        status,
		Error:         errorMsg,
		CapturedAt:    at,
	}, nil
}

// Repository is the consumer-defined persistence interface for the
// replay buffer. The SQLite implementation lives in
// internal/store; a Postgres implementation lives in
// internal/store/postgres (M1 follow-on). Content-addressed dedup
// is a write-time invariant enforced by the implementations.
type Repository interface {
	// Put returns the existing record when the ExecutionHash is
	// already known, or stores the supplied record and returns it
	// when it is new. Implementations must never overwrite an
	// existing record.
	Put(ctx context.Context, r *Record) (existing *Record, isNew bool, err error)
	Get(ctx context.Context, executionHash string) (*Record, error)
	ListForWorkspace(ctx context.Context, workspaceID string, limit, offset int) ([]*Record, error)
	ListForRelease(ctx context.Context, releaseID string, limit, offset int) ([]*Record, error)
}
