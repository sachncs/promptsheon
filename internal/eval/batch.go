package eval

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"promptsheon/internal/models"
)

// BatchConfig controls batch evaluation behavior.
type BatchConfig struct {
	PromptHash  string
	PromptText  string
	Datasets    []*models.TestDataset
	Model       string
	MaxTokens   int
	Concurrency int // max parallel eval runs (0 = len(Datasets))
}

// BatchResult holds the report from a single dataset evaluation within a batch.
type BatchResult struct {
	DatasetID string
	Report    *models.EvalReport
	Error     error
}

// BatchReport is the aggregated output of a batch evaluation.
type BatchReport struct {
	Results   []*BatchResult `json:"results"`
	Aggregate models.Aggregate `json:"aggregate"`
}

// BatchRunner executes evaluations across multiple datasets or model variants
// in parallel.
type BatchRunner struct {
	runner *Runner
}

// NewBatchRunner creates a BatchRunner with the given base runner.
func NewBatchRunner(runner *Runner) *BatchRunner {
	return &BatchRunner{runner: runner}
}

// RunBatch executes evaluations for all datasets concurrently.
func (br *BatchRunner) RunBatch(ctx context.Context, cfg *BatchConfig) (*BatchReport, error) {
	if len(cfg.Datasets) == 0 {
		return nil, fmt.Errorf("at least one dataset is required")
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = len(cfg.Datasets)
	}

	type indexedResult struct {
		index  int
		result *BatchResult
	}

	results := make([]*BatchResult, len(cfg.Datasets))
	ch := make(chan indexedResult, len(cfg.Datasets))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for i, ds := range cfg.Datasets {
		wg.Add(1)
		go func(idx int, dataset *models.TestDataset) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			runCfg := &RunConfig{
				PromptHash: cfg.PromptHash,
				PromptText: cfg.PromptText,
				Dataset:    dataset,
				Model:      cfg.Model,
				MaxTokens:  cfg.MaxTokens,
			}

			report, err := br.runner.Run(ctx, runCfg)
			ch <- indexedResult{
				index: idx,
				result: &BatchResult{
					DatasetID: dataset.ID,
					Report:    report,
					Error:     err,
				},
			}
		}(i, ds)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		results[r.index] = r.result
	}

	// Merge all results into a single aggregate
	allResults := mergeResults(results)
	return &BatchReport{
		Results:   results,
		Aggregate: aggregate(allResults),
	}, nil
}

func mergeResults(batchResults []*BatchResult) []*models.EvalResult {
	var all []*models.EvalResult
	for _, br := range batchResults {
		if br.Error == nil && br.Report != nil {
			all = append(all, br.Report.Results...)
		}
	}
	return all
}

// RunMultiModel runs the same prompt + dataset against multiple models.
func (br *BatchRunner) RunMultiModel(ctx context.Context, promptHash, promptText string, dataset *models.TestDataset, modelNames []string, maxTokens int) (*MultiModelReport, error) {
	reports := make(map[string]*models.EvalReport)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, model := range modelNames {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			cfg := &RunConfig{
				PromptHash: promptHash,
				PromptText: promptText,
				Dataset:    dataset,
				Model:      m,
				MaxTokens:  maxTokens,
			}
			report, err := br.runner.Run(ctx, cfg)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				reports[m] = report
			} else {
				reports[m] = &models.EvalReport{
					Model:  m,
					Aggregate: models.Aggregate{},
				}
			}
		}(model)
	}
	wg.Wait()

	return &MultiModelReport{
		PromptHash: promptHash,
		DatasetID:  dataset.ID,
		Reports:    reports,
	}, nil
}

// MultiModelReport holds comparison data across models.
type MultiModelReport struct {
	PromptHash string                          `json:"prompt_hash"`
	DatasetID  string                          `json:"dataset_id"`
	Reports    map[string]*models.EvalReport   `json:"reports"`
}

// String returns a human-readable comparison table.
func (mmr *MultiModelReport) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Model Comparison (prompt: %s, dataset: %s)\n", mmr.PromptHash, mmr.DatasetID))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	sb.WriteString(fmt.Sprintf("%-20s %8s %8s %10s %10s\n", "Model", "Pass%", "AvgScore", "Latencyms", "Tokens"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for model, report := range mmr.Reports {
		sb.WriteString(fmt.Sprintf("%-20s %7.1f%% %8.3f %10.0f %10d\n",
			model,
			report.Aggregate.PassRate*100,
			report.Aggregate.AvgScore,
			report.Aggregate.AvgLatencyMs,
			report.Aggregate.TotalTokens,
		))
	}
	return sb.String()
}
