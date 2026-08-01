//go:build tests_migration


package eval

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// fakeJudge is a JudgeClient for tests. The Verdict function
// is supplied per call so a single test can drive a sequence
// of cases through PASS / FAIL / ERROR responses.
type fakeJudge struct {
	verdict string
	calls   int
}

func (f *fakeJudge) Complete(_ context.Context, prompt string) (string, error) {
	f.calls++
	return f.verdict, nil
}

func TestParseJudgeResponse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		want   judgeVerdict
		ratLen int
	}{
		{"VERDICT: PASS\nRATIONALE: looks good", verdictPass, 1},
		{"VERDICT: FAIL\nRATIONALE: too short", verdictFail, 1},
		{"verdict: pass\nrationale: lower case ok", verdictPass, 1},
		{"VERDICT: PASS", verdictPass, 0},
		{"", verdictError, 0},
		{"VERDICT: BOGUS", verdictError, 0},
	}
	for _, tc := range cases {
		got, rationale := parseJudgeResponse(tc.in)
		if got != tc.want {
			t.Errorf("parseJudgeResponse(%q) verdict=%q want %q", tc.in, got, tc.want)
		}
		if tc.ratLen == 0 && rationale != "" {
			t.Errorf("parseJudgeResponse(%q) rationale=%q want empty", tc.in, rationale)
		}
		if tc.ratLen == 1 && rationale == "" {
			t.Errorf("parseJudgeResponse(%q) rationale empty", tc.in)
		}
	}
}

func TestBuildJudgePromptShape(t *testing.T) {
	t.Parallel()
	prompt := buildJudgePrompt("be polite", "hello world")
	for _, fragment := range []string{"RUBRIC", "ACTUAL OUTPUT", "VERDICT", "RATIONALE"} {
		if !contains(prompt, fragment) {
			t.Errorf("prompt missing %q", fragment)
		}
	}
}

func TestLLMJudgeScoreCase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		verdict  string
		wantPass bool
		wantErr  bool
	}{
		{"pass", "VERDICT: PASS\nRATIONALE: ok", true, false},
		{"fail", "VERDICT: FAIL\nRATIONALE: no", false, false},
		{"unparseable", "I cannot decide.", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			judge := &fakeJudge{verdict: tc.verdict}
			s := LLMJudge{Judge: judge, Timeout: time.Second}
			actual := json.RawMessage(`"hi"`)
			expected := json.RawMessage(`"be polite"`)
			pass, err := s.ScoreCase(actual, expected)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if pass != tc.wantPass {
				t.Errorf("pass=%v wantPass=%v", pass, tc.wantPass)
			}
			if judge.calls != 1 {
				t.Errorf("expected 1 judge call, got %d", judge.calls)
			}
		})
	}
}

func TestLLMJudgeScoreCaseNilJudge(t *testing.T) {
	t.Parallel()
	s := LLMJudge{}
	_, err := s.ScoreCase(json.RawMessage(`"x"`), json.RawMessage(`"y"`))
	if err == nil {
		t.Fatal("expected error when JudgeClient is nil")
	}
	if !contains(err.Error(), "JudgeClient is nil") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLLMJudgeScoreCaseTimeout(t *testing.T) {
	t.Parallel()
	slow := &slowJudge{}
	s := LLMJudge{Judge: slow, Timeout: 10 * time.Millisecond}
	_, err := s.ScoreCase(json.RawMessage(`"x"`), json.RawMessage(`"y"`))
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !contains(err.Error(), "judge call") {
		t.Errorf("unexpected error: %v", err)
	}
}

type slowJudge struct{}

func (s *slowJudge) Complete(ctx context.Context, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func TestRegisterLLMJudge(t *testing.T) {
	t.Parallel()
	// Save and restore the scorer registry to avoid leaking
	// the test scorer into other tests.
	defer func() {
		// Re-register a no-op judge so subsequent tests have a
		// stable scorer; the global map is intentionally shared.
		RegisterLLMJudge(&fakeJudge{verdict: "VERDICT: PASS"})
	}()
	RegisterLLMJudge(&fakeJudge{verdict: "VERDICT: FAIL"})
	s, ok := Lookup(ScorerLLMJudge)
	if !ok {
		t.Fatal("llm_judge must be registered")
	}
	pass, err := s.ScoreCase(json.RawMessage(`"a"`), json.RawMessage(`"b"`))
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if pass {
		t.Error("expected FAIL")
	}
}

func TestValidScorersIncludesLLMJudge(t *testing.T) {
	t.Parallel()
	if !ValidScorers(ScorerLLMJudge) {
		t.Error("llm_judge must be a valid scorer name")
	}
}

// contains is a tiny strings.Contains alias that survives
// future imports of the strings package in this test file.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Unused but exported so future tests can compose judges.
var _ JudgeClient = (*fakeJudge)(nil)
