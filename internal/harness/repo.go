package harness

import "context"

// Repository is the consumer-defined persistence interface for the
// harness-engineering aggregates (Datasets, Preconditions, EvalRuns).
// Storage implementations in internal/store satisfy this interface.
type Repository interface {
	// Datasets.
	CreateDataset(ctx context.Context, d *Dataset) error
	GetDataset(ctx context.Context, id string) (*Dataset, error)
	ListDatasetsForCapability(ctx context.Context, capabilityID string) ([]*Dataset, error)
	DeleteDataset(ctx context.Context, id string) error

	// Dataset cases.
	UpsertDatasetCases(ctx context.Context, datasetID string, cases []DatasetCase) error
	ListDatasetCases(ctx context.Context, datasetID string) ([]DatasetCase, error)

	// Preconditions.
	CreatePrecondition(ctx context.Context, p *Precondition) error
	GetPrecondition(ctx context.Context, id string) (*Precondition, error)
	ListPreconditionsForCapability(ctx context.Context, capabilityID string) ([]*Precondition, error)
	UpdatePrecondition(ctx context.Context, p *Precondition) error
	DeletePrecondition(ctx context.Context, id string) error

	// EvalRuns.
	CreateEvalRun(ctx context.Context, run *EvalRun) error
	UpdateEvalRun(ctx context.Context, run *EvalRun) error
	GetEvalRun(ctx context.Context, id string) (*EvalRun, error)
	ListEvalRunsForRelease(ctx context.Context, releaseID string) ([]*EvalRun, error)

	// EvalResults.
	CreateEvalResults(ctx context.Context, results []EvalResult) error
	// PERF-EVAL-2: CreateEvalResult persists a single result row.
	// Used by EvalRunner.Run to stream results to the DB as each
	// case finishes, so memory stays bounded for large datasets.
	// Implementations may fall back to CreateEvalResults internally.
	CreateEvalResult(ctx context.Context, result *EvalResult) error
	ListEvalResultsForRun(ctx context.Context, runID string) ([]EvalResult, error)
}
