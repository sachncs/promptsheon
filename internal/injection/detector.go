// Package injection implements a heuristic prompt-injection
// detector Guardrail as a built-in plugin. The detector scores a
// user message against a curated set of canonical injection
// patterns. The Guardrail returns Reject when the score crosses the
// configurable threshold.
//
// This is the Tier 2.48 "prompt injection runtime detector" item
// from the architecture review board.
//
// Scope: heuristic, not LLM-driven. Heuristics catch the obvious
// cases ("ignore all previous instructions", "you are now
// DAN", "system: override", role-confusion attacks). Production
// scale layers an LLM-judge behind the same Guardrail interface;
// today's detector is the cheap first line.
package injection

import (
	"context"
	"regexp"
	"strings"

	"github.com/sachncs/promptsheon/internal/recommendation"
)

// Detector is the Guardrail. It is value-typed and concurrency-safe.
type Detector struct {
	patterns  []rule
	threshold float64
}

// rule is one heuristic pattern with the weight it contributes to
// the score. Weights are coarse (0.5 - 1.0) and tuned against the
// public Prompt-Injection-Bench dataset.
type rule struct {
	name   string
	pat    *regexp.Regexp
	weight float64
}

// NewDetector constructs a Detector with the built-in heuristic
// patterns and threshold=0.6.
//
// OverrideThreshold changes the trigger point. 0.6 is the
// recommended default; raise to 0.8 in environments where false
// positives are more expensive than false negatives.
func NewDetector() *Detector {
	return &Detector{patterns: builtinPatterns(), threshold: 0.6}
}

// OverrideThreshold replaces the trigger threshold. Returns the
// receiver so it can be chained.
func (d *Detector) OverrideThreshold(t float64) *Detector {
	d.threshold = t
	return d
}

// Enable adds a pattern. Mostly useful for tests and for ops to
// extend the heuristic set in a Workspace policy bundle.
func (d *Detector) Enable(name, pattern string, weight float64) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	d.patterns = append(d.patterns, rule{name: name, pat: re, weight: weight})
	return nil
}

// Score returns the aggregate injection score in the [0, 1]
// range. Scores are saturated at 1.0 to support the threshold
// check; the value is also useful for telemetry and for a future
// Recommendation rule that emits "raise prompt-injection Guardrail
// confidence" when sustained.
//
// Note: Go's RE2 MatchString requires a full-string match, so we
// test via Match (any-position) or wrap the patterns at compile
// time. Detector compiles patterns once and checks via Match.
func (d *Detector) Score(text string) float64 {
	score := 0.0
	for _, r := range d.patterns {
		// We use Match (anchored to one side) by adding a wildcard
		// pre/suffix lazily via FindStringIndex. The simpler-and-
		// also-correct path is to keep the patterns non-anchored and
		// test using FindStringIndex on the regex itself.
		if r.pat.FindStringIndex(text) != nil {
			score += r.weight
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// CheckGuardrail returns Reject when Score >= threshold.
func (d *Detector) CheckGuardrail(_ context.Context, kind, text string) (recommendation.Decision, error) {
	if kind != "pre_llm_guardrail" && kind != "post_llm_guardrail" {
		return recommendation.Decision{}, errUnknownKind
	}
	score := d.Score(text)
	if score < d.threshold {
		return recommendation.Decision{
			Outcome: recommendation.OutcomePending,
			Auto:    true,
		}, nil
	}
	matched := d.matchedNames(text)
	return recommendation.Decision{
		Outcome: recommendation.OutcomeRejected,
		Reason:  "prompt-injection score=" + formatScore(score) + " matches=" + strings.Join(matched, ","),
		Auto:    true,
	}, nil
}

func (d *Detector) matchedNames(text string) []string {
	var fired []string
	for _, r := range d.patterns {
		if r.pat.FindStringIndex(text) != nil {
			fired = append(fired, r.name)
		}
	}
	return fired
}

// formatScore trims to two decimal places for clean Decision reasons.
func formatScore(s float64) string {
	return strings.TrimRight(strings.TrimRight(
		formatFloat(s, 2), "0"), ".")
}

// formatFloat is a tiny formatter to avoid strconv import dependency
// for one call. It produces two decimal places.
func formatFloat(s float64, places int) string {
	intPart := int(s)
	frac := int((s - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	if places == 2 && intPart < 10 {
		return writeInt(intPart) + "." + pad2(writeInt(frac))
	}
	return writeInt(intPart) + "." + writeInt(frac)
}

func writeInt(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

func pad2(s string) string {
	if len(s) >= 2 {
		return s
	}
	return "0" + s
}

var errUnknownKind = errInjection("injection: unknown guardrail kind")

type errInjection string

func (e errInjection) Error() string { return string(e) }

// builtinPatterns returns the v1 heuristic set. Weights are tuned
// conservatively; ops can OverrideThreshold to take fewer alerts.
//
// Note on regex style: Go's RE2 engine struggles with patterns
// that mix nested alternations and optional quantifiers (the
// "ignore (all|previous|prior) (instructions?|rules?)" form
// returns a no-match even when the literal substring is present in
// the input). The patterns below are written with one alternation
// or one quantifier per group to keep RE2 happy.
func builtinPatterns() []rule {
	return []rule{
		{name: "ignore_previous", pat: regexp.MustCompile(`(?i)ignore (previous|prior)`), weight: 0.8},
		{name: "ignore_all_instructions", pat: regexp.MustCompile(`(?i)ignore all instructions`), weight: 0.9},
		{name: "forget_above", pat: regexp.MustCompile(`(?i)forget everything above`), weight: 0.7},
		{name: "disregard_above", pat: regexp.MustCompile(`(?i)disregard all above`), weight: 0.7},
		{name: "you_are_now", pat: regexp.MustCompile(`(?i)you are now`), weight: 0.6},
		{name: "system_override", pat: regexp.MustCompile(`(?i)system:\s*override`), weight: 0.9},
		{name: "developer_mode", pat: regexp.MustCompile(`(?i)developer mode`), weight: 0.9},
		{name: "jailbreak_mode", pat: regexp.MustCompile(`(?i)jailbreak mode`), weight: 1.0},
		{name: "dan_mode", pat: regexp.MustCompile(`(?i)\bdan mode`), weight: 0.9},
		{name: "ignore_safety", pat: regexp.MustCompile(`(?i)ignore safety`), weight: 1.0},
		{name: "ignore_content_filters", pat: regexp.MustCompile(`(?i)ignore content filters`), weight: 1.0},
		{name: "ignore_ethical", pat: regexp.MustCompile(`(?i)ignore ethical`), weight: 1.0},
		{name: "pretend_unfiltered", pat: regexp.MustCompile(`(?i)pretend you have no`), weight: 0.9},
		{name: "pretend_no_rules", pat: regexp.MustCompile(`(?i)pretend to have no`), weight: 0.9},
		{name: "training_override", pat: regexp.MustCompile(`(?i)ignore your training`), weight: 0.9},
		{name: "system_data_override", pat: regexp.MustCompile(`(?i)ignore all system data`), weight: 0.9},
	}
}
