package executor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
)

func TestRunHappyPath(t *testing.T) {
	t.Parallel()
	e := New(nil, func(_ context.Context, req InvokeRequest) (InvokeResult, error) {
		if req.WorkspaceID != "ws" {
			t.Errorf("ws mismatch")
		}
		return InvokeResult{Output: json.RawMessage(`{"ok":true}`), Status: "ok", PromptTokens: 10, OutputTokens: 5, CostUSDMicro: 100, LatencyMS: 200}, nil
	})
	rec, err := e.Run(context.Background(), "ws", "rel-1", "prod", json.RawMessage(`{"q":"hi"}`))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rec.Status != "ok" {
		t.Fatalf("expected ok, got %s", rec.Status)
	}
	if rec.CostUSD <= 0 {
		t.Fatalf("expected nonzero cost, got %f", rec.CostUSD)
	}
	if rec.InputHash == "" {
		t.Fatalf("expected input hash")
	}
}

func TestRunCapturesCallerError(t *testing.T) {
	t.Parallel()
	e := New(nil, func(_ context.Context, _ InvokeRequest) (InvokeResult, error) {
		return InvokeResult{}, errors.New("rate limited")
	})
	rec, err := e.Run(context.Background(), "ws", "rel-1", "prod", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Run should swallow caller errors, got %v", err)
	}
	if rec.Status != "error" {
		t.Fatalf("expected error status, got %s", rec.Status)
	}
	if rec.Error == "" {
		t.Fatalf("expected error captured")
	}
}

func TestHandleScheduleEventPublishesExecutionFinished(t *testing.T) {
	t.Parallel()
	p := &fakeBus{}
	e := New(p, func(_ context.Context, _ InvokeRequest) (InvokeResult, error) {
		return InvokeResult{Status: "ok", Output: json.RawMessage(`{}`)}, nil
	})
	ev := capability.Event{
		Type:        capability.EventType("schedule.fired"),
		AggregateID: "sched-1",
		Data: map[string]any{
			"workspace_id": "ws",
			"release_id":   "rel-1",
		},
	}
	if err := e.HandleScheduleEvent(context.Background(), ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(p.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(p.events))
	}
	if p.events[0].Type != capability.EventExecutionFinished {
		t.Fatalf("expected ExecutionFinished, got %s", p.events[0].Type)
	}
}

func TestHandleScheduleEventRejectsEmptyEvent(t *testing.T) {
	t.Parallel()
	e := New(nil, nil)
	ev := capability.Event{Type: capability.EventType("schedule.fired")}
	if err := e.HandleScheduleEvent(context.Background(), ev); err == nil {
		t.Fatalf("expected error for empty event data")
	}
}

type fakeBus struct {
	events []capability.Event
}

func (f *fakeBus) Subscribe(_ eventbus.Handler, _ ...capability.EventType) (eventbus.Subscription, error) {
	return nil, nil
}
func (f *fakeBus) Publish(ev capability.Event) error {
	f.events = append(f.events, ev)
	return nil
}
