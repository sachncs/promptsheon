package capability

// EvaluationResult is generated after benchmarking a capability version
// against its evaluation suite.
type EvaluationResult struct {
	CapabilityVersionID string             `json:"capability_version_id"`
	SuiteName           string             `json:"suite_name,omitempty"`
	Accuracy            float64            `json:"accuracy"`
	Precision           float64            `json:"precision"`
	Recall              float64            `json:"recall"`
	Hallucination       float64            `json:"hallucination"`
	LatencyMs           float64            `json:"latency_ms"`
	CostUSD             float64            `json:"cost_usd"`
	Schema              float64            `json:"schema"`
	Groundedness        float64            `json:"groundedness"`
	PerMetric           map[string]float64 `json:"per_metric,omitempty"`
	ThresholdsMet       bool               `json:"thresholds_met"`
}
