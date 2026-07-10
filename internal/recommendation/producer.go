// Package recommendation wires the deterministic RuleEngine
// (internal/optimizer/rules) to the Execution lifecycle. Each
// scheduled or execution.finished event drives an Aggregation
// pass, and the emitted Observations feed the RuleEngine to
// produce capability.Recommendation values that are persisted and
// published on the EventBus.
//
// The package depends on:
//   - internal/capability   (Recommendation type)
//   - internal/observation  (Aggregator)
//   - internal/optimizer/rules  (Engine)
//   - internal/eventbus      (Publisher)
//
// Consumers compose this package with their own recommendation
// store (today SQLite; Postgres in Tier 1.10). The split between
// "we just emitted Recommendations" and "they were persisted" is
// intentional -- a Recommendation that cannot be persisted is
// still surfaced on the bus for observability.
package recommendation

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/observation"
	"github.com/sachncs/promptsheon/internal/optimizer/rules"
)

// SourceFunc is the in-process bridge from Aggregator to Producer.
// Tests and product wiring both pass a closure that knows how to
// fetch the Aggregator.
type SourceFunc func() []rules.Observation

// SinkFunc persists or forwards Recommendations. production wiring
// delivers to the SQLite / Postgres Repository; tests pass a
// no-op closure.
type SinkFunc func(ctx context.Context, rec *capability.Recommendation) error

// Producer runs on each scheduled tick. It aggregates Observations,
// evaluates the RuleEngine, persists/emits Recommendations, and is
// itself registered on the EventBus so external schedulers can
// "tick" it without an explicit call.
type Producer struct {
	mu        sync.Mutex
	agg       *observation.Aggregator
	engine    *rules.Engine
	publisher eventbus.Publisher
	sink      SinkFunc
	logger    *slog.Logger
}

// New constructs a Producer. sink may be nil for in-memory tests.
func New(agg *observation.Aggregator, eng *rules.Engine, pub eventbus.Publisher, sink SinkFunc, logger *slog.Logger) *Producer {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(noopWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	return &Producer{agg: agg, engine: eng, publisher: pub, sink: sink, logger: logger}
}

// Tick runs one observation pass and emits any produced
// Recommendations. Tick is idempotent: if the aggregator window is
// empty, no Recommendations are produced.
func (p *Producer) Tick(ctx context.Context, now time.Time) ([]capability.Recommendation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	obs := p.agg.Aggregate(now)
	var all []capability.Recommendation
	for _, o := range obs {
		recs := p.engine.Evaluate(ctx, o)
		for i := range recs {
			r := &recs[i]
			if r.ID == "" {
				r.ID = generateID("rec")
			}
			r.CreatedAt = now
			if p.sink != nil {
				if err := p.sink(ctx, r); err != nil {
					p.logger.Warn("recommendation sink error", "err", err, "id", r.ID)
				}
			}
			all = append(all, *r)
		}
	}
	for i := range all {
		r := all[i]
		if err := p.publisher.Publish(capability.Event{
			Type:          capability.EventRecommendationGenerated,
			AggregateID:   r.ID,
			AggregateType: "recommendation",
			Data: map[string]any{
				"capability_version_id": r.CapabilityVersionID,
				"type":                  string(r.Type),
				"confidence":            r.Confidence,
			},
		}); err != nil {
			p.logger.Warn("recommendation publish error", "err", err)
		}
	}
	return all, nil
}

// Subscribe registers the Producer as a handler for the supplied
// EventTypes on the EventBus. The handler ignores events that are
// not Tick triggers; production schedules a periodic tick via
// scheduler.New which calls Tick directly.
func (p *Producer) Subscribe(bus eventbus.Publisher, kinds ...capability.EventType) (eventbus.Subscription, error) {
	return bus.Subscribe(func(ev capability.Event) {
		_, _ = p.Tick(context.Background(), time.Now().UTC())
	}, kinds...)
}

// generateID produces a stable-enough local ID. Real uniqueness
// comes from the persistence layer's UUID.
func generateID(prefix string) string {
	return prefix + "-" + time.Now().UTC().Format("20060102T150405.000000000")
}

// noopWriter avoids an unused-import lint with no logger.
type noopWriter struct{}

func (noopWriter) Write(b []byte) (int, error) { return len(b), nil }

// JSON serialisation helper used by tests and external emitters.
func marshalJSON(v any) ([]byte, error) { return json.Marshal(v) }
