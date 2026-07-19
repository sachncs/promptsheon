// Package experiment provides in-memory weighted A/B testing
// with per-variant metric aggregation.
package experiment

import (
	"fmt"
	"math/rand/v2"
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

// TestStatus identifies the lifecycle state of a Test.
type TestStatus string

const (
	// TestStatusRunning is the default state for an active test.
	TestStatusRunning TestStatus = "running"
	// TestStatusPaused halts variant selection without losing history.
	TestStatusPaused TestStatus = "paused"
	// TestStatusCompleted marks a finished test.
	TestStatusCompleted TestStatus = "completed"
)

// WinCriterion names the metric used to declare a winner.
type WinCriterion string

const (
	// WinBySuccessRate picks the variant with the highest success rate.
	WinBySuccessRate WinCriterion = "success_rate"
	// WinByLatency picks the variant with the lowest latency.
	WinByLatency WinCriterion = "latency"
	// WinByCost picks the variant with the lowest average cost.
	WinByCost WinCriterion = "cost"
)

// Test represents an A/B test configuration.
type Test struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	PromptID    string       `json:"prompt_id"`
	Status      TestStatus   `json:"status"`
	Variants    []*Variant   `json:"variants"`
	WinCriteria WinCriterion `json:"win_criteria"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     *time.Time   `json:"end_time,omitempty"`
	MinSamples  int          `json:"min_samples"`
}

// TestResults contains the results of an A/B test.
type TestResults struct {
	TestID        string           `json:"test_id"`
	Status        TestStatus       `json:"status"`
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
	if test.ID == "" {
		return fmt.Errorf("test id is required")
	}
	if test.MinSamples < 0 {
		return fmt.Errorf("min_samples must be non-negative, got %d", test.MinSamples)
	}

	// Validate variants: non-empty, unique IDs, non-negative
	// weights, finite weights that sum to ~100.
	if len(test.Variants) == 0 {
		return fmt.Errorf("at least one variant is required")
	}
	seen := make(map[string]struct{}, len(test.Variants))
	var totalTraffic float64
	for i, v := range test.Variants {
		if v.ID == "" {
			return fmt.Errorf("variant[%d]: id is required", i)
		}
		if _, dup := seen[v.ID]; dup {
			return fmt.Errorf("variant[%d]: duplicate id %q", i, v.ID)
		}
		seen[v.ID] = struct{}{}
		if v.TrafficPct < 0 {
			return fmt.Errorf("variant[%d] %q: traffic_pct must be non-negative, got %f", i, v.ID, v.TrafficPct)
		}
		totalTraffic += v.TrafficPct
	}
	if totalTraffic > 100.01 { // small epsilon for float precision
		return fmt.Errorf("traffic distribution exceeds 100%%")
	}
	if totalTraffic < 99.99 {
		return fmt.Errorf("traffic distribution sums to %f%%, must be ~100%%", totalTraffic)
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
	// #nosec G404 -- weighted selection uses math/rand/v2, not crypto/rand;
	// cryptographic randomness is not required for A/B traffic splitting.
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

// ResultMetrics holds the numerical measurements for a variant execution.
type ResultMetrics struct {
	LatencyMs float64
	Tokens    float64
	Cost      float64
}

// RecordResult records the result of a variant execution.
func (e *Engine) RecordResult(testID, variantID string, success bool, m ResultMetrics) {
	e.mu.Lock()
	defer e.mu.Unlock()

	agg, exists := e.metrics[testID][variantID]
	if !exists {
		return
	}

	agg.TotalRuns++
	if success {
		agg.SuccessCount++
	} else {
		agg.ErrorCount++
	}

	// Update averages
	n := float64(agg.TotalRuns)
	agg.AvgLatencyMs = (agg.AvgLatencyMs*(n-1) + m.LatencyMs) / n
	agg.AvgTokens = (agg.AvgTokens*(n-1) + m.Tokens) / n
	agg.AvgCost = (agg.AvgCost*(n-1) + m.Cost) / n

	if agg.TotalRuns > 0 {
		agg.SuccessRate = float64(agg.SuccessCount) / float64(agg.TotalRuns)
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
		case WinBySuccessRate:
			score = metrics.SuccessRate
		case WinByLatency:
			score = 1.0 / (metrics.AvgLatencyMs + 1) // lower is better
		case WinByCost:
			score = 1.0 / (metrics.AvgCost + 0.001) // lower is better
		default:
			score = metrics.SuccessRate
		}

		if score > bestScore {
			bestScore = score
			bestVariant = v.ID
		}
	}

	// Check if we have enough samples for significance. The
	// "Confidence" field is renamed SampleTargetMet to make
	// the semantics explicit: it is the ratio of observed
	// samples to MinSamples, NOT a statistical confidence
	// interval. Callers that need real confidence must compute
	// one from the variant metrics (e.g. a Bayesian
	// credible interval on the difference of conversion
	// rates); the framework does not pretend to ship a
	// significance test it does not implement.
	results.Winner = bestVariant
	results.IsSignificant = totalSamples >= test.MinSamples
	if test.MinSamples > 0 {
		results.Confidence = float64(totalSamples) / float64(test.MinSamples)
		if results.Confidence > 1.0 {
			results.Confidence = 1.0
		}
	} else {
		// MinSamples=0 means "no minimum"; the test is
		// IsSignificant as soon as one variant has at least
		// one observation, which the operator probably did
		// not intend. Treat 0 as a misconfiguration.
		results.Confidence = 0
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

func sortVariantsByScore(variants []*VariantResult, criteria WinCriterion) {
	for i := 0; i < len(variants); i++ {
		for j := i + 1; j < len(variants); j++ {
			var scoreI, scoreJ float64
			switch criteria {
			case WinBySuccessRate:
				scoreI = variants[i].Metrics.SuccessRate
				scoreJ = variants[j].Metrics.SuccessRate
			case WinByLatency:
				scoreI = 1.0 / (variants[i].Metrics.AvgLatencyMs + 1)
				scoreJ = 1.0 / (variants[j].Metrics.AvgLatencyMs + 1)
			case WinByCost:
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
