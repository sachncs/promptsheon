package slo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeRepo is a minimal Repository for the evaluator tests.
type fakeRepo struct {
	mu   sync.Mutex
	slos []*SLO
}

func (f *fakeRepo) CreateSLO(_ context.Context, s *SLO) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.slos = append(f.slos, s)
	return nil
}
func (f *fakeRepo) GetSLO(_ context.Context, id string) (*SLO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.slos {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, errors.New("slo: not found")
}
func (f *fakeRepo) ListSLOsForCapability(_ context.Context, capabilityID string) ([]*SLO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []*SLO{}
	for _, s := range f.slos {
		if s.CapabilityID == capabilityID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeRepo) ListSLOs(_ context.Context, _ string) ([]*SLO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*SLO, len(f.slos))
	copy(out, f.slos)
	return out, nil
}
func (f *fakeRepo) UpdateSLO(_ context.Context, s *SLO) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, existing := range f.slos {
		if existing.ID == s.ID {
			f.slos[i] = s
			return nil
		}
	}
	return errors.New("slo: not found")
}
func (f *fakeRepo) DeleteSLO(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.slos {
		if s.ID == id {
			f.slos = append(f.slos[:i], f.slos[i+1:]...)
			return nil
		}
	}
	return errors.New("slo: not found")
}

func validSLO() *SLO {
	return &SLO{
		ID:           "s1",
		WorkspaceID:  "w1",
		CapabilityID: "c1",
		Goal: Goal{
			Signal: SignalP95LatencyMS,
			Op:     OpLT,
			Target: 1000,
			Window: Window5Min,
			Breach: 1.0,
		},
		Severity: "ticket",
	}
}

func TestEvaluatorReportsBreach(t *testing.T) {
	repo := &fakeRepo{}
	if err := repo.CreateSLO(context.Background(), validSLO()); err != nil {
		t.Fatal(err)
	}

	source := func(_ context.Context, s *SLO) (float64, error) {
		if s.ID == "s1" {
			return 1500, nil // over the 1000 target
		}
		return 0, nil
	}

	var breached []BreachEvent
	var mu sync.Mutex
	e := NewEvaluator(repo, source, nil, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		e.Start(ctx, func(_ context.Context, b BreachEvent) error {
			mu.Lock()
			breached = append(breached, b)
			mu.Unlock()
			return nil
		})
		close(done)
	}()

	// Wait for at least one tick.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(breached) == 0 {
		t.Fatal("expected at least one breach event")
	}
	if breached[0].SLO.ID != "s1" {
		t.Errorf("expected SLO s1, got %s", breached[0].SLO.ID)
	}
	if breached[0].BurnRate != 1.5 {
		t.Errorf("expected burn rate 1.5, got %f", breached[0].BurnRate)
	}
}

func TestEvaluatorDoesNotBreachWhenSatisfied(t *testing.T) {
	repo := &fakeRepo{}
	if err := repo.CreateSLO(context.Background(), validSLO()); err != nil {
		t.Fatal(err)
	}

	source := func(_ context.Context, _ *SLO) (float64, error) {
		return 200, nil // under the 1000 target
	}

	e := NewEvaluator(repo, source, nil, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var count int
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		e.Start(ctx, func(_ context.Context, _ BreachEvent) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if count != 0 {
		t.Errorf("expected zero breaches when SLO satisfied, got %d", count)
	}
}

func TestEvaluatorSourceError(t *testing.T) {
	repo := &fakeRepo{}
	if err := repo.CreateSLO(context.Background(), validSLO()); err != nil {
		t.Fatal(err)
	}

	source := func(_ context.Context, _ *SLO) (float64, error) {
		return 0, errors.New("source unavailable")
	}

	e := NewEvaluator(repo, source, nil, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		e.Start(ctx, nil) // no breach callback
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	// No panic, no crash; the source error is logged and the loop
	// continues. The test passes if the goroutine exits cleanly.
}

func TestEvaluatorLastRun(t *testing.T) {
	e := NewEvaluator(nil, nil, nil, time.Hour)
	if !e.LastRun().IsZero() {
		t.Errorf("expected zero LastRun before tick, got %v", e.LastRun())
	}

	e.lastRun = time.Now() // simulate a tick
	if e.LastRun().IsZero() {
		t.Error("expected non-zero LastRun after manual set")
	}
}
