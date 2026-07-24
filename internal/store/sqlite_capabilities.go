package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/schedule"
)

// ensure SQLite implements the consumer-defined capability.Repository
// and schedule.Repository interfaces.
var (
	_ capability.Repository = (*SQLite)(nil)
	_ schedule.Repository   = (*SQLite)(nil)
)

// ---------------------------------------------------------------------------
// Workspaces
// ---------------------------------------------------------------------------

func (s *SQLite) CreateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, organization, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.Organization, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert workspace: %w", err)
	}
	return nil
}

func (s *SQLite) GetWorkspace(ctx context.Context, id string) (*capability.Workspace, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, organization, created_at, updated_at FROM workspaces WHERE id = ?`, id,
	)
	return scanWorkspace(row)
}

func (s *SQLite) ListWorkspaces(ctx context.Context) ([]*capability.Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, organization, created_at, updated_at FROM workspaces ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

func (s *SQLite) UpdateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces SET name = ?, organization = ?, updated_at = ? WHERE id = ?`,
		w.Name, w.Organization, w.UpdatedAt, w.ID,
	)
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteWorkspace(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

func scanWorkspace(scanner interface {
	Scan(dest ...any) error
}) (*capability.Workspace, error) {
	var w capability.Workspace
	err := scanner.Scan(&w.ID, &w.Name, &w.Organization, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan workspace: %w", err)
	}
	return &w, nil
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

func (s *SQLite) CreateProject(ctx context.Context, p *capability.Project) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.WorkspaceID, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

func (s *SQLite) GetProject(ctx context.Context, id string) (*capability.Project, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at FROM projects WHERE id = ?`, id,
	)
	return scanProject(row)
}

func (s *SQLite) ListProjects(ctx context.Context, workspaceID string) ([]*capability.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at FROM projects WHERE workspace_id = ? ORDER BY name`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *SQLite) UpdateProject(ctx context.Context, p *capability.Project) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Description, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteProject(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

func scanProject(scanner interface {
	Scan(dest ...any) error
}) (*capability.Project, error) {
	var p capability.Project
	err := scanner.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	return &p, nil
}

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------

func (s *SQLite) CreateCapability(ctx context.Context, c *capability.Capability) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Description, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert capability: %w", err)
	}
	return nil
}

func (s *SQLite) GetCapability(ctx context.Context, id string) (*capability.Capability, error) {
	// PERF-DB-1: use the prepared statement when available.
	var row interface {
		Scan(...any) error
	}
	if s.stmtGetCapability != nil {
		row = s.stmtGetCapability.QueryRowContext(ctx, id)
	} else {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, project_id, name, description, created_at, updated_at
			 FROM capabilities WHERE id = ?`, id,
		)
	}
	return scanCapability(row)
}

func (s *SQLite) ListCapabilities(ctx context.Context, projectID string) ([]*capability.Capability, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, description, created_at, updated_at
		 FROM capabilities WHERE project_id = ? ORDER BY name`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list capabilities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Capability
	for rows.Next() {
		c, err := scanCapability(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *SQLite) UpdateCapability(ctx context.Context, c *capability.Capability) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE capabilities SET name = ?, description = ?, updated_at = ?
		 WHERE id = ?`,
		c.Name, c.Description, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update capability: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteCapability(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM capabilities WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete capability: %w", err)
	}
	return nil
}

func scanCapability(scanner interface {
	Scan(dest ...any) error
}) (*capability.Capability, error) {
	var c capability.Capability
	err := scanner.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan capability: %w", err)
	}
	// State and CurrentVersionID are derived from Release state
	// (capability.DeriveState). Migration 044 dropped the
	// vestigial columns; callers that need them should call
	// DeriveState explicitly.
	return &c, nil
}

// ---------------------------------------------------------------------------
// Capability Versions
// ---------------------------------------------------------------------------

// marshalField is the single remaining helper from the legacy
// versionJSONFields shape. F-05 forward-only: Version carries
// only the Manifest; the per-artifact JSON columns are gone.
func marshalField(v any) (string, error) {
	b, err := marshalOrErr(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *SQLite) CreateVersion(ctx context.Context, v *capability.Version) error {
	manifestJSON, err := marshalOrErr(v.Manifest)
	if err != nil {
		return fmt.Errorf("marshal version manifest: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO capability_versions
		 (id, capability_id, version, manifest, manifest_hash, created_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.CapabilityID, v.Version, string(manifestJSON), v.ManifestHash,
		v.CreatedAt, v.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}
	return nil
}

func (s *SQLite) GetVersion(ctx context.Context, id string) (*capability.Version, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE id = ?`, id,
	)
	return scanCapabilityVersion(row)
}

func (s *SQLite) ListVersions(ctx context.Context, capabilityID string) ([]*capability.Version, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? ORDER BY version DESC`, capabilityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Version
	for rows.Next() {
		v, err := scanCapabilityVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *SQLite) GetLatestVersion(ctx context.Context, capabilityID string) (*capability.Version, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? ORDER BY version DESC LIMIT 1`, capabilityID,
	)
	return scanCapabilityVersion(row)
}

func scanCapabilityVersion(scanner interface {
	Scan(dest ...any) error
}) (*capability.Version, error) {
	var v capability.Version
	var manifestJSON string

	err := scanner.Scan(&v.ID, &v.CapabilityID, &v.Version,
		&manifestJSON, &v.ManifestHash, &v.CreatedAt, &v.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan version: %w", err)
	}

	if manifestJSON != "" && manifestJSON != "{}" {
		mustUnmarshal([]byte(manifestJSON), &v.Manifest)
	}

	return &v, nil
}

// ---------------------------------------------------------------------------
// Executions
// ---------------------------------------------------------------------------

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
		limit = -1
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
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan execution: %w", err)
	}

	mustUnmarshal([]byte(inputsJSON), &e.Inputs)
	mustUnmarshal([]byte(outputsJSON), &e.Outputs)

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	return &e, nil
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------

func (s *SQLite) CreateSchedule(ctx context.Context, sc *schedule.Schedule) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO schedules (id, workspace_id, release_id, kind, cron, webhook_path, next_fire_at, last_fire_at, fired_count, enabled, created_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.ID, sc.WorkspaceID, sc.ReleaseID, string(sc.Kind), sc.Cron, sc.WebhookPath,
		sc.NextFireAt, sc.LastFireAt, sc.FiredCount, sc.Enabled, sc.CreatedAt, sc.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("insert schedule: %w", err)
	}
	return nil
}

// ListDueSchedules returns schedules due to fire at-or-before now.
// DEAD-Store-2 removed the unused Create/Get/List/Delete
// variants; ListDueSchedules + UpdateSchedule is the only path
// the scheduler needs.
func (s *SQLite) ListDueSchedules(ctx context.Context, now time.Time, limit int) ([]*schedule.Schedule, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, release_id, kind, cron, webhook_path, next_fire_at, last_fire_at, fired_count, enabled, created_at, created_by
		 FROM schedules WHERE enabled = 1 AND next_fire_at <= ? ORDER BY next_fire_at ASC LIMIT ?`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list due schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*schedule.Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *SQLite) UpdateSchedule(ctx context.Context, sc *schedule.Schedule) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE schedules SET next_fire_at = ?, last_fire_at = ?, fired_count = ?, enabled = ?
		 WHERE id = ?`,
		sc.NextFireAt, sc.LastFireAt, sc.FiredCount, sc.Enabled, sc.ID,
	)
	if err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	return nil
}

func scanSchedule(scanner interface {
	Scan(dest ...any) error
}) (*schedule.Schedule, error) {
	var sc schedule.Schedule
	var kindStr string
	err := scanner.Scan(
		&sc.ID, &sc.WorkspaceID, &sc.ReleaseID, &kindStr, &sc.Cron, &sc.WebhookPath,
		&sc.NextFireAt, &sc.LastFireAt, &sc.FiredCount, &sc.Enabled,
		&sc.CreatedAt, &sc.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan schedule: %w", err)
	}
	sc.Kind = schedule.Kind(kindStr)
	return &sc, nil
}
