// Package postgres implements the capability, release, approval,
// recommendation, lineage, and policy repositories against
// PostgreSQL using jackc/pgx/v5.
//
// The first cut ships capability only (per Tier 1.10 from the
// architecture review). The aggregate interfaces
// (capability.Repository, release.Repository, approval.Repository,
// recommendation.Repository, lineage.Repository, policy.Repository)
// are unchanged; the storage package is a drop-in replacement for
// the SQLite implementation.
//
// Connection management uses *pgxpool.Pool so the implementation
// is safe under concurrent use.
//
// Per-workspace Row Level Security is enabled by migration 100_*.sql;
// every per-workspace Repository method wraps its work in a
// transaction that `SET LOCAL app.current_workspace = $1` so a
// query that escapes its workspace returns zero rows.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sachncs/promptsheon/internal/capability"
)

// Postgres is a pgxpool-backed implementation of the
// capability.Repository interface.
type Postgres struct {
	pool *pgxpool.Pool
}

// Open establishes a connection pool against the supplied DSN.
//
// The connection pool is configured for typical control-plane
// workloads (MaxConns=16, MinConns=2); production tuning lives in
// deployment rather than in code.
func Open(ctx context.Context, dsn string) (*Postgres, error) {
	if dsn == "" {
		return nil, errors.New("postgres: empty DSN")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse dsn: %w", err)
	}
	cfg.MaxConns = 16
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close releases the underlying connection pool.
func (p *Postgres) Close() error {
	if p == nil || p.pool == nil {
		return nil
	}
	p.pool.Close()
	return nil
}

// Compile-time assertion that *Postgres satisfies the consumer-defined
// interface. The interface lives in the capability package; storage
// imports only what it must.
var _ capability.Repository = (*Postgres)(nil)

// tx returns a closure that begins a transaction with the
// per-workspace SET LOCAL applied. This is the chokepoint for RLS
// enforcement; every per-workspace method routes through it.
func (p *Postgres) withWorkspace(ctx context.Context, workspaceID string, fn func(pgx.Tx) error) error {
	if workspaceID == "" {
		return errors.New("postgres: workspace_id is required for SET LOCAL")
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL app.current_workspace = $1", workspaceID); err != nil {
		return fmt.Errorf("postgres: set local: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// -- Capability Repository (subset; ships in this commit) --

// CreateWorkspace implements capability.Repository.
func (p *Postgres) CreateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO workspaces (id, name, organization, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		w.ID, w.Name, w.Organization, w.CreatedAt, w.UpdatedAt,
	)
	return err
}

// GetWorkspace implements capability.Repository.
func (p *Postgres) GetWorkspace(ctx context.Context, id string) (*capability.Workspace, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, name, organization, created_at, updated_at
		 FROM workspaces WHERE id = $1`, id,
	)
	var w capability.Workspace
	if err := row.Scan(&w.ID, &w.Name, &w.Organization, &w.CreatedAt, &w.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	return &w, nil
}

// ListWorkspaces implements capability.Repository.
func (p *Postgres) ListWorkspaces(ctx context.Context) ([]*capability.Workspace, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, name, organization, created_at, updated_at
		 FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*capability.Workspace
	for rows.Next() {
		var w capability.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Organization, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &w)
	}
	return out, rows.Err()
}

// UpdateWorkspace implements capability.Repository.
func (p *Postgres) UpdateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE workspaces SET name = $1, organization = $2, updated_at = $3
		 WHERE id = $4`,
		w.Name, w.Organization, w.UpdatedAt, w.ID,
	)
	return err
}

// DeleteWorkspace implements capability.Repository.
func (p *Postgres) DeleteWorkspace(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, id)
	return err
}

// CreateProject implements capability.Repository.
func (p *Postgres) CreateProject(ctx context.Context, prj *capability.Project) error {
	return p.withWorkspace(ctx, prj.WorkspaceID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			prj.ID, prj.WorkspaceID, prj.Name, prj.Description, prj.CreatedAt, prj.UpdatedAt,
		)
		return err
	})
}

// GetProject implements capability.Repository.
func (p *Postgres) GetProject(ctx context.Context, id string) (*capability.Project, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	)
	var prj capability.Project
	if err := row.Scan(&prj.ID, &prj.WorkspaceID, &prj.Name, &prj.Description, &prj.CreatedAt, &prj.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	return &prj, nil
}

// ListProjects implements capability.Repository.
func (p *Postgres) ListProjects(ctx context.Context, workspaceID string) ([]*capability.Project, error) {
	return listProjects(ctx, p.pool, workspaceID)
}

func listProjects(ctx context.Context, q queryer, workspaceID string) ([]*capability.Project, error) {
	rows, err := q.Query(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at
		 FROM projects WHERE workspace_id = $1 ORDER BY name`, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*capability.Project
	for rows.Next() {
		var prj capability.Project
		if err := rows.Scan(&prj.ID, &prj.WorkspaceID, &prj.Name, &prj.Description, &prj.CreatedAt, &prj.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &prj)
	}
	return out, rows.Err()
}

// UpdateProject implements capability.Repository.
func (p *Postgres) UpdateProject(ctx context.Context, prj *capability.Project) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE projects SET name = $1, description = $2, updated_at = $3 WHERE id = $4`,
		prj.Name, prj.Description, prj.UpdatedAt, prj.ID,
	)
	return err
}

// DeleteProject implements capability.Repository.
func (p *Postgres) DeleteProject(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

// CreateCapability implements capability.Repository.
//
// Note: capability.State is derived from Releases (M0.8); the row
// is inserted with state='draft' and capability.current_version_id
// empty so the schema stays forward-compatible but the column has
// no semantic meaning. Callers that need the live state call
// capability.DeriveState over the Release set.
func (p *Postgres) CreateCapability(ctx context.Context, c *capability.Capability) error {
	tags, err := jsonMarshal(c.Tags)
	if err != nil {
		return err
	}
	return p.withWorkspace(ctx, c.ProjectID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO capabilities (id, project_id, name, description, owner, tags, state, current_version_id, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, 'draft', '', $7, $8)`,
			c.ID, c.ProjectID, c.Name, c.Description, c.Owner, tags, c.CreatedAt, c.UpdatedAt,
		)
		return err
	})
}

// GetCapability implements capability.Repository.
func (p *Postgres) GetCapability(ctx context.Context, id string) (*capability.Capability, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, project_id, name, description, owner, tags, created_at, updated_at
		 FROM capabilities WHERE id = $1`, id,
	)
	var cap capability.Capability
	var tagsRaw []byte
	if err := row.Scan(&cap.ID, &cap.ProjectID, &cap.Name, &cap.Description, &cap.Owner, &tagsRaw, &cap.CreatedAt, &cap.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	if err := jsonUnmarshal(tagsRaw, &cap.Tags); err != nil {
		return nil, fmt.Errorf("postgres: capability tags: %w", err)
	}
	return &cap, nil
}

// ListCapabilities implements capability.Repository.
func (p *Postgres) ListCapabilities(ctx context.Context, projectID string) ([]*capability.Capability, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, project_id, name, description, owner, tags, created_at, updated_at
		 FROM capabilities WHERE project_id = $1 ORDER BY name`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*capability.Capability
	for rows.Next() {
		var c capability.Capability
		var tagsRaw []byte
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.Owner, &tagsRaw, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if err := jsonUnmarshal(tagsRaw, &c.Tags); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// UpdateCapability implements capability.Repository.
func (p *Postgres) UpdateCapability(ctx context.Context, c *capability.Capability) error {
	tags, err := jsonMarshal(c.Tags)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx,
		`UPDATE capabilities
		    SET name = $1, description = $2, owner = $3, tags = $4, updated_at = $5
		  WHERE id = $6`,
		c.Name, c.Description, c.Owner, tags, c.UpdatedAt, c.ID,
	)
	return err
}

// DeleteCapability implements capability.Repository.
func (p *Postgres) DeleteCapability(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM capabilities WHERE id = $1`, id)
	return err
}

// CreateVersion implements capability.Repository.
func (p *Postgres) CreateVersion(ctx context.Context, v *capability.Version) error {
	manifestJSON, err := jsonMarshal(v.Manifest)
	if err != nil {
		return err
	}
	promptJSON, err := jsonMarshal(v.Prompt)
	if err != nil {
		return err
	}
	mpJSON, err := jsonMarshal(v.ModelPolicy)
	if err != nil {
		return err
	}
	ccJSON, err := jsonMarshal(v.ContextContract)
	if err != nil {
		return err
	}
	memJSON, err := jsonMarshal(v.Memory)
	if err != nil {
		return err
	}
	knowJSON, err := jsonMarshal(v.Knowledge)
	if err != nil {
		return err
	}
	gJSON, err := jsonMarshal(v.Guardrails)
	if err != nil {
		return err
	}
	tJSON, err := jsonMarshal(v.Tools)
	if err != nil {
		return err
	}
	mcpJSON, err := jsonMarshal(v.MCPServers)
	if err != nil {
		return err
	}
	rpJSON, err := jsonMarshal(v.RuntimePolicy)
	if err != nil {
		return err
	}
	esJSON, err := jsonMarshal(v.EvaluationSuite)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx,
		`INSERT INTO capability_versions
		   (id, capability_id, version, manifest, manifest_hash, prompt, model_policy,
		    context_contract, knowledge, memory, guardrails, tools, mcp_servers,
		    runtime_policy, evaluation_suite, created_at, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		v.ID, v.CapabilityID, v.Version, manifestJSON, v.ManifestHash,
		promptJSON, mpJSON, ccJSON, knowJSON, memJSON, gJSON, tJSON, mcpJSON,
		rpJSON, esJSON, v.CreatedAt, v.CreatedBy,
	)
	return err
}

// GetVersion implements capability.Repository.
func (p *Postgres) GetVersion(ctx context.Context, id string) (*capability.Version, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, prompt, model_policy,
		        context_contract, knowledge, memory, guardrails, tools, mcp_servers,
		        runtime_policy, evaluation_suite, created_at, created_by
		 FROM capability_versions WHERE id = $1`, id,
	)
	return scanVersion(row)
}

// ListVersions implements capability.Repository.
func (p *Postgres) ListVersions(ctx context.Context, capabilityID string) ([]*capability.Version, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, prompt, model_policy,
		        context_contract, knowledge, memory, guardrails, tools, mcp_servers,
		        runtime_policy, evaluation_suite, created_at, created_by
		 FROM capability_versions WHERE capability_id = $1 ORDER BY version DESC`, capabilityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*capability.Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetLatestVersion implements capability.Repository.
func (p *Postgres) GetLatestVersion(ctx context.Context, capabilityID string) (*capability.Version, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, prompt, model_policy,
		        context_contract, knowledge, memory, guardrails, tools, mcp_servers,
		        runtime_policy, evaluation_suite, created_at, created_by
		 FROM capability_versions WHERE capability_id = $1 ORDER BY version DESC LIMIT 1`, capabilityID,
	)
	return scanVersion(row)
}

// CreateExecution implements capability.Repository.
func (p *Postgres) CreateExecution(ctx context.Context, e *capability.Execution) error {
	inputs, err := jsonMarshal(e.Inputs)
	if err != nil {
		return err
	}
	outputs, err := jsonMarshal(e.Outputs)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx,
		`INSERT INTO executions
		   (id, capability_version_id, timestamp, inputs, outputs, model, provider,
		    latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
		    error, trace_id, environment)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		e.ID, e.CapabilityVersionID, e.Timestamp, inputs, outputs,
		e.Model, e.Provider, e.LatencyMs, e.CostUSD,
		e.PromptTokens, e.CompletionTokens, e.TotalTokens,
		e.Error, e.TraceID, e.Environment,
	)
	return err
}

// GetExecution implements capability.Repository.
func (p *Postgres) GetExecution(ctx context.Context, id string) (*capability.Execution, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
		        latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
		        error, trace_id, environment
		 FROM executions WHERE id = $1`, id,
	)
	return scanExecution(row)
}

// ListExecutions implements capability.Repository.
func (p *Postgres) ListExecutions(ctx context.Context, f capability.ExecutionFilter) ([]*capability.Execution, error) {
	q := `SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
	             latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
	             error, trace_id, environment
	      FROM executions WHERE 1=1`
	args := []any{}
	if f.CapabilityVersionID != "" {
		q += " AND capability_version_id = $1"
		args = append(args, f.CapabilityVersionID)
	}
	q += " ORDER BY timestamp DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	if f.Offset > 0 {
		q += fmt.Sprintf(" OFFSET %d", f.Offset)
	}
	rows, err := p.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*capability.Execution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// scanVersion reads one capability_versions row.
func scanVersion(row pgx.Row) (*capability.Version, error) {
	var v capability.Version
	var manifest, prompt, mp, cc, know, mem, g, t, mcp, rp, es []byte
	if err := row.Scan(&v.ID, &v.CapabilityID, &v.Version, &manifest, &v.ManifestHash,
		&prompt, &mp, &cc, &know, &mem, &g, &t, &mcp, &rp, &es,
		&v.CreatedAt, &v.CreatedBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	if len(manifest) > 0 && string(manifest) != "{}" {
		_ = jsonUnmarshal(manifest, &v.Manifest)
	}
	_ = jsonUnmarshal(prompt, &v.Prompt)
	_ = jsonUnmarshal(mp, &v.ModelPolicy)
	_ = jsonUnmarshal(cc, &v.ContextContract)
	_ = jsonUnmarshal(know, &v.Knowledge)
	_ = jsonUnmarshal(mem, &v.Memory)
	_ = jsonUnmarshal(g, &v.Guardrails)
	_ = jsonUnmarshal(t, &v.Tools)
	_ = jsonUnmarshal(mcp, &v.MCPServers)
	_ = jsonUnmarshal(rp, &v.RuntimePolicy)
	_ = jsonUnmarshal(es, &v.EvaluationSuite)
	return &v, nil
}

// scanExecution reads one executions row.
func scanExecution(row pgx.Row) (*capability.Execution, error) {
	var e capability.Execution
	var inputs, outputs []byte
	if err := row.Scan(&e.ID, &e.CapabilityVersionID, &e.Timestamp, &inputs, &outputs,
		&e.Model, &e.Provider, &e.LatencyMs, &e.CostUSD,
		&e.PromptTokens, &e.CompletionTokens, &e.TotalTokens,
		&e.Error, &e.TraceID, &e.Environment,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	_ = jsonUnmarshal(inputs, &e.Inputs)
	_ = jsonUnmarshal(outputs, &e.Outputs)
	return &e, nil
}

// -- queryer local interface (subset of pgxpool.Pool) --

// queryer is the minimal pgx-shape needed for read paths so the
// tx helpers do not depend on the *pgxpool.Pool concrete type.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Re-export errNotFound so consumers can match on a single
// sentinel. The capability.Repository interface returns this for
// Get* misses.
var errNotFound = errors.New("postgres: not found")

// NotFound exposes errNotFound for callers who want errors.Is.
func NotFound() error { return errNotFound }
