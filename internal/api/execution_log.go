package api

import (
	"context"
	"time"

	"github.com/sachn-cs/promptsheon/internal/llm"
	"github.com/sachn-cs/promptsheon/internal/models"
)

// executionLogInput is the small bundle of fields every prompt
// execution produces. The helper below only reads it; defining a
// struct keeps the call sites symmetric between the streaming
// and non-streaming code paths in handleRunPrompt.
type executionLogInput struct {
	Prompt     *models.Prompt
	Provider   string
	Model      string
	Usage      llm.Usage
	CostUSD    float64
	Latency    time.Duration
	TraceID    string
	Status     string
	Variables  map[string]string
	SysPrompt  string
	MsgCount   int
	RecordedAt time.Time
}

// recordExecutionLog persists a single execution_log row. The
// helper exists to remove a 20-line duplicated block from
// handleRunPrompt: the streaming branch and the non-streaming
// fallback both produce the same row, and keeping one copy
// means a future schema change touches one place instead of
// two.
//
// The function never returns an error: the underlying
// SaveExecutionLog is fire-and-forget on the request goroutine,
// matching the previous behaviour. Callers that want richer
// error reporting can replace this helper with a returned
// error and propagate it up.
func (s *Server) recordExecutionLog(ctx context.Context, in executionLogInput) {
	if s.db == nil {
		return
	}
	recorded := in.RecordedAt
	if recorded.IsZero() {
		recorded = time.Now()
	}
	status := in.Status
	if status == "" {
		status = "success"
	}
	row := &models.ExecutionLog{
		ID:               generateID(),
		PromptID:         in.Prompt.ID,
		PromptName:       in.Prompt.Name,
		PromptVersion:    in.Prompt.Version,
		Provider:         in.Provider,
		Model:            in.Model,
		Status:           status,
		Variables:        in.Variables,
		SystemPrompt:     in.SysPrompt,
		RequestMessages:  in.MsgCount,
		PromptTokens:     in.Usage.PromptTokens,
		CompletionTokens: in.Usage.CompletionTokens,
		TotalTokens:      in.Usage.TotalTokens,
		CostUSD:          in.CostUSD,
		LatencyMs:        in.Latency.Milliseconds(),
		TraceID:          in.TraceID,
		Environment:      in.Prompt.Environment,
		CreatedAt:        recorded,
	}
	s.db.SaveExecutionLog(ctx, row)
}
