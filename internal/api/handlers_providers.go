package api

import (
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/llm"
)

const keyProvider = "provider"
const valError = "error"
const fieldModel = "model"

func (s *Server) handleListProviders(w http.ResponseWriter, _ *http.Request) error {
	names := llm.Default().Providers()
	writeJSON(w, http.StatusOK, map[string]any{"providers": names})
	return nil
}

func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	names := llm.Default().Providers()
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
		keyName:   name,
		keyStatus: "registered",
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

	provider, err := llm.Default().Get(name)
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
			keyProvider:  name,
			fieldModel:   req.Model,
			keyStatus:    valError,
			valError:     err.Error(),
			"latency_ms": latency.Milliseconds(),
		})
		return nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		keyProvider:  name,
		fieldModel:   resp.Model,
		keyStatus:    dbStatusOK,
		"content":    resp.Content,
		"usage":      resp.Usage,
		"latency_ms": latency.Milliseconds(),
	})
	return nil
}
