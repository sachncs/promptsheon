// Package observation aggregates execution windows into Observation
// values that the rule engine and other consumers can read.
//
// An Observation summarises a (capability, version, environment)
// over a fixed window. The set of metrics is deliberately small:
// the v1 rule engine (internal/optimizer/rules) reads only a few
// scalars. Future engines -- bandit, LLM-judge -- will ask for
// more, at which point this package grows new ObserveWindow events
// or new fields per the consumer-defined-interface principle.
//
// Aggregation is in-memory and runs in the same goroutine as the
// executor's caller; for production scale the aggregator will be
// backed by ClickHouse (Tier 3 follow-on). For v1 the aggregator is
// the right shape for unit tests and for self-hosted single-node
// deployments where petabytes per day is not the target.
package observation

import (
	"sync"
	"time"

	"github.com/beorn7/perks/quantile"

	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/optimizer/rules"
)

// Source is anything that produces ExecutionRecord values; the
// Executor implements it.
type Source interface {
	Records() <-chan executor.ExecutionRecord
}

// Window is the rolling time interval the aggregator considers.
type Window time.Duration

const (
	WindowFiveMinutes Window = Window(5 * time.Minute)
	WindowHour        Window = Window(time.Hour)
)

// HallucinationFunc classifies an ExecutionRecord as a hallucination
// (true) or not (false). The guardrail manager supplies one at
// construction; when nil, hallucination rate is reported as zero
// (the conservative reading is "no signal").
type HallucinationFunc func(executor.ExecutionRecord) bool

// Aggregator rolls a window of ExecutionRecord values into
// Observation values keyed by (capability_id, version_id, environment).
//
// Aggregator is safe for concurrent use; the same mutex guards every
// operation. The producer goroutine should call Add once per
// ExecutionRecord. The consumer goroutine should call Aggregate
// once per window tick.
type Aggregator struct {
	mu             sync.Mutex
	records        map[windowKey][]executor.ExecutionRecord
	hallucinationF HallucinationFunc
}

// windowKey is the dimension over which observations are aggregated.
type windowKey struct {
	CapabilityID string
	VersionID    string
	Environment  string
}

// NewAggregator constructs an empty Aggregator. The supplied
// HallucinationFunc classifies records for the hallucination-rate
// field; pass nil to report a zero rate.
func NewAggregator(hallucinationF HallucinationFunc) *Aggregator {
	return &Aggregator{
		records:        map[windowKey][]executor.ExecutionRecord{},
		hallucinationF: hallucinationF,
	}
}

// Add records one execution. The caller may continue to mutate
// the record after this call; Aggregator copies the fields it needs.
func (a *Aggregator) Add(r executor.ExecutionRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()
	k := windowKey{CapabilityID: r.CapabilityID, VersionID: "", Environment: r.Environment}
	if k.VersionID == "" {
		// Until Version tracking lands on ExecutionRecord we
		// use the Release ID as the version key, falling back to
		// ReleaseID when CapabilityID+Environment is the only
		// available discriminator.
		k.VersionID = r.ReleaseID
	}
	a.records[k] = append(a.records[k], r)
}

// Aggregate returns one Observation per (capability, version, env)
// tuple. RulesEngine consumes them.
//
// P95 latency is computed via a streaming CKMS quantile sketch
// (github.com/beorn7/perks/quantile) so the aggregator runs in
// O(log epsilon^-1) per insertion. Hallucination rate is the
// proportion of records classified as hallucinations by the
// HallucinationFunc supplied at construction.
func (a *Aggregator) Aggregate(now time.Time) []rules.Observation {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]rules.Observation, 0, len(a.records))
	for k, recs := range a.records {
		if len(recs) == 0 {
			continue
		}
		out = append(out, summarise(a.hallucinationF, k, recs))
	}
	return out
}

// summarise projects the records slice into the scalar shape the
// rules engine reads. P95 is computed via the streaming quantile
// sketch, not the maximum, so a single outlier no longer
// dominates the trigger.
func summarise(halluF HallucinationFunc, k windowKey, recs []executor.ExecutionRecord) rules.Observation {
	q := quantile.NewTargeted(map[float64]float64{0.95: 0.001}) // p95 with 0.1% error
	var (
		totalCostMicro int64
		halluCount     int64
		failures       int64
		successes      int64
	)
	for _, r := range recs {
		q.Insert(float64(r.LatencyMS))
		totalCostMicro += int64(r.CostUSD * 1e6)
		if r.Status == "ok" {
			successes++
		} else {
			failures++
		}
		if halluF != nil && halluF(r) {
			halluCount++
		}
	}
	p95 := int64(q.Query(0.95))
	return rules.Observation{
		CapabilityID:      k.CapabilityID,
		CapabilityVersion: k.VersionID,
		Environment:       k.Environment,
		WindowExecutions:  int64(len(recs)),
		P95LatencyMS:      p95,
		AvgCostUSDMicro:   totalCostMicro / int64(len(recs)),
		HallucinationRate: float64(halluCount) / float64(len(recs)),
		SuccessRate:       float64(successes) / float64(len(recs)),
	}
}
