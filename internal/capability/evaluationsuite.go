package capability

// EvaluationSuite defines how quality is measured for a capability version.
type EvaluationSuite struct {
	Datasets        []EvalDatasetRef   `json:"datasets,omitempty"`
	Metrics         []string           `json:"metrics,omitempty"`    // "accuracy", "precision", "recall", "hallucination", "latency", "cost"
	Thresholds      map[string]float64 `json:"thresholds,omitempty"` // metric name → minimum acceptable value
	Benchmarks      []BenchmarkRef     `json:"benchmarks,omitempty"`
	RegressionTests []string           `json:"regression_tests,omitempty"`
	SecurityTests   []string           `json:"security_tests,omitempty"`
}

// EvalDatasetRef references a dataset used for evaluation.
type EvalDatasetRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// BenchmarkRef references an external benchmark for comparison.
type BenchmarkRef struct {
	Name    string  `json:"name"`
	Version string  `json:"version,omitempty"`
	Target  float64 `json:"target,omitempty"`
}
