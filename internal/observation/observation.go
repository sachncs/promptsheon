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

// Aggregator rolls a window of ExecutionRecord values into
// Observation values keyed by (capability_id, version_id, environment).
//
// Aggregator is safe for concurrent use; the same mutex guards every
// operation. The producer goroutine should call Add once per
// ExecutionRecord. The consumer goroutine should call Aggregate
// once per window tick.
type Aggregator struct {
	mu      sync.Mutex
	records map[windowKey][]executor.ExecutionRecord
}

// windowKey is the dimension over which observations are aggregated.
type windowKey struct {
	CapabilityID string
	VersionID    string
	Environment  string
}

// NewAggregator constructs an empty Aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{records: map[windowKey][]executor.ExecutionRecord{}}
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
func (a *Aggregator) Aggregate(now time.Time) []rules.Observation {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]rules.Observation, 0, len(a.records))
	for k, recs := range a.records {
		if len(recs) == 0 {
			continue
		}
		out = append(out, summarise(k, recs))
	}
	return out
}

// summarise projects the records slice into the scalar shape the
// rules engine reads. P95 latency is approximated by the maximum in
// the window; the rule engine treats it as a coarse trigger.
func summarise(k windowKey, recs []executor.ExecutionRecord) rules.Observation {
	var (
		totalCostMicro int64
		latencyMax     int64
		halluSum       float64
		failures       int64
		successes      int64
	)
	for _, r := range recs {
		totalCostMicro += int64(r.CostUSD * 1e6)
		if r.LatencyMS > latencyMax {
			latencyMax = r.LatencyMS
		}
		if r.Status == "ok" {
			successes++
		} else {
			failures++
		}
		// Hallucination rate is not yet measured by the executor; the
		// Observation struct carries it as zero today. The
		// guardrail evaluator (M2 follow-on) will populate it.
	}
	return rules.Observation{
		CapabilityID:      k.CapabilityID,
		CapabilityVersion: k.VersionID,
		Environment:       k.Environment,
		WindowExecutions:  int64(len(recs)),
		P95LatencyMS:      latencyMax,
		AvgCostUSDMicro:   totalCostMicro / int64(len(recs)),
		HallucinationRate: halluSum / float64(len(recs)),
		SuccessRate:       float64(successes) / float64(len(recs)),
	}
}
