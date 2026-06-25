package api

import (
	"net/http"

	"github.com/sachn-cs/promptsheon/internal/llm"
	"github.com/sachn-cs/promptsheon/internal/optimizer"
)

func (s *Server) handleOptimizePrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}

	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	// Get provider for optimization
	var providerName string
	if p.Binding != nil && p.Binding.Provider != "" {
		providerName = p.Binding.Provider
	} else {
		providerName = "openai"
	}

	provider, err := llm.Global.Get(providerName)
	if err != nil {
		return badRequest("provider not available: " + providerName)
	}

	opt := optimizer.NewOptimizer(provider)
	report, err := opt.OptimizePrompt(r.Context(), p)
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, report)
	return nil
}

func (s *Server) handleAnalyzePrompt(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}

	p, err := s.db.GetPrompt(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	opt := optimizer.NewOptimizer(nil)
	metrics := opt.AnalyzePrompt(p)

	writeJSON(w, http.StatusOK, metrics)
	return nil
}

func (s *Server) handleGetOptimizationTips(w http.ResponseWriter, r *http.Request) error {
	tips := optimizer.GetOptimizationTips()
	writeJSON(w, http.StatusOK, map[string]any{
		"tips": tips,
	})
	return nil
}
