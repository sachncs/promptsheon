package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Content     string                 `json:"content"`
		Variables   []models.Variable      `json:"variables"`
		Tags        []string               `json:"tags"`
		ModelHint   string                 `json:"model_hint"`
		Binding     *models.ProviderBinding `json:"binding,omitempty"`
		Environment string                 `json:"environment"`
		Metadata    map[string]string      `json:"metadata"`
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
		CreatedBy:   "api",
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
		Name        *string               `json:"name"`
		Description *string               `json:"description"`
		Content     *string               `json:"content"`
		Variables   []models.Variable     `json:"variables"`
		Tags        []string              `json:"tags"`
		ModelHint   *string               `json:"model_hint"`
		Binding     *models.ProviderBinding `json:"binding,omitempty"`
		Status      *string               `json:"status"`
		Metadata    map[string]string     `json:"metadata"`
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
	if err := s.db.DeletePrompt(r.Context(), id); err != nil {
		return ErrNotFound
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

	// Resolve provider
	providerName := req.Provider
	if providerName == "" && p.Binding != nil {
		providerName = p.Binding.Provider
	}
	if providerName == "" {
		providerName = "openai"
	}

	model := req.Model
	if model == "" && p.Binding != nil {
		model = p.Binding.Model
	}
	if model == "" {
		model = p.ModelHint
	}

	// Resolve API key from vault if binding has api_key_ref.
	// Note: API key is resolved here but currently passed to the provider
	// through environment variables. Per-call key injection will be supported
	// when the provider registry adds per-call configuration.
	var apiKey string
	if p.Binding != nil && p.Binding.APIKeyRef != "" && s.vault != nil {
		decrypted, err := s.vault.Decrypt(p.Binding.APIKeyRef)
		if err == nil {
			apiKey = decrypted
		}
	}

	// Get provider (with API key override if resolved from vault)
	provider, err := llm.Global.Get(providerName)
	if err != nil {
		return badRequest("provider not available: " + err.Error())
	}
	_ = apiKey // Reserved for future per-call key injection

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

	// --- WRAP PROVIDER WITH RETRY + INSTRUMENTATION ---
	instrumented := llm.NewInstrumented(provider, func(m llm.CallMetrics) {
		// Metrics are collected by the global collector in the Instrumented wrapper
	}, s.logger)
	retrying := llm.NewRetrying(instrumented, llm.DefaultRetryConfig())

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := retrying.Complete(ctx, llmReq)
	latency := time.Since(start)

	if err != nil {
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

	// Wrap provider with retry
	retrying := llm.NewRetrying(provider, llm.DefaultRetryConfig())

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Make the streaming call
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

	// Send the complete response as a single event (since provider doesn't stream to us)
	fmt.Fprintf(w, "event: token\ndata: %s\n\n", `{"content":`+jsonEscape(resp.Content)+`}`)
	flusher.Flush()

	// Send done event
	doneData, _ := json.Marshal(map[string]any{
		"model":      resp.Model,
		"usage":      resp.Usage,
		"cost_usd":   llm.CalculateCost(model, resp.Usage),
		"latency_ms": time.Since(start).Milliseconds(),
	})
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
	b, _ := json.Marshal(s)
	return string(b)
}
