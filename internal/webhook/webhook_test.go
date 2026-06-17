package webhook

import (
	"sync"
	"testing"
	"time"
)

func TestDispatcherEmit(t *testing.T) {
	d := NewDispatcher(nil).WithMaxRetries(0)

	ep := &Endpoint{
		ID:     "ep-1",
		URL:    "http://localhost:99999/webhook", // will fail
		Events: []EventType{EventEvalCompleted},
		Active: true,
	}
	d.Register(ep)

	if len(d.ListEndpoints()) != 1 {
		t.Fatal("expected 1 endpoint")
	}

	evt := Event{
		ID:        "evt-1",
		Type:      EventEvalCompleted,
		Resource:  "prompt:abc",
		Data:      map[string]any{"pass_rate": 0.95},
		Timestamp: time.Now(),
	}
	d.Emit(evt)

	// Give async delivery time to run (including retries)
	time.Sleep(1 * time.Second)

	deliveries := d.ListDeliveries()
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0].Success {
		t.Fatal("expected failed delivery (bad URL)")
	}
}

func TestDispatcherInactiveEndpoint(t *testing.T) {
	d := NewDispatcher(nil)

	ep := &Endpoint{
		ID:     "ep-inactive",
		URL:    "http://localhost:99999",
		Events: []EventType{EventEvalCompleted},
		Active: false,
	}
	d.Register(ep)

	d.Emit(Event{ID: "evt-2", Type: EventEvalCompleted})
	time.Sleep(100 * time.Millisecond)

	if len(d.ListDeliveries()) != 0 {
		t.Fatal("expected no deliveries for inactive endpoint")
	}
}

func TestDispatcherEventFiltering(t *testing.T) {
	d := NewDispatcher(nil)

	ep := &Endpoint{
		ID:     "ep-filter",
		URL:    "http://localhost:99999",
		Events: []EventType{EventReviewApproved}, // only review events
		Active: true,
	}
	d.Register(ep)

	d.Emit(Event{ID: "evt-eval", Type: EventEvalCompleted})
	time.Sleep(100 * time.Millisecond)

	if len(d.ListDeliveries()) != 0 {
		t.Fatal("expected no deliveries for non-matching event type")
	}
}

func TestDispatcherRemove(t *testing.T) {
	d := NewDispatcher(nil)
	d.Register(&Endpoint{ID: "ep-rm", URL: "http://x", Events: []EventType{EventEvalCompleted}, Active: true})
	if len(d.ListEndpoints()) != 1 {
		t.Fatal("expected 1")
	}
	d.Remove("ep-rm")
	if len(d.ListEndpoints()) != 0 {
		t.Fatal("expected 0 after remove")
	}
}

func TestDispatcherConcurrentEmit(t *testing.T) {
	d := NewDispatcher(nil).WithMaxRetries(0)
	ep := &Endpoint{
		ID:     "ep-concurrent",
		URL:    "http://localhost:99999",
		Events: []EventType{EventEvalCompleted},
		Active: true,
	}
	d.Register(ep)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Emit(Event{ID: "evt-concurrent", Type: EventEvalCompleted})
		}()
	}
	wg.Wait()
	time.Sleep(2 * time.Second)

	d.mu.RLock()
	count := len(d.deliveries)
	d.mu.RUnlock()
	if count != 10 {
		t.Fatalf("expected 10 deliveries, got %d", count)
	}
}
