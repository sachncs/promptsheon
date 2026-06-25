package abtesting_test

import (
	"testing"

	"github.com/sachn-cs/promptsheon/internal/abtesting"
)

func TestCreateTest(t *testing.T) {
	engine := abtesting.NewEngine(nil)

	test := &abtesting.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*abtesting.Variant{
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
	engine := abtesting.NewEngine(nil)

	test := &abtesting.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*abtesting.Variant{
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
	engine := abtesting.NewEngine(nil)

	test := &abtesting.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*abtesting.Variant{
			{ID: "v1", TrafficPct: 50},
			{ID: "v2", TrafficPct: 50},
		},
	}

	engine.CreateTest(test)

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
	engine := abtesting.NewEngine(nil)

	test := &abtesting.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*abtesting.Variant{
			{ID: "v1", TrafficPct: 100},
		},
	}

	engine.CreateTest(test)

	// Record some results
	for i := 0; i < 10; i++ {
		engine.RecordResult("test1", "v1", true, 100, 50, 0.001)
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
	engine := abtesting.NewEngine(nil)

	test := &abtesting.Test{
		ID:       "test1",
		Name:     "My Test",
		PromptID: "prompt1",
		Variants: []*abtesting.Variant{
			{ID: "v1", TrafficPct: 100},
		},
	}

	engine.CreateTest(test)
	engine.StopTest("test1")

	got, _ := engine.GetTest("test1")
	if got.Status != "completed" {
		t.Errorf("expected status completed, got %s", got.Status)
	}
}

func TestListTests(t *testing.T) {
	engine := abtesting.NewEngine(nil)

	engine.CreateTest(&abtesting.Test{
		ID:       "test1",
		Name:     "Test 1",
		PromptID: "prompt1",
		Variants: []*abtesting.Variant{{ID: "v1", TrafficPct: 100}},
	})

	engine.CreateTest(&abtesting.Test{
		ID:       "test2",
		Name:     "Test 2",
		PromptID: "prompt2",
		Variants: []*abtesting.Variant{{ID: "v1", TrafficPct: 100}},
	})

	tests := engine.ListTests()
	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}
}
