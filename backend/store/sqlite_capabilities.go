package store

import (
	"github.com/sachncs/promptsheon/backend/errs"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/backend/capability"
	"github.com/sachncs/promptsheon/backend/schedule"
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
		return nil, errs.ErrorStoreNotFound
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
		return nil, errs.ErrorStoreNotFound
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
			`SELECT id, project_id, name, description, created_at, updated_at,
			 self_evolve_enabled, self_evolve_min_score, self_evolve_max_revisions, self_evolve_cooldown_sec,
			 self_evolve_target_env, self_evolve_dataset_id
			 FROM capabilities WHERE id = ?`, id,
		)
	}
	return scanCapability(row)
}

func (s *SQLite) ListCapabilities(ctx context.Context, projectID string) ([]*capability.Capability, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, description, created_at, updated_at,
		 self_evolve_enabled, self_evolve_min_score, self_evolve_max_revisions, self_evolve_cooldown_sec,
		 self_evolve_target_env, self_evolve_dataset_id
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

// CatalogSearch returns Capabilities whose name matches the
// query within the supplied workspace. Empty query returns all
// Capabilities in the workspace. Pagination via limit (0 means
// no limit, capped at 1000).
func (s *SQLite) CatalogSearch(ctx context.Context, workspaceID, query string, limit int) ([]*capability.Capability, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.project_id, c.name, c.description, c.created_at, c.updated_at,
		 c.self_evolve_enabled, c.self_evolve_min_score, c.self_evolve_max_revisions, c.self_evolve_cooldown_sec,
		 c.self_evolve_target_env, c.self_evolve_dataset_id
		FROM capabilities c
		JOIN projects p ON c.project_id = p.id
		WHERE p.workspace_id = ?
		  AND (? = '' OR LOWER(c.name) LIKE '%' || LOWER(?) || '%')
		ORDER BY c.name
		LIMIT ?`, workspaceID, query, query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog search: %w", err)
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

// UpdateSelfEvolveConfig sets the closed-loop self-evolution
// policy on a Capability. Idempotent: same value on repeat is a
// no-op. Returns errs.ErrorStoreNotFound if the capability does not exist.
func (s *SQLite) UpdateSelfEvolveConfig(ctx context.Context, capabilityID string, cfg capability.SelfEvolveConfig) error {
	dataset := cfg.DatasetID
	if dataset == "" {
		dataset = ""
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE capabilities
		   SET self_evolve_enabled = ?,
		       self_evolve_min_score = ?,
		       self_evolve_max_revisions = ?,
		       self_evolve_cooldown_sec = ?,
		       self_evolve_target_env = ?,
		       self_evolve_dataset_id = ?,
		       updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		boolToInt(cfg.Enabled), cfg.MinScore, cfg.MaxRevisions, cfg.CooldownSec,
		cfg.TargetEnv, dataset, capabilityID,
	)
	if err != nil {
		return fmt.Errorf("update self-evolve config: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return errs.ErrorStoreNotFound
	}
	return nil
}

// SelfEvolveState is the persisted cycle state for a
// (capability, target_env) pair. Stored in the self_evolve_state
// table. The evolver reads/writes one row per cycle to track
// cooldown across daemon restarts.
type SelfEvolveState struct {
	CapabilityID    string
	TargetEnv       string
	LastAttemptAt   *time.Time
	LastPromoteAt   *time.Time
	LastScore       float64
	LastRevisionIdx int
	CycleStartedAt  *time.Time
	LastStatus      string
	LastError       string
	RevisionIndex   int
}

func (s *SQLite) LoadSelfEvolveState(ctx context.Context, capabilityID, targetEnv string) (*SelfEvolveState, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT capability_id, target_env, last_attempt_at, last_promote_at, last_score,
		       last_revision_index, cycle_started_at, last_status, last_error
		  FROM self_evolve_state
		 WHERE capability_id = ? AND target_env = ?`, capabilityID, targetEnv)
	var st SelfEvolveState
	var lastAttempt, lastPromote, cycleStart sql.NullTime
	if err := row.Scan(&st.CapabilityID, &st.TargetEnv, &lastAttempt, &lastPromote, &st.LastScore,
		&st.LastRevisionIdx, &cycleStart, &st.LastStatus, &st.LastError); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load self_evolve_state: %w", err)
	}
	if lastAttempt.Valid {
		t := lastAttempt.Time
		st.LastAttemptAt = &t
	}
	if lastPromote.Valid {
		t := lastPromote.Time
		st.LastPromoteAt = &t
	}
	if cycleStart.Valid {
		t := cycleStart.Time
		st.CycleStartedAt = &t
	}
	st.RevisionIndex = st.LastRevisionIdx
	return &st, nil
}

// SaveSelfEvolveState upserts the cycle state row. The caller
// passes a fully-populated state; the method stamps
// last_attempt_at = now when it is nil.
func (s *SQLite) SaveSelfEvolveState(ctx context.Context, st *SelfEvolveState) error {
	if st == nil {
		return fmt.Errorf("save self_evolve_state: nil state")
	}
	if st.LastAttemptAt == nil {
		now := time.Now().UTC()
		st.LastAttemptAt = &now
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO self_evolve_state
		  (capability_id, target_env, last_attempt_at, last_promote_at, last_score,
		   last_revision_index, cycle_started_at, last_status, last_error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (capability_id, target_env) DO UPDATE SET
		  last_attempt_at = excluded.last_attempt_at,
		  last_promote_at = excluded.last_promote_at,
		  last_score = excluded.last_score,
		  last_revision_index = excluded.last_revision_index,
		  cycle_started_at = excluded.cycle_started_at,
		  last_status = excluded.last_status,
		  last_error = excluded.last_error`,
		st.CapabilityID, st.TargetEnv,
		nullableTime(st.LastAttemptAt), nullableTime(st.LastPromoteAt),
		st.LastScore, st.LastRevisionIdx, nullableTime(st.CycleStartedAt),
		st.LastStatus, st.LastError,
	)
	if err != nil {
		return fmt.Errorf("save self_evolve_state: %w", err)
	}
	return nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// SetCapabilityContract upserts the contract attached to a
// Capability. Pass nil to clear. Returns
// capability.ErrCapabilityNotFound (re-exported via the
// repository surface) when the capability id does not exist.
func (s *SQLite) SetCapabilityContract(ctx context.Context, capabilityID string, c *capability.CapabilityContract) error {
	if c == nil {
		if _, err := s.db.ExecContext(ctx,
			`DELETE FROM capability_contracts WHERE capability_id = ?`, capabilityID,
		); err != nil {
			return fmt.Errorf("clear contract: %w", err)
		}
		return nil
	}
	inSchema, _ := json.Marshal(c.InputSchema)
	outSchema, _ := json.Marshal(c.OutputSchema)
	auto := 0
	if c.AutoPromotable {
		auto = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO capability_contracts
		    (capability_id, blast_radius, success_rubric, auto_promotable,
		     input_schema, output_schema, slo_max_p95_ms, slo_min_success,
		     slo_max_hallu, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(capability_id) DO UPDATE SET
		    blast_radius=excluded.blast_radius,
		    success_rubric=excluded.success_rubric,
		    auto_promotable=excluded.auto_promotable,
		    input_schema=excluded.input_schema,
		    output_schema=excluded.output_schema,
		    slo_max_p95_ms=excluded.slo_max_p95_ms,
		    slo_min_success=excluded.slo_min_success,
		    slo_max_hallu=excluded.slo_max_hallu,
		    updated_at=CURRENT_TIMESTAMP
	`, capabilityID, string(c.BlastRadius), c.SuccessRubric, auto,
		string(inSchema), string(outSchema),
		c.SLOTarget.MaxP95LatencyMS, c.SLOTarget.MinSuccessRate,
		c.SLOTarget.MaxHallucinationRate)
	if err != nil {
		return fmt.Errorf("upsert contract: %w", err)
	}
	return nil
}

// GetCapabilityContract returns the contract attached to a
// Capability. Returns errs.ErrorStoreNotFound when no contract is attached
// or the capability id does not exist.
func (s *SQLite) GetCapabilityContract(ctx context.Context, capabilityID string) (*capability.CapabilityContract, error) {
	var (
		blast, rubric, inJSON, outJSON string
		auto                           int
		maxP95                         int
		minSuccess, maxHallu           float64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT blast_radius, success_rubric, auto_promotable,
		       input_schema, output_schema,
		       slo_max_p95_ms, slo_min_success, slo_max_hallu
		FROM capability_contracts WHERE capability_id = ?`, capabilityID,
	).Scan(&blast, &rubric, &auto, &inJSON, &outJSON, &maxP95, &minSuccess, &maxHallu)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrorStoreNotFound
		}
		return nil, fmt.Errorf("get contract: %w", err)
	}
	c := &capability.CapabilityContract{
		BlastRadius:    capability.BlastRadius(blast),
		SuccessRubric:  rubric,
		AutoPromotable: auto == 1,
		SLOTarget: capability.SLOTarget{
			MaxP95LatencyMS:      maxP95,
			MinSuccessRate:       minSuccess,
			MaxHallucinationRate: maxHallu,
		},
	}
	if inJSON != "" && inJSON != "{}" {
		_ = json.Unmarshal([]byte(inJSON), &c.InputSchema)
	}
	if outJSON != "" && outJSON != "{}" {
		_ = json.Unmarshal([]byte(outJSON), &c.OutputSchema)
	}
	return c, nil
}

// GetCapabilityReputation derives a trust score for the
// Capability from the executions and eval_results tables.
// Returns a zero-valued Reputation when the Capability has no
// history.
func (s *SQLite) GetCapabilityReputation(ctx context.Context, capabilityID string) (capability.Reputation, error) {
	r := capability.Reputation{CapabilityID: capabilityID}

	var evalsTotal, evalsPassed int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(e.id), COALESCE(SUM(CASE WHEN e.status = 'passed' THEN 1 ELSE 0 END), 0)
		FROM eval_results e
		JOIN eval_runs r ON e.run_id = r.id
		JOIN capability_versions v ON r.release_id IN (
		    SELECT id FROM releases WHERE capability_id = ?
		)
		WHERE v.capability_id = ?
	`, capabilityID, capabilityID).Scan(&evalsTotal, &evalsPassed)
	if err == nil && evalsTotal > 0 {
		r.EvalPassRate = float64(evalsPassed) / float64(evalsTotal)
		r.SampleSize = evalsTotal
	}

	var execTotal, execOK int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(id), COALESCE(SUM(CASE WHEN status = 'ok' THEN 1 ELSE 0 END), 0)
		FROM executions e
		JOIN capability_versions v ON e.capability_version_id = v.id
		WHERE v.capability_id = ?
	`, capabilityID).Scan(&execTotal, &execOK)
	if err == nil && execTotal > 0 {
		r.SLOAdherenceRate = float64(execOK) / float64(execTotal)
		r.SampleSize += execTotal
	}

	var decTotal, decAdopted int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT recommendation_id), COALESCE(SUM(CASE WHEN outcome = 'adopted' THEN 1 ELSE 0 END), 0)
		FROM decisions d
		JOIN recommendations rec ON d.recommendation_id = rec.id
		JOIN capability_versions v ON rec.capability_version_id = v.id
		WHERE v.capability_id = ?
	`, capabilityID).Scan(&decTotal, &decAdopted)
	if err == nil && decTotal > 0 {
		r.DecisionAdoptionRate = float64(decAdopted) / float64(decTotal)
		r.SampleSize += decTotal
	}

	// Trust score is the product of the three rates, weighted
	// by sample size. A Capability with zero history gets 0;
	// the operator should interpret that as "unknown", not
	// "untrusted".
	if r.SampleSize == 0 {
		r.TrustScore = 0
	} else {
		r.TrustScore = r.EvalPassRate * r.SLOAdherenceRate * r.DecisionAdoptionRate
	}
	return r, nil
}

func scanCapability(scanner interface {
	Scan(dest ...any) error
}) (*capability.Capability, error) {
	var c capability.Capability
	var seEnabled int
	var seMinScore float64
	var seMaxRevisions int
	var seCooldownSec int
	var seTargetEnv string
	var seDatasetID sql.NullString
	err := scanner.Scan(
		&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
		&seEnabled, &seMinScore, &seMaxRevisions, &seCooldownSec, &seTargetEnv, &seDatasetID,
	)
	if err == sql.ErrNoRows {
		return nil, errs.ErrorStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan capability: %w", err)
	}
	c.SelfEvolve = capability.SelfEvolveConfig{
		Enabled:      seEnabled != 0,
		MinScore:     seMinScore,
		MaxRevisions: seMaxRevisions,
		CooldownSec:  seCooldownSec,
		TargetEnv:    seTargetEnv,
		DatasetID:    seDatasetID.String,
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

// GetVersionByNumber returns the Version whose integer
// `version` column matches the supplied counter for the
// Capability. Used by the diff endpoint.
func (s *SQLite) GetVersionByNumber(ctx context.Context, capabilityID string, version int) (*capability.Version, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? AND version = ?`, capabilityID, version,
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
		return nil, errs.ErrorStoreNotFound
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
		return nil, errs.ErrorStoreNotFound
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
//
// Webhook and manual schedules have no NextFireAt semantics —
// they are driven by external events, not by the cron tick.
// Returning them from the tick list would cause the scheduler to
// fire them on every tick (NextFireAt defaults to the epoch and
// never advances for non-cron kinds), creating runaway execution
// traffic. We filter them out here; webhook fires go through the
// dedicated HTTP handler and manual fires go through the CLI.
func (s *SQLite) ListDueSchedules(ctx context.Context, now time.Time, limit int) ([]*schedule.Schedule, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, release_id, kind, cron, webhook_path, next_fire_at, last_fire_at, fired_count, enabled, created_at, created_by
		 FROM schedules WHERE enabled = 1 AND kind = 'cron' AND next_fire_at <= ? ORDER BY next_fire_at ASC LIMIT ?`,
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

// ClaimDueSchedule atomically transitions a schedule from
// "due" to "in-flight" by advancing next_fire_at to the next
// computed fire time and stamping last_fire_at. Returns false
// (and nil error) when the row is no longer due — the caller
// raced with another scheduler and should skip publication.
//
// The previous code path used ListDueSchedules followed by a
// separate UpdateSchedule, allowing two schedulers to both
// publish the same schedule event. ClaimDueSchedule makes the
// read+write atomic.
func (s *SQLite) ClaimDueSchedule(ctx context.Context, sc *schedule.Schedule, newNextFireAt time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE schedules SET next_fire_at = ?, last_fire_at = ?, fired_count = fired_count + 1, enabled = ?
		 WHERE id = ? AND next_fire_at = ?`,
		newNextFireAt, sc.LastFireAt, sc.Enabled, sc.ID, sc.NextFireAt,
	)
	if err != nil {
		return false, fmt.Errorf("claim due schedule: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim due schedule rows: %w", err)
	}
	return n == 1, nil
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

// BulkUpdateSchedules persists a batch of schedule updates in a
// single transaction. PERF-SCH-1: TickOnce uses this instead of
// looping per-row UPDATE, dropping the round-trip count from N
// to 1.
func (s *SQLite) BulkUpdateSchedules(ctx context.Context, scs []*schedule.Schedule) error {
	if len(scs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bulk update tx: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`UPDATE schedules SET next_fire_at = ?, last_fire_at = ?, fired_count = ?, enabled = ?
		 WHERE id = ?`,
	)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare bulk update: %w", err)
	}
	defer stmt.Close()
	for _, sc := range scs {
		if _, err := stmt.ExecContext(ctx, sc.NextFireAt, sc.LastFireAt, sc.FiredCount, sc.Enabled, sc.ID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("bulk update %s: %w", sc.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bulk update: %w", err)
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
		return nil, errs.ErrorStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan schedule: %w", err)
	}
	sc.Kind = schedule.Kind(kindStr)
	return &sc, nil
}
