// Package eval implements the scoring strategies used by harness
// eval runs. Each Scorer receives the case's expected value and the
// actual value the LLM produced and reports pass/fail.
//
// Built-in scorers: exact_match, contains, regex, json_schema. New
// scorers can be registered at runtime via Register.
package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// Scorer is the registered name of a built-in or user-supplied
// scoring strategy.
type Scorer string

const (
	ScorerExactMatch Scorer = "exact_match"
	ScorerContains   Scorer = "contains"
	ScorerRegex      Scorer = "regex"
	ScorerJSONSchema Scorer = "json_schema"
)

// ValidScorers reports whether s is a recognised scorer name.
func ValidScorers(s Scorer) bool {
	switch s {
	case ScorerExactMatch, ScorerContains, ScorerRegex, ScorerJSONSchema:
		return true
	}
	return false
}

// Strategy is the contract every scoring strategy satisfies.
type Strategy interface {
	// Name returns the Scorer enum value this Strategy implements.
	Name() Scorer

	// ScoreCase reports whether the LLM-produced actual matches
	// the case's expected, plus any error encountered during
	// scoring itself (a malformed regex, etc.). A non-nil error
	// marks the case as failed.
	ScoreCase(actual, expected json.RawMessage) (passed bool, err error)
}

var (
	scorerMu sync.RWMutex
	scorers  = map[Scorer]Strategy{}
)

// Register makes a Strategy available by name. Re-registering the
// same name overwrites the prior registration.
func Register(s Strategy) {
	scorerMu.Lock()
	defer scorerMu.Unlock()
	scorers[s.Name()] = s
}

// Lookup returns the registered Strategy for the given name.
func Lookup(name Scorer) (Strategy, bool) {
	scorerMu.RLock()
	defer scorerMu.RUnlock()
	s, ok := scorers[name]
	return s, ok
}

// Names returns the registered scorer names in no particular order.
func Names() []Scorer {
	scorerMu.RLock()
	defer scorerMu.RUnlock()
	out := make([]Scorer, 0, len(scorers))
	for n := range scorers {
		out = append(out, n)
	}
	return out
}

// ExactMatch scores true when actual and expected are byte-equal
// (after JSON canonicalisation if both decode successfully).
type ExactMatch struct{}

func (ExactMatch) Name() Scorer { return ScorerExactMatch }
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

func (Contains) Name() Scorer { return ScorerContains }
func (Contains) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	return strings.Contains(stringof(actual), stringof(expected)), nil
}

// Regex compiles expected as a Go regex and matches it against
// actual. Compilation failure marks the case as failed.
type Regex struct{}

func (Regex) Name() Scorer { return ScorerRegex }
func (Regex) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	pattern := stringof(expected)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("regex compile: %w", err)
	}
	return re.MatchString(stringof(actual)), nil
}

// JSONSchema validates actual against a JSON Schema supplied via
// expected. The implementation supports the JSON Schema Draft 7
// subset operators that cover the eval use cases:
//
//   - type (string|number|integer|boolean|array|object|null)
//   - required (array of property names; only meaningful for object)
//   - properties (map of property name to subschema)
//   - enum (array of allowed values)
//
// Anything outside this supported subset is rejected with
// ErrUnsupportedSchema (SEC-3): a schema that uses only unsupported
// keywords (e.g. allOf without any of the above) used to be
// silently accepted. The new behaviour surfaces the unsupported
// keyword in the error so users know to add type/required/
// properties/enum or to switch scorers.
//
// The implementation does not pull in a full JSON-Schema library:
// the supported subset is small enough that a focused validator is
// cheaper to audit than a vendored copy of santhosh-tekuri or
// xeipuuv/gojsonschema.
type JSONSchema struct{}

func (JSONSchema) Name() Scorer { return ScorerJSONSchema }
func (JSONSchema) ScoreCase(actual, expected json.RawMessage) (bool, error) {
	if len(expected) == 0 {
		return false, errors.New("json_schema: expected (schema) is empty")
	}
	var schema map[string]any
	if err := json.Unmarshal(expected, &schema); err != nil {
		return false, fmt.Errorf("json_schema: schema is not an object: %w", err)
	}
	if u := unsupportedSchemaKeywords(schema); u != "" {
		return false, fmt.Errorf("%w: %s", ErrUnsupportedSchema, u)
	}
	var doc any
	if err := json.Unmarshal(actual, &doc); err != nil {
		return false, fmt.Errorf("json_schema: actual is not JSON: %w", err)
	}
	return validateSchema(doc, schema, "")
}

// ErrUnsupportedSchema is returned by JSONSchema.ScoreCase when
// the supplied schema uses only keywords outside the supported
// subset. Callers should switch to a different scorer or add
// one of the supported keywords to their schema.
var ErrUnsupportedSchema = errors.New("json_schema: schema uses unsupported keywords")

// supportedSchemaKeywords is the closed set of JSON Schema keys
// this scorer honours. Anything outside the set is rejected.
var supportedSchemaKeywords = map[string]struct{}{
	"type":       {},
	"required":   {},
	"properties": {},
	"enum":       {},
}

// unsupportedSchemaKeywords returns the first unsupported key
// found in schema (depth-first). Returns "" when every key is in
// the supported set. The check is structural: it inspects the
// schema's own keys plus every nested subschema under properties,
// required values, and enum values.
func unsupportedSchemaKeywords(schema map[string]any) string {
	for k := range schema {
		if _, ok := supportedSchemaKeywords[k]; !ok {
			return k
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for _, sub := range props {
			if m, ok := sub.(map[string]any); ok {
				if u := unsupportedSchemaKeywords(m); u != "" {
					return u
				}
			}
		}
	}
	return ""
}

// validateSchema is the recursive validator. The path argument
// identifies the JSON path inside the document for error messages.
func validateSchema(doc any, schema map[string]any, path string) (bool, error) {
	if t, ok := schema["type"].(string); ok {
		if !matchesType(doc, t) {
			return false, nil
		}
	}
	if enum, ok := schema["enum"].([]any); ok {
		if !enumContains(doc, enum) {
			return false, nil
		}
	}
	if required, ok := schema["required"].([]any); ok {
		m, isObj := doc.(map[string]any)
		if !isObj {
			return false, nil
		}
		for _, k := range required {
			name, ok := k.(string)
			if !ok {
				continue
			}
			if _, present := m[name]; !present {
				return false, nil
			}
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		m, isObj := doc.(map[string]any)
		if isObj {
			for name, sub := range props {
				subSchema, ok := sub.(map[string]any)
				if !ok {
					continue
				}
				if v, present := m[name]; present {
					subPath := path
					if subPath == "" {
						subPath = name
					} else {
						subPath = subPath + "." + name
					}
					ok, err := validateSchema(v, subSchema, subPath)
					if err != nil {
						return false, err
					}
					if !ok {
						return false, nil
					}
				}
			}
		}
	}
	return true, nil
}

// matchesType reports whether doc satisfies the supplied JSON Schema
// "type" value. The "number" type matches both float and integer Go
// values; "integer" requires a whole number.
func matchesType(doc any, t string) bool {
	if doc == nil {
		return t == "null"
	}
	switch t {
	case "string":
		_, ok := doc.(string)
		return ok
	case "number":
		switch doc.(type) {
		case float64, float32, int, int32, int64:
			return true
		}
		return false
	case "integer":
		v, ok := toFloat(doc)
		if !ok {
			return false
		}
		return v == float64(int64(v))
	case "boolean":
		_, ok := doc.(bool)
		return ok
	case "array":
		_, ok := doc.([]any)
		return ok
	case "object":
		_, ok := doc.(map[string]any)
		return ok
	case "null":
		return doc == nil
	}
	return true
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

// enumContains reports whether doc appears in enum. JSON equality is
// implemented via reflect.DeepEqual.
func enumContains(doc any, enum []any) bool {
	for _, e := range enum {
		if reflect.DeepEqual(doc, e) {
			return true
		}
	}
	return false
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
