// Package trace provides distributed tracing for LLM and workflow operations.
package trace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// SQLite stores traces in a SQLite database. Finish is asynchronous:
// spans are queued onto a buffered channel and drained by a background
// worker that batches inserts every flushInterval or whenever the
// batch fills up. The request goroutine never waits on the SQLite
// write. Close drains the queue.
type SQLite struct {
	db *sql.DB

	queue    chan *Span
	stop     chan struct{}
	done     chan struct{}
	wg       sync.WaitGroup
	dropped  atomic.Int64
	inFlight atomic.Int64

	pendingMu sync.Mutex
	pending   int64
}

// traceQueueSize bounds the in-memory span queue. When the queue is
// full, Finish drops the span and increments SQLite.Dropped.
const traceQueueSize = 4096

// traceBatchSize is the number of spans drained from the queue per
// batch insert.
const traceBatchSize = 64

// traceFlushInterval is the maximum time a span waits before the
// worker flushes a partial batch.
const traceFlushInterval = 250 * time.Millisecond

// NewSQLite creates a SQLite-backed tracer. The traces table is created
// automatically if it does not exist. The background worker is started
// here; the caller must invoke Close to drain the queue and stop the
// worker.
func NewSQLite(db *sql.DB) (*SQLite, error) {
	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS traces (
			id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL DEFAULT '',
			parent_id TEXT,
			operation TEXT NOT NULL,
			service TEXT DEFAULT 'promptsheon',
			status TEXT DEFAULT '',
			attributes TEXT DEFAULT '{}',
			error TEXT DEFAULT '',
			started_at DATETIME NOT NULL,
			ended_at DATETIME,
			duration_ms INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("create traces table: %w", err)
	}
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_traces_trace_id ON traces(trace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_parent_id ON traces(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_started_at ON traces(started_at)`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			return nil, fmt.Errorf("create traces index: %w", err)
		}
	}

	s := &SQLite{
		db:    db,
		queue: make(chan *Span, traceQueueSize),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.worker()
	return s, nil
}

// Dropped returns the number of spans dropped because the in-memory
// queue was full. Exposed for tests and metrics.
func (s *SQLite) Dropped() int64 { return s.dropped.Load() }

// pendingWrites tracks how many spans are currently sitting in
// the worker's local batch buffer waiting to be flushed. This is
// separate from inFlight (which counts spans actually being
// inserted in a transaction).
func (s *SQLite) pendingWrites() int64 {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return s.pending
}

// flushPending notes that n spans were appended to the worker's
// local batch.
func (s *SQLite) flushPending(n int) {
	s.pendingMu.Lock()
	s.pending += int64(n)
	s.pendingMu.Unlock()
}

// donePending notes that n spans were written and removed from
// the local batch.
func (s *SQLite) donePending(n int) {
	s.pendingMu.Lock()
	s.pending -= int64(n)
	s.pendingMu.Unlock()
}

// Flush waits for every currently-queued span to be persisted.
// Intended for tests and graceful shutdown paths that need
// synchronous read-after-write semantics. Spans submitted after
// Flush is called are not included in the wait.
func (s *SQLite) Flush(ctx context.Context) error {
	for {
		ql := len(s.queue)
		ifl := s.inFlight.Load()
		pl := s.pendingWrites()
		if ql == 0 && ifl == 0 && pl == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// Close drains the queue and stops the background worker. Safe to
// call multiple times.
func (s *SQLite) Close() error {
	select {
	case <-s.stop:
		return nil
	default:
		close(s.stop)
	}
	s.wg.Wait()
	close(s.done)
	return nil
}

// worker drains the queue and batch-inserts spans. It runs until
// stop is closed; after stop, it flushes whatever remains in the
// queue and exits.
//
// Spans that arrive while the worker is busy with a flush are
// counted in pendingWrites until the next flush picks them up.
// Flush waits on both queue len and pendingWrites so that spans
// sitting in the worker's local batch are still observed.
func (s *SQLite) worker() {
	defer s.wg.Done()
	ticker := time.NewTicker(traceFlushInterval)
	defer ticker.Stop()
	batch := make([]*Span, 0, traceBatchSize)
	for {
		select {
		case <-s.stop:
			for {
				select {
				case span := <-s.queue:
					batch = append(batch, span)
					s.flushPending(1)
					if len(batch) >= traceBatchSize {
						s.flush(batch)
						s.donePending(len(batch))
						batch = batch[:0]
					}
				default:
					if len(batch) > 0 {
						n := len(batch)
						s.flush(batch)
						s.donePending(n)
					}
					return
				}
			}
		case span := <-s.queue:
			batch = append(batch, span)
			s.flushPending(1)
			if len(batch) >= traceBatchSize {
				s.flush(batch)
				s.donePending(len(batch))
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				n := len(batch)
				s.flush(batch)
				s.donePending(n)
				batch = batch[:0]
			}
		}
	}
}

// flush writes a batch of spans. Each span is its own
// transaction so a slow span does not block the rest and a
// connection-pool visibility issue cannot mask a successful
// write from a subsequent read.
func (s *SQLite) flush(batch []*Span) {
	if len(batch) == 0 {
		return
	}
	s.inFlight.Add(int64(len(batch)))
	defer s.inFlight.Add(-int64(len(batch)))
	for _, span := range batch {
		attrs, err := json.Marshal(span.Attributes)
		if err != nil {
			attrs = []byte("{}")
		}
		if _, err := s.db.Exec(
			`INSERT INTO traces (id, trace_id, parent_id, operation, service, status, attributes, error, started_at, ended_at, duration_ms)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			span.ID, span.TraceID, span.ParentID, span.Operation, span.Service,
			string(span.Status), string(attrs), span.Error,
			span.StartedAt, span.EndedAt, span.DurationMs,
		); err != nil {
			slog.Warn("trace: span insert failed", "err", err, "id", span.ID)
		}
	}
}

// Start creates a new root span for the given operation.
func (s *SQLite) Start(_ context.Context, operation string) *Span {
	return &Span{
		ID:        GenerateID(),
		TraceID:   GenerateTraceID(),
		Operation: operation,
		Service:   "promptsheon",
		Status:    StatusOK,
		StartedAt: time.Now(),
	}
}

// StartChild creates a new child span under the given parent.
func (s *SQLite) StartChild(_ context.Context, parent *Span, operation string) *Span {
	return &Span{
		ID:        GenerateID(),
		TraceID:   parent.TraceID,
		ParentID:  parent.ID,
		Operation: operation,
		Service:   parent.Service,
		Status:    StatusOK,
		StartedAt: time.Now(),
	}
}

// Finish enqueues the span for asynchronous persistence. The
// request goroutine never blocks on the SQLite write; under a
// burst the queue absorbs and the worker batches. When the queue
// is full the span is dropped and SQLite.Dropped is incremented.
func (s *SQLite) Finish(span *Span) error {
	if span == nil {
		return errors.New("trace: nil span")
	}
	select {
	case s.queue <- span:
		return nil
	default:
		s.dropped.Add(1)
		slog.Warn("trace: queue full, span dropped", "id", span.ID, "dropped_total", s.dropped.Load())
		return nil
	}
}

// GetSpan retrieves a span by ID.
func (s *SQLite) GetSpan(ctx context.Context, id string) (*Span, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, trace_id, parent_id, operation, service, status, attributes, error, started_at, ended_at, duration_ms
		 FROM traces WHERE id = ?`, id,
	)
	return scanSpan(row)
}

// ListSpans returns spans matching the filter.
func (s *SQLite) ListSpans(ctx context.Context, filter *SpanFilter) ([]*Span, error) {
	query := "SELECT id, trace_id, parent_id, operation, service, status, attributes, error, started_at, ended_at, duration_ms FROM traces WHERE 1=1"
	args := []any{}

	if filter.Operation != "" {
		query += " AND operation = ?"
		args = append(args, filter.Operation)
	}
	if filter.Service != "" {
		query += " AND service = ?"
		args = append(args, filter.Service)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	if filter.TraceID != "" {
		query += " AND trace_id = ?"
		args = append(args, filter.TraceID)
	}
	if filter.Since != nil {
		query += " AND started_at >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND started_at <= ?"
		args = append(args, *filter.Until)
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list spans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var spans []*Span
	for rows.Next() {
		span, err := scanSpanRow(rows)
		if err != nil {
			return nil, err
		}
		spans = append(spans, span)
	}
	return spans, rows.Err()
}

// GetTraceTree retrieves all spans for a trace_id and reconstructs the parent-child tree.
func (s *SQLite) GetTraceTree(ctx context.Context, traceID string) ([]*Span, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, trace_id, parent_id, operation, service, status, attributes, error, started_at, ended_at, duration_ms
		 FROM traces WHERE trace_id = ? ORDER BY started_at ASC`, traceID,
	)
	if err != nil {
		return nil, fmt.Errorf("get trace tree: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var spans []*Span
	for rows.Next() {
		span, err := scanSpanRow(rows)
		if err != nil {
			return nil, err
		}
		spans = append(spans, span)
	}
	return spans, rows.Err()
}

// SpanFilter defines criteria for listing spans.
type SpanFilter struct {
	Operation string
	Service   string
	Status    Status
	TraceID   string
	Since     *time.Time
	Until     *time.Time
	Limit     int
}

func scanSpan(row scannable) (*Span, error) {
	var s Span
	var attrs string
	var endedAt *time.Time
	var status string
	err := row.Scan(
		&s.ID, &s.TraceID, &s.ParentID, &s.Operation, &s.Service,
		&status, &attrs, &s.Error,
		&s.StartedAt, &endedAt, &s.DurationMs,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("span not found")
		}
		return nil, fmt.Errorf("scan span: %w", err)
	}
	s.EndedAt = endedAt
	s.Status = Status(status)
	if err := json.Unmarshal([]byte(attrs), &s.Attributes); err != nil {
		slog.Error("failed to unmarshal span attributes", "err", err, "id", s.ID)
	}
	return &s, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanSpanRow(rows *sql.Rows) (*Span, error) {
	return scanSpan(rows)
}

// --- In-memory tracer (for testing) ---

// InMemory stores spans in memory. Useful for tests.
type InMemory struct {
	mu    sync.Mutex
	Spans []*Span
}

// NewInMemory creates an in-memory tracer.
func NewInMemory() *InMemory {
	return &InMemory{}
}

// Start creates a new root span for the given operation.
func (m *InMemory) Start(_ context.Context, operation string) *Span {
	span := &Span{
		ID:        GenerateID(),
		TraceID:   GenerateTraceID(),
		Operation: operation,
		Service:   "promptsheon",
		Status:    StatusOK,
		StartedAt: time.Now(),
	}
	return span
}

// StartChild creates a new child span under the given parent.
func (m *InMemory) StartChild(_ context.Context, parent *Span, operation string) *Span {
	span := &Span{
		ID:        GenerateID(),
		TraceID:   parent.TraceID,
		ParentID:  parent.ID,
		Operation: operation,
		Service:   parent.Service,
		Status:    StatusOK,
		StartedAt: time.Now(),
	}
	return span
}

// Finish records a span's completion.
func (m *InMemory) Finish(span *Span) error {
	span.Finish()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Spans = append(m.Spans, span)
	return nil
}
