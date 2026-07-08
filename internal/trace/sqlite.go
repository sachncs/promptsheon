// Package trace provides distributed tracing for LLM and workflow operations.
package trace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SQLite stores traces in a SQLite database.
type SQLite struct {
	db *sql.DB
}

// NewSQLite creates a SQLite-backed tracer. The traces table is created
// automatically if it does not exist.
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
	_, err = db.ExecContext(context.Background(), `CREATE INDEX IF NOT EXISTS idx_traces_trace_id ON traces(trace_id)`)
	if err != nil {
		return nil, fmt.Errorf("create traces index: %w", err)
	}
	_, err = db.ExecContext(context.Background(), `CREATE INDEX IF NOT EXISTS idx_traces_parent_id ON traces(parent_id)`)
	if err != nil {
		return nil, fmt.Errorf("create traces parent index: %w", err)
	}
	// L-7 fix: ListSpans filters on started_at in the Since/Until
	// range queries, but no index covered that column. For a busy
	// trace store the query degraded to a full scan. Add the
	// index here so it is created at the same time as the other
	// indexes.
	_, err = db.ExecContext(context.Background(), `CREATE INDEX IF NOT EXISTS idx_traces_started_at ON traces(started_at)`)
	if err != nil {
		return nil, fmt.Errorf("create traces started_at index: %w", err)
	}
	// The trace_id column is created above; the previous code did a
	// no-op ALTER TABLE that always failed silently. Removed.
	return &SQLite{db: db}, nil
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

// Finish persists a span. The context is intentionally fresh: the
// span should be recorded even if the request that produced it has
// already returned or been cancelled. The previous implementation
// used context.Background() too, but accepting an explicit context
// here means the trace store honours shutdown cancellation when the
// caller (the metrics middleware) supplies one.
func (s *SQLite) Finish(span *Span) error {
	attrs, err := json.Marshal(span.Attributes)
	if err != nil {
		attrs = []byte("{}")
	}
	_, err = s.db.ExecContext(context.Background(),
		`INSERT INTO traces (id, trace_id, parent_id, operation, service, status, attributes, error, started_at, ended_at, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		span.ID, span.TraceID, span.ParentID, span.Operation, span.Service,
		string(span.Status), string(attrs), span.Error,
		span.StartedAt, span.EndedAt, span.DurationMs,
	)
	return err
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
