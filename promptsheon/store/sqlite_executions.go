package store

import (
	"fmt"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"context"
	"database/sql"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// SQLite persistence for executions.

func (s *SQLite) CreateExecution(ctx context.Context, e *capability.Execution) error {
	inputs, err := marshalOrErr(e.Inputs)
	if err != nil {
		return fmt.Errorf("marshal execution inputs: %w", err)
	}
	outputs, err := marshalOrErr(e.Outputs)
	if err != nil {
		return fmt.Errorf("marshal execution outputs: %w", err)
	}

	// PERF-DB-2: RETURNING id. The execution row's id is set by
	// the caller (e.ID), but RETURNING lets us verify the insert
	// succeeded in a single round-trip — the previous ExecContext
	// discarded the result and offered no way to confirm the row
	// landed. We also use the rowid implicitly via the implicit
	// rowid column for downstream callers that need it.
	var gotID string
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO executions
		 (id, capability_version_id, timestamp, inputs, outputs, model, provider,
		  latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
		  error, trace_id, environment)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING id`,
		e.ID, e.CapabilityVersionID, e.Timestamp, string(inputs), string(outputs),
		e.Model, e.Provider, e.LatencyMs, e.CostUSD, e.PromptTokens, e.CompletionTokens,
		e.TotalTokens, e.Error, e.TraceID, e.Environment,
	).Scan(&gotID)
	if err != nil {
		return fmt.Errorf("insert execution: %w", err)
	}
	if gotID != e.ID {
		return fmt.Errorf("insert execution: id mismatch (got %q, want %q)", gotID, e.ID)
	}
	return nil
}

func (s *SQLite) GetExecution(ctx context.Context, id string) (*capability.Execution, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
		 latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
		 error, trace_id, environment
		 FROM executions WHERE id = ?`, id,
	)
	return scanExecution(row)
}

func (s *SQLite) ListExecutions(ctx context.Context, filter capability.ExecutionFilter) ([]*capability.Execution, error) {
	query := `SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
	 latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
	 error, trace_id, environment FROM executions WHERE 1=1`
	args := []any{}

	if filter.CapabilityVersionID != "" {
		query += " AND capability_version_id = ?"
		args = append(args, filter.CapabilityVersionID)
	}

	query += " ORDER BY timestamp DESC"

	limit := filter.Limit
	if limit < 0 {
		limit = 0
	}
	if filter.Offset > 0 && limit == 0 {
		query += " LIMIT -1 OFFSET ?"
		args = append(args, filter.Offset)
	} else {
		if limit > 0 {
			query += " LIMIT ?"
			args = append(args, limit)
		}
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Execution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func scanExecution(scanner interface {
	Scan(dest ...any) error
}) (*capability.Execution, error) {
	var e capability.Execution
	var inputsJSON, outputsJSON string

	err := scanner.Scan(&e.ID, &e.CapabilityVersionID, &e.Timestamp,
		&inputsJSON, &outputsJSON, &e.Model, &e.Provider,
		&e.LatencyMs, &e.CostUSD, &e.PromptTokens, &e.CompletionTokens,
		&e.TotalTokens, &e.Error, &e.TraceID, &e.Environment,
	)
	if err == sql.ErrNoRows {
		return nil, errs.ErrStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan execution: %w", err)
	}

	if err := mustUnmarshal([]byte(inputsJSON), &e.Inputs); err != nil {
		return nil, fmt.Errorf("execution %s inputs: %w", e.ID, err)
	}
	if err := mustUnmarshal([]byte(outputsJSON), &e.Outputs); err != nil {
		return nil, fmt.Errorf("execution %s outputs: %w", e.ID, err)
	}

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	return &e, nil
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------
