package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sachn-cs/promptsheon/internal/models"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// marshalOrErr marshals v to JSON or returns a wrapped error. The
// previous implementation (mustMarshal) silently returned "{}" on
// failure and logged to slog.Error from arbitrary goroutines, which
// dropped data in production. The fix makes the error visible at the
// call site so callers can fail loudly and the operator can act on
// the root cause.
func marshalOrErr(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return b, nil
}

// mustUnmarshal is intentionally a best-effort helper: it unmarshals
// into v and logs on failure but does not return an error, because the
// stored JSON is whatever was previously marshalled (i.e., we know it
// is valid JSON at write time). A read failure indicates data drift
// (manual SQL edit, version skew) and is worth logging.
func mustUnmarshal(data []byte, v any) {
	if len(data) == 0 {
		return
	}
	if err := json.Unmarshal(data, v); err != nil {
		slog.Error("failed to unmarshal JSON", "err", err)
	}
}

// SQLite implements Repository using a SQLite database.
type SQLite struct {
	db *sql.DB
	// auditMu serialises AppendAudit calls. SQLite's serializable
	// transactions are too coarse for high-throughput audit writes
	// (any concurrent writer gets SQLITE_BUSY). The in-process
	// mutex is much cheaper than DB-level locking and is sufficient
	// because the chain is a process-local concern.
	auditMu sync.Mutex
}

// NewSQLite opens or creates a SQLite database at the given path and runs
// pending migrations.
func NewSQLite(dbPath string) (*SQLite, error) {
	// For in-memory databases, use the shared cache so multiple
	// connections (we now allow 4) see the same schema and data.
	// Without this, each connection gets its own private database
	// and the test suite (which shares a single :memory: db between
	// the main store and the trace store) breaks.
	pragmas := "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	dsn := dbPath
	if dbPath == ":memory:" {
		// Use shared in-memory cache so multiple connections (we now
		// allow 4) see the same schema and data. WAL is incompatible
		// with in-memory DBs, so use MEMORY journal mode.
		dsn = "file::memory:?cache=shared&_pragma=journal_mode(MEMORY)&_pragma=busy_timeout(5000)"
	} else {
		dsn = dbPath + "?" + pragmas
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite serialises writers internally, but reads can proceed in
	// parallel with the WAL journal. A small pool of 4 connections
	// gives readers room to run concurrently without serialising
	// every query behind the single connection the previous config
	// used.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := migrate(db, migrationsFS); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

// Ping verifies the database connection is alive.
func (s *SQLite) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// DB returns the underlying *sql.DB for use by other packages (e.g. tracing).
func (s *SQLite) DB() *sql.DB {
	return s.db
}

// ---------------------------------------------------------------------------
// Prompts
// ---------------------------------------------------------------------------

func (s *SQLite) CreatePrompt(ctx context.Context, p *models.Prompt) error {
	variables, err := marshalOrErr(p.Variables)
	if err != nil {
		return fmt.Errorf("marshal prompt variables: %w", err)
	}
	tags, err := marshalOrErr(p.Tags)
	if err != nil {
		return fmt.Errorf("marshal prompt tags: %w", err)
	}
	metadata, err := marshalOrErr(p.Metadata)
	if err != nil {
		return fmt.Errorf("marshal prompt metadata: %w", err)
	}
	binding, err := marshalOrErr(p.Binding)
	if err != nil {
		return fmt.Errorf("marshal prompt binding: %w", err)
	}
	generation, err := marshalOrErr(p.Generation)
	if err != nil {
		return fmt.Errorf("marshal prompt generation: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO prompts (id, name, description, content, system_prompt, variables, tags, model_hint,
		 version, status, environment, cas_hash, created_by, created_at, updated_at, metadata, binding, generation)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.Content, p.SystemPrompt, string(variables), string(tags),
		p.ModelHint, p.Version, string(p.Status), p.Environment, p.CASHash,
		p.CreatedBy, p.CreatedAt, p.UpdatedAt, string(metadata), string(binding), string(generation),
	)
	if err != nil {
		return fmt.Errorf("insert prompt: %w", err)
	}
	return nil
}

func (s *SQLite) GetPrompt(ctx context.Context, id string) (*models.Prompt, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, content, system_prompt, variables, tags, model_hint,
		 version, status, environment, cas_hash, created_by, created_at, updated_at, metadata, binding, generation
		 FROM prompts WHERE id = ?`, id,
	)
	return scanPrompt(row)
}

func (s *SQLite) ListPrompts(ctx context.Context, filter models.PromptFilter) ([]*models.Prompt, error) {
	query := "SELECT id, name, description, content, system_prompt, variables, tags, model_hint, version, status, environment, cas_hash, created_by, created_at, updated_at, metadata, binding, generation FROM prompts WHERE 1=1"
	args := []any{}

	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, st := range filter.Status {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		query += " AND status IN (" + strings.Join(placeholders, ",") + ")"
	}

	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			// Escape % and _ in the user-supplied tag so the
			// wildcard semantics are honoured literally.
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(tag)
			query += ` AND tags LIKE ? ESCAPE '\'`
			args = append(args, "%"+escaped+"%")
		}
	}

	if filter.Search != "" {
		query += " AND (name LIKE ? OR description LIKE ? OR content LIKE ?)"
		s := "%" + filter.Search + "%"
		args = append(args, s, s, s)
	}

	if filter.Environment != "" {
		query += " AND environment = ?"
		args = append(args, filter.Environment)
	}

	query += " ORDER BY updated_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	defer rows.Close()

	var prompts []*models.Prompt
	for rows.Next() {
		p, err := scanPromptRow(rows)
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, p)
	}
	return prompts, rows.Err()
}

func (s *SQLite) UpdatePrompt(ctx context.Context, p *models.Prompt) error {
	variables, err := marshalOrErr(p.Variables)
	if err != nil {
		return fmt.Errorf("marshal prompt variables: %w", err)
	}
	tags, err := marshalOrErr(p.Tags)
	if err != nil {
		return fmt.Errorf("marshal prompt tags: %w", err)
	}
	metadata, err := marshalOrErr(p.Metadata)
	if err != nil {
		return fmt.Errorf("marshal prompt metadata: %w", err)
	}
	binding, err := marshalOrErr(p.Binding)
	if err != nil {
		return fmt.Errorf("marshal prompt binding: %w", err)
	}
	generation, err := marshalOrErr(p.Generation)
	if err != nil {
		return fmt.Errorf("marshal prompt generation: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE prompts SET name=?, description=?, content=?, system_prompt=?, variables=?, tags=?,
		 model_hint=?, version=?, status=?, environment=?, cas_hash=?, updated_at=?, metadata=?, binding=?, generation=?
		 WHERE id=?`,
		p.Name, p.Description, p.Content, p.SystemPrompt, string(variables), string(tags),
		p.ModelHint, p.Version, string(p.Status), p.Environment, p.CASHash,
		p.UpdatedAt, string(metadata), string(binding), string(generation), p.ID,
	)
	if err != nil {
		return fmt.Errorf("update prompt: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("prompt not found: %s", p.ID)
	}
	return nil
}

func (s *SQLite) DeletePrompt(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM prompts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete prompt: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("prompt not found: %s", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func (s *SQLite) CreateAgent(ctx context.Context, a *models.Agent) error {
	steps, err := marshalOrErr(a.Steps)
	if err != nil {
		return fmt.Errorf("marshal agent steps: %w", err)
	}
	tools, err := marshalOrErr(a.Tools)
	if err != nil {
		return fmt.Errorf("marshal agent tools: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO agents (id, name, description, steps, tools, status, cas_hash, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Description, string(steps), string(tools),
		string(a.Status), a.CASHash, a.CreatedBy, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert agent: %w", err)
	}
	return nil
}

func (s *SQLite) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, steps, tools, status, cas_hash, created_by, created_at, updated_at
		 FROM agents WHERE id = ?`, id,
	)
	return scanAgent(row)
}

func (s *SQLite) ListAgents(ctx context.Context) ([]*models.Agent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, steps, tools, status, cas_hash, created_by, created_at, updated_at
		 FROM agents ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []*models.Agent
	for rows.Next() {
		a, err := scanAgentRow(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *SQLite) UpdateAgent(ctx context.Context, a *models.Agent) error {
	steps, err := marshalOrErr(a.Steps)
	if err != nil {
		return fmt.Errorf("marshal agent steps: %w", err)
	}
	tools, err := marshalOrErr(a.Tools)
	if err != nil {
		return fmt.Errorf("marshal agent tools: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE agents SET name=?, description=?, steps=?, tools=?, status=?, cas_hash=?, updated_at=?
		 WHERE id=?`,
		a.Name, a.Description, string(steps), string(tools),
		string(a.Status), a.CASHash, a.UpdatedAt, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent not found: %s", a.ID)
	}
	return nil
}

func (s *SQLite) DeleteAgent(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test Datasets
// ---------------------------------------------------------------------------

func (s *SQLite) CreateDataset(ctx context.Context, d *models.TestDataset) error {
	cases, err := marshalOrErr(d.Cases)
	if err != nil {
		return fmt.Errorf("marshal dataset cases: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO test_datasets (id, name, cases, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		d.ID, d.Name, string(cases), d.CreatedBy, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert dataset: %w", err)
	}
	return nil
}

func (s *SQLite) GetDataset(ctx context.Context, id string) (*models.TestDataset, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, cases, created_by, created_at
		 FROM test_datasets WHERE id = ?`, id,
	)
	return scanDataset(row)
}

func (s *SQLite) ListDatasets(ctx context.Context) ([]*models.TestDataset, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, cases, created_by, created_at
		 FROM test_datasets ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	defer rows.Close()

	var datasets []*models.TestDataset
	for rows.Next() {
		d, err := scanDatasetRow(rows)
		if err != nil {
			return nil, err
		}
		datasets = append(datasets, d)
	}
	return datasets, rows.Err()
}

func (s *SQLite) DeleteDataset(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM test_datasets WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete dataset: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("dataset not found: %s", id)
	}
	return nil
}

func (s *SQLite) UpdateDataset(ctx context.Context, d *models.TestDataset) error {
	cases, err := marshalOrErr(d.Cases)
	if err != nil {
		return fmt.Errorf("marshal dataset cases: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE test_datasets SET name=?, cases=? WHERE id=?`,
		d.Name, string(cases), d.ID,
	)
	if err != nil {
		return fmt.Errorf("update dataset: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("dataset not found: %s", d.ID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Evaluations
// ---------------------------------------------------------------------------

func (s *SQLite) SaveEvalResults(ctx context.Context, results []*models.EvalResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO eval_results (test_case_id, prompt_hash, model, dataset_id, output,
		 score, latency_ms, token_usage, hallucination_score, passed, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		usage, err := marshalOrErr(r.TokenUsage)
		if err != nil {
			return fmt.Errorf("marshal token usage: %w", err)
		}
		passed := 0
		if r.Passed {
			passed = 1
		}
		if _, err := stmt.ExecContext(ctx,
			r.TestCaseID, r.PromptHash, r.Model, r.DatasetID,
			r.Output, r.Score, r.LatencyMs, string(usage),
			r.HallucinationScore, passed, r.Error, r.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert eval result: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLite) GetEvalResults(ctx context.Context, promptHash string) ([]*models.EvalResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT test_case_id, prompt_hash, model, dataset_id, output, score, latency_ms,
		 token_usage, hallucination_score, passed, error, created_at
		 FROM eval_results WHERE prompt_hash = ? ORDER BY created_at DESC`, promptHash,
	)
	if err != nil {
		return nil, fmt.Errorf("query eval results: %w", err)
	}
	defer rows.Close()
	return scanEvalResults(rows)
}

func (s *SQLite) GetEvalResultsByDataset(ctx context.Context, datasetID string) ([]*models.EvalResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT test_case_id, prompt_hash, model, dataset_id, output, score, latency_ms,
		 token_usage, hallucination_score, passed, error, created_at
		 FROM eval_results WHERE dataset_id = ? ORDER BY created_at DESC`, datasetID,
	)
	if err != nil {
		return nil, fmt.Errorf("query eval results by dataset: %w", err)
	}
	defer rows.Close()
	return scanEvalResults(rows)
}

// ---------------------------------------------------------------------------
// Eval Runs
// ---------------------------------------------------------------------------

func (s *SQLite) SaveEvalRun(ctx context.Context, run *models.EvalRun) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO eval_runs
		 (id, prompt_hash, dataset_id, model, status, total_cases, passed_cases,
		  pass_rate, avg_score, avg_latency_ms, avg_hallucination, total_tokens,
		  started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.PromptHash, run.DatasetID, run.Model, run.Status,
		run.TotalCases, run.PassedCases, run.PassRate, run.AvgScore,
		run.AvgLatencyMs, run.AvgHallucination, run.TotalTokens,
		run.StartedAt, run.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("save eval run: %w", err)
	}
	return nil
}

func (s *SQLite) GetEvalRun(ctx context.Context, id string) (*models.EvalRun, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, prompt_hash, dataset_id, model, status, total_cases, passed_cases,
		 pass_rate, avg_score, avg_latency_ms, avg_hallucination, total_tokens,
		 started_at, completed_at
		 FROM eval_runs WHERE id = ?`, id,
	)
	var r models.EvalRun
	err := row.Scan(
		&r.ID, &r.PromptHash, &r.DatasetID, &r.Model, &r.Status,
		&r.TotalCases, &r.PassedCases, &r.PassRate, &r.AvgScore,
		&r.AvgLatencyMs, &r.AvgHallucination, &r.TotalTokens,
		&r.StartedAt, &r.CompletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get eval run: %w", err)
	}
	return &r, nil
}

func (s *SQLite) ListEvalRuns(ctx context.Context, filter models.EvalRunFilter) ([]*models.EvalRun, error) {
	query := `SELECT id, prompt_hash, dataset_id, model, status, total_cases, passed_cases,
		pass_rate, avg_score, avg_latency_ms, avg_hallucination, total_tokens,
		started_at, completed_at FROM eval_runs WHERE 1=1`
	args := []any{}

	if filter.PromptHash != "" {
		query += " AND prompt_hash = ?"
		args = append(args, filter.PromptHash)
	}
	if filter.DatasetID != "" {
		query += " AND dataset_id = ?"
		args = append(args, filter.DatasetID)
	}
	if filter.Model != "" {
		query += " AND model = ?"
		args = append(args, filter.Model)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list eval runs: %w", err)
	}
	defer rows.Close()

	var runs []*models.EvalRun
	for rows.Next() {
		var r models.EvalRun
		if err := rows.Scan(
			&r.ID, &r.PromptHash, &r.DatasetID, &r.Model, &r.Status,
			&r.TotalCases, &r.PassedCases, &r.PassRate, &r.AvgScore,
			&r.AvgLatencyMs, &r.AvgHallucination, &r.TotalTokens,
			&r.StartedAt, &r.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan eval run: %w", err)
		}
		runs = append(runs, &r)
	}
	return runs, rows.Err()
}

// ---------------------------------------------------------------------------
// Workflows
// ---------------------------------------------------------------------------

func (s *SQLite) SaveWorkflow(ctx context.Context, w *models.Workflow) error {
	input, err := marshalOrErr(w.Input)
	if err != nil {
		return fmt.Errorf("marshal workflow input: %w", err)
	}
	output, err := marshalOrErr(w.Output)
	if err != nil {
		return fmt.Errorf("marshal workflow output: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO workflows
		 (id, agent_id, status, input, output, error, started_at, completed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.AgentID, string(w.Status), string(input), string(output),
		w.Error, w.StartedAt, w.CompletedAt, w.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}
	return nil
}

func (s *SQLite) GetWorkflow(ctx context.Context, id string) (*models.Workflow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_id, status, input, output, error, started_at, completed_at, created_at
		 FROM workflows WHERE id = ?`, id,
	)
	return scanWorkflow(row)
}

func (s *SQLite) ListWorkflows(ctx context.Context, filter models.WorkflowFilter) ([]*models.Workflow, error) {
	query := "SELECT id, agent_id, status, input, output, error, started_at, completed_at, created_at FROM workflows WHERE 1=1"
	args := []any{}

	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer rows.Close()

	var workflows []*models.Workflow
	for rows.Next() {
		w, err := scanWorkflowRow(rows)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

func (s *SQLite) SaveWorkflowStep(ctx context.Context, step *models.WorkflowStep) error {
	input, err := marshalOrErr(step.Input)
	if err != nil {
		return fmt.Errorf("marshal step input: %w", err)
	}
	output, err := marshalOrErr(step.Output)
	if err != nil {
		return fmt.Errorf("marshal step output: %w", err)
	}
	toolCalls, err := marshalOrErr(step.ToolCalls)
	if err != nil {
		return fmt.Errorf("marshal step tool_calls: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO workflow_steps
		 (id, workflow_id, step_id, status, input, output, error, tool_calls, latency_ms, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.ID, step.WorkflowID, step.StepID, step.Status,
		string(input), string(output), step.Error, string(toolCalls),
		step.LatencyMs, step.StartedAt, step.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("save workflow step: %w", err)
	}
	return nil
}

func (s *SQLite) GetWorkflowSteps(ctx context.Context, workflowID string) ([]*models.WorkflowStep, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workflow_id, step_id, status, input, output, error, tool_calls, latency_ms, started_at, finished_at
		 FROM workflow_steps WHERE workflow_id = ? ORDER BY started_at`, workflowID,
	)
	if err != nil {
		return nil, fmt.Errorf("get workflow steps: %w", err)
	}
	defer rows.Close()

	var steps []*models.WorkflowStep
	for rows.Next() {
		var step models.WorkflowStep
		var input, output, toolCalls string
		if err := rows.Scan(
			&step.ID, &step.WorkflowID, &step.StepID, &step.Status,
			&input, &output, &step.Error, &toolCalls,
			&step.LatencyMs, &step.StartedAt, &step.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow step: %w", err)
		}
		json.Unmarshal([]byte(input), &step.Input)
		json.Unmarshal([]byte(output), &step.Output)
		json.Unmarshal([]byte(toolCalls), &step.ToolCalls)
		steps = append(steps, &step)
	}
	return steps, rows.Err()
}

func scanWorkflow(row scannable) (*models.Workflow, error) {
	var w models.Workflow
	var input, output, status string
	var completedAt *time.Time
	err := row.Scan(
		&w.ID, &w.AgentID, &status, &input, &output,
		&w.Error, &w.StartedAt, &completedAt, &w.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan workflow: %w", err)
	}
	w.Status = models.WorkflowStatus(status)
	w.CompletedAt = completedAt
	if err := json.Unmarshal([]byte(input), &w.Input); err != nil {
		return nil, fmt.Errorf("unmarshal workflow input: %w", err)
	}
	if err := json.Unmarshal([]byte(output), &w.Output); err != nil {
		return nil, fmt.Errorf("unmarshal workflow output: %w", err)
	}
	return &w, nil
}

func scanWorkflowRow(rows *sql.Rows) (*models.Workflow, error) {
	return scanWorkflow(rows)
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

// AppendAudit adds an entry to the tamper-evident audit log.
//
// SECURITY: the hash chain is computed inside a single IMMEDIATE
// transaction so the read of the previous hash and the insert of the
// new row are atomic. Without the transaction, two concurrent calls
// would both read the same prevHash and produce a forked chain.
//
// The hash covers id, user_id, action, resource, details, timestamp
// and previous_hash. The previous implementation only hashed
// id/user_id/action/resource/previous_hash, which let an attacker with
// write access to the SQLite file mutate details and timestamp
// without breaking the chain.
func (s *SQLite) AppendAudit(ctx context.Context, entry *models.AuditEntry) error {
	// M-6 fix: serialise AppendAudit calls in-process. The chain
	// head pointer is process-local state, and the previous design
	// relied on SQLite's serializable transactions which can return
	// SQLITE_BUSY under concurrent writers. The in-process mutex is
	// microseconds vs milliseconds for DB-level locking.
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	details, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// C-2 fix: normalise the timestamp to UTC and store the canonical
	// RFC3339Nano string. The previous binary encoding of the
	// timezone offset (int32(off) shifted into a byte slice) was
	// non-canonical for negative offsets and made the audit chain
	// fail verification on a machine with a different timezone. The
	// canonical string is now stored alongside the timestamp and
	// used directly in the hash.
	entry.Timestamp = entry.Timestamp.UTC()
	timestampStr := entry.Timestamp.Format(time.RFC3339Nano)

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin audit tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var prevHash string
	// M-6 fix: read the last hash from the dedicated single-row state
	// table instead of scanning audit_entries on every write. The
	// state table is the new hot row (1 row, locked by the IMMEDIATE
	// transaction) but it is much smaller than the full audit
	// table, so contention is dramatically lower.
	err = tx.QueryRowContext(ctx,
		`SELECT last_hash FROM audit_chain_state WHERE id = 0`,
	).Scan(&prevHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("fetch previous audit hash: %w", err)
	}

	entry.PreviousHash = prevHash
	entry.EntryHash = computeAuditHash(entry, string(details), timestampStr)

	insertRes, err := tx.ExecContext(ctx,
		`INSERT INTO audit_entries (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.UserID, entry.Action, entry.Resource,
		string(details), entry.Timestamp, entry.PreviousHash, entry.EntryHash, timestampStr,
	)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	rowID, err := insertRes.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	// Update the state row to point at the new head of the chain.
	// Upsert so the first audit entry creates the row.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO audit_chain_state (id, last_hash, last_rowid)
		 VALUES (0, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET last_hash = excluded.last_hash, last_rowid = excluded.last_rowid`,
		entry.EntryHash, rowID,
	); err != nil {
		return fmt.Errorf("update audit chain state: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit audit: %w", err)
	}
	return nil
}

// computeAuditHash produces the SHA-256 hash of the audit entry. The
// timestamp is included as a canonical RFC3339Nano string in UTC (see
// AppendAudit). The previous binary encoding was non-canonical for
// negative timezone offsets, which is why we switched to a string.
func computeAuditHash(e *models.AuditEntry, detailsJSON, timestampStr string) string {
	h := sha256.New()
	h.Write([]byte(e.ID))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.UserID))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.Action))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.Resource))
	h.Write([]byte{0x1f})
	h.Write([]byte(detailsJSON))
	h.Write([]byte{0x1f})
	h.Write([]byte(timestampStr))
	h.Write([]byte{0x1f})
	h.Write([]byte(e.PreviousHash))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyAuditChain reads all audit entries and validates the hash chain.
func (s *SQLite) VerifyAuditChain(ctx context.Context) (bool, string, error) {
	// M-17 fix: page the verification in chunks of auditVerifyPageSize
	// so a large chain does not hold a single goroutine + connection
	// for the full duration. The caller (handleVerifyAuditChain)
	// invokes this synchronously, so page size is a direct latency
	// trade-off; 1000 is small enough to keep row reads <10ms and
	// large enough to amortise the per-page overhead.
	const pageSize = 1000
	var prevHash string
	var lastRowID int64
	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}
		nextPrev, ok, why, last, err := s.verifyAuditPage(ctx, prevHash, lastRowID, pageSize)
		if err != nil {
			return false, "", err
		}
		if !ok {
			return false, why, nil
		}
		if last == 0 {
			break
		}
		prevHash = nextPrev
		lastRowID = last
	}
	return true, "", nil
}

// verifyAuditPage verifies one page of the chain starting at afterRowID
// and returns the new prevHash (i.e., the last entry_hash seen), the
// last rowid processed (0 when the table is exhausted), and the
// verification result.
func (s *SQLite) verifyAuditPage(ctx context.Context, prevHash string, afterRowID int64, limit int) (string, bool, string, int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rowid, id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str
		 FROM audit_entries
		 WHERE rowid > ?
		 ORDER BY rowid ASC
		 LIMIT ?`,
		afterRowID, limit,
	)
	if err != nil {
		return prevHash, false, "", 0, fmt.Errorf("query audit chain page: %w", err)
	}
	defer rows.Close()

	var lastRowID int64
	for rows.Next() {
		var rowID int64
		var id, userID, action, resource, detailsJSON, storedPrev, storedHash, timestampStr string
		var ts time.Time
		if err := rows.Scan(&rowID, &id, &userID, &action, &resource, &detailsJSON, &ts, &storedPrev, &storedHash, &timestampStr); err != nil {
			return prevHash, false, "", 0, fmt.Errorf("scan audit chain: %w", err)
		}
		if storedPrev != prevHash {
			return prevHash, false, fmt.Sprintf("chain break at entry %s: expected prev_hash %q, got %q", id, prevHash, storedPrev), 0, nil
		}
		// C-2 fix: prefer the canonical string. Fall back to
		// formatting the time.Time the driver hands back so that
		// rows inserted before the migration still verify (their
		// timestamp_str is backfilled by the migration's UPDATE).
		if timestampStr == "" {
			timestampStr = ts.UTC().Format(time.RFC3339Nano)
		}
		e := &models.AuditEntry{ID: id, UserID: userID, Action: action, Resource: resource, PreviousHash: storedPrev, Timestamp: ts}
		expected := computeAuditHash(e, detailsJSON, timestampStr)
		if expected != storedHash {
			return prevHash, false, fmt.Sprintf("tampered entry %s: expected hash %q, got %q", id, expected, storedHash), 0, nil
		}
		prevHash = storedHash
		lastRowID = rowID
	}
	return prevHash, true, "", lastRowID, rows.Err()
}

func (s *SQLite) ListAudit(ctx context.Context, filter models.AuditFilter) ([]*models.AuditEntry, error) {
	query := "SELECT id, user_id, action, resource, details, timestamp, previous_hash, entry_hash FROM audit_entries WHERE 1=1"
	args := []any{}

	if filter.UserID != "" {
		query += " AND user_id = ?"
		args = append(args, filter.UserID)
	}
	if filter.Resource != "" {
		query += " AND resource = ?"
		args = append(args, filter.Resource)
	}
	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}
	if filter.Since != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filter.Until)
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()

	var entries []*models.AuditEntry
	for rows.Next() {
		e, err := scanAuditRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ---------------------------------------------------------------------------
// Reviews
// ---------------------------------------------------------------------------

func (s *SQLite) CreateReview(ctx context.Context, r *models.Review) error {
	comments, err := marshalOrErr(r.Comments)
	if err != nil {
		return fmt.Errorf("marshal review comments: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO reviews (id, resource_id, resource_type, author, status, comments, created_at, resolved_at, quorum_required, approvals_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ResourceID, r.ResourceType, r.Author,
		string(r.Status), string(comments), r.CreatedAt, r.ResolvedAt,
		r.QuorumRequired, r.ApprovalsCount,
	)
	if err != nil {
		return fmt.Errorf("insert review: %w", err)
	}
	return nil
}

func (s *SQLite) GetReview(ctx context.Context, id string) (*models.Review, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, resource_id, resource_type, author, status, comments, created_at, resolved_at,
		 quorum_required, approvals_count
		 FROM reviews WHERE id = ?`, id,
	)
	return scanReview(row)
}

func (s *SQLite) UpdateReview(ctx context.Context, r *models.Review) error {
	comments, err := marshalOrErr(r.Comments)
	if err != nil {
		return fmt.Errorf("marshal review comments: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE reviews SET status=?, comments=?, resolved_at=?, approvals_count=? WHERE id=?`,
		string(r.Status), string(comments), r.ResolvedAt, r.ApprovalsCount, r.ID,
	)
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("review not found: %s", r.ID)
	}
	return nil
}

func (s *SQLite) ListPendingReviews(ctx context.Context) ([]*models.Review, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, resource_id, resource_type, author, status, comments, created_at, resolved_at,
		 quorum_required, approvals_count
		 FROM reviews WHERE status = 'pending' ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending reviews: %w", err)
	}
	defer rows.Close()
	return scanReviews(rows)
}

func (s *SQLite) ListReviewsByResource(ctx context.Context, resourceID, resourceType string) ([]*models.Review, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, resource_id, resource_type, author, status, comments, created_at, resolved_at,
		 quorum_required, approvals_count
		 FROM reviews WHERE resource_id = ? AND resource_type = ? ORDER BY created_at DESC`,
		resourceID, resourceType,
	)
	if err != nil {
		return nil, fmt.Errorf("list reviews by resource: %w", err)
	}
	defer rows.Close()
	return scanReviews(rows)
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

type scannable interface {
	Scan(dest ...any) error
}

func scanPrompt(row scannable) (*models.Prompt, error) {
	var p models.Prompt
	var variables, tags, metadata, status, binding, systemPrompt, generation string
	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Content, &systemPrompt, &variables, &tags,
		&p.ModelHint, &p.Version, &status, &p.Environment, &p.CASHash,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt, &metadata, &binding, &generation,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan prompt: %w", err)
	}
	if err := json.Unmarshal([]byte(variables), &p.Variables); err != nil {
		return nil, fmt.Errorf("unmarshal prompt variables: %w", err)
	}
	if err := json.Unmarshal([]byte(tags), &p.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal prompt tags: %w", err)
	}
	if err := json.Unmarshal([]byte(metadata), &p.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal prompt metadata: %w", err)
	}
	if err := json.Unmarshal([]byte(binding), &p.Binding); err != nil {
		return nil, fmt.Errorf("unmarshal prompt binding: %w", err)
	}
	if err := json.Unmarshal([]byte(generation), &p.Generation); err != nil {
		return nil, fmt.Errorf("unmarshal prompt generation: %w", err)
	}
	p.Status = models.PromptStatus(status)
	p.SystemPrompt = systemPrompt
	return &p, nil
}

func scanPromptRow(rows *sql.Rows) (*models.Prompt, error) {
	return scanPrompt(rows)
}

func scanAgent(row scannable) (*models.Agent, error) {
	var a models.Agent
	var steps, tools, status string
	err := row.Scan(
		&a.ID, &a.Name, &a.Description, &steps, &tools,
		&status, &a.CASHash, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan agent: %w", err)
	}
	if err := json.Unmarshal([]byte(steps), &a.Steps); err != nil {
		return nil, fmt.Errorf("unmarshal agent steps: %w", err)
	}
	if err := json.Unmarshal([]byte(tools), &a.Tools); err != nil {
		return nil, fmt.Errorf("unmarshal agent tools: %w", err)
	}
	a.Status = models.PromptStatus(status)
	return &a, nil
}

func scanAgentRow(rows *sql.Rows) (*models.Agent, error) {
	return scanAgent(rows)
}

func scanDataset(row scannable) (*models.TestDataset, error) {
	var d models.TestDataset
	var cases string
	err := row.Scan(&d.ID, &d.Name, &cases, &d.CreatedBy, &d.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan dataset: %w", err)
	}
	json.Unmarshal([]byte(cases), &d.Cases)
	return &d, nil
}

func scanDatasetRow(rows *sql.Rows) (*models.TestDataset, error) {
	return scanDataset(rows)
}

func scanEvalResults(rows *sql.Rows) ([]*models.EvalResult, error) {
	var results []*models.EvalResult
	for rows.Next() {
		var r models.EvalResult
		var tokenUsage string
		var passed int
		err := rows.Scan(
			&r.TestCaseID, &r.PromptHash, &r.Model, &r.DatasetID, &r.Output,
			&r.Score, &r.LatencyMs, &tokenUsage,
			&r.HallucinationScore, &passed, &r.Error, &r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan eval result: %w", err)
		}
		json.Unmarshal([]byte(tokenUsage), &r.TokenUsage)
		r.Passed = passed == 1
		results = append(results, &r)
	}
	return results, rows.Err()
}

func scanAuditRow(rows *sql.Rows) (*models.AuditEntry, error) {
	var e models.AuditEntry
	var details, prevHash, entryHash string
	err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Resource, &details, &e.Timestamp, &prevHash, &entryHash)
	if err != nil {
		return nil, fmt.Errorf("scan audit entry: %w", err)
	}
	json.Unmarshal([]byte(details), &e.Details)
	e.PreviousHash = prevHash
	e.EntryHash = entryHash
	return &e, nil
}

func scanReview(row scannable) (*models.Review, error) {
	var r models.Review
	var status, comments string
	err := row.Scan(
		&r.ID, &r.ResourceID, &r.ResourceType, &r.Author,
		&status, &comments, &r.CreatedAt, &r.ResolvedAt,
		&r.QuorumRequired, &r.ApprovalsCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan review: %w", err)
	}
	r.Status = models.ReviewStatus(status)
	json.Unmarshal([]byte(comments), &r.Comments)
	return &r, nil
}

func scanReviews(rows *sql.Rows) ([]*models.Review, error) {
	var reviews []*models.Review
	for rows.Next() {
		var r models.Review
		var status, comments string
		err := rows.Scan(
			&r.ID, &r.ResourceID, &r.ResourceType, &r.Author,
			&status, &comments, &r.CreatedAt, &r.ResolvedAt,
			&r.QuorumRequired, &r.ApprovalsCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		r.Status = models.ReviewStatus(status)
		json.Unmarshal([]byte(comments), &r.Comments)
		reviews = append(reviews, &r)
	}
	return reviews, rows.Err()
}

// --- API Keys ---

func (s *SQLite) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	const q = `INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, role, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		key.ID, key.UserID, key.Name, key.KeyHash, key.KeyPrefix,
		key.Role, key.ExpiresAt, key.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *SQLite) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE key_hash = ? AND revoked = 0`
	var k models.APIKey
	err := s.db.QueryRowContext(ctx, q, keyHash).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	return &k, nil
}

func (s *SQLite) GetAPIKeyByID(ctx context.Context, id string) (*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE id = ?`
	var k models.APIKey
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by id: %w", err)
	}
	return &k, nil
}

func (s *SQLite) DeleteAPIKey(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET revoked = 1 WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}

func (s *SQLite) ListAPIKeysByUser(ctx context.Context, userID string) ([]*models.APIKey, error) {
	const q = `SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
		FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
			&k.Role, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt, &k.Revoked,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}

func (s *SQLite) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET last_used = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, now(), id)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	return nil
}

// Now returns the current time. Extracted for testability.
var now = time.Now

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *SQLite) CreateUser(ctx context.Context, u *models.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.Role, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (s *SQLite) GetUser(ctx context.Context, id string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, role, created_at, updated_at FROM users WHERE id = ?`, id,
	)
	return scanUser(row)
}

func (s *SQLite) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, role, created_at, updated_at FROM users WHERE email = ?`, email,
	)
	return scanUser(row)
}

func (s *SQLite) ListUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, email, name, role, created_at, updated_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *SQLite) UpdateUser(ctx context.Context, u *models.User) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET email=?, name=?, role=?, updated_at=? WHERE id=?`,
		u.Email, u.Name, u.Role, u.UpdatedAt, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found: %s", u.ID)
	}
	return nil
}

func (s *SQLite) DeleteUser(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found: %s", id)
	}
	return nil
}

func scanUser(row scannable) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func scanUserRow(rows *sql.Rows) (*models.User, error) {
	return scanUser(rows)
}

// ---------------------------------------------------------------------------
// Audit Export
// ---------------------------------------------------------------------------

func (s *SQLite) ExportAudit(ctx context.Context, filter models.AuditFilter) ([]*models.AuditEntry, error) {
	// Export uses the same query but without limit for full data export.
	exportFilter := filter
	exportFilter.Limit = 0
	exportFilter.Offset = 0
	return s.ListAudit(ctx, exportFilter)
}

// ---------------------------------------------------------------------------
// Provider Keys
// ---------------------------------------------------------------------------

func (s *SQLite) SaveProviderKey(ctx context.Context, pk *models.ProviderKey) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO provider_keys (id, provider_name, key_name, encrypted_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET encrypted_key=excluded.encrypted_key, updated_at=excluded.updated_at`,
		pk.ID, pk.ProviderName, pk.KeyName, pk.EncryptedKey, pk.CreatedAt, pk.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save provider key: %w", err)
	}
	return nil
}

func (s *SQLite) GetProviderKey(ctx context.Context, id string) (*models.ProviderKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys WHERE id = ?`, id,
	)
	return scanProviderKey(row)
}

func (s *SQLite) GetProviderKeyByName(ctx context.Context, providerName, keyName string) (*models.ProviderKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys WHERE provider_name = ? AND key_name = ?`, providerName, keyName,
	)
	return scanProviderKey(row)
}

func (s *SQLite) DeleteProviderKey(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM provider_keys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete provider key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("provider key not found: %s", id)
	}
	return nil
}

func (s *SQLite) ListProviderKeys(ctx context.Context) ([]*models.ProviderKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list provider keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.ProviderKey
	for rows.Next() {
		k, err := scanProviderKeyRow(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func scanProviderKey(row scannable) (*models.ProviderKey, error) {
	var pk models.ProviderKey
	err := row.Scan(&pk.ID, &pk.ProviderName, &pk.KeyName, &pk.EncryptedKey, &pk.CreatedAt, &pk.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan provider key: %w", err)
	}
	return &pk, nil
}

// --- Execution Logs ---

func (s *SQLite) SaveExecutionLog(ctx context.Context, el *models.ExecutionLog) error {
	varsJSON, _ := json.Marshal(el.Variables)
	violationsJSON, _ := json.Marshal(el.Violations)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO execution_logs
		(id, prompt_id, prompt_name, prompt_version, provider, model, status,
		 variables, system_prompt, request_messages, prompt_tokens, completion_tokens,
		 total_tokens, cost_usd, latency_ms, trace_id, error, violations, environment, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		el.ID, el.PromptID, el.PromptName, el.PromptVersion, el.Provider, el.Model,
		el.Status, string(varsJSON), el.SystemPrompt, el.RequestMessages,
		el.PromptTokens, el.CompletionTokens, el.TotalTokens, el.CostUSD,
		el.LatencyMs, el.TraceID, el.Error, string(violationsJSON), el.Environment,
		el.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save execution log: %w", err)
	}
	return nil
}

func (s *SQLite) GetExecutionLog(ctx context.Context, id string) (*models.ExecutionLog, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, prompt_id, prompt_name, prompt_version, provider, model, status,
		       variables, system_prompt, request_messages, prompt_tokens, completion_tokens,
		       total_tokens, cost_usd, latency_ms, trace_id, error, violations, environment, created_at
		 FROM execution_logs WHERE id = ?`, id)
	return scanExecutionLog(row)
}

func (s *SQLite) ListExecutionLogs(ctx context.Context, filter models.ExecutionLogFilter) ([]*models.ExecutionLog, error) {
	query := `SELECT id, prompt_id, prompt_name, prompt_version, provider, model, status,
	       variables, system_prompt, request_messages, prompt_tokens, completion_tokens,
	       total_tokens, cost_usd, latency_ms, trace_id, error, violations, environment, created_at
		 FROM execution_logs WHERE 1=1`
	args := []any{}

	if filter.PromptID != "" {
		query += " AND prompt_id = ?"
		args = append(args, filter.PromptID)
	}
	if filter.Provider != "" {
		query += " AND provider = ?"
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		query += " AND model = ?"
		args = append(args, filter.Model)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Since != nil {
		query += " AND created_at >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND created_at <= ?"
		args = append(args, *filter.Until)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list execution logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.ExecutionLog
	for rows.Next() {
		el, err := scanExecutionLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, el)
	}
	return logs, rows.Err()
}

func scanExecutionLog(row scannable) (*models.ExecutionLog, error) {
	var el models.ExecutionLog
	var varsJSON, violationsJSON string
	err := row.Scan(
		&el.ID, &el.PromptID, &el.PromptName, &el.PromptVersion, &el.Provider, &el.Model,
		&el.Status, &varsJSON, &el.SystemPrompt, &el.RequestMessages,
		&el.PromptTokens, &el.CompletionTokens, &el.TotalTokens, &el.CostUSD,
		&el.LatencyMs, &el.TraceID, &el.Error, &violationsJSON, &el.Environment,
		&el.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan execution log: %w", err)
	}
	if varsJSON != "" {
		json.Unmarshal([]byte(varsJSON), &el.Variables)
	}
	if violationsJSON != "" {
		json.Unmarshal([]byte(violationsJSON), &el.Violations)
	}
	return &el, nil
}

func scanProviderKeyRow(rows *sql.Rows) (*models.ProviderKey, error) {
	return scanProviderKey(rows)
}

// --- Guardrail Rules ---

func (s *SQLite) SaveGuardrailRule(ctx context.Context, r *models.GuardrailRule) error {
	configJSON, _ := json.Marshal(r.Config)
	envsJSON, _ := json.Marshal(r.Environments)
	promptsJSON, _ := json.Marshal(r.PromptIDs)
	agentsJSON, _ := json.Marshal(r.AgentIDs)

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO guardrail_rules
		(id, name, type, severity, enabled, config, environments, prompt_ids, agent_ids, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Type, r.Severity, r.Enabled, string(configJSON),
		string(envsJSON), string(promptsJSON), string(agentsJSON),
		r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save guardrail rule: %w", err)
	}
	return nil
}

func (s *SQLite) GetGuardrailRule(ctx context.Context, id string) (*models.GuardrailRule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, severity, enabled, config, environments, prompt_ids, agent_ids, created_at, updated_at
		 FROM guardrail_rules WHERE id = ?`, id)
	return scanGuardrailRule(row)
}

func (s *SQLite) DeleteGuardrailRule(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM guardrail_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete guardrail rule: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("guardrail rule not found: %s", id)
	}
	return nil
}

func (s *SQLite) ListGuardrailRules(ctx context.Context) ([]*models.GuardrailRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, severity, enabled, config, environments, prompt_ids, agent_ids, created_at, updated_at
		 FROM guardrail_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list guardrail rules: %w", err)
	}
	defer rows.Close()

	var rules []*models.GuardrailRule
	for rows.Next() {
		r, err := scanGuardrailRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func scanGuardrailRule(row scannable) (*models.GuardrailRule, error) {
	var r models.GuardrailRule
	var configJSON, envsJSON, promptsJSON, agentsJSON string
	err := row.Scan(
		&r.ID, &r.Name, &r.Type, &r.Severity, &r.Enabled, &configJSON,
		&envsJSON, &promptsJSON, &agentsJSON, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan guardrail rule: %w", err)
	}
	if configJSON != "" {
		json.Unmarshal([]byte(configJSON), &r.Config)
	}
	if envsJSON != "" {
		json.Unmarshal([]byte(envsJSON), &r.Environments)
	}
	if promptsJSON != "" {
		json.Unmarshal([]byte(promptsJSON), &r.PromptIDs)
	}
	if agentsJSON != "" {
		json.Unmarshal([]byte(agentsJSON), &r.AgentIDs)
	}
	return &r, nil
}

// --- Guardrail Violations ---

func (s *SQLite) SaveGuardrailViolation(ctx context.Context, v *models.GuardrailViolationRecord) error {
	detailsJSON, _ := json.Marshal(v.Details)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO guardrail_violations
		(id, rule_id, rule_name, type, severity, resource_type, resource_id, user_id, message, details, resolved, resolved_by, resolved_at, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.RuleID, v.RuleName, v.Type, v.Severity, v.ResourceType, v.ResourceID,
		v.UserID, v.Message, string(detailsJSON), v.Resolved, v.ResolvedBy, v.ResolvedAt, v.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("save guardrail violation: %w", err)
	}
	return nil
}

func (s *SQLite) ListGuardrailViolations(ctx context.Context, resolved bool) ([]*models.GuardrailViolationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, rule_id, rule_name, type, severity, resource_type, resource_id, user_id, message, details, resolved, resolved_by, resolved_at, timestamp
		 FROM guardrail_violations WHERE resolved = ? ORDER BY timestamp DESC`, resolved)
	if err != nil {
		return nil, fmt.Errorf("list guardrail violations: %w", err)
	}
	defer rows.Close()

	var violations []*models.GuardrailViolationRecord
	for rows.Next() {
		v, err := scanGuardrailViolation(rows)
		if err != nil {
			return nil, err
		}
		violations = append(violations, v)
	}
	return violations, rows.Err()
}

func (s *SQLite) UpdateGuardrailViolation(ctx context.Context, v *models.GuardrailViolationRecord) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE guardrail_violations SET resolved=?, resolved_by=?, resolved_at=? WHERE id=?`,
		v.Resolved, v.ResolvedBy, v.ResolvedAt, v.ID,
	)
	if err != nil {
		return fmt.Errorf("update guardrail violation: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("guardrail violation not found: %s", v.ID)
	}
	return nil
}

func scanGuardrailViolation(row scannable) (*models.GuardrailViolationRecord, error) {
	var v models.GuardrailViolationRecord
	var detailsJSON string
	var resolvedAt *time.Time
	err := row.Scan(
		&v.ID, &v.RuleID, &v.RuleName, &v.Type, &v.Severity, &v.ResourceType, &v.ResourceID,
		&v.UserID, &v.Message, &detailsJSON, &v.Resolved, &v.ResolvedBy, &resolvedAt, &v.Timestamp,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan guardrail violation: %w", err)
	}
	if detailsJSON != "" {
		json.Unmarshal([]byte(detailsJSON), &v.Details)
	}
	v.ResolvedAt = resolvedAt
	return &v, nil
}

// --- Contexts ---

func (s *SQLite) CreateContext(ctx context.Context, c *models.Context) error {
	messagesJSON, _ := json.Marshal(c.Messages)
	metadataJSON, _ := json.Marshal(c.Metadata)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO contexts
		(id, name, description, type, system_prompt, messages, token_budget, token_count, truncation_strategy, agent_id, version, status, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Description, c.Type, c.SystemPrompt, string(messagesJSON),
		c.TokenBudget, c.TokenCount, c.TruncationStrategy, c.AgentID, c.Version,
		c.Status, string(metadataJSON), c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create context: %w", err)
	}
	return nil
}

func (s *SQLite) GetContext(ctx context.Context, id string) (*models.Context, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, type, system_prompt, messages, token_budget, token_count, truncation_strategy, agent_id, version, status, metadata, created_at, updated_at
		 FROM contexts WHERE id = ?`, id)
	return scanContext(row)
}

func (s *SQLite) ListContexts(ctx context.Context, filter models.ContextFilter) ([]*models.Context, error) {
	query := `SELECT id, name, description, type, system_prompt, messages, token_budget, token_count, truncation_strategy, agent_id, version, status, metadata, created_at, updated_at FROM contexts WHERE 1=1`
	args := []any{}

	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, filter.Type)
	}
	if filter.Search != "" {
		query += " AND (name LIKE ? OR description LIKE ?)"
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
	}
	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, st := range filter.Status {
			placeholders[i] = "?"
			args = append(args, st)
		}
		query += " AND status IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += " ORDER BY updated_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list contexts: %w", err)
	}
	defer rows.Close()

	var contexts []*models.Context
	for rows.Next() {
		c, err := scanContext(rows)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, c)
	}
	return contexts, rows.Err()
}

func (s *SQLite) UpdateContext(ctx context.Context, c *models.Context) error {
	messagesJSON, _ := json.Marshal(c.Messages)
	metadataJSON, _ := json.Marshal(c.Metadata)
	result, err := s.db.ExecContext(ctx, `
		UPDATE contexts SET name=?, description=?, type=?, system_prompt=?, messages=?,
		token_budget=?, token_count=?, truncation_strategy=?, agent_id=?, version=?, status=?,
		metadata=?, updated_at=?
		WHERE id=?`,
		c.Name, c.Description, c.Type, c.SystemPrompt, string(messagesJSON),
		c.TokenBudget, c.TokenCount, c.TruncationStrategy, c.AgentID, c.Version,
		c.Status, string(metadataJSON), c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update context: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("context not found: %s", c.ID)
	}
	return nil
}

func (s *SQLite) DeleteContext(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM contexts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete context: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("context not found: %s", id)
	}
	return nil
}

func scanContext(row scannable) (*models.Context, error) {
	var c models.Context
	var messagesJSON, metadataJSON string
	err := row.Scan(
		&c.ID, &c.Name, &c.Description, &c.Type, &c.SystemPrompt, &messagesJSON,
		&c.TokenBudget, &c.TokenCount, &c.TruncationStrategy, &c.AgentID, &c.Version,
		&c.Status, &metadataJSON, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan context: %w", err)
	}
	if messagesJSON != "" {
		json.Unmarshal([]byte(messagesJSON), &c.Messages)
	}
	if metadataJSON != "" {
		json.Unmarshal([]byte(metadataJSON), &c.Metadata)
	}
	return &c, nil
}

// --- Agent Guardrail Configs ---

func (s *SQLite) SaveAgentGuardrailConfig(ctx context.Context, c *models.AgentGuardrailConfig) error {
	contentPolicyJSON, _ := json.Marshal(c.ContentPolicy)
	restrictedTermsJSON, _ := json.Marshal(c.RestrictedTerms)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_guardrail_configs
		(id, agent_id, name, enabled, max_cost_per_run, max_latency_ms, max_tokens_per_step, content_policy, restricted_terms, stop_on_violation, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.AgentID, c.Name, c.Enabled, c.MaxCostPerRun, c.MaxLatencyMs,
		c.MaxTokensPerStep, string(contentPolicyJSON), string(restrictedTermsJSON),
		c.StopOnViolation, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save agent guardrail config: %w", err)
	}
	return nil
}

func (s *SQLite) GetAgentGuardrailConfig(ctx context.Context, id string) (*models.AgentGuardrailConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, agent_id, name, enabled, max_cost_per_run, max_latency_ms, max_tokens_per_step, content_policy, restricted_terms, stop_on_violation, created_at, updated_at
		 FROM agent_guardrail_configs WHERE id = ?`, id)
	return scanAgentGuardrailConfig(row)
}

func (s *SQLite) GetAgentGuardrailConfigByAgent(ctx context.Context, agentID string) (*models.AgentGuardrailConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, agent_id, name, enabled, max_cost_per_run, max_latency_ms, max_tokens_per_step, content_policy, restricted_terms, stop_on_violation, created_at, updated_at
		 FROM agent_guardrail_configs WHERE agent_id = ? ORDER BY updated_at DESC LIMIT 1`, agentID)
	return scanAgentGuardrailConfig(row)
}

func (s *SQLite) DeleteAgentGuardrailConfig(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM agent_guardrail_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete agent guardrail config: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent guardrail config not found: %s", id)
	}
	return nil
}

func scanAgentGuardrailConfig(row scannable) (*models.AgentGuardrailConfig, error) {
	var c models.AgentGuardrailConfig
	var contentPolicyJSON, restrictedTermsJSON string
	err := row.Scan(
		&c.ID, &c.AgentID, &c.Name, &c.Enabled, &c.MaxCostPerRun, &c.MaxLatencyMs,
		&c.MaxTokensPerStep, &contentPolicyJSON, &restrictedTermsJSON,
		&c.StopOnViolation, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan agent guardrail config: %w", err)
	}
	if contentPolicyJSON != "" {
		json.Unmarshal([]byte(contentPolicyJSON), &c.ContentPolicy)
	}
	if restrictedTermsJSON != "" {
		json.Unmarshal([]byte(restrictedTermsJSON), &c.RestrictedTerms)
	}
	return &c, nil
}

// --- Agent Executions ---

func (s *SQLite) SaveAgentExecution(ctx context.Context, e *models.AgentExecution) error {
	inputJSON, _ := json.Marshal(e.Input)
	outputJSON, _ := json.Marshal(e.Output)
	stepsJSON, _ := json.Marshal(e.Steps)
	violationsJSON, _ := json.Marshal(e.GuardrailViolations)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_executions
		(id, agent_id, workflow_id, status, input, output, steps, total_cost_usd, total_latency_ms, guardrail_violations, context_id, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.AgentID, e.WorkflowID, e.Status, string(inputJSON), string(outputJSON),
		string(stepsJSON), e.TotalCostUSD, e.TotalLatencyMs, string(violationsJSON),
		e.ContextID, e.CreatedAt, e.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("save agent execution: %w", err)
	}
	return nil
}

func (s *SQLite) GetAgentExecution(ctx context.Context, id string) (*models.AgentExecution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, agent_id, workflow_id, status, input, output, steps, total_cost_usd, total_latency_ms, guardrail_violations, context_id, created_at, completed_at
		 FROM agent_executions WHERE id = ?`, id)
	return scanAgentExecution(row)
}

func (s *SQLite) ListAgentExecutions(ctx context.Context, agentID string, limit, offset int) ([]*models.AgentExecution, error) {
	query := `SELECT id, agent_id, workflow_id, status, input, output, steps, total_cost_usd, total_latency_ms, guardrail_violations, context_id, created_at, completed_at
		 FROM agent_executions WHERE agent_id = ? ORDER BY created_at DESC`
	args := []any{agentID}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agent executions: %w", err)
	}
	defer rows.Close()

	var execs []*models.AgentExecution
	for rows.Next() {
		e, err := scanAgentExecution(rows)
		if err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

func scanAgentExecution(row scannable) (*models.AgentExecution, error) {
	var e models.AgentExecution
	var inputJSON, outputJSON, stepsJSON, violationsJSON string
	var completedAt *time.Time
	err := row.Scan(
		&e.ID, &e.AgentID, &e.WorkflowID, &e.Status, &inputJSON, &outputJSON,
		&stepsJSON, &e.TotalCostUSD, &e.TotalLatencyMs, &violationsJSON,
		&e.ContextID, &e.CreatedAt, &completedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan agent execution: %w", err)
	}
	if inputJSON != "" {
		json.Unmarshal([]byte(inputJSON), &e.Input)
	}
	if outputJSON != "" {
		json.Unmarshal([]byte(outputJSON), &e.Output)
	}
	if stepsJSON != "" {
		json.Unmarshal([]byte(stepsJSON), &e.Steps)
	}
	if violationsJSON != "" {
		json.Unmarshal([]byte(violationsJSON), &e.GuardrailViolations)
	}
	e.CompletedAt = completedAt
	return &e, nil
}

// Alert Rules

func (s *SQLite) SaveAlertRule(ctx context.Context, r *models.AlertRuleRecord) error {
	configJSON, err := marshalOrErr(r.Config)
	if err != nil {
		return fmt.Errorf("marshal alert rule config: %w", err)
	}
	// Upsert that preserves created_at across updates. The previous
	// INSERT OR REPLACE on the primary key deleted and re-inserted the
	// row, destroying the original timestamp.
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO alert_rules (id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			severity = excluded.severity,
			enabled = excluded.enabled,
			threshold = excluded.threshold,
			duration = excluded.duration,
			window = excluded.window,
			config = excluded.config,
			updated_at = excluded.updated_at`,
		r.ID, r.Name, r.Type, r.Severity, r.Enabled, r.Threshold, r.Duration, r.Window, configJSON, r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save alert rule: %w", err)
	}
	return nil
}

func (s *SQLite) GetAlertRule(ctx context.Context, id string) (*models.AlertRuleRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at
		FROM alert_rules WHERE id = ?`, id)
	return scanAlertRule(row)
}

func (s *SQLite) DeleteAlertRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	return nil
}

func (s *SQLite) ListAlertRules(ctx context.Context) ([]*models.AlertRuleRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, severity, enabled, threshold, duration, window, config, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	var rules []*models.AlertRuleRecord
	for rows.Next() {
		r, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func scanAlertRule(row scannable) (*models.AlertRuleRecord, error) {
	var r models.AlertRuleRecord
	var configJSON string
	err := row.Scan(
		&r.ID, &r.Name, &r.Type, &r.Severity, &r.Enabled, &r.Threshold, &r.Duration, &r.Window,
		&configJSON, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan alert rule: %w", err)
	}
	if configJSON != "" {
		json.Unmarshal([]byte(configJSON), &r.Config)
	}
	return &r, nil
}

// Alerts

func (s *SQLite) SaveAlert(ctx context.Context, a *models.AlertRecord) error {
	detailsJSON, err := marshalOrErr(a.Details)
	if err != nil {
		return fmt.Errorf("marshal alert details: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO alerts (id, rule_id, rule_name, severity, status, message, details, triggered_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.RuleID, a.RuleName, a.Severity, a.Status, a.Message, detailsJSON, a.TriggeredAt, a.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("save alert: %w", err)
	}
	return nil
}

func (s *SQLite) GetAlert(ctx context.Context, id string) (*models.AlertRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, rule_id, rule_name, severity, status, message, details, triggered_at, resolved_at
		FROM alerts WHERE id = ?`, id)
	return scanAlert(row)
}

func (s *SQLite) UpdateAlert(ctx context.Context, a *models.AlertRecord) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE alerts SET status=?, resolved_at=? WHERE id=?`,
		a.Status, a.ResolvedAt, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("alert not found: %s", a.ID)
	}
	return nil
}

func (s *SQLite) ListAlerts(ctx context.Context, status string) ([]*models.AlertRecord, error) {
	query := `SELECT id, rule_id, rule_name, severity, status, message, details, triggered_at, resolved_at FROM alerts`
	var args []any
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY triggered_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var alerts []*models.AlertRecord
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func scanAlert(row scannable) (*models.AlertRecord, error) {
	var a models.AlertRecord
	var detailsJSON string
	var resolvedAt *time.Time
	err := row.Scan(
		&a.ID, &a.RuleID, &a.RuleName, &a.Severity, &a.Status, &a.Message,
		&detailsJSON, &a.TriggeredAt, &resolvedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan alert: %w", err)
	}
	if detailsJSON != "" {
		json.Unmarshal([]byte(detailsJSON), &a.Details)
	}
	a.ResolvedAt = resolvedAt
	return &a, nil
}

// Notification Groups

func (s *SQLite) SaveNotificationGroup(ctx context.Context, g *models.NotificationGroupRecord) error {
	channelsJSON, err := marshalOrErr(g.Channels)
	if err != nil {
		return fmt.Errorf("marshal notification channels: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO notification_groups (id, name, channels)
		VALUES (?, ?, ?)`,
		g.ID, g.Name, channelsJSON,
	)
	if err != nil {
		return fmt.Errorf("save notification group: %w", err)
	}
	return nil
}

func (s *SQLite) GetNotificationGroup(ctx context.Context, id string) (*models.NotificationGroupRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, channels FROM notification_groups WHERE id = ?`, id)
	return scanNotificationGroup(row)
}

func (s *SQLite) DeleteNotificationGroup(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notification_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete notification group: %w", err)
	}
	return nil
}

func (s *SQLite) ListNotificationGroups(ctx context.Context) ([]*models.NotificationGroupRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, channels FROM notification_groups ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list notification groups: %w", err)
	}
	defer rows.Close()

	var groups []*models.NotificationGroupRecord
	for rows.Next() {
		g, err := scanNotificationGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// ---------------------------------------------------------------------------
// Webhook Endpoints
// ---------------------------------------------------------------------------

func (s *SQLite) SaveWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpointRecord) error {
	events := strings.Join(ep.Events, ",")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_endpoints (id, url, secret, events, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			url = excluded.url,
			secret = excluded.secret,
			events = excluded.events,
			active = excluded.active`,
		ep.ID, ep.URL, ep.Secret, events, ep.Active, ep.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save webhook endpoint: %w", err)
	}
	return nil
}

func (s *SQLite) GetWebhookEndpoint(ctx context.Context, id string) (*models.WebhookEndpointRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, url, secret, events, active, created_at FROM webhook_endpoints WHERE id = ?`, id)
	ep, err := scanWebhookEndpoint(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return ep, err
}

func (s *SQLite) DeleteWebhookEndpoint(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhook_endpoints WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	return nil
}

func (s *SQLite) ListWebhookEndpoints(ctx context.Context) ([]*models.WebhookEndpointRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret, events, active, created_at FROM webhook_endpoints ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer rows.Close()
	var eps []*models.WebhookEndpointRecord
	for rows.Next() {
		ep, err := scanWebhookEndpoint(rows)
		if err != nil {
			return nil, err
		}
		eps = append(eps, ep)
	}
	return eps, rows.Err()
}

func scanWebhookEndpoint(row scannable) (*models.WebhookEndpointRecord, error) {
	var ep models.WebhookEndpointRecord
	var events string
	err := row.Scan(&ep.ID, &ep.URL, &ep.Secret, &events, &ep.Active, &ep.CreatedAt)
	if err != nil {
		return nil, err
	}
	if events != "" {
		ep.Events = strings.Split(events, ",")
	}
	return &ep, nil
}

func scanNotificationGroup(row scannable) (*models.NotificationGroupRecord, error) {
	var g models.NotificationGroupRecord
	var channelsJSON string
	err := row.Scan(&g.ID, &g.Name, &channelsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan notification group: %w", err)
	}
	if channelsJSON != "" {
		json.Unmarshal([]byte(channelsJSON), &g.Channels)
	}
	return &g, nil
}
