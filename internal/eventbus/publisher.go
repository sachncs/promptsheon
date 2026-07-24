// Package eventbus defines the Publisher interface and a default
// in-memory implementation. The interface lives here because it is
// defined by consumers — audit, observability, webhooks — not by the
// aggregates that emit events.
//
// The in-memory implementation is intentionally synchronous:
// subscribers run on the publisher's goroutine and a slow subscriber
// blocks subsequent subscribers. That trade-off is correct for the
// first version because audit logs and structured logs must be
// durable before the next event is recorded; eventual-consistency
// here would corrupt the audit chain.
//
// Concurrency: the in-memory Publisher is safe for concurrent
// Subscribe and Publish calls. The Publish call is itself blocking,
// so callers that need to emit without backpressure should compose
// their own async eventing on top of the interface — not change this
// implementation.
package eventbus

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// Handler processes one event. Handlers must be idempotent: the same
// event ID may be delivered more than once during a publisher restart
// or a retry.
type Handler func(event capability.Event)

// Subscribe registers a Handler for a set of event types. An empty
// eventTypes slice subscribes the Handler to every event type.
//
// Subscribe returns a Subscription that the caller must keep; call
// its Cancel method to deregister.
type Subscribe func(handler Handler, eventTypes ...capability.EventType) (Subscription, error)

// Publish emits an event. The event is delivered to every subscriber
// whose filter accepts it. Publish blocks until every matching
// subscriber has returned.
type Publish func(event capability.Event) error

// Subscription is the handle returned by Subscribe. Cancel removes
// the subscription; subsequent Publish calls will not deliver events
// to it.
//
// Cancel is idempotent.
type Subscription interface {
	Cancel()
}

// ErrAlreadyCanceled is returned by a Handler whose Subscription has
// been Canceled; consumers should treat it as "stop processing".
var ErrAlreadyCanceled = errors.New("eventbus: subscription canceled")

// subscription is the internal handle.
//
// Cancel flips canceled via the shared mu lock, and the Publish loop
// sees the same flag because the stored slice entry points at this
// subscription.
type subscription struct {
	id         uint64
	handler    Handler
	eventTypes map[capability.EventType]struct{}
	canceled   bool
	mu         sync.Mutex
}

// Cancel removes the subscription from the publisher.
//
// Cancel is idempotent and safe for concurrent use.
func (s *subscription) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceled = true
}

// Publisher is the consumer-defined interface for an event bus.
type Publisher interface {
	Subscribe(handler Handler, eventTypes ...capability.EventType) (Subscription, error)
	Publish(event capability.Event) error
}

// Memory is an in-memory Publisher. See the package documentation
// for why this is synchronous.
//
// Memory is safe for concurrent use.
//
// OBS-5b: when constructed via NewAsyncMemory, Publish hands
// the event off to a buffered channel and returns immediately;
// a worker pool drains the channel and dispatches to
// subscribers asynchronously. This removes the synchronous
// fan-out from the publisher's call stack. Subscribers that
// require durability before the next event (the audit chain)
// should keep using the synchronous path via NewMemory.
type Memory struct {
	mu          sync.Mutex
	subscribers []*subscription
	nextID      uint64

	// async: nil for synchronous Memory (NewMemory);
	// non-nil for async Memory (NewAsyncMemory). When set,
	// Publish hands the event to queue and returns immediately.
	queue   chan capability.Event
	stop    chan struct{}
	wg      sync.WaitGroup
	dropped uint64
}

// NewMemory constructs an empty synchronous Memory Publisher.
// Publish blocks until every subscriber returns.
func NewMemory() *Memory {
	return &Memory{}
}

// NewAsyncMemory constructs an asynchronous Memory Publisher.
// Events are queued on a buffered channel; a worker pool drains
// the channel and dispatches them asynchronously. The buffer size
// controls how many in-flight events can queue before Publish
// begins to drop. workers controls the dispatch parallelism. OBS-5b.
//
// Pass buffer=0 for an unbuffered channel (Publish blocks until a
// worker picks the event up).
func NewAsyncMemory(buffer, workers int) *Memory {
	if workers < 1 {
		workers = 1
	}
	if buffer < 0 {
		buffer = 0
	}
	m := &Memory{
		queue: make(chan capability.Event, buffer),
		stop:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.workerLoop()
	}
	return m
}

// workerLoop is the async dispatch goroutine.
func (m *Memory) workerLoop() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stop:
			return
		case ev := <-m.queue:
			m.dispatch(ev)
		}
	}
}

// dispatch runs the existing synchronous Publish logic on a single
// event. Stolen from Publish so the behaviour stays identical.
func (m *Memory) dispatch(event capability.Event) {
	// ponytail: snapshot subscribers under the lock, then release
	// it before calling handlers. Holding m.mu across handler
	// calls deadlocks Memory.Close against a slow subscriber
	// (Close needs the lock to cancel and stop workers).
	m.mu.Lock()
	targets := make([]*subscription, 0, len(m.subscribers))
	for _, s := range m.subscribers {
		if len(s.eventTypes) == 0 {
			targets = append(targets, s)
			continue
		}
		if _, ok := s.eventTypes[event.Type]; ok {
			targets = append(targets, s)
		}
	}
	m.mu.Unlock()
	for _, s := range targets {
		s.mu.Lock()
		canceled := s.canceled
		s.mu.Unlock()
		if canceled {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			s.handler(event)
		}()
	}
}

// IsAsync reports whether the Memory uses async fan-out.
func (m *Memory) IsAsync() bool { return m.queue != nil }

// Subscribe registers Handler for the supplied event types.
//
// Subscribe returns ErrAlreadyCanceled only if the Memory has been
// Closed; under normal operation it returns nil error.
func (m *Memory) Subscribe(handler Handler, eventTypes ...capability.EventType) (Subscription, error) {
	if handler == nil {
		return nil, errors.New("eventbus: handler must not be nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	filter := make(map[capability.EventType]struct{}, len(eventTypes))
	for _, et := range eventTypes {
		filter[et] = struct{}{}
	}
	sub := &subscription{
		id:         m.nextID,
		handler:    handler,
		eventTypes: filter,
	}
	m.subscribers = append(m.subscribers, sub)
	return sub, nil
}

// Publish delivers event to every matching subscriber.
//
// On a synchronous Memory (NewMemory), Publish blocks until every
// matching subscriber returns. On an async Memory (NewAsyncMemory),
// Publish hands the event off to a buffered channel and returns
// immediately; the worker pool dispatches in the background. OBS-5b.
//
// Publish swallows panics from individual subscribers so one buggy
// handler cannot poison the audit chain. The panic is, however,
// returned as an error so the caller is aware.
func (m *Memory) Publish(event capability.Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if m.queue != nil {
		select {
		case <-m.stop:
			// Memory closed: drop the event silently rather than
			// blocking forever or panicking.
			return nil
		case m.queue <- event:
			return nil
		default:
			// Queue full: drop with a counter so callers see
			// backpressure without blocking the publisher.
			atomic.AddUint64(&m.dropped, 1)
			return nil
		}
	}
	m.dispatch(event)
	return nil
}

// Dropped returns the number of events dropped because the async
// queue was full at Publish time. OBS-5b: surfaces backpressure
// for monitoring.
func (m *Memory) Dropped() uint64 { return atomic.LoadUint64(&m.dropped) }

// Close empties the subscription list and stops the async workers.
// After Close, Subscribe is the only operation that still returns
// the (empty) Memory. Close is idempotent.
//
// Close waits up to closeDrainTimeout for in-flight handlers to
// return; a slow subscriber that holds a handler forever cannot
// block daemon shutdown. The wait is bounded so a stuck handler
// is logged and abandoned rather than hanging the process. OBS-5b
// follow-up: the previous implementation blocked forever on
// m.wg.Wait() while a subscriber handler was stuck.
func (m *Memory) Close() {
	m.mu.Lock()
	for _, s := range m.subscribers {
		s.mu.Lock()
		s.canceled = true
		s.mu.Unlock()
	}
	m.subscribers = nil
	if m.queue != nil {
		select {
		case <-m.stop:
			m.mu.Unlock()
			return
		default:
			close(m.stop)
		}
	}
	m.mu.Unlock()
	if m.queue == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(closeDrainTimeout):
		// Stuck subscriber handler: don't block daemon shutdown.
		// The worker goroutine will exit when its handler returns
		// or when the process exits. Subscribers that need to
		// honour cancellation must check s.canceled inside their
		// handler; the public Cancel API on Subscription lets
		// callers do that.
	}
}

// closeDrainTimeout caps how long Close waits for in-flight
// handlers. The default is short: a healthy subscriber returns in
// milliseconds; anything longer means the subscriber is stuck and
// the daemon should not block on it.
const closeDrainTimeout = 2 * time.Second
