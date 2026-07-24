// Package harnessrepo is a shared in-memory fixture for the
// harness.Repository interface (Datasets, Preconditions,
// EvalRuns, EvalResults). It exists so package-internal test
// fixtures (harness/memrepo_test.go and the harness.Repository
// methods on release/service_test.go's memStore) don't
// duplicate ~120 lines of identical boilerplate.
//
// TEST-3: the fixture lives under internal/testutil because
// the only places that need it are _test.go files. The public
// surface is just MemRepo + New. Callers may read MemRepo's
// internal maps directly to assert on the persisted state.
//
// Concurrency: MemRepo is safe for concurrent use; production
// code paths that share a MemRepo across goroutines are
// supported so the harness package's tests can hand the same
// instance to the runner, the service, and the audit chain
// without re-locking.
package harnessrepo

import (
	"context"
	"errors"
	"sync"

	"github.com/sachncs/promptsheon/internal/harness"
)

// ErrNotFound is returned for any lookup that misses. Tests
// can match with errors.Is(err, harnessrepo.ErrNotFound) to
// keep the assertion independent of the package-internal
// sentinel.
var ErrNotFound = errors.New("harnessrepo: not found")

// MemRepo is an in-memory harness.Repository. The maps are
// exported for assertions; callers that mutate them must
// take the mutex.
type MemRepo struct {
	mu          sync.Mutex
	Datasets    map[string]*harness.Dataset
	Cases       map[string][]harness.DatasetCase
	Preconds    map[string]*harness.Precondition
	EvalRuns    map[string]*harness.EvalRun
	EvalResults []harness.EvalResult
}

// New returns an empty MemRepo ready for use.
func New() *MemRepo {
	return &MemRepo{
		Datasets: map[string]*harness.Dataset{},
		Cases:    map[string][]harness.DatasetCase{},
		Preconds: map[string]*harness.Precondition{},
		EvalRuns: map[string]*harness.EvalRun{},
	}
}

// ----- Datasets -----

func (r *MemRepo) CreateDataset(_ context.Context, d *harness.Dataset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Datasets[d.ID]; ok {
		return errors.New("duplicate dataset id")
	}
	cp := *d
	r.Datasets[d.ID] = &cp
	return nil
}

func (r *MemRepo) GetDataset(_ context.Context, id string) (*harness.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.Datasets[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (r *MemRepo) ListDatasetsForCapability(_ context.Context, capabilityID string) ([]*harness.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*harness.Dataset
	for _, d := range r.Datasets {
		if d.CapabilityID == capabilityID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *MemRepo) DeleteDataset(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Datasets, id)
	delete(r.Cases, id)
	return nil
}

// ----- Dataset cases -----

func (r *MemRepo) UpsertDatasetCases(_ context.Context, datasetID string, cases []harness.DatasetCase) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]harness.DatasetCase, len(cases))
	copy(cp, cases)
	r.Cases[datasetID] = cp
	return nil
}

func (r *MemRepo) ListDatasetCases(_ context.Context, datasetID string) ([]harness.DatasetCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	src := r.Cases[datasetID]
	out := make([]harness.DatasetCase, len(src))
	copy(out, src)
	return out, nil
}

// ----- Preconditions -----

func (r *MemRepo) CreatePrecondition(_ context.Context, p *harness.Precondition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *p
	r.Preconds[p.ID] = &cp
	return nil
}

func (r *MemRepo) ListPreconditionsForCapability(_ context.Context, capabilityID string) ([]*harness.Precondition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*harness.Precondition
	for _, p := range r.Preconds {
		if p.CapabilityID == capabilityID {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *MemRepo) GetPrecondition(_ context.Context, id string) (*harness.Precondition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.Preconds[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *MemRepo) UpdatePrecondition(_ context.Context, p *harness.Precondition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Preconds[p.ID]; !ok {
		return ErrNotFound
	}
	cp := *p
	r.Preconds[p.ID] = &cp
	return nil
}

func (r *MemRepo) DeletePrecondition(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Preconds, id)
	return nil
}

// ----- EvalRuns -----

func (r *MemRepo) CreateEvalRun(_ context.Context, run *harness.EvalRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.EvalRuns[run.ID]; ok {
		return errors.New("duplicate eval run id")
	}
	cp := *run
	r.EvalRuns[run.ID] = &cp
	return nil
}

func (r *MemRepo) UpdateEvalRun(_ context.Context, run *harness.EvalRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.EvalRuns[run.ID]; !ok {
		return ErrNotFound
	}
	cp := *run
	r.EvalRuns[run.ID] = &cp
	return nil
}

func (r *MemRepo) GetEvalRun(_ context.Context, id string) (*harness.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	got, ok := r.EvalRuns[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *got
	return &cp, nil
}

func (r *MemRepo) ListEvalRunsForRelease(_ context.Context, releaseID string) ([]*harness.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*harness.EvalRun
	for _, e := range r.EvalRuns {
		if e.ReleaseID == releaseID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ----- EvalResults -----

func (r *MemRepo) CreateEvalResults(_ context.Context, results []harness.EvalResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.EvalResults = append(r.EvalResults, results...)
	return nil
}

func (r *MemRepo) CreateEvalResult(_ context.Context, res *harness.EvalResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.EvalResults = append(r.EvalResults, *res)
	return nil
}

func (r *MemRepo) ListEvalResultsForRun(_ context.Context, runID string) ([]harness.EvalResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []harness.EvalResult
	for _, x := range r.EvalResults {
		if x.RunID == runID {
			out = append(out, x)
		}
	}
	return out, nil
}
