package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/sachn-cs/promptsheon/internal/llm"
)

// HallucinationDetector scores how likely an output is to contain
// hallucinated information relative to the prompt context.
type HallucinationDetector struct{}

// NewHallucinationDetector creates a detector.
func NewHallucinationDetector() *HallucinationDetector {
	return &HallucinationDetector{}
}

// Score returns a hallucination score between 0.0 (no hallucination) and
// 1.0 (highly likely hallucinated).
func (hd *HallucinationDetector) Score(ctx context.Context, provider llm.Provider, prompt, output string, tc any) float64 {
	checks := []func(context.Context, llm.Provider, string, string) float64{
		hd.factualGrounding,
		hd.citationCheck,
		hd.contradictionCheck,
	}

	total := 0.0
	for _, check := range checks {
		total += check(ctx, provider, prompt, output)
	}
	return total / float64(len(checks))
}

// factualGrounding checks if the output contains claims not grounded in the prompt.
// Uses the LLM itself as a judge (LLM-as-judge pattern).
func (hd *HallucinationDetector) factualGrounding(ctx context.Context, provider llm.Provider, prompt, output string) float64 {
	judgePrompt := fmt.Sprintf(`You are a fact-checking judge. Given a SOURCE text and an OUTPUT, determine what percentage of claims in the OUTPUT are NOT supported by the SOURCE.

SOURCE:
%s

OUTPUT:
%s

Respond with ONLY a number between 0.0 and 1.0 representing the fraction of unsupported claims.
0.0 = all claims are supported, 1.0 = all claims are unsupported.
Do not explain. Just the number.`, prompt, output)

	resp, err := provider.Complete(ctx, &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{
			{Role: "user", Content: judgePrompt},
		},
		MaxTokens:   10,
		Temperature: 0,
	})
	if err != nil {
		return 0.5 // conservative default on error
	}

	score := parseScore(resp.Content)
	return clamp(score)
}

// citationCheck verifies that any references or citations in the output exist in the source.
func (hd *HallucinationDetector) citationCheck(_ context.Context, _ llm.Provider, prompt, output string) float64 {
	// Simple heuristic: look for numbered references [1], [2] etc.
	// and check if they appear in the source.
	refs := extractReferences(output)
	if len(refs) == 0 {
		return 0.0 // no citations to check
	}

	unsupported := 0
	for _, ref := range refs {
		if !strings.Contains(prompt, ref) {
			unsupported++
		}
	}
	return float64(unsupported) / float64(len(refs))
}

// contradictionCheck detects if the output contradicts the source.
func (hd *HallucinationDetector) contradictionCheck(ctx context.Context, provider llm.Provider, prompt, output string) float64 {
	judgePrompt := fmt.Sprintf(`You are a contradiction detector. Given a SOURCE and an OUTPUT, determine if the OUTPUT contradicts any information in the SOURCE.

SOURCE:
%s

OUTPUT:
%s

Respond with ONLY "CONTRADICTION" if contradictions exist, or "CONSISTENT" if no contradictions.
Do not explain.`, prompt, output)

	resp, err := provider.Complete(ctx, &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{
			{Role: "user", Content: judgePrompt},
		},
		MaxTokens:   15,
		Temperature: 0,
	})
	if err != nil {
		return 0.0 // conservative: assume no contradiction on error
	}

	if strings.Contains(strings.ToUpper(resp.Content), "CONTRADICTION") {
		return 1.0
	}
	return 0.0
}

func parseScore(s string) float64 {
	s = strings.TrimSpace(s)
	var score float64
	_, err := fmt.Sscanf(s, "%f", &score)
	if err != nil {
		return 0.5
	}
	return score
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func extractReferences(text string) []string {
	var refs []string
	inBracket := false
	var buf strings.Builder

	for _, ch := range text {
		switch {
		case ch == '[':
			inBracket = true
			buf.Reset()
		case ch == ']' && inBracket:
			inBracket = false
			ref := buf.String()
			if ref != "" {
				refs = append(refs, ref)
			}
		case inBracket:
			buf.WriteRune(ch)
		}
	}
	return refs
}
