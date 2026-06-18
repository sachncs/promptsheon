package eval

import (
	"regexp"
	"strings"

	"promptsheon/internal/models"
)

// Scorer evaluates an LLM output against a test case and returns a score [0, 1].
type Scorer interface {
	Score(output string, tc *models.TestCase) float64
	Name() string
}

// ExactMatchScorer checks if the output exactly matches ExpectedOutput.
type ExactMatchScorer struct{}

func (ExactMatchScorer) Name() string { return "exact_match" }

func (ExactMatchScorer) Score(output string, tc *models.TestCase) float64 {
	if tc.ExpectedOutput == "" {
		return 1.0
	}
	if strings.TrimSpace(output) == strings.TrimSpace(tc.ExpectedOutput) {
		return 1.0
	}
	return 0.0
}

// ContainsScorer checks if the output contains all expected substrings.
type ContainsScorer struct{}

func (ContainsScorer) Name() string { return "contains" }

func (ContainsScorer) Score(output string, tc *models.TestCase) float64 {
	if len(tc.ExpectedContains) == 0 {
		return 1.0
	}
	lower := strings.ToLower(output)
	matched := 0
	for _, expected := range tc.ExpectedContains {
		if strings.Contains(lower, strings.ToLower(expected)) {
			matched++
		}
	}
	return float64(matched) / float64(len(tc.ExpectedContains))
}

// RegexScorer checks if the output matches a regex pattern stored in Tags[0].
type RegexScorer struct{}

func (RegexScorer) Name() string { return "regex" }

func (RegexScorer) Score(output string, tc *models.TestCase) float64 {
	if len(tc.Tags) == 0 {
		return 1.0
	}
	pattern := tc.Tags[0]
	matched, err := regexp.MatchString(pattern, output)
	if err != nil || !matched {
		return 0.0
	}
	return 1.0
}

// CompositeScorer combines multiple scorers with weights.
type CompositeScorer struct {
	Scorers []WeightedScorer
}

// WeightedScorer pairs a scorer with a relative weight for composite scoring.
type WeightedScorer struct {
	Scorer Scorer
	Weight float64
}

func (CompositeScorer) Name() string { return "composite" }

func (cs CompositeScorer) Score(output string, tc *models.TestCase) float64 {
	if len(cs.Scorers) == 0 {
		return 1.0
	}
	totalWeight := 0.0
	totalScore := 0.0
	for _, ws := range cs.Scorers {
		totalScore += ws.Scorer.Score(output, tc) * ws.Weight
		totalWeight += ws.Weight
	}
	if totalWeight == 0 {
		return 1.0
	}
	return totalScore / totalWeight
}

// PassThruScorer always returns 1.0. Used when no scoring is needed.
type PassThruScorer struct{}

func (PassThruScorer) Name() string                           { return "pass_thru" }
func (PassThruScorer) Score(string, *models.TestCase) float64 { return 1.0 }
