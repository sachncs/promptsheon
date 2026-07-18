package observation

import (
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/executor"
)

func mk(cap, version, env string, status string, latencyMS int64, costUSD float64) executor.ExecutionRecord {
	now := time.Now().UTC()
	return executor.ExecutionRecord{
		ID:           "exec-" + cap + "-" + version + "-" + env,
		CapabilityID: cap,
		ReleaseID:    version,
		Environment:  env,
		LatencyMS:    latencyMS,
		CostUSD:      costUSD,
		Status:       status,
		StartedAt:    now,
		FinishedAt:   now.Add(time.Duration(latencyMS) * time.Millisecond),
		PromptTokens: 100,
		OutputTokens: 50,
	}
}

func TestAggregatorEmptyByDefault(t *testing.T) {
	t.Parallel()
	a := NewAggregator(nil)
	if got := a.Aggregate(time.Now()); len(got) != 0 {
		t.Fatalf("expected empty aggregation, got %d", len(got))
	}
}

func TestAggregatorGroupsByCapEnv(t *testing.T) {
	t.Parallel()
	a := NewAggregator(nil)
	a.Add(mk("c1", "v1", "prod", "ok", 200, 0.01))
	a.Add(mk("c1", "v1", "prod", "ok", 400, 0.02))
	a.Add(mk("c2", "v1", "prod", "ok", 100, 0.005))
	got := a.Aggregate(time.Now())
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(got))
	}
	// Both groups should have reasonable stats.
	for _, o := range got {
		if o.WindowExecutions == 0 {
			t.Fatalf("empty window")
		}
		if o.AvgCostUSDMicro <= 0 {
			t.Fatalf("expected non-zero avg cost")
		}
	}
}

func TestAggregatorTracksErrors(t *testing.T) {
	t.Parallel()
	a := NewAggregator(nil)
	a.Add(mk("c", "v", "prod", "ok", 100, 0.01))
	a.Add(mk("c", "v", "prod", "error", 50, 0))
	a.Add(mk("c", "v", "prod", "ok", 100, 0.01))
	got := a.Aggregate(time.Now())
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	if got[0].SuccessRate <= 0 || got[0].SuccessRate >= 1 {
		t.Fatalf("expected partial success rate, got %f", got[0].SuccessRate)
	}
}

func TestAggregatorP95IsRealQuantile(t *testing.T) {
	t.Parallel()
	a := NewAggregator(nil)
	// 19 fast + 1 outlier; p95 should be near the fast cluster, not 5000.
	for i := 0; i < 19; i++ {
		a.Add(mk("c", "v", "prod", "ok", 100, 0.01))
	}
	a.Add(mk("c", "v", "prod", "ok", 5000, 0.01))
	got := a.Aggregate(time.Now())
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	if got[0].P95LatencyMS > 500 {
		t.Errorf("p95 latency = %d, expected <= 500 (the outlier should not dominate)", got[0].P95LatencyMS)
	}
	if got[0].P95LatencyMS < 50 {
		t.Errorf("p95 latency = %d, expected >= 50 (should reflect the cluster)", got[0].P95LatencyMS)
	}
}

func TestAggregatorHallucinationRate(t *testing.T) {
	t.Parallel()
	halluF := func(r executor.ExecutionRecord) bool {
		return r.Status == "hallucination"
	}
	a := NewAggregator(halluF)
	a.Add(mk("c", "v", "prod", "ok", 100, 0.01))
	a.Add(mk("c", "v", "prod", "hallucination", 100, 0.01))
	a.Add(mk("c", "v", "prod", "hallucination", 100, 0.01))
	a.Add(mk("c", "v", "prod", "ok", 100, 0.01))
	got := a.Aggregate(time.Now())
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	if got[0].HallucinationRate != 0.5 {
		t.Errorf("HallucinationRate = %f, want 0.5", got[0].HallucinationRate)
	}
}

func TestAggregatorNoHallucinationFuncReportsZero(t *testing.T) {
	t.Parallel()
	a := NewAggregator(nil)
	a.Add(mk("c", "v", "prod", "hallucination", 100, 0.01))
	got := a.Aggregate(time.Now())
	if got[0].HallucinationRate != 0 {
		t.Errorf("HallucinationRate with nil func = %f, want 0", got[0].HallucinationRate)
	}
}
