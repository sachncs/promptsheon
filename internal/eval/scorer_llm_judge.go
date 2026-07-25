// LLM-judge scorer. The "expected" field on the eval case
// carries the judge's rubric (a free-text description of what
// makes a good answer). The judge LLM is called with
// (rubric, actual) and returns a verdict; the scorer parses
// the verdict and reports pass/fail.
//
// The judge is a thin abstraction over the existing LLM
// provider registry: production wiring supplies a JudgeClient
// that wraps a Provider; tests use a fake. The judge is
// deliberately synchronous — the eval runner already runs cases
// serially, so per-case latency is the dominant cost, not
// judge throughput.
package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ScorerLLMJudge is the registered name of the LLM-judge scorer.
const ScorerLLMJudge Scorer = "llm_judge"

// JudgeClient is the abstraction the LLMJudge scorer depends
// on. Production wiring implements JudgeClient over the LLM
// registry (the judge model lives in a different ModelPolicy
// than the one under test); tests use a fake.
type JudgeClient interface {
	// Complete sends prompt to the judge model and returns the
	// raw response. The scorer is responsible for parsing.
	Complete(ctx context.Context, prompt string) (string, error)
}

// LLMJudge is the scorer implementation. The Judge field is
// the only state the scorer holds; the rubric is per-case (it
// lives in the DatasetCase.Expected field).
type LLMJudge struct {
	Judge JudgeClient
	// Timeout is the per-case judge timeout. Zero means no
	// timeout; the run loop's own deadline wins.
	Timeout time.Duration
}

// Name returns the Scorer enum value this Strategy implements.
func (LLMJudge) Name() Scorer { return ScorerLLMJudge }

// ScoreCase calls the judge model with (rubric, actual) and
// parses the verdict. The rubric is the Expected field; the
// model output is Actual. The judge prompt is fixed (see
// buildJudgePrompt) so different judge models see the same
// instructions.
func (j LLMJudge) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	if j.Judge == nil {
		return false, errors.New("llm_judge: JudgeClient is nil")
	}
	ctx := context.Background()
	if j.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, j.Timeout)
		defer cancel()
	}
	prompt := buildJudgePrompt(stringof(expected), stringof(actual))
	raw, err := j.Judge.Complete(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("llm_judge: judge call: %w", err)
	}
	verdict, rationale := parseJudgeResponse(raw)
	switch verdict {
	case verdictPass:
		return true, nil
	case verdictFail:
		return false, nil
	default:
		return false, fmt.Errorf("llm_judge: unparseable verdict: %q (rationale: %q)", raw, rationale)
	}
}

// judgeVerdict is the canonical return value from the judge
// model. Anything else is treated as an error so an LLM that
// drifts off the protocol surfaces a clear failure mode.
type judgeVerdict string

const (
	verdictPass  judgeVerdict = "PASS"
	verdictFail  judgeVerdict = "FAIL"
	verdictError judgeVerdict = "ERROR"
)

// buildJudgePrompt composes the prompt sent to the judge. The
// format is intentionally rigid: the scorer parses VERDICT:
// <PASS|FAIL> as the first non-empty line of the response.
//
// The prompt asks for the rationale on the next line so the
// eval case can carry an auditable justification.
func buildJudgePrompt(rubric, actual string) string {
	var b strings.Builder
	b.WriteString("You are a strict evaluator. Given the rubric and ")
	b.WriteString("the model's actual output, decide whether the ")
	b.WriteString("output satisfies the rubric.\n\n")
	b.WriteString("RUBRIC:\n")
	b.WriteString(rubric)
	b.WriteString("\n\nACTUAL OUTPUT:\n")
	b.WriteString(actual)
	b.WriteString("\n\nRespond EXACTLY in this format on two lines:\n")
	b.WriteString("VERDICT: PASS\nRATIONALE: <one short sentence>\n")
	b.WriteString("or\n")
	b.WriteString("VERDICT: FAIL\nRATIONALE: <one short sentence>\n")
	return b.String()
}

// parseJudgeResponse reads the judge's two-line response. The
// first non-empty line must be "VERDICT: PASS" or "VERDICT:
// FAIL". The second line is treated as the rationale.
//
// parseJudgeResponse is intentionally permissive on
// whitespace and case; LLMs occasionally drift on whitespace.
func parseJudgeResponse(raw string) (judgeVerdict, string) {
	var verdict judgeVerdict
	var rationale string
	verdictSet := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "VERDICT:"):
			rest := strings.TrimSpace(line[len("VERDICT:"):])
			upperRest := strings.ToUpper(rest)
			switch upperRest {
			case "PASS", "YES", "TRUE":
				verdict = verdictPass
			case "FAIL", "NO", "FALSE":
				verdict = verdictFail
			default:
				// unknown verdict value — treat as error so
				// the caller can surface the raw response.
				verdict = verdictError
			}
			verdictSet = true
		case strings.HasPrefix(upper, "RATIONALE:"):
			rationale = strings.TrimSpace(line[len("RATIONALE:"):])
		default:
			// First non-prefixed non-empty line becomes the
			// rationale if we haven't seen a RATIONALE yet.
			// Keeps a verbose judge parseable.
			if rationale == "" {
				rationale = line
			}
		}
	}
	if !verdictSet {
		return verdictError, rationale
	}
	return verdict, rationale
}

// judgeCache memoises judge responses within a single run.
// The eval runner already runs cases serially so this is a
// small optimisation; it primarily absorbs prompt-variance
// when the same (rubric, actual) pair repeats.
//
// The cache is keyed on the SHA-like tuple (rubric, actual);
// collisions across distinct cases are vanishingly rare in
// practice and a false hit only costs a stale verdict, not
// correctness.
type judgeCache struct {
	mu      sync.Mutex
	entries map[string]judgeCacheEntry
}

type judgeCacheEntry struct {
	verdict   judgeVerdict
	rationale string
}

func newJudgeCache() *judgeCache {
	return &judgeCache{entries: map[string]judgeCacheEntry{}}
}

func (c *judgeCache) get(rubric, actual string) (judgeCacheEntry, bool) {
	if c == nil {
		return judgeCacheEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[rubric+"\x00"+actual]
	return e, ok
}

func (c *judgeCache) put(rubric, actual string, v judgeCacheEntry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[rubric+"\x00"+actual] = v
}
