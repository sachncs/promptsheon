// Package eval provides evaluation runners for capability versions.
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/llm"
)

// Dataset is a list of evaluation examples. Each example carries
// the input variables, the expected output, and optional
// per-example metadata for the scorer.
type Dataset struct {
	Name     string
	Examples []Example
}

// Example is one input/expected pair.
type Example struct {
	Name     string
	Inputs   map[string]string
	Expected string
	Tags     []string
}

// Scorer classifies a (expected, actual) pair into a 0.0-1.0 score.
// The default scorer is ExactMatch; production wiring may plug in
// an LLM-judge-based scorer.
type Scorer interface {
	Score(expected, actual string, example Example) float64
}

// ExactMatch returns 1.0 when expected equals actual, 0.0
// otherwise. The default Scorer.
type ExactMatch struct{}

// Score implements Scorer.
func (ExactMatch) Score(expected, actual string, _ Example) float64 {
	if expected == actual {
		return 1.0
	}
	return 0.0
}

// ContainsMatch returns 1.0 when expected is a substring of
// actual (case-insensitive). Useful for keyword-style evals.
type ContainsMatch struct{}

// Score implements Scorer.
func (ContainsMatch) Score(expected, actual string, _ Example) float64 {
	if expected == "" {
		return 0.0
	}
	return containsFold(actual, expected)
}

// containsFold is a tiny case-insensitive substring test.
func containsFold(haystack, needle string) float64 {
	if len(needle) > len(haystack) {
		return 0.0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if equalFold(haystack[i:i+len(needle)], needle) {
			return 1.0
		}
	}
	return 0.0
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Result is the aggregate outcome of one evaluation run.
type Result struct {
	DatasetName   string
	TotalExamples int
	AverageScore  float64
	Passed        int
	Failed        int
	Duration      time.Duration
	PerExample    []ExampleResult
}

// ExampleResult is one example's outcome.
type ExampleResult struct {
	ExampleName string
	Score       float64
	Passed      bool
	Actual      string
	Error       string
}

// Runner executes evaluation runs for capability versions. The
// Runner dials the LLM provider once per example, scores the
// output against expected, and aggregates the per-example
// outcomes into a Result.
type Runner struct {
	provider llm.Provider
	scorer   Scorer
}

// NewRunner creates a new Runner with the supplied provider. The
// scorer defaults to ExactMatch; callers wanting a different
// policy call SetScorer.
func NewRunner(provider llm.Provider) *Runner {
	return &Runner{provider: provider, scorer: ExactMatch{}}
}

// SetScorer overrides the default ExactMatch scorer.
func (r *Runner) SetScorer(s Scorer) {
	if s != nil {
		r.scorer = s
	}
}

// Evaluate runs the supplied dataset against the supplied
// capability version. Each Example's Inputs are substituted into
// the version's prompt template via {{var}} placeholders. The
// resulting text is sent to the LLM provider; the response is
// scored against the example's Expected.
//
// Concurrency: Evaluate runs examples serially today. The
// production-grade path uses a worker pool sized by the
// provider's MaxConcurrency; future commits add that. For
// control-plane eval sets (tens of examples) serial is fine.
func (r *Runner) Evaluate(ctx context.Context, version *capability.Version, ds Dataset) (*Result, error) {
	if version == nil {
		return nil, fmt.Errorf("eval: nil version")
	}
	if r.provider == nil {
		return nil, fmt.Errorf("eval: nil provider")
	}
	if r.scorer == nil {
		r.scorer = ExactMatch{}
	}
	start := time.Now().UTC()
	res := &Result{DatasetName: ds.Name, TotalExamples: len(ds.Examples)}
	res.PerExample = make([]ExampleResult, 0, len(ds.Examples))

	var mu sync.Mutex
	for _, ex := range ds.Examples {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}
		prompt := substitute(version.Manifest.Prompt.Hash, ex.Inputs)
		llmReq := &llm.Request{
			Messages: []llm.Message{{Role: "user", Content: prompt}},
		}
		er := ExampleResult{ExampleName: ex.Name}
		resp, err := r.provider.Complete(ctx, llmReq)
		if err != nil {
			er.Error = err.Error()
		} else {
			er.Actual = resp.Content
			er.Score = r.scorer.Score(ex.Expected, resp.Content, ex)
			er.Passed = er.Score >= 0.5
		}
		mu.Lock()
		res.PerExample = append(res.PerExample, er)
		if er.Passed {
			res.Passed++
		} else {
			res.Failed++
		}
		mu.Unlock()
	}
	if res.TotalExamples > 0 {
		var sum float64
		for _, e := range res.PerExample {
			sum += e.Score
		}
		res.AverageScore = sum / float64(res.TotalExamples)
	}
	res.Duration = time.Since(start)
	return res, nil
}

// substitute replaces every {{key}} in tmpl with the matching
// value from inputs. Unmatched keys remain as literal text so
// the test failure is observable.
func substitute(tmpl string, inputs map[string]string) string {
	if len(inputs) == 0 {
		return tmpl
	}
	out := tmpl
	for k, v := range inputs {
		out = replaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

// replaceAll is a tiny strings.ReplaceAll equivalent to avoid the
// import. The function is small enough that inlining keeps the
// dependency surface flat for the eval package.
func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	out := make([]byte, 0, len(s))
	i := 0
	for {
		j := indexOf(s[i:], old)
		if j < 0 {
			out = append(out, s[i:]...)
			return string(out)
		}
		out = append(out, s[i:i+j]...)
		out = append(out, new...)
		i += j + len(old)
	}
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	if len(sub) > len(s) {
		return -1
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// MarshalJSON returns a stable JSON encoding of the result.
// Useful for shipping Result off to a downstream aggregator.
func (r Result) MarshalJSON() ([]byte, error) {
	type wire struct {
		DatasetName   string          `json:"dataset_name"`
		TotalExamples int             `json:"total_examples"`
		AverageScore  float64         `json:"average_score"`
		Passed        int             `json:"passed"`
		Failed        int             `json:"failed"`
		DurationMS    int64           `json:"duration_ms"`
		PerExample    []ExampleResult `json:"per_example"`
	}
	return json.Marshal(wire{
		DatasetName:   r.DatasetName,
		TotalExamples: r.TotalExamples,
		AverageScore:  r.AverageScore,
		Passed:        r.Passed,
		Failed:        r.Failed,
		DurationMS:    r.Duration.Milliseconds(),
		PerExample:    r.PerExample,
	})
}
