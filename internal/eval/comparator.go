package eval

import (
	"sort"

	"promptsheon/internal/models"
)

// Comparison holds a side-by-side comparison of two evaluation reports.
type Comparison struct {
	A    *models.EvalReport `json:"a"`
	B    *models.EvalReport `json:"b"`
	Diff *DiffSummary       `json:"diff"`
}

// DiffSummary quantifies the differences between two reports.
type DiffSummary struct {
	PassRateDelta    float64        `json:"pass_rate_delta"`
	AvgScoreDelta    float64        `json:"avg_score_delta"`
	AvgLatencyDelta  float64        `json:"avg_latency_delta"`
	AvgHallucDelta   float64        `json:"avg_hallucination_delta"`
	TotalTokensDelta int            `json:"total_tokens_delta"`
	TestCaseDiffs    []TestCaseDiff `json:"test_case_diffs"`
}

// TestCaseDiff shows the difference for a specific test case across two runs.
type TestCaseDiff struct {
	TestCaseID string  `json:"test_case_id"`
	ScoreA     float64 `json:"score_a"`
	ScoreB     float64 `json:"score_b"`
	ScoreDelta float64 `json:"score_delta"`
	PassedA    bool    `json:"passed_a"`
	PassedB    bool    `json:"passed_b"`
}

// CompareReports produces a side-by-side comparison of two eval reports.
func CompareReports(a, b *models.EvalReport) *Comparison {
	diff := &DiffSummary{
		PassRateDelta:    b.Aggregate.PassRate - a.Aggregate.PassRate,
		AvgScoreDelta:    b.Aggregate.AvgScore - a.Aggregate.AvgScore,
		AvgLatencyDelta:  b.Aggregate.AvgLatencyMs - a.Aggregate.AvgLatencyMs,
		AvgHallucDelta:   b.Aggregate.AvgHallucination - a.Aggregate.AvgHallucination,
		TotalTokensDelta: b.Aggregate.TotalTokens - a.Aggregate.TotalTokens,
	}

	// Build lookup of B results by test case ID
	bByTC := make(map[string]*models.EvalResult)
	for _, r := range b.Results {
		bByTC[r.TestCaseID] = r
	}

	// Collect all unique test case IDs
	tcIDs := make(map[string]bool)
	for _, r := range a.Results {
		tcIDs[r.TestCaseID] = true
	}
	for _, r := range b.Results {
		tcIDs[r.TestCaseID] = true
	}

	for tcID := range tcIDs {
		td := TestCaseDiff{TestCaseID: tcID}
		if r, ok := bByTC[tcID]; ok {
			td.ScoreB = r.Score
			td.PassedB = r.Passed
		}
		for _, r := range a.Results {
			if r.TestCaseID == tcID {
				td.ScoreA = r.Score
				td.PassedA = r.Passed
				break
			}
		}
		td.ScoreDelta = td.ScoreB - td.ScoreA
		diff.TestCaseDiffs = append(diff.TestCaseDiffs, td)
	}

	sort.Slice(diff.TestCaseDiffs, func(i, j int) bool {
		return diff.TestCaseDiffs[i].TestCaseID < diff.TestCaseDiffs[j].TestCaseID
	})

	return &Comparison{
		A:    a,
		B:    b,
		Diff: diff,
	}
}
