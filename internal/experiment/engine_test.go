package experiment_test

import (
	"testing"

	"github.com/sachncs/promptsheon/internal/experiment"
)

// makeTest is a helper to create a test with two variants.
func makeTest(id string) *experiment.Test {
	return &experiment.Test{
		ID:       id,
		Name:     "Test " + id,
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", Name: "Control", PromptID: "prompt1", TrafficPct: 50},
			{ID: "v2", Name: "Variant", PromptID: "prompt2", TrafficPct: 50},
		},
		WinCriteria: "success_rate",
		MinSamples:  100,
	}
}

func TestCreateTest(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", Name: "Control", PromptID: "prompt1", TrafficPct: 50},
			{ID: "v2", Name: "Variant", PromptID: "prompt2", TrafficPct: 50},
		},
		WinCriteria: "success_rate",
		MinSamples:  100,
	}

	err := engine.CreateTest(test)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := engine.GetTest("test1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "My Test" {
		t.Errorf("expected name My Test, got %s", got.Name)
	}
	if got.Status != "running" {
		t.Errorf("expected status running, got %s", got.Status)
	}
}

func TestCreateTestInvalidTraffic(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", TrafficPct: 60},
			{ID: "v2", TrafficPct: 60}, // Total > 100
		},
	}

	err := engine.CreateTest(test)
	if err == nil {
		t.Error("expected error for traffic > 100%")
	}
}

func TestSelectVariant(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", TrafficPct: 50},
			{ID: "v2", TrafficPct: 50},
		},
	}

	_ = engine.CreateTest(test)

	// Run multiple selections to verify distribution
	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		v, err := engine.SelectVariant("test1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[v.ID]++
	}

	// Should be roughly 50/50
	if counts["v1"] < 400 || counts["v1"] > 600 {
		t.Errorf("unexpected distribution for v1: %d", counts["v1"])
	}
}

func TestRecordResult(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", TrafficPct: 100},
		},
	}

	_ = engine.CreateTest(test)

	// Record some results
	for i := 0; i < 10; i++ {
		engine.RecordResult("test1", "v1", true, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
	}

	results, err := engine.GetResults("test1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results.Variants) != 1 {
		t.Fatalf("expected 1 variant result, got %d", len(results.Variants))
	}

	metrics := results.Variants[0].Metrics
	if metrics.TotalRuns != 10 {
		t.Errorf("expected 10 total runs, got %d", metrics.TotalRuns)
	}
	if metrics.SuccessCount != 10 {
		t.Errorf("expected 10 successes, got %d", metrics.SuccessCount)
	}
	if metrics.SuccessRate != 1.0 {
		t.Errorf("expected success rate 1.0, got %f", metrics.SuccessRate)
	}
}

func TestStopTest(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", TrafficPct: 100},
		},
	}

	_ = engine.CreateTest(test)
	_ = engine.StopTest("test1")

	got, _ := engine.GetTest("test1")
	if got.Status != "completed" {
		t.Errorf("expected status completed, got %s", got.Status)
	}
}

func TestListTests(t *testing.T) {
	engine := experiment.NewEngine(nil)

	_ = engine.CreateTest(&experiment.Test{
		ID:       "test1",
		Name:     "Test 1",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{{ID: "v1", TrafficPct: 100}},
	})

	_ = engine.CreateTest(&experiment.Test{
		ID:       "test2",
		Name:     "Test 2",
		PromptID: "prompt2",
		Variants: []*experiment.Variant{{ID: "v1", TrafficPct: 100}},
	})

	tests := engine.ListTests()
	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}
}

func TestCreateTestDuplicateID(t *testing.T) {
	engine := experiment.NewEngine(nil)
	_ = engine.CreateTest(makeTest("dup"))
	err := engine.CreateTest(makeTest("dup"))
	if err == nil {
		t.Fatal("expected error for duplicate test ID")
	}
}

func TestGetTestNotFound(t *testing.T) {
	engine := experiment.NewEngine(nil)
	_, err := engine.GetTest("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent test")
	}
}

func TestStopTestNotFound(t *testing.T) {
	engine := experiment.NewEngine(nil)
	err := engine.StopTest("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent test")
	}
}

func TestSelectVariantNotFound(t *testing.T) {
	engine := experiment.NewEngine(nil)
	_, err := engine.SelectVariant("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent test")
	}
}

func TestSelectVariantNotRunning(t *testing.T) {
	engine := experiment.NewEngine(nil)
	_ = engine.CreateTest(makeTest("stopped"))
	_ = engine.StopTest("stopped")
	_, err := engine.SelectVariant("stopped")
	if err == nil {
		t.Fatal("expected error for stopped test")
	}
}

func TestRecordResultMixedSuccessAndFailure(t *testing.T) {
	engine := experiment.NewEngine(nil)
	test := &experiment.Test{
		ID:       "mixed",
		Name:     "Mixed",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", TrafficPct: 100},
		},
		WinCriteria: "success_rate",
		MinSamples:  10,
	}
	_ = engine.CreateTest(test)

	for i := 0; i < 7; i++ {
		engine.RecordResult("mixed", "v1", true, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
	}
	for i := 0; i < 3; i++ {
		engine.RecordResult("mixed", "v1", false, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
	}

	results, err := engine.GetResults("mixed")
	if err != nil {
		t.Fatal(err)
	}
	m := results.Variants[0].Metrics
	if m.TotalRuns != 10 {
		t.Errorf("TotalRuns = %d, want 10", m.TotalRuns)
	}
	if m.SuccessCount != 7 {
		t.Errorf("SuccessCount = %d, want 7", m.SuccessCount)
	}
	if m.ErrorCount != 3 {
		t.Errorf("ErrorCount = %d, want 3", m.ErrorCount)
	}
	if m.SuccessRate != 0.7 {
		t.Errorf("SuccessRate = %f, want 0.7", m.SuccessRate)
	}
}

func TestRecordResultNonexistentVariant(_ *testing.T) {
	engine := experiment.NewEngine(nil)
	_ = engine.CreateTest(makeTest("rrtest"))
	// Should not panic or error
	engine.RecordResult("rrtest", "nonexistent", true, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
}

func TestGetResultsLatencyCriteria(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "lat-test",
		Name:     "Latency Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "fast", Name: "Fast", PromptID: "p1", TrafficPct: 50},
			{ID: "slow", Name: "Slow", PromptID: "p2", TrafficPct: 50},
		},
		WinCriteria: "latency",
		MinSamples:  10,
	}
	_ = engine.CreateTest(test)

	for i := 0; i < 5; i++ {
		engine.RecordResult("lat-test", "fast", true, experiment.ResultMetrics{LatencyMs: 10, Tokens: 50, Cost: 0.001})
		engine.RecordResult("lat-test", "slow", true, experiment.ResultMetrics{LatencyMs: 500, Tokens: 50, Cost: 0.001})
	}

	results, err := engine.GetResults("lat-test")
	if err != nil {
		t.Fatal(err)
	}
	if results.Winner != "fast" {
		t.Errorf("Winner = %s, want fast", results.Winner)
	}
	if len(results.Variants) != 2 {
		t.Fatalf("expected 2 variant results, got %d", len(results.Variants))
	}
	if results.Variants[0].Rank != 1 {
		t.Errorf("Rank[0] = %d, want 1", results.Variants[0].Rank)
	}
	if results.Variants[1].Rank != 2 {
		t.Errorf("Rank[1] = %d, want 2", results.Variants[1].Rank)
	}
}

func TestGetResultsCostCriteria(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "cost-test",
		Name:     "Cost Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "cheap", Name: "Cheap", PromptID: "p1", TrafficPct: 50},
			{ID: "expensive", Name: "Expensive", PromptID: "p2", TrafficPct: 50},
		},
		WinCriteria: "cost",
		MinSamples:  10,
	}
	_ = engine.CreateTest(test)

	for i := 0; i < 5; i++ {
		engine.RecordResult("cost-test", "cheap", true, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
		engine.RecordResult("cost-test", "expensive", true, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.100})
	}

	results, err := engine.GetResults("cost-test")
	if err != nil {
		t.Fatal(err)
	}
	if results.Winner != "cheap" {
		t.Errorf("Winner = %s, want cheap", results.Winner)
	}
}

func TestGetResultsSuccessRateCriteria(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "sr-test",
		Name:     "Success Rate Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "good", Name: "Good", PromptID: "p1", TrafficPct: 50},
			{ID: "bad", Name: "Bad", PromptID: "p2", TrafficPct: 50},
		},
		WinCriteria: "success_rate",
		MinSamples:  10,
	}
	_ = engine.CreateTest(test)

	for i := 0; i < 10; i++ {
		engine.RecordResult("sr-test", "good", true, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
		engine.RecordResult("sr-test", "bad", false, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
	}

	results, err := engine.GetResults("sr-test")
	if err != nil {
		t.Fatal(err)
	}
	if results.Winner != "good" {
		t.Errorf("Winner = %s, want good", results.Winner)
	}
	if !results.IsSignificant {
		t.Error("expected IsSignificant = true")
	}
	if results.Confidence != 1.0 {
		t.Errorf("Confidence = %f, want 1.0", results.Confidence)
	}
	if results.Status != "running" {
		t.Errorf("Status = %s, want running", results.Status)
	}
}

func TestGetResultsConfidenceClamped(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "conf-test",
		Name:     "Confidence",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", TrafficPct: 100},
		},
		MinSamples: 5,
	}
	_ = engine.CreateTest(test)

	for i := 0; i < 10; i++ {
		engine.RecordResult("conf-test", "v1", true, experiment.ResultMetrics{LatencyMs: 100, Tokens: 50, Cost: 0.001})
	}

	results, err := engine.GetResults("conf-test")
	if err != nil {
		t.Fatal(err)
	}
	if results.Confidence != 1.0 {
		t.Errorf("Confidence = %f, want 1.0", results.Confidence)
	}
	if !results.IsSignificant {
		t.Error("expected IsSignificant = true")
	}
}

func TestSelectVariantFallback(t *testing.T) {
	engine := experiment.NewEngine(nil)

	// Test with a single variant at 100% traffic to exercise the fallback path
	test := &experiment.Test{
		ID:       "fallback",
		Name:     "Fallback",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "only", TrafficPct: 100},
		},
	}
	_ = engine.CreateTest(test)

	v, err := engine.SelectVariant("fallback")
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != "only" {
		t.Errorf("expected 'only', got %s", v.ID)
	}
}

func TestCreateTestWeights(t *testing.T) {
	engine := experiment.NewEngine(nil)

	test := &experiment.Test{
		ID:       "weight-test",
		Name:     "Weight Test",
		PromptID: "prompt1",
		Variants: []*experiment.Variant{
			{ID: "v1", TrafficPct: 25},
			{ID: "v2", TrafficPct: 75},
		},
	}
	err := engine.CreateTest(test)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := engine.GetTest("weight-test")
	for _, v := range got.Variants {
		expectedWeight := v.TrafficPct / 100.0
		if v.Weight != expectedWeight {
			t.Errorf("Variant %s: Weight = %f, want %f", v.ID, v.Weight, expectedWeight)
		}
	}
}
