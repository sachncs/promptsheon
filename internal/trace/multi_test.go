package trace

import (
	"context"
	"sync"
	"testing"
	"time"
)

type countingTracer struct {
	mu       sync.Mutex
	started  int
	finished int
}

func (c *countingTracer) Start(_ context.Context, _ string) *Span {
	c.mu.Lock()
	c.started++
	c.mu.Unlock()
	return &Span{ID: "test", TraceID: "trace", StartedAt: time.Now()}
}
func (c *countingTracer) StartChild(_ context.Context, _ *Span, _ string) *Span {
	return &Span{ID: "child", TraceID: "trace", StartedAt: time.Now()}
}
func (c *countingTracer) Finish(_ *Span) error {
	c.mu.Lock()
	c.finished++
	c.mu.Unlock()
	return nil
}

func TestMultiDispatchesAll(t *testing.T) {
	a := &countingTracer{}
	b := &countingTracer{}
	m := NewMulti(a, b)
	span := m.Start(context.Background(), "op")
	_ = m.Finish(span)
	if a.started != 1 || b.started != 1 {
		t.Errorf("primary/secondary start counts: %d / %d", a.started, b.started)
	}
	if a.finished != 1 || b.finished != 1 {
		t.Errorf("primary/secondary finish counts: %d / %d", a.finished, b.finished)
	}
}

func TestMultiWithOnlyPrimary(t *testing.T) {
	a := &countingTracer{}
	m := NewMulti(a)
	span := m.Start(context.Background(), "op")
	_ = m.Finish(span)
	if a.started != 1 || a.finished != 1 {
		t.Errorf("primary-only counts: %d / %d", a.started, a.finished)
	}
}

func TestMultiAllNilReturnsNoop(t *testing.T) {
	m := NewMulti(nil, nil, nil)
	span := m.Start(context.Background(), "op")
	if span == nil {
		t.Fatal("Start must return a Span even for no-op")
	}
	if err := m.Finish(span); err != nil {
		t.Errorf("Finish on no-op must return nil: %v", err)
	}
}

func TestMultiFallbackWhenPrimaryNil(t *testing.T) {
	a := &countingTracer{}
	m := NewMulti(nil, a)
	span := m.Start(context.Background(), "op")
	_ = m.Finish(span)
	if a.started != 1 || a.finished != 1 {
		t.Errorf("fallback counts: %d / %d", a.started, a.finished)
	}
}
