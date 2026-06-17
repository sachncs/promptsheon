package models

import "time"

// TestCase defines a single evaluation input with expected behavior.
type TestCase struct {
	ID               string         `json:"id"`
	Input            map[string]any `json:"input"`
	ExpectedOutput   string         `json:"expected_output,omitempty"`
	ExpectedContains []string       `json:"expected_contains,omitempty"`
	Tags             []string       `json:"tags"`
}

// TestDataset is a collection of test cases for prompt evaluation.
type TestDataset struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Cases     []TestCase `json:"cases"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
}
