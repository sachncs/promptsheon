package eval_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/eval"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func TestExactMatch(t *testing.T) {
	cases := []struct {
		name     string
		actual   string
		expected string
		want     bool
	}{
		{"equal strings", `"hi"`, `"hi"`, true},
		{"different strings", `"hi"`, `"bye"`, false},
		{"equal objects", `{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		{"equal whitespace", `{"a":1}`, `{ "a": 1 }`, true},
		{"numbers vs strings", `1`, `"1"`, false},
		{"empty equals empty", ``, ``, true},
	}
	s := eval.ExactMatch{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ScoreCase(raw(tc.actual), raw(tc.expected))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		actual, expected string
		want             bool
	}{
		{`"hello world"`, `"world"`, true},
		{`"hello world"`, `"WORLD"`, false},
		{`"hi"`, `""`, true},
		{`"hi"`, `"bye"`, false},
	}
	s := eval.Contains{}
	for _, tc := range cases {
		t.Run(tc.actual+"|"+tc.expected, func(t *testing.T) {
			got, err := s.ScoreCase(raw(tc.actual), raw(tc.expected))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestRegex(t *testing.T) {
	cases := []struct {
		actual, pattern string
		want            bool
	}{
		{`"claude-opus-4"`, `^claude-`, true},
		{`"claude-opus-4"`, `^gpt-`, false},
		{`"123-abc"`, `\d+-\w+`, true},
	}
	s := eval.Regex{}
	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			got, err := s.ScoreCase(raw(tc.actual), raw(tc.pattern))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestRegexInvalid(t *testing.T) {
	s := eval.Regex{}
	_, err := s.ScoreCase(raw(`"x"`), raw(`[`))
	if err == nil {
		t.Fatal("expected compile error")
	}
	if !strings.Contains(err.Error(), "regex compile") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJSONSchemaPlaceholder(t *testing.T) {
	s := eval.JSONSchema{}
	got, err := s.ScoreCase(raw(`"x"`), raw(`{}`))
	if err == nil {
		t.Fatal("expected error from placeholder scorer")
	}
	if got {
		t.Fatal("placeholder scorer must return passed=false")
	}
}

func TestRegisteredScorers(t *testing.T) {
	names := eval.Names()
	want := map[eval.Scorer]bool{
		eval.ScorerExactMatch: false,
		eval.ScorerContains:   false,
		eval.ScorerRegex:      false,
		eval.ScorerJSONSchema: false,
	}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, present := range want {
		if !present {
			t.Fatalf("missing registered scorer: %q", n)
		}
	}
}

func TestLookup(t *testing.T) {
	s, ok := eval.Lookup(eval.ScorerExactMatch)
	if !ok || s == nil {
		t.Fatal("expected ExactMatch scorer to be registered")
	}
	if s.Name() != eval.ScorerExactMatch {
		t.Fatalf("got %q want %q", s.Name(), eval.ScorerExactMatch)
	}
}
