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
// backed by ClickHouse. For v1 the aggregator is the right shape
// for unit tests and for self-hosted single-node deployments where
// petabytes per day is not the target.
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
// Aggregator is safe for concurrent use. PERF-4b: the global
// mutex was replaced with per-key locks so Adds for different
// (cap, version, env) tuples no longer serialise against each
// other. The outer mutex only protects the map; each bucket has
// its own sync.Mutex for the record slice.
type Aggregator struct {
	mu             sync.Mutex
	records        map[windowKey]*obsBucket
	hallucinationF HallucinationFunc
}

// obsBucket holds the per-window record list and its own mutex.
// Two producers touching different (capability, version, env)
// tuples run in parallel; the contention now is per-key, not
// global.
type obsBucket struct {
	mu      sync.Mutex
	records []executor.ExecutionRecord
}

// windowKey is the dimension over which observations are aggregated.
type windowKey struct {
	CapabilityID string
	VersionID    string
	Environment  string
}

// NewAggregator constructs an empty Aggregator. The supplied
// HallucinationFunc classifies records for the hallucination-rate
// maxRecordsPerWindow caps the per-window record list so a busy
// deployment cannot OOM the daemon. 4096 is roughly an hour at
// 1 invocation/second per window; the recompute is O(N) so this
// is the ceiling above which Aggregate becomes expensive.
const maxRecordsPerWindow = 4096

// PERF-5b: per-tenant memory ceiling. Caps the total number of
// buckets (keyed by cap/version/env) the aggregator will keep in
// memory. A multi-tenant deployment with thousands of unused
// capabilities cannot grow the map without bound; the oldest
// bucket is evicted when the cap is hit. 4096 buckets * 4096
// records per bucket = ~16M records max, way below the 64M
// ceiling on a 64-bit Go runtime.
const maxBuckets = 4096

// field; pass nil to report a zero rate.
func NewAggregator(hallucinationF HallucinationFunc) *Aggregator {
	return &Aggregator{
		records:        map[windowKey]*obsBucket{},
		hallucinationF: hallucinationF,
	}
}

// Add records one execution. The caller may continue to mutate
// the record after this call; Aggregator copies the fields it needs.
//
// The window's record list is bounded to maxRecordsPerWindow;
// once the cap is hit the oldest record is dropped on every
// insert. The previous implementation append-unbounded grew the
// map with every invocation and never pruned, leaking memory
// until the daemon was restarted.
func (a *Aggregator) Add(r executor.ExecutionRecord) {
	k := windowKey{CapabilityID: r.CapabilityID, VersionID: "", Environment: r.Environment}
	if k.VersionID == "" {
		// Until Version tracking lands on ExecutionRecord we
		// use the Release ID as the version key, falling back to
		// ReleaseID when CapabilityID+Environment is the only
		// available discriminator.
		k.VersionID = r.ReleaseID
	}
	a.mu.Lock()
	bucket, ok := a.records[k]
	if !ok {
		bucket = &obsBucket{}
		a.records[k] = bucket
	}
	// PERF-5b: enforce the per-tenant ceiling. If the map grew
	// past maxBuckets, evict the oldest bucket (the one with
	// the smallest "first seen" key by Go's map iteration order
	// is non-deterministic — we instead pick the bucket whose
	// records slice is shortest, i.e. the one least recently
	// active).
	if len(a.records) > maxBuckets {
		var evictKey windowKey
		evictBucket, evictSet := bucket, false
		for k2, b2 := range a.records {
			if k2 == k {
				continue
			}
			b2.mu.Lock()
			isEvict := !evictSet || len(b2.records) < len(evictBucket.records)
			b2.mu.Unlock()
			if isEvict {
				evictKey, evictBucket, evictSet = k2, b2, true
			}
		}
		if evictSet {
			delete(a.records, evictKey)
		}
	}
	a.mu.Unlock()
	bucket.mu.Lock()
	recs := append(bucket.records, r)
	if len(recs) > maxRecordsPerWindow {
		// Drop oldest entries. O(N) copy; N is bounded.
		recs = recs[len(recs)-maxRecordsPerWindow:]
	}
	bucket.records = recs
	bucket.mu.Unlock()
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
	keys := make([]windowKey, 0, len(a.records))
	buckets := make([]*obsBucket, 0, len(a.records))
	for k, b := range a.records {
		keys = append(keys, k)
		buckets = append(buckets, b)
	}
	a.mu.Unlock()
	out := make([]rules.Observation, 0, len(buckets))
	for i, b := range buckets {
		b.mu.Lock()
		recs := b.records
		b.mu.Unlock()
		if len(recs) == 0 {
			continue
		}
		out = append(out, summarise(a.hallucinationF, keys[i], recs))
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
