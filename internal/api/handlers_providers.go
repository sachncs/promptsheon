package api

import (
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/llm"
)

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) error {
	names := llm.Global.Providers()
	writeJSON(w, http.StatusOK, map[string]any{"providers": names})
	return nil
}

func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	names := llm.Global.Providers()
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
		"name":   name,
		"status": "registered",
	})
	return nil
}

func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")

	var req struct {
		Model string `json:"model"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Model == "" {
		req.Model = "gpt-3.5-turbo"
	}

	provider, err := llm.Global.Get(name)
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
			"provider":   name,
			"model":      req.Model,
			"status":     "error",
			"error":      err.Error(),
			"latency_ms": latency.Milliseconds(),
		})
		return nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"provider":   name,
		"model":      resp.Model,
		"status":     "ok",
		"content":    resp.Content,
		"usage":      resp.Usage,
		"latency_ms": latency.Milliseconds(),
	})
	return nil
}
