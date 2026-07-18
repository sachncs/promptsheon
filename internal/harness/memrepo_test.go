package harness_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/harness"
)

// memRepo is a tiny in-memory harness.Repository for tests. It is
// not concurrency-safe across goroutines — single test goroutine use
// only.
type memRepo struct {
	mu          sync.Mutex
	datasets    map[string]*harness.Dataset
	cases       map[string][]harness.DatasetCase
	preconds    map[string]*harness.Precondition
	evalRuns    map[string]*harness.EvalRun
	evalResults []harness.EvalResult
}

func newRepo(t testingTB) *memRepo {
	r := &memRepo{
		datasets: map[string]*harness.Dataset{},
		cases:    map[string][]harness.DatasetCase{},
		preconds: map[string]*harness.Precondition{},
		evalRuns: map[string]*harness.EvalRun{},
	}
	t.Cleanup(func() {})
	return r
}

type testingTB interface {
	Cleanup(func())
	Helper()
	Errorf(format string, args ...any)
	FailNow()
}

func (r *memRepo) CreateDataset(_ context.Context, d *harness.Dataset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.datasets[d.ID]; ok {
		return errors.New("duplicate dataset id")
	}
	cp := *d
	r.datasets[d.ID] = &cp
	return nil
}

func (r *memRepo) GetDataset(_ context.Context, id string) (*harness.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.datasets[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *d
	return &cp, nil
}

func (r *memRepo) ListDatasetsForCapability(_ context.Context, capabilityID string) ([]*harness.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*harness.Dataset
	for _, d := range r.datasets {
		if d.CapabilityID == capabilityID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) DeleteDataset(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.datasets, id)
	delete(r.cases, id)
	return nil
}

func (r *memRepo) UpsertDatasetCases(_ context.Context, datasetID string, cases []harness.DatasetCase) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]harness.DatasetCase, len(cases))
	copy(cp, cases)
	r.cases[datasetID] = cp
	return nil
}

func (r *memRepo) ListDatasetCases(_ context.Context, datasetID string) ([]harness.DatasetCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	src := r.cases[datasetID]
	out := make([]harness.DatasetCase, len(src))
	copy(out, src)
	return out, nil
}

func (r *memRepo) CreatePrecondition(_ context.Context, p *harness.Precondition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *p
	r.preconds[p.ID] = &cp
	return nil
}

func (r *memRepo) ListPreconditionsForCapability(_ context.Context, capabilityID string) ([]*harness.Precondition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*harness.Precondition
	for _, p := range r.preconds {
		if p.CapabilityID == capabilityID {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) DeletePrecondition(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.preconds, id)
	return nil
}

func (r *memRepo) CreateEvalRun(_ context.Context, run *harness.EvalRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.evalRuns[run.ID]; ok {
		return errors.New("duplicate eval run id")
	}
	cp := *run
	r.evalRuns[run.ID] = &cp
	return nil
}

func (r *memRepo) UpdateEvalRun(_ context.Context, run *harness.EvalRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.evalRuns[run.ID]; !ok {
		return errors.New("eval run not found")
	}
	cp := *run
	r.evalRuns[run.ID] = &cp
	return nil
}

func (r *memRepo) GetEvalRun(_ context.Context, id string) (*harness.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	got, ok := r.evalRuns[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *got
	return &cp, nil
}

func (r *memRepo) ListEvalRunsForRelease(_ context.Context, releaseID string) ([]*harness.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*harness.EvalRun
	for _, e := range r.evalRuns {
		if e.ReleaseID == releaseID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memRepo) CreateEvalResults(_ context.Context, results []harness.EvalResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evalResults = append(r.evalResults, results...)
	return nil
}

func (r *memRepo) ListEvalResultsForRun(_ context.Context, runID string) ([]harness.EvalResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []harness.EvalResult
	for _, x := range r.evalResults {
		if x.RunID == runID {
			out = append(out, x)
		}
	}
	return out, nil
}

var _ = time.Time{}
