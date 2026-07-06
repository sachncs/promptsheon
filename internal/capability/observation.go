package capability

import "time"

// Observation contains aggregated metrics derived from executions.
//
// Observations are not stored on the Prompt or any configuration artifact.
// They are derived from execution telemetry and represent the system's
// view of how a capability version is performing in production.
type Observation struct {
	CapabilityVersionID string    `json:"capability_version_id"`
	PeriodStart         time.Time `json:"period_start"`
	PeriodEnd           time.Time `json:"period_end"`
	P95LatencyMs        float64   `json:"p95_latency_ms"`
	P99LatencyMs        float64   `json:"p99_latency_ms"`
	AvgCostUSD          float64   `json:"avg_cost_usd"`
	HallucinationRate   float64   `json:"hallucination_rate"`
	SuccessRate         float64   `json:"success_rate"`
	Availability        float64   `json:"availability"`
	ExecutionCount      int64     `json:"execution_count"`
}
