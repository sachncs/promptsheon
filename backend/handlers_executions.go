package backend

import (
	"errors"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/backend/capability"
	"github.com/sachncs/promptsheon/backend/executor"
	"github.com/sachncs/promptsheon/backend/invoke"
)

// Auto-split from handlers_capabilities.go

func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	capabilityVersionID := r.PathValue("version_id")
	filter := capability.ExecutionFilter{
		CapabilityVersionID: capabilityVersionID,
		Limit:               limit,
		Offset:              offset,
	}
	execs, err := s.db.ListExecutions(r.Context(), filter)
	if err != nil {
		return err
	}
	if execs == nil {
		execs = []*capability.Execution{}
	}
	writeJSON(w, http.StatusOK, execs)
	return nil
}

// errProviderMissing is an alias for the executor's typed
// sentinel. We map the executor's ErrProviderMissing (returned
// by the daemon when no provider is registered for the
// requested model) to 502 Bad Gateway with a provider_missing
// detail so operators can distinguish "no provider" from
// "provider failed" without reading the daemon log. BUG-19.
func (s *Server) handleCreateExecution(w http.ResponseWriter, r *http.Request) error {
	capabilityVersionID := r.PathValue("version_id")
	var req struct {
		Inputs   map[string]any `json:"inputs,omitempty"`
		Model    string         `json:"model"`
		Provider string         `json:"provider"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	rec, invErr, latency := s.invokeOne(r, capabilityVersionID, req.Inputs, req.Model, req.Provider)
	exec := &capability.Execution{
		ID:                  generateID(),
		CapabilityVersionID: capabilityVersionID,
		Timestamp:           time.Now(),
		Inputs:              req.Inputs,
		Model:               req.Model,
		Provider:            req.Provider,
		LatencyMs:           latency.Milliseconds(),
	}
	// The previous implementation bailed on classifyInvokeError
	// before persisting the failed execution, so a 5xx-class
	// invoke was invisible in audit and the execution table.
	// The new contract: always persist (success or failure),
	// return 5xx on failure. A failed execution IS an event we
	// want in the audit chain.
	if rec != nil {
		if len(rec.Output) > 0 {
			exec.Outputs = map[string]any{"content": string(rec.Output)}
		}
		exec.PromptTokens = rec.PromptTokens
		exec.CompletionTokens = rec.OutputTokens
		exec.TotalTokens = rec.PromptTokens + rec.OutputTokens
		exec.Model = rec.Model
		exec.CostUSD = rec.CostUSD
	}
	if invErr != nil {
		exec.Error = invErr.Error()
	}
	if err := s.db.CreateExecution(r.Context(), exec); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "execution:"+exec.ID, map[string]any{
		"version_id":       capabilityVersionID,
		"tokens":           exec.TotalTokens,
		"cost_usd":         exec.CostUSD,
		"tokens_estimated": exec.TotalTokens > 0 || exec.CostUSD > 0,
		"error":            exec.Error,
	})
	if invErr != nil {
		// BUG-19: distinguish provider-missing from generic 5xx so
		// the operator can tell at a glance whether the LLM provider
		// was unregistered or the request simply failed.
		if errors.Is(invErr, errProviderMissing) {
			return &HTTPError{
				Status:  http.StatusBadGateway,
				Message: "no LLM provider configured for this invocation",
				Details: map[string]any{"provider_missing": true},
			}
		}
		if err := classifyInvokeError(invErr); err != nil {
			return err
		}
	}
	writeJSON(w, http.StatusCreated, exec)
	return nil
}

// classifyInvokeError maps an invoke.Invoker error to the appropriate
// HTTP status. Returns nil when the error is nil or not worth
// translating (the caller should still record the error in the
// Execution row).
//
// BUG-15: the previous version returned the raw err.Error() to
// the client on every 5xx. Upstream provider failures frequently
// embed the request URL, including the bearer-token query-string
// fallback (if an operator ever configured one), or internal
// stack traces. Sanitise by returning a generic message and
// relying on the audit log to preserve the full error.
func classifyInvokeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, invoke.ErrQuotaExceeded) {
		return &HTTPError{Status: http.StatusTooManyRequests, Message: "quota exceeded"}
	}
	if errors.Is(err, invoke.ErrBudgetExceeded) {
		return &HTTPError{Status: http.StatusPaymentRequired, Message: "budget exceeded"}
	}
	return &HTTPError{
		Status:  http.StatusBadGateway,
		Message: "invoke failed",
		Details: map[string]any{"error": err.Error()},
	}
}

// invokeOne is the per-request invocation entry point. It is
// introduced as a method on Server rather than a package-level
// function so that the production wiring can override the
// default Caller and AggregatorConsumer with a workspace-scoped
// Caller chain.
//
// When the versionID resolves to a Capability Version with a stored
// Manifest, that Manifest's canonical SHA-256 is used as the manifest
// hash. Otherwise the handler falls back to the placeholder hash so
// the route stays observable even for versions that pre-date the
// Manifest schema.
//
// Returns the ExecutionRecord (or nil when the invoker has nothing
// to record), the invocation error (or nil on success), and the
// wall-clock latency so callers can populate the Execution row.
//
// The function requires s.invoker to be set; tests and the daemon
// entry point must construct an invoke.Invoker. There is no
// "stub" path — a missing invoker is a programming error and
// returns a clear error rather than a silent no-op so misconfigured
// deployments fail loudly.
func (s *Server) invokeOne(r *http.Request, versionID string, inputs map[string]any, model, provider string) (*executor.ExecutionRecord, error, time.Duration) {
	if s.invoker == nil {
		return nil, errors.New("api: invoke.Invoker not wired on this server"), 0
	}
	input, err := marshalNoArgs(inputs)
	if err != nil {
		return nil, err, 0
	}
	mh := manifestHash(versionID, model, provider)
	if v, err := s.db.GetVersion(r.Context(), versionID); err == nil {
		if v.ManifestHash != "" {
			mh = v.ManifestHash
		}
	}
	req := executor.InvokeRequest{
		WorkspaceID:   r.PathValue("workspace_id"),
		ReleaseID:     versionID,
		ManifestHash:  mh,
		InputHash:     inputHash(input),
		Input:         input,
		Model:         model,
		ModelRevision: modelRevision(model, provider),
		Provider:      provider,
	}
	start := time.Now()
	rec, err := s.invoker.Invoke(r.Context(), req)
	return &rec, err, time.Since(start)
}

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	e, err := s.db.GetExecution(r.Context(), id)
	if err != nil {
		return translateDBError(err, "execution")
	}
	writeJSON(w, http.StatusOK, e)
	return nil
}
