package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/capability"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// SQLite persistence for capabilities.

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

func (s *SQLite) CreateCapability(ctx context.Context, c *capability.Capability) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Description, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return errf.Errorf("insert capability: %w", err)
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
		return nil, errf.Errorf("list capabilities: %w", err)
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
		return nil, errf.Errorf("catalog search: %w", err)
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
		return errf.Errorf("update capability: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteCapability(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM capabilities WHERE id = ?`, id)
	if err != nil {
		return errf.Errorf("delete capability: %w", err)
	}
	return nil
}

// UpdateSelfEvolveConfig sets the closed-loop self-evolution
// policy on a Capability. Idempotent: same value on repeat is a
// no-op. Returns errs.ErrStoreNotFound if the capability does not exist.

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
		return errf.Errorf("update self-evolve config: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return errf.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return errs.ErrStoreNotFound
	}
	return nil
}

// SelfEvolveState is the persisted cycle state for a
// (capability, target_env) pair. Stored in the self_evolve_state
// table. The evolver reads/writes one row per cycle to track
// cooldown across daemon restarts.

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
		return nil, errf.Errorf("load self_evolve_state: %w", err)
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
		return errf.Errorf("save self_evolve_state: nil state")
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
		return errf.Errorf("save self_evolve_state: %w", err)
	}
	return nil
}

func (s *SQLite) SetCapabilityContract(ctx context.Context, capabilityID string, c *capability.CapabilityContract) error {
	if c == nil {
		if _, err := s.db.ExecContext(ctx,
			`DELETE FROM capability_contracts WHERE capability_id = ?`, capabilityID,
		); err != nil {
			return errf.Errorf("clear contract: %w", err)
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
		return errf.Errorf("upsert contract: %w", err)
	}
	return nil
}

// GetCapabilityContract returns the contract attached to a
// Capability. Returns errs.ErrStoreNotFound when no contract is attached
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
			return nil, errs.ErrStoreNotFound
		}
		return nil, errf.Errorf("get contract: %w", err)
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
	// P4.1: surface corrupt JSON to the caller as a
	// contextual error rather than silently returning a
	// capability with empty schemas. The capability is the
	// surface the harness and SDK see; an empty schema is a
	// silent contract drift. The CapabilityContract struct
	// does not carry the capability id (it is the row's
	// primary key in the join), so we use the function
	// argument capabilityID for the error message.
	if inJSON != "" && inJSON != "{}" {
		if err := json.Unmarshal([]byte(inJSON), &c.InputSchema); err != nil {
			return nil, errf.Errorf("capability contract %s: decode input schema: %w", capabilityID, err)
		}
	}
	if outJSON != "" && outJSON != "{}" {
		if err := json.Unmarshal([]byte(outJSON), &c.OutputSchema); err != nil {
			return nil, errf.Errorf("capability contract %s: decode output schema: %w", capabilityID, err)
		}
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
		return nil, errs.ErrStoreNotFound
	}
	if err != nil {
		return nil, errf.Errorf("scan capability: %w", err)
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
