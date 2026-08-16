package promptsheon

import (
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/llm"
)

// ListProviders lists the providers.
func (s *Server) handleListProviders(w http.ResponseWriter, _ *http.Request) error {
	if s.providers == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []string{}})
		return nil
	}
	names := s.providers.Providers()
	writeJSON(w, http.StatusOK, map[string]any{"providers": names})
	return nil
}

// GetProvider returns the provider.
func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) error {
	if s.providers == nil {
		return notFound("providers not configured")
	}
	name := r.PathValue("name")
	names := s.providers.Providers()
	found := false
	for _, n := range names {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		return notFound("provider not found: " + name)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		KeyName:   name,
		KeyStatus: "registered",
	})
	return nil
}

// handleTestProvider runs a live completion against a registered
// provider/model pair. The model is required; the previous
// default-to-gpt-3.5-turbo behaviour was removed because it
// surprised providers configured with non-OpenAI endpoints.
func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request) error {
	if s.providers == nil {
		return notFound("providers not configured")
	}
	name := r.PathValue("name")

	var req struct {
		Model string `json:"model"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Model == "" {
		// BUG-15: refuse to silently pick a default model. The
		// operator must say which model they want to test; an
		// implicit gpt-3.5-turbo would surprise providers that
		// only have Anthropic or custom endpoints configured.
		return badRequest("model is required")
	}

	provider, err := s.providers.Get(name)
	if err != nil {
		return badRequest("provider not available: " + err.Error())
	}

	start := time.Now()
	resp, err := provider.Complete(r.Context(), &llm.Request{
		Model: req.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "Say hello in one word."},
		},
		MaxTokens: 10,
	})
	latency := time.Since(start)

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			FieldProvider: name,
			FieldModel:    req.Model,
			KeyStatus:     FieldError,
			FieldError:    err.Error(),
			"latency_ms":  latency.Milliseconds(),
		})
		return nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		FieldProvider: name,
		FieldModel:    resp.Model,
		KeyStatus:     FieldOK,
		"content":     resp.Content,
		"usage":       resp.Usage,
		"latency_ms":  latency.Milliseconds(),
	})
	return nil
}
