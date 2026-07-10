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
	a := NewAggregator()
	if got := a.Aggregate(time.Now()); len(got) != 0 {
		t.Fatalf("expected empty aggregation, got %d", len(got))
	}
}

func TestAggregatorGroupsByCapEnv(t *testing.T) {
	t.Parallel()
	a := NewAggregator()
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
	a := NewAggregator()
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
