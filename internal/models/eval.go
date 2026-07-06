package models

import (
	"time"

	"github.com/sachn-cs/promptsheon/internal/llm"
)

// EvalResult records the outcome of running a prompt against one test case.
type EvalResult struct {
	ID                 string    `json:"id"`
	TestCaseID         string    `json:"test_case_id"`
	PromptHash         string    `json:"prompt_hash"`
	DatasetID          string    `json:"dataset_id"`
	Model              string    `json:"model"`
	Output             string    `json:"output"`
	Score              float64   `json:"score"`
	LatencyMs          int64     `json:"latency_ms"`
	TokenUsage         llm.Usage `json:"token_usage"`
	HallucinationScore float64   `json:"hallucination_score"`
	Passed             bool      `json:"passed"`
	Error              string    `json:"error,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// Aggregate holds summary statistics across a batch of evaluation results.
type Aggregate struct {
	TotalCases       int     `json:"total_cases"`
	PassedCases      int     `json:"passed_cases"`
	PassRate         float64 `json:"pass_rate"`
	AvgScore         float64 `json:"avg_score"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	AvgHallucination float64 `json:"avg_hallucination"`
	TotalTokens      int     `json:"total_tokens"`
}

// EvalReport is the full output of an evaluation run.
type EvalReport struct {
	PromptHash  string        `json:"prompt_hash"`
	DatasetID   string        `json:"dataset_id"`
	Model       string        `json:"model"`
	Results     []*EvalResult `json:"results"`
	Aggregate   Aggregate     `json:"aggregate"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
}

// EvalConfig controls evaluation behavior.
type EvalConfig struct {
	PromptHash string
	DatasetID  string
	Model      string
}

// EvalRun tracks a single evaluation execution.
type EvalRun struct {
	ID               string     `json:"id"`
	PromptHash       string     `json:"prompt_hash"`
	DatasetID        string     `json:"dataset_id"`
	Model            string     `json:"model"`
	Status           string     `json:"status"` // running, completed, failed
	TotalCases       int        `json:"total_cases"`
	PassedCases      int        `json:"passed_cases"`
	PassRate         float64    `json:"pass_rate"`
	AvgScore         float64    `json:"avg_score"`
	AvgLatencyMs     float64    `json:"avg_latency_ms"`
	AvgHallucination float64    `json:"avg_hallucination"`
	TotalTokens      int        `json:"total_tokens"`
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// EvalRunFilter defines criteria for listing eval runs.
type EvalRunFilter struct {
	PromptHash string
	DatasetID  string
	Model      string
	Status     string
	Limit      int
	Offset     int
}
