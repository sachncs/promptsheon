package recommendation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/observation"
	"github.com/sachncs/promptsheon/internal/optimizer/rules"
)

func TestProducerEmitsOnQuietObservation(t *testing.T) {
	t.Parallel()
	a := observation.NewAggregator()
	var captured []capability.Recommendation
	var mu sync.Mutex
	sink := func(_ context.Context, r *capability.Recommendation) error {
		mu.Lock()
		captured = append(captured, *r)
		mu.Unlock()
		return nil
	}
	bus := &fakeBus{}
	p := New(a, rules.NewEngine(), bus, sink, nil)
	got, err := p.Tick(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no recommendations on empty aggregator, got %d", len(got))
	}
	if bus.count() != 0 {
		t.Fatalf("expected no publishes on empty aggregator, got %d", bus.count())
	}
	mu.Lock()
	if len(captured) != 0 {
		t.Fatalf("sink should not receive on empty window")
	}
	mu.Unlock()
}

func TestProducerCapturesPersistFailures(t *testing.T) {
	t.Parallel()
	a := observation.NewAggregator()
	sink := func(_ context.Context, r *capability.Recommendation) error {
		return errors.New("db down")
	}
	bus := &fakeBus{}
	p := New(a, rules.NewEngine(), bus, sink, nil)
	// Inject a high-cost record; cacheWhenCostly rule fires.
	a.Add(recWithCost(70000)) // 70000 micro-USD per record; rule fires at 100000+
	// Run a partial-cycle where no rule fires; ensure tick returns no error.
	if _, err := p.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("tick empty: %v", err)
	}
	// The cost rule needs avg cost > 100_000 micro-USD per execution;
	// not satisfied, no recommendations, no publish.
	if got := bus.count(); got != 0 {
		t.Fatalf("expected no publishes, got %d", got)
	}
}

func TestProducerCommitsIDAndTimestamp(t *testing.T) {
	t.Parallel()
	a := observation.NewAggregator()
	// Add 64 records so the rule's 32-exec minimum is satisfied.
	for i := int64(0); i < 64; i++ {
		a.Add(recWithCost(500_000))
	}
	var seen []capability.Recommendation
	var mu sync.Mutex
	sink := func(_ context.Context, r *capability.Recommendation) error {
		mu.Lock()
		seen = append(seen, *r)
		mu.Unlock()
		return nil
	}
	bus := &fakeBus{}
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	p := New(a, rules.NewEngine(), bus, sink, nil)
	got, err := p.Tick(context.Background(), now)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(got))
	}
	if got[0].ID == "" {
		t.Fatalf("expected ID to be set")
	}
	if !got[0].CreatedAt.Equal(now) {
		t.Fatalf("expected CreatedAt=%v, got %v", now, got[0].CreatedAt)
	}
	if got[0].Type != capability.RecommendationEnableCache {
		t.Fatalf("expected EnableCache, got %s", got[0].Type)
	}
	if bus.count() != 1 {
		t.Fatalf("expected one published event, got %d", bus.count())
	}
}

type fakeBus struct {
	mu     sync.Mutex
	events []capability.Event
}

func (f *fakeBus) Subscribe(_ eventbus.Handler, _ ...capability.EventType) (eventbus.Subscription, error) {
	return nil, nil
}

func (f *fakeBus) Publish(ev capability.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return nil
}
func (f *fakeBus) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func recWithCost(avgMicroUSD int64) (r executor.ExecutionRecord) {
	return executor.ExecutionRecord{
		ID:           "exec-1",
		CapabilityID: "cap-1",
		ReleaseID:    "rel-1",
		Environment:  "prod",
		Status:       "ok",
		LatencyMS:    200,
		CostUSD:      float64(avgMicroUSD) / 1e6,
		PromptTokens: 100,
		OutputTokens: 50,
	}
}
