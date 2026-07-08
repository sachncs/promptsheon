// Package abtesting provides A/B testing capabilities for prompts.
package abtesting

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/llm"
)

// Variant represents an A/B test variant.
type Variant struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	PromptID   string  `json:"prompt_id"`
	TrafficPct float64 `json:"traffic_pct"` // 0-100
	Weight     float64 `json:"weight"`      // calculated from traffic
}

// Metric tracks performance metrics for a variant.
type Metric struct {
	VariantID    string  `json:"variant_id"`
	TotalRuns    int64   `json:"total_runs"`
	SuccessCount int64   `json:"success_count"`
	ErrorCount   int64   `json:"error_count"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	AvgTokens    float64 `json:"avg_tokens"`
	AvgCost      float64 `json:"avg_cost_usd"`
	SuccessRate  float64 `json:"success_rate"`
}

// Test represents an A/B test configuration.
type Test struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	PromptID    string     `json:"prompt_id"`
	Status      string     `json:"status"` // "running", "paused", "completed"
	Variants    []*Variant `json:"variants"`
	WinCriteria string     `json:"win_criteria"` // "success_rate", "latency", "cost"
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	MinSamples  int        `json:"min_samples"`
}

// TestResults contains the results of an A/B test.
type TestResults struct {
	TestID        string           `json:"test_id"`
	Status        string           `json:"status"`
	Variants      []*VariantResult `json:"variants"`
	Winner        string           `json:"winner,omitempty"`
	Confidence    float64          `json:"confidence"`
	IsSignificant bool             `json:"is_significant"`
}

// VariantResult contains results for a single variant.
type VariantResult struct {
	Variant *Variant `json:"variant"`
	Metrics *Metric  `json:"metrics"`
	Rank    int      `json:"rank"`
}

// Engine manages A/B tests.
type Engine struct {
	mu       sync.RWMutex
	tests    map[string]*Test
	metrics  map[string]map[string]*Metric // testID -> variantID -> metrics
	provider llm.Provider
}

// NewEngine creates a new A/B testing engine.
func NewEngine(provider llm.Provider) *Engine {
	return &Engine{
		tests:    make(map[string]*Test),
		metrics:  make(map[string]map[string]*Metric),
		provider: provider,
	}
}

// CreateTest creates a new A/B test.
func (e *Engine) CreateTest(test *Test) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.tests[test.ID]; exists {
		return fmt.Errorf("test already exists: %s", test.ID)
	}

	// Validate traffic distribution
	var totalTraffic float64
	for _, v := range test.Variants {
		totalTraffic += v.TrafficPct
	}
	if totalTraffic > 100.01 { // small epsilon for float precision
		return fmt.Errorf("traffic distribution exceeds 100%%")
	}

	// Calculate weights
	for _, v := range test.Variants {
		v.Weight = v.TrafficPct / 100.0
	}

	test.Status = "running"
	test.StartTime = time.Now()
	e.tests[test.ID] = test
	e.metrics[test.ID] = make(map[string]*Metric)

	// Initialize metrics for each variant
	for _, v := range test.Variants {
		e.metrics[test.ID][v.ID] = &Metric{
			VariantID: v.ID,
		}
	}

	return nil
}

// StopTest stops a running A/B test.
func (e *Engine) StopTest(testID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	test, exists := e.tests[testID]
	if !exists {
		return fmt.Errorf("test not found: %s", testID)
	}

	test.Status = "completed"
	now := time.Now()
	test.EndTime = &now

	return nil
}

// GetTest retrieves a test by ID.
func (e *Engine) GetTest(testID string) (*Test, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	test, exists := e.tests[testID]
	if !exists {
		return nil, fmt.Errorf("test not found: %s", testID)
	}

	return test, nil
}

// SelectVariant selects a variant based on traffic distribution.
func (e *Engine) SelectVariant(testID string) (*Variant, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	test, exists := e.tests[testID]
	if !exists {
		return nil, fmt.Errorf("test not found: %s", testID)
	}

	if test.Status != "running" {
		return nil, fmt.Errorf("test is not running")
	}

	// Weighted random selection
	r := rand.Float64()
	cumulative := 0.0

	for _, v := range test.Variants {
		cumulative += v.Weight
		if r <= cumulative {
			return v, nil
		}
	}

	// Fallback to first variant
	return test.Variants[0], nil
}

// RecordResult records the result of a variant execution.
func (e *Engine) RecordResult(testID, variantID string, success bool, latencyMs, tokens, cost float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	metrics, exists := e.metrics[testID][variantID]
	if !exists {
		return
	}

	metrics.TotalRuns++
	if success {
		metrics.SuccessCount++
	} else {
		metrics.ErrorCount++
	}

	// Update averages
	n := float64(metrics.TotalRuns)
	metrics.AvgLatencyMs = (metrics.AvgLatencyMs*(n-1) + latencyMs) / n
	metrics.AvgTokens = (metrics.AvgTokens*(n-1) + tokens) / n
	metrics.AvgCost = (metrics.AvgCost*(n-1) + cost) / n

	if metrics.TotalRuns > 0 {
		metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.TotalRuns)
	}
}

// GetResults calculates and returns the results of an A/B test.
func (e *Engine) GetResults(testID string) (*TestResults, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	test, exists := e.tests[testID]
	if !exists {
		return nil, fmt.Errorf("test not found: %s", testID)
	}

	results := &TestResults{
		TestID: testID,
		Status: test.Status,
	}

	var bestVariant string
	var bestScore float64
	totalSamples := 0

	for _, v := range test.Variants {
		metrics := e.metrics[testID][v.ID]
		totalSamples += int(metrics.TotalRuns)

		result := &VariantResult{
			Variant: v,
			Metrics: metrics,
		}
		results.Variants = append(results.Variants, result)

		// Determine winner based on criteria
		var score float64
		switch test.WinCriteria {
		case "success_rate":
			score = metrics.SuccessRate
		case "latency":
			score = 1.0 / (metrics.AvgLatencyMs + 1) // lower is better
		case "cost":
			score = 1.0 / (metrics.AvgCost + 0.001) // lower is better
		default:
			score = metrics.SuccessRate
		}

		if score > bestScore {
			bestScore = score
			bestVariant = v.ID
		}
	}

	// Check if we have enough samples for significance
	results.Winner = bestVariant
	results.IsSignificant = totalSamples >= test.MinSamples
	results.Confidence = float64(totalSamples) / float64(test.MinSamples)
	if results.Confidence > 1.0 {
		results.Confidence = 1.0
	}

	// Rank variants
	sortVariantsByScore(results.Variants, test.WinCriteria)
	for i, v := range results.Variants {
		v.Rank = i + 1
	}

	return results, nil
}

// ListTests returns all A/B tests.
func (e *Engine) ListTests() []*Test {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tests := make([]*Test, 0, len(e.tests))
	for _, test := range e.tests {
		tests = append(tests, test)
	}
	return tests
}

func sortVariantsByScore(variants []*VariantResult, criteria string) {
	for i := 0; i < len(variants); i++ {
		for j := i + 1; j < len(variants); j++ {
			var scoreI, scoreJ float64
			switch criteria {
			case "success_rate":
				scoreI = variants[i].Metrics.SuccessRate
				scoreJ = variants[j].Metrics.SuccessRate
			case "latency":
				scoreI = 1.0 / (variants[i].Metrics.AvgLatencyMs + 1)
				scoreJ = 1.0 / (variants[j].Metrics.AvgLatencyMs + 1)
			case "cost":
				scoreI = 1.0 / (variants[i].Metrics.AvgCost + 0.001)
				scoreJ = 1.0 / (variants[j].Metrics.AvgCost + 0.001)
			default:
				scoreI = variants[i].Metrics.SuccessRate
				scoreJ = variants[j].Metrics.SuccessRate
			}
			if scoreI < scoreJ {
				variants[i], variants[j] = variants[j], variants[i]
			}
		}
	}
}
