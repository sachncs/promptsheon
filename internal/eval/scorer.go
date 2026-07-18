// Package eval implements the scoring strategies used by harness
// eval runs. Each Scorer receives the case's expected value and the
// actual value the LLM produced and reports pass/fail.
//
// Built-in scorers today: exact_match, contains, regex, json_schema
// (the last is a placeholder; M3 follow-on). New scorers can be
// registered at runtime via Register.
package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/sachncs/promptsheon/internal/harness"
)

// Scorer is the contract every scoring strategy satisfies.
type Scorer interface {
	// Name returns the harness.Scorer enum value this Scorer
	// implements. Used to dispatch case-by-case to the right
	// registered scorer.
	Name() harness.Scorer

	// ScoreCase reports whether the LLM-produced actual matches
	// the case's expected, plus any error encountered during
	// scoring itself (a malformed regex, etc.). A non-nil error
	// marks the case as failed.
	ScoreCase(actual, expected json.RawMessage) (passed bool, err error)
}

var (
	scorerMu sync.RWMutex
	scorers  = map[harness.Scorer]Scorer{}
)

// Register makes a Scorer available by name. Re-registering the
// same name overwrites the prior registration (useful for
// operators who want to swap a built-in for their own implementation
// at process start).
func Register(s Scorer) {
	scorerMu.Lock()
	defer scorerMu.Unlock()
	scorers[s.Name()] = s
}

// Lookup returns the registered Scorer for the given name.
func Lookup(name harness.Scorer) (Scorer, bool) {
	scorerMu.RLock()
	defer scorerMu.RUnlock()
	s, ok := scorers[name]
	return s, ok
}

// Names returns the registered scorer names in no particular order.
func Names() []harness.Scorer {
	scorerMu.RLock()
	defer scorerMu.RUnlock()
	out := make([]harness.Scorer, 0, len(scorers))
	for n := range scorers {
		out = append(out, n)
	}
	return out
}

// ExactMatch scores true when actual and expected are byte-equal
// (after JSON canonicalisation if both decode successfully).
type ExactMatch struct{}

func (ExactMatch) Name() harness.Scorer { return harness.ScorerExactMatch }
func (ExactMatch) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	ac, err := canonicalise(actual)
	if err != nil {
		return false, err
	}
	ex, err := canonicalise(expected)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ac, ex), nil
}

// Contains scores true when the case's expected string is contained
// in the actual string. Both inputs are stringified via JSON
// unmarshal; if either side is non-string the scorer falls back to
// the raw bytes treated as UTF-8 (which may be a JSON document).
type Contains struct{}

func (Contains) Name() harness.Scorer { return harness.ScorerContains }
func (Contains) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	return strings.Contains(stringof(actual), stringof(expected)), nil
}

// Regex compiles expected as a Go regex and matches it against
// actual. Compilation failure marks the case as failed.
type Regex struct{}

func (Regex) Name() harness.Scorer { return harness.ScorerRegex }
func (Regex) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	pattern := stringof(expected)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("regex compile: %w", err)
	}
	return re.MatchString(stringof(actual)), nil
}

// JSONSchema is a placeholder. The M3 follow-on will replace it
// with a JSON-Schema validator backed by a small embedded schema
// engine; today it returns false so callers see the failure rather
// than a silent pass.
type JSONSchema struct{}

func (JSONSchema) Name() harness.Scorer { return harness.ScorerJSONSchema }
func (JSONSchema) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	return false, errors.New("json_schema scorer not yet implemented (M3 follow-on)")
}

// canonicalise decodes v as JSON and re-encodes it compactly so
// whitespace differences don't break exact_match. Falls back to the
// raw bytes when v isn't valid JSON.
func canonicalise(v json.RawMessage) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	var any any
	if err := json.Unmarshal(v, &any); err != nil {
		// Not JSON — treat as opaque bytes (a prompt string, etc).
		return v, nil
	}
	return json.Marshal(any)
}

// stringof returns the JSON string form of v if v is a string,
// otherwise the raw bytes interpreted as UTF-8. Used by scorers
// that operate on a string comparison.
func stringof(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s
	}
	// Best effort: strip surrounding quotes if present.
	out := string(v)
	if len(out) >= 2 && out[0] == '"' && out[len(out)-1] == '"' {
		return out[1 : len(out)-1]
	}
	return out
}

func init() {
	Register(ExactMatch{})
	Register(Contains{})
	Register(Regex{})
	Register(JSONSchema{})
}
