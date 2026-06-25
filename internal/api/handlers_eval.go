package api

import (
	"net/http"
	"strconv"
	"time"

	"promptsheon/internal/eval"
	"promptsheon/internal/models"
)

func (s *Server) handleRunEval(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PromptHash string   `json:"prompt_hash"`
		PromptText string   `json:"prompt_text"`
		DatasetID  string   `json:"dataset_id"`
		Model      string   `json:"model"`
		Scorers    []string `json:"scorers"`
		MaxTokens  int      `json:"max_tokens"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.PromptHash == "" || req.DatasetID == "" {
		return ErrBadRequest
	}

	dataset, err := s.db.GetDataset(r.Context(), req.DatasetID)
	if err != nil {
		return ErrNotFound
	}

	// SECURITY/RELIABILITY: do not silently fall back to a mock LLM
	// provider. The previous behaviour returned a fake "success" with
	// canned text, which made operators believe their model had run
	// when it had not. If no eval runner is configured, refuse the
	// request with 503 so the operator notices the misconfiguration.
	if s.evalRunner == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "no eval runner configured: install an LLM provider or call WithEvalRunner"}
	}
	runner := s.evalRunner

	startedAt := time.Now()

	// If prompt_text is empty, try to load it from CAS or use a placeholder.
	promptText := req.PromptText
	if promptText == "" {
		promptText = "You are a helpful assistant. Please answer the following:\n{{input}}"
	}

	report, err := runner.Run(r.Context(), &eval.RunConfig{
		PromptHash: req.PromptHash,
		PromptText: promptText,
		Dataset:    dataset,
		Model:      req.Model,
		MaxTokens:  req.MaxTokens,
	})
	if err != nil {
		return err
	}

	// Persist results
	if err := s.db.SaveEvalResults(r.Context(), report.Results); err != nil {
		return err
	}

	// Persist eval run
	run := &models.EvalRun{
		ID:               generateID(),
		PromptHash:       req.PromptHash,
		DatasetID:        req.DatasetID,
		Model:            req.Model,
		Status:           "completed",
		TotalCases:       report.Aggregate.TotalCases,
		PassedCases:      report.Aggregate.PassedCases,
		PassRate:         report.Aggregate.PassRate,
		AvgScore:         report.Aggregate.AvgScore,
		AvgLatencyMs:     report.Aggregate.AvgLatencyMs,
		AvgHallucination: report.Aggregate.AvgHallucination,
		TotalTokens:      report.Aggregate.TotalTokens,
		StartedAt:        startedAt,
		CompletedAt:      &report.CompletedAt,
	}
	if err := s.db.SaveEvalRun(r.Context(), run); err != nil {
		s.logger.Error("failed to save eval run", "err", err, "run_id", run.ID)
	}

	s.audit(r.Context(), "eval_run", "prompt:"+req.PromptHash, map[string]any{
		"dataset_id": req.DatasetID,
		"model":      req.Model,
		"total":      report.Aggregate.TotalCases,
		"passed":     report.Aggregate.PassedCases,
	})

	writeJSON(w, http.StatusOK, report)
	return nil
}

func (s *Server) handleListEvalResults(w http.ResponseWriter, r *http.Request) error {
	promptHash := r.URL.Query().Get("prompt_hash")
	datasetID := r.URL.Query().Get("dataset_id")

	var results []*models.EvalResult
	var err error

	if promptHash != "" {
		results, err = s.db.GetEvalResults(r.Context(), promptHash)
	} else if datasetID != "" {
		results, err = s.db.GetEvalResultsByDataset(r.Context(), datasetID)
	} else {
		return ErrBadRequest
	}
	if err != nil {
		return err
	}
	if results == nil {
		results = []*models.EvalResult{}
	}
	writeJSON(w, http.StatusOK, results)
	return nil
}

func (s *Server) handleGetEvalReport(w http.ResponseWriter, r *http.Request) error {
	promptHash := r.URL.Query().Get("prompt_hash")
	if promptHash == "" {
		return ErrBadRequest
	}

	results, err := s.db.GetEvalResults(r.Context(), promptHash)
	if err != nil {
		return err
	}
	if results == nil {
		results = []*models.EvalResult{}
	}

	var totalScore, totalHallucination, totalLatency float64
	var totalTokens, passedCount int
	for _, r := range results {
		totalScore += r.Score
		totalHallucination += r.HallucinationScore
		totalLatency += float64(r.LatencyMs)
		totalTokens += r.TokenUsage.TotalTokens
		if r.Passed {
			passedCount++
		}
	}

	n := float64(len(results))
	report := &models.EvalReport{
		PromptHash: promptHash,
		Results:    results,
		Aggregate: models.Aggregate{
			TotalCases:       len(results),
			PassedCases:      passedCount,
			PassRate:         safeDivide(float64(passedCount), n),
			AvgScore:         safeDivide(totalScore, n),
			AvgLatencyMs:     safeDivide(totalLatency, n),
			AvgHallucination: safeDivide(totalHallucination, n),
			TotalTokens:      totalTokens,
		},
	}

	writeJSON(w, http.StatusOK, report)
	return nil
}

func (s *Server) handleCompareEval(w http.ResponseWriter, r *http.Request) error {
	hashA := r.URL.Query().Get("a")
	hashB := r.URL.Query().Get("b")
	if hashA == "" || hashB == "" {
		return ErrBadRequest
	}

	resultsA, err := s.db.GetEvalResults(r.Context(), hashA)
	if err != nil {
		return err
	}
	resultsB, err := s.db.GetEvalResults(r.Context(), hashB)
	if err != nil {
		return err
	}

	reportA := buildReport(hashA, resultsA)
	reportB := buildReport(hashB, resultsB)

	comp := eval.CompareReports(reportA, reportB)
	writeJSON(w, http.StatusOK, comp)
	return nil
}

func (s *Server) handleListEvalRuns(w http.ResponseWriter, r *http.Request) error {
	filter := models.EvalRunFilter{
		PromptHash: r.URL.Query().Get("prompt_hash"),
		DatasetID:  r.URL.Query().Get("dataset_id"),
		Model:      r.URL.Query().Get("model"),
		Status:     r.URL.Query().Get("status"),
		Limit:      50,
		Offset:     0,
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

	runs, err := s.db.ListEvalRuns(r.Context(), filter)
	if err != nil {
		return err
	}
	if runs == nil {
		runs = []*models.EvalRun{}
	}
	writeJSON(w, http.StatusOK, runs)
	return nil
}

func buildReport(promptHash string, results []*models.EvalResult) *models.EvalReport {
	if results == nil {
		results = []*models.EvalResult{}
	}
	var totalScore, totalHallucination, totalLatency float64
	var totalTokens, passedCount int
	for _, r := range results {
		totalScore += r.Score
		totalHallucination += r.HallucinationScore
		totalLatency += float64(r.LatencyMs)
		totalTokens += r.TokenUsage.TotalTokens
		if r.Passed {
			passedCount++
		}
	}
	n := float64(len(results))
	return &models.EvalReport{
		PromptHash: promptHash,
		Results:    results,
		Aggregate: models.Aggregate{
			TotalCases:       len(results),
			PassedCases:      passedCount,
			PassRate:         safeDivide(float64(passedCount), n),
			AvgScore:         safeDivide(totalScore, n),
			AvgLatencyMs:     safeDivide(totalLatency, n),
			AvgHallucination: safeDivide(totalHallucination, n),
			TotalTokens:      totalTokens,
		},
	}
}

func buildScorers(names []string) []eval.Scorer {
	if len(names) == 0 {
		return []eval.Scorer{eval.ContainsScorer{}}
	}
	var scorers []eval.Scorer
	for _, name := range names {
		switch name {
		case "exact_match":
			scorers = append(scorers, eval.ExactMatchScorer{})
		case "contains":
			scorers = append(scorers, eval.ContainsScorer{})
		case "regex":
			scorers = append(scorers, eval.RegexScorer{})
		case "pass_thru":
			scorers = append(scorers, eval.PassThruScorer{})
		default:
			scorers = append(scorers, eval.ContainsScorer{})
		}
	}
	return scorers
}

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
