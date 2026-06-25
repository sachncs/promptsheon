package api

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"promptsheon/internal/guardrail"
	"promptsheon/internal/llm"
	"promptsheon/internal/models"
	"promptsheon/internal/trace"
	"promptsheon/internal/webhook"
)

func (s *Server) handleListPrompts(w http.ResponseWriter, r *http.Request) error {
	filter := models.PromptFilter{
		Search:      r.URL.Query().Get("search"),
		Environment: r.URL.Query().Get("environment"),
		Limit:       50,
		Offset:      0,
	}

	// Parse limit parameter
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			filter.Limit = n
		}
	}

	// Parse offset parameter
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	prompts, err := s.db.ListPrompts(r.Context(), filter)
	if err != nil {
		return err
	}
	if prompts == nil {
		prompts = []*models.Prompt{}
	}
	writeJSON(w, http.StatusOK, prompts)
	return nil
}

func (s *Server) handleCreatePrompt(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name        string                  `json:"name"`
		Description string                  `json:"description"`
		Content     string                  `json:"content"`
		Variables   []models.Variable       `json:"variables"`
		Tags        []string                `json:"tags"`
		ModelHint   string                  `json:"model_hint"`
		Binding     *models.ProviderBinding `json:"binding,omitempty"`
		Environment string                  `json:"environment"`
		Metadata    map[string]string       `json:"metadata"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" || req.Content == "" {
		return ErrBadRequest
	}

	now := time.Now()
	p := &models.Prompt{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Variables:   req.Variables,
		Tags:        req.Tags,
		ModelHint:   req.ModelHint,
		Binding:     req.Binding,
		Version:     1,
		Status:      models.StatusDraft,
		Environment: req.Environment,
		CreatedBy:   callerID(r),
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    req.Metadata,
	}

	if err := s.db.CreatePrompt(r.Context(), p); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "prompt:"+p.ID, map[string]any{"name": p.Name})
	writeJSON(w, http.StatusCreated, p)
	return nil
}

func (s *Server) handleGetPrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}

func (s *Server) handleUpdatePrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Name        *string                 `json:"name"`
		Description *string                 `json:"description"`
		Content     *string                 `json:"content"`
		Variables   []models.Variable       `json:"variables"`
		Tags        []string                `json:"tags"`
		ModelHint   *string                 `json:"model_hint"`
		Binding     *models.ProviderBinding `json:"binding,omitempty"`
		Status      *string                 `json:"status"`
		Metadata    map[string]string       `json:"metadata"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Content != nil {
		existing.Content = *req.Content
		existing.Version++
	}
	if req.Variables != nil {
		existing.Variables = req.Variables
	}
	if req.Tags != nil {
		existing.Tags = req.Tags
	}
	if req.ModelHint != nil {
		existing.ModelHint = *req.ModelHint
	}
	if req.Binding != nil {
		existing.Binding = req.Binding
	}
	if req.Status != nil {
		newStatus := models.PromptStatus(*req.Status)
		// Enforce review-gated transitions: cannot directly set approved without a review
		if newStatus == models.StatusApproved && existing.Status != models.StatusApproved {
			reviews, _ := s.db.ListReviewsByResource(r.Context(), existing.ID, "prompt")
			approvedCount := 0
			quorumRequired := 1
			for _, rv := range reviews {
				if rv.Status == models.ReviewApproved {
					approvedCount++
				}
				if rv.QuorumRequired > quorumRequired {
					quorumRequired = rv.QuorumRequired
				}
			}
			if approvedCount < quorumRequired {
				return badRequestf("cannot approve prompt: need %d approvals, have %d", quorumRequired, approvedCount)
			}
		}
		s.auditDiff(r.Context(), "update_status", "prompt:"+existing.ID, existing.Status, newStatus)
		existing.Status = newStatus
	}
	if req.Metadata != nil {
		existing.Metadata = req.Metadata
	}
	existing.UpdatedAt = time.Now()

	if err := s.db.UpdatePrompt(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "prompt:"+existing.ID, map[string]any{"name": existing.Name, "version": existing.Version})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeletePrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	// Pre-check existence so we can return a clean 404 and skip the
	// audit write. The previous implementation relied on the SQL
	// DELETE returning an error for unknown IDs, which produced an
	// inconsistent audit trail (some 404s logged a delete, others
	// did not). Fix: look up the prompt first, then delete + audit
	// only if it exists.
	if _, err := s.db.GetPrompt(r.Context(), id); err != nil {
		return ErrNotFound
	}
	if err := s.db.DeletePrompt(r.Context(), id); err != nil {
		return err
	}
	s.audit(r.Context(), "delete", "prompt:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleDeployPrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	if p.Status != models.StatusApproved {
		return badRequest("can only deploy approved prompts")
	}
	p.Status = models.StatusDeployed
	p.UpdatedAt = time.Now()
	if err := s.db.UpdatePrompt(r.Context(), p); err != nil {
		return err
	}
	s.audit(r.Context(), "deploy", "prompt:"+p.ID, nil)

	if s.webhooks != nil {
		s.webhooks.Emit(webhook.Event{
			ID:        generateID(),
			Type:      webhook.EventPromptDeployed,
			Resource:  "prompt:" + p.ID,
			Data:      map[string]any{"name": p.Name, "version": p.Version},
			Timestamp: time.Now(),
		})
	}

	writeJSON(w, http.StatusOK, p)
	return nil
}

func (s *Server) handleArchivePrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	if p.Status != models.StatusDeployed && p.Status != models.StatusApproved {
		return badRequest("can only archive deployed or approved prompts")
	}
	p.Status = models.StatusArchived
	p.UpdatedAt = time.Now()
	if err := s.db.UpdatePrompt(r.Context(), p); err != nil {
		return err
	}
	s.audit(r.Context(), "archive", "prompt:"+p.ID, nil)

	if s.webhooks != nil {
		s.webhooks.Emit(webhook.Event{
			ID:        generateID(),
			Type:      webhook.EventPromptArchived,
			Resource:  "prompt:" + p.ID,
			Data:      map[string]any{"name": p.Name, "version": p.Version},
			Timestamp: time.Now(),
		})
	}

	writeJSON(w, http.StatusOK, p)
	return nil
}

func (s *Server) handleRunPrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	if p.Status != models.StatusApproved && p.Status != models.StatusDeployed {
		return badRequest("can only run approved or deployed prompts")
	}

	var req struct {
		Variables    map[string]string `json:"variables"`
		Provider     string            `json:"provider,omitempty"`
		Model        string            `json:"model,omitempty"`
		SystemPrompt string            `json:"system_prompt,omitempty"`
		MaxTokens    int               `json:"max_tokens,omitempty"`
		Temperature  *float64          `json:"temperature,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	// Validate required variables
	for _, v := range p.Variables {
		if v.Required {
			if _, ok := req.Variables[v.Name]; !ok {
				if v.Default == "" {
					return badRequestf("missing required variable: %s", v.Name)
				}
				req.Variables[v.Name] = v.Default
			}
		}
	}

	// Resolve provider. H-8: when the prompt has a binding that pins
	// a specific provider, the caller's per-request `provider` is
	// rejected if it disagrees with the binding. The previous
	// behaviour silently overrode the binding with the caller's
	// choice, which meant a vault-bound OpenAI key could be sent to
	// the Anthropic endpoint and exfiltrated via the per-call key
	// mechanism.
	providerName := req.Provider
	if p.Binding != nil && p.Binding.Provider != "" {
		if req.Provider != "" && req.Provider != p.Binding.Provider {
			return badRequestf("prompt is bound to provider %q; cannot override to %q", p.Binding.Provider, req.Provider)
		}
		providerName = p.Binding.Provider
	}
	if providerName == "" {
		providerName = "openai"
	}

	model := req.Model
	if p.Binding != nil && p.Binding.Model != "" {
		if req.Model != "" && req.Model != p.Binding.Model {
			return badRequestf("prompt is bound to model %q; cannot override to %q", p.Binding.Model, req.Model)
		}
		model = p.Binding.Model
	}
	if model == "" {
		model = p.ModelHint
	}

	// Resolve API key from vault if binding has api_key_ref. The key is
	// injected into the per-call context so providers honour it for this
	// single call without mutating the registry.
	if p.Binding != nil && p.Binding.APIKeyRef != "" && s.vault != nil {
		decrypted, err := s.vault.Decrypt(p.Binding.APIKeyRef)
		if err == nil && decrypted != "" {
			r = r.WithContext(llm.WithPerCallKey(r.Context(), decrypted))
		} else if s.logger != nil {
			s.logger.Warn("vault: could not decrypt binding api_key_ref", "err", err, "prompt_id", p.ID)
		}
	}

	// Get provider.
	provider, err := llm.Global.Get(providerName)
	if err != nil {
		return badRequest("provider not available: " + err.Error())
	}

	// --- PRE-EXECUTION GUARDRAILS ---
	if s.guardrailManager != nil {
		promptText := p.Content
		for k, v := range req.Variables {
			promptText = strings.ReplaceAll(promptText, "{{"+k+"}}", v)
		}
		violations := s.guardrailManager.RunAllStaticChecks(r.Context(), promptText, model, p.Environment)
		if len(violations) > 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":      "guardrail violations",
				"violations": violations,
			})
			return nil
		}
	}

	// Build prompt by substituting variables
	promptText := p.Content
	for k, v := range req.Variables {
		promptText = strings.ReplaceAll(promptText, "{{"+k+"}}", v)
	}

	// Build messages
	messages := []llm.Message{}
	if req.SystemPrompt != "" {
		messages = append(messages, llm.Message{Role: "system", Content: req.SystemPrompt})
	} else if p.SystemPrompt != "" {
		messages = append(messages, llm.Message{Role: "system", Content: p.SystemPrompt})
	}
	messages = append(messages, llm.Message{Role: "user", Content: promptText})

	// Set max tokens (request override > prompt config > default)
	maxTokens := req.MaxTokens
	if maxTokens == 0 && p.Generation != nil && p.Generation.MaxTokens > 0 {
		maxTokens = p.Generation.MaxTokens
	}
	if maxTokens == 0 {
		maxTokens = 2048
	}

	// Build LLM request
	llmReq := &llm.Request{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	// Apply generation parameters (request override > prompt config)
	if req.Temperature != nil {
		llmReq.Temperature = *req.Temperature
	} else if p.Generation != nil && p.Generation.Temperature > 0 {
		llmReq.Temperature = p.Generation.Temperature
	}

	if p.Generation != nil {
		if p.Generation.TopP > 0 {
			llmReq.TopP = p.Generation.TopP
		}
		if len(p.Generation.Stop) > 0 {
			llmReq.Stop = p.Generation.Stop
		}
	}

	// --- TRACE SPAN ---
	var span *trace.Span
	if s.spans != nil {
		span = s.spans.Start(r.Context(), "prompt.run")
		span.SetAttribute("prompt_id", p.ID)
		span.SetAttribute("prompt_name", p.Name)
		span.SetAttribute("provider", providerName)
		span.SetAttribute("model", model)
		span.SetAttribute("version", fmt.Sprintf("%d", p.Version))
		ctx := trace.WithSpanContext(r.Context(), span)
		r = r.WithContext(ctx)
	}

	// --- WRAP PROVIDER WITH CIRCUIT BREAKER + TIMEOUT + RETRY + INSTRUMENTATION ---
	breakered := llm.NewCircuitBreakerMiddleware(provider, llm.CircuitBreakerConfig{
		FailureThreshold: s.cfg.CircuitBreakerFailureThreshold,
		SuccessThreshold: s.cfg.CircuitBreakerSuccessThreshold,
		Cooldown:         time.Duration(s.cfg.CircuitBreakerCooldown) * time.Second,
	})
	instrumented := llm.NewInstrumented(breakered, func(m llm.CallMetrics) {
		// Metrics are collected by the global collector in the Instrumented wrapper
	}, s.logger)
	timeouting := llm.NewTimeouting(instrumented, 30*time.Second)
	retrying := llm.NewRetrying(timeouting, llm.DefaultRetryConfig())

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := retrying.Complete(ctx, llmReq)
	latency := time.Since(start)

	if err != nil {
		// Save failed execution log
		if s.db != nil {
			execLog := &models.ExecutionLog{
				ID:         fmt.Sprintf("exec-%d", time.Now().UnixNano()),
				PromptID:   p.ID,
				PromptName: p.Name,
				Provider:   providerName,
				Model:      model,
				Status:     "error",
				Error:      err.Error(),
				LatencyMs:  time.Since(start).Milliseconds(),
				CreatedAt:  time.Now(),
			}
			s.db.SaveExecutionLog(r.Context(), execLog) //nolint:errcheck
		}
		if span != nil {
			span.SetError(err)
			span.Finish()
			s.spans.Finish(span) //nolint:errcheck
		}
		return err
	}

	// --- POST-EXECUTION GUARDRAILS ---
	var postViolations []*guardrail.Violation
	if s.guardrailManager != nil {
		if result := s.guardrailManager.CheckResponseFormat(resp.Content, "json"); !result.Passed {
			postViolations = append(postViolations, result.Violation)
		}
		if result := s.guardrailManager.CheckContentPolicy(resp.Content, []string{"no_pii", "no_harmful"}); !result.Passed {
			postViolations = append(postViolations, result.Violation)
		}
		for _, v := range postViolations {
			s.guardrailManager.RecordViolation(v)
		}
	}

	// --- CALCULATE COST ---
	costUSD := llm.CalculateCost(model, resp.Usage)

	// --- RECORD METRICS ---
	if s.collector != nil {
		s.collector.LLMCallsTotal.Inc()
		s.collector.LLMLatency.Observe(latency.Seconds())
		s.collector.LLMTokensTotal.Add(float64(resp.Usage.TotalTokens))
		s.collector.LLMInputTokens.Add(float64(resp.Usage.PromptTokens))
		s.collector.LLMOutputTokens.Add(float64(resp.Usage.CompletionTokens))
		s.collector.LLMCostUSD.Add(costUSD)
	}

	// --- RECORD USAGE ---
	if s.usageTracker != nil {
		s.usageTracker.RecordPromptUsage(p.ID, p.Name, resp.Usage.TotalTokens, float64(latency.Milliseconds()))
	}

	// --- FINISH TRACE SPAN ---
	if span != nil {
		span.SetAttribute("llm.latency_ms", fmt.Sprintf("%d", latency.Milliseconds()))
		span.SetAttribute("llm.tokens", fmt.Sprintf("%d", resp.Usage.TotalTokens))
		span.SetAttribute("llm.cost_usd", fmt.Sprintf("%.6f", costUSD))
		span.SetAttribute("llm.model", resp.Model)
		span.Finish()
		s.spans.Finish(span) //nolint:errcheck
	}

	// --- AUDIT LOG ---
	traceID := ""
	if span != nil {
		traceID = span.TraceID
	}
	s.audit(r.Context(), "run", "prompt:"+p.ID, map[string]any{
		"provider":          providerName,
		"model":             model,
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":      resp.Usage.TotalTokens,
		"cost_usd":          costUSD,
		"latency_ms":        latency.Milliseconds(),
		"trace_id":          traceID,
	})

	// --- PERSIST EXECUTION LOG ---
	execLog := &models.ExecutionLog{
		ID:               generateID(),
		PromptID:         p.ID,
		PromptName:       p.Name,
		PromptVersion:    p.Version,
		Provider:         providerName,
		Model:            model,
		Status:           "success",
		Variables:        req.Variables,
		SystemPrompt:     req.SystemPrompt,
		RequestMessages:  len(messages),
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		CostUSD:          costUSD,
		LatencyMs:        latency.Milliseconds(),
		TraceID:          traceID,
		Environment:      p.Environment,
		CreatedAt:        time.Now(),
	}
	if len(postViolations) > 0 {
		execLog.Status = "guardrail_violated"
		for _, v := range postViolations {
			execLog.Violations = append(execLog.Violations, string(v.Type))
		}
	}
	s.db.SaveExecutionLog(r.Context(), execLog) //nolint:errcheck

	// --- BUILD RESPONSE ---
	result := map[string]any{
		"content":    resp.Content,
		"model":      resp.Model,
		"usage":      resp.Usage,
		"cost_usd":   costUSD,
		"latency_ms": latency.Milliseconds(),
		"trace_id":   traceID,
	}
	if len(postViolations) > 0 {
		result["guardrail_violations"] = postViolations
	}

	writeJSON(w, http.StatusOK, result)
	return nil
}

func (s *Server) handlePreviewPrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Variables map[string]string `json:"variables"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	// Render prompt with variable substitution
	rendered := p.Content
	for k, v := range req.Variables {
		rendered = strings.ReplaceAll(rendered, "{{"+k+"}}", v)
	}

	// Estimate tokens
	inputTokens := llm.EstimateTokens(rendered)
	outputTokens := 0 // unknown until execution

	// Estimate cost if model is known
	var cost float64
	if p.Binding != nil && p.Binding.Model != "" {
		cost = llm.EstimateCost(inputTokens, outputTokens, p.Binding.Model)
	} else if p.ModelHint != "" {
		cost = llm.EstimateCost(inputTokens, outputTokens, p.ModelHint)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"rendered":         rendered,
		"estimated_tokens": inputTokens,
		"estimated_cost":   cost,
		"variables_used":   req.Variables,
	})
	return nil
}

func (s *Server) handleStreamPrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	if p.Status != models.StatusApproved && p.Status != models.StatusDeployed {
		return badRequest("can only run approved or deployed prompts")
	}

	var reqBody struct {
		Variables    map[string]string `json:"variables"`
		Provider     string            `json:"provider,omitempty"`
		Model        string            `json:"model,omitempty"`
		SystemPrompt string            `json:"system_prompt,omitempty"`
		MaxTokens    int               `json:"max_tokens,omitempty"`
		Temperature  *float64          `json:"temperature,omitempty"`
	}
	if err := readJSON(r, &reqBody); err != nil {
		return ErrBadRequest
	}

	// Validate required variables
	for _, v := range p.Variables {
		if v.Required {
			if _, ok := reqBody.Variables[v.Name]; !ok {
				if v.Default == "" {
					return badRequestf("missing required variable: %s", v.Name)
				}
				reqBody.Variables[v.Name] = v.Default
			}
		}
	}

	// Resolve provider
	providerName := reqBody.Provider
	if providerName == "" && p.Binding != nil {
		providerName = p.Binding.Provider
	}
	if providerName == "" {
		providerName = "openai"
	}

	model := reqBody.Model
	if model == "" && p.Binding != nil {
		model = p.Binding.Model
	}
	if model == "" {
		model = p.ModelHint
	}

	// Get provider
	provider, err := llm.Global.Get(providerName)
	if err != nil {
		return badRequest("provider not available: " + err.Error())
	}

	// Pre-execution guardrails
	if s.guardrailManager != nil {
		promptText := p.Content
		for k, v := range reqBody.Variables {
			promptText = strings.ReplaceAll(promptText, "{{"+k+"}}", v)
		}
		violations := s.guardrailManager.RunAllStaticChecks(r.Context(), promptText, model, p.Environment)
		if len(violations) > 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":      "guardrail violations",
				"violations": violations,
			})
			return nil
		}
	}

	// Build prompt by substituting variables
	promptText := p.Content
	for k, v := range reqBody.Variables {
		promptText = strings.ReplaceAll(promptText, "{{"+k+"}}", v)
	}

	// Build messages
	messages := []llm.Message{}
	if reqBody.SystemPrompt != "" {
		messages = append(messages, llm.Message{Role: "system", Content: reqBody.SystemPrompt})
	} else if p.SystemPrompt != "" {
		messages = append(messages, llm.Message{Role: "system", Content: p.SystemPrompt})
	}
	messages = append(messages, llm.Message{Role: "user", Content: promptText})

	// Set max tokens
	maxTokens := reqBody.MaxTokens
	if maxTokens == 0 && p.Generation != nil && p.Generation.MaxTokens > 0 {
		maxTokens = p.Generation.MaxTokens
	}
	if maxTokens == 0 {
		maxTokens = 2048
	}

	// Build LLM request with streaming enabled
	llmReq := &llm.Request{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	// Apply generation parameters
	if reqBody.Temperature != nil {
		llmReq.Temperature = *reqBody.Temperature
	} else if p.Generation != nil && p.Generation.Temperature > 0 {
		llmReq.Temperature = p.Generation.Temperature
	}

	if p.Generation != nil {
		if p.Generation.TopP > 0 {
			llmReq.TopP = p.Generation.TopP
		}
		if len(p.Generation.Stop) > 0 {
			llmReq.Stop = p.Generation.Stop
		}
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Lift the http.Server WriteTimeout for this specific response.
	// Without this, the global 60s WriteTimeout would terminate any
	// streaming LLM response that takes longer. SetReadDeadline is
	// also lifted to time.Time{} (no deadline) so the connection
	// stays open for the duration of the stream.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		// SetWriteDeadline may not be supported on some transports
		// (e.g. HTTP/2). Log and continue; the response may simply
		// time out at the server-wide limit.
		if s.logger != nil {
			s.logger.Debug("stream: SetWriteDeadline unsupported", "err", err)
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return badRequest("streaming not supported")
	}

	// Send initial event
	fmt.Fprintf(w, "event: start\ndata: %s\n\n", `{"model":"`+model+`"}`)
	flusher.Flush()

	// Create trace span
	var span *trace.Span
	if s.spans != nil {
		span = s.spans.Start(r.Context(), "prompt.stream")
		span.SetAttribute("prompt_id", p.ID)
		span.SetAttribute("provider", providerName)
		span.SetAttribute("model", model)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Check if provider supports streaming
	streamProvider, canStream := provider.(llm.StreamingProvider)
	if canStream {
		// Use true streaming
		streamReader, err := streamProvider.Stream(ctx, llmReq)
		if err != nil {
			if span != nil {
				span.SetError(err)
				span.Finish()
				s.spans.Finish(span)
			}
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", `{"error":"`+err.Error()+`"}`)
			flusher.Flush()
			return nil
		}
		defer streamReader.Close()

		// Read and forward tokens
		scanner := bufio.NewScanner(streamReader)
		var content strings.Builder
		var ttft time.Duration
		firstToken := true
		var usage models.Usage

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			if firstToken {
				ttft = time.Since(start)
				firstToken = false
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				content.WriteString(chunk.Choices[0].Delta.Content)
				// Send token event
				tokenData, _ := json.Marshal(map[string]string{
					"content": chunk.Choices[0].Delta.Content,
				})
				fmt.Fprintf(w, "event: token\ndata: %s\n\n", string(tokenData))
				flusher.Flush()
			}
			if chunk.Usage != nil {
				usage.PromptTokens = chunk.Usage.PromptTokens
				usage.CompletionTokens = chunk.Usage.CompletionTokens
				usage.TotalTokens = chunk.Usage.TotalTokens
			}
		}

		// Send done event
		doneData, _ := json.Marshal(map[string]any{
			"model":      model,
			"usage":      usage,
			"cost_usd":   llm.CalculateCost(model, usage),
			"latency_ms": time.Since(start).Milliseconds(),
			"ttft_ms":    ttft.Milliseconds(),
		})
		fmt.Fprintf(w, "event: done\ndata: %s\n\n", string(doneData))
		flusher.Flush()

		// Record metrics
		latency := time.Since(start)
		costUSD := llm.CalculateCost(model, usage)

		if s.collector != nil {
			s.collector.LLMCallsTotal.Inc()
			s.collector.LLMLatency.Observe(latency.Seconds())
			s.collector.LLMTokensTotal.Add(float64(usage.TotalTokens))
			s.collector.LLMInputTokens.Add(float64(usage.PromptTokens))
			s.collector.LLMOutputTokens.Add(float64(usage.CompletionTokens))
			s.collector.LLMCostUSD.Add(costUSD)
		}

		if s.usageTracker != nil {
			s.usageTracker.RecordPromptUsage(p.ID, p.Name, usage.TotalTokens, float64(latency.Milliseconds()))
		}

		// Finish trace span
		if span != nil {
			span.SetAttribute("llm.latency_ms", fmt.Sprintf("%d", latency.Milliseconds()))
			span.SetAttribute("llm.tokens", fmt.Sprintf("%d", usage.TotalTokens))
			span.SetAttribute("llm.ttft_ms", fmt.Sprintf("%d", ttft.Milliseconds()))
			span.Finish()
			s.spans.Finish(span)
		}

		// Get trace ID for execution log
		traceID := ""
		if span != nil {
			traceID = span.TraceID
		}

		// Persist execution log
		execLog := &models.ExecutionLog{
			ID:               generateID(),
			PromptID:         p.ID,
			PromptName:       p.Name,
			PromptVersion:    p.Version,
			Provider:         providerName,
			Model:            model,
			Status:           "success",
			Variables:        reqBody.Variables,
			SystemPrompt:     reqBody.SystemPrompt,
			RequestMessages:  len(messages),
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			CostUSD:          costUSD,
			LatencyMs:        latency.Milliseconds(),
			TraceID:          traceID,
			Environment:      p.Environment,
			CreatedAt:        time.Now(),
		}
		s.db.SaveExecutionLog(r.Context(), execLog)

		return nil
	}

	// Fallback: non-streaming provider (send complete response as single event)
	breakered := llm.NewCircuitBreakerMiddleware(provider, llm.CircuitBreakerConfig{
		FailureThreshold: s.cfg.CircuitBreakerFailureThreshold,
		SuccessThreshold: s.cfg.CircuitBreakerSuccessThreshold,
		Cooldown:         time.Duration(s.cfg.CircuitBreakerCooldown) * time.Second,
	})
	timeouting := llm.NewTimeouting(breakered, 60*time.Second)
	retrying := llm.NewRetrying(timeouting, llm.DefaultRetryConfig())

	resp, err := retrying.Complete(ctx, llmReq)
	if err != nil {
		if span != nil {
			span.SetError(err)
			span.Finish()
			s.spans.Finish(span)
		}
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", `{"error":"`+err.Error()+`"}`)
		flusher.Flush()
		return nil
	}

	// Send the complete response as a single event
	fmt.Fprintf(w, "event: token\ndata: %s\n\n", `{"content":`+jsonEscape(resp.Content)+`}`)
	flusher.Flush()

	// Send done event
	doneData, err := json.Marshal(map[string]any{
		"model":      resp.Model,
		"usage":      resp.Usage,
		"cost_usd":   llm.CalculateCost(model, resp.Usage),
		"latency_ms": time.Since(start).Milliseconds(),
	})
	if err != nil {
		doneData = []byte("{}")
	}
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", string(doneData))
	flusher.Flush()

	// Record metrics
	latency := time.Since(start)
	costUSD := llm.CalculateCost(model, resp.Usage)

	if s.collector != nil {
		s.collector.LLMCallsTotal.Inc()
		s.collector.LLMLatency.Observe(latency.Seconds())
		s.collector.LLMTokensTotal.Add(float64(resp.Usage.TotalTokens))
		s.collector.LLMInputTokens.Add(float64(resp.Usage.PromptTokens))
		s.collector.LLMOutputTokens.Add(float64(resp.Usage.CompletionTokens))
		s.collector.LLMCostUSD.Add(costUSD)
	}

	if s.usageTracker != nil {
		s.usageTracker.RecordPromptUsage(p.ID, p.Name, resp.Usage.TotalTokens, float64(latency.Milliseconds()))
	}

	// Finish trace span
	if span != nil {
		span.SetAttribute("llm.latency_ms", fmt.Sprintf("%d", latency.Milliseconds()))
		span.SetAttribute("llm.tokens", fmt.Sprintf("%d", resp.Usage.TotalTokens))
		span.Finish()
		s.spans.Finish(span)
	}

	// Get trace ID for execution log
	traceID := ""
	if span != nil {
		traceID = span.TraceID
	}

	// Persist execution log
	execLog := &models.ExecutionLog{
		ID:               generateID(),
		PromptID:         p.ID,
		PromptName:       p.Name,
		PromptVersion:    p.Version,
		Provider:         providerName,
		Model:            model,
		Status:           "success",
		Variables:        reqBody.Variables,
		SystemPrompt:     reqBody.SystemPrompt,
		RequestMessages:  len(messages),
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		CostUSD:          costUSD,
		LatencyMs:        latency.Milliseconds(),
		TraceID:          traceID,
		Environment:      p.Environment,
		CreatedAt:        time.Now(),
	}
	s.db.SaveExecutionLog(r.Context(), execLog)

	return nil
}

func jsonEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `"` + s + `"`
	}
	return string(b)
}

func (s *Server) handleClonePrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Name string `json:"name,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	// Clone the prompt
	cloneName := req.Name
	if cloneName == "" {
		cloneName = existing.Name + " (clone)"
	}

	clone := &models.Prompt{
		ID:          generateID(),
		Name:        cloneName,
		Description: existing.Description,
		Content:     existing.Content,
		Variables:   existing.Variables,
		Tags:        append(existing.Tags, "cloned"),
		ModelHint:   existing.ModelHint,
		Binding:     existing.Binding,
		Version:     1,
		Status:      models.StatusDraft,
		Environment: existing.Environment,
		Metadata:    existing.Metadata,
		CASHash:     existing.CASHash,
		CreatedBy:   callerID(r),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.CreatePrompt(r.Context(), clone); err != nil {
		return err
	}

	s.audit(r.Context(), "clone", "prompt:"+clone.ID, map[string]any{
		"source_id": existing.ID,
		"name":      cloneName,
	})

	writeJSON(w, http.StatusCreated, clone)
	return nil
}
