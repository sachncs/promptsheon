package selfevolve

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/store"
)

// Evolver is the closed-loop self-evolution orchestrator.
// It reads the latest EvalRun for a capability's active
// release; if the score is below the configured
// threshold, it runs the revise → validate → promote
// loop, capped by SelfEvolveConfig.MaxRevisions and
// guarded by SelfEvolveConfig.CooldownSec.
type Evolver struct {
	Repo      Repository
	Loader    PromptLoader
	Revision  RevisionLLM
	Validator Validator
	Promoter  *Promoter
	Auditor   Auditor
	Logger    *slog.Logger
	Now       func() time.Time
}

// NewEvolver wires the orchestrator. nil logger falls
// back to slog.Default(); nil Now falls back to UTC.
func NewEvolver(
	repo Repository,
	loader PromptLoader,
	revision RevisionLLM,
	validator Validator,
	promoter *Promoter,
	auditor Auditor,
	logger *slog.Logger,
) *Evolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &Evolver{
		Repo:      repo,
		Loader:    loader,
		Revision:  revision,
		Validator: validator,
		Promoter:  promoter,
		Auditor:   auditor,
		Logger:    logger,
		Now:       func() time.Time { return time.Now().UTC() },
	}
}

// RunOnce is one orchestrator tick. It returns
// (Result, error). A non-nil error is a hard failure
// (DB error, missing config, etc.); a non-nil Result
// records what actually happened, including Skipped
// when the cooldown is active or the score is already
// above threshold.
//
// A cycle, when started, runs up to MaxRevisions
// attempts. Each attempt: revise via the LLM,
// validate the candidate against the dataset, and
// promote on validation success. On MaxRevisions
// without a promote the cycle is rejected and the
// cooldown is stamped.
func (e *Evolver) RunOnce(ctx context.Context, capabilityID string) (*Result, error) {
	if capabilityID == "" {
		return nil, fmt.Errorf("selfevolve: empty capabilityID")
	}
	start := e.Now()
	res := &Result{CapabilityID: capabilityID}

	cap, err := e.Repo.GetCapability(ctx, capabilityID)
	if err != nil {
		return res, fmt.Errorf("selfevolve: get capability: %w", err)
	}
	cfg := cap.SelfEvolve
	if !cfg.Enabled {
		res.Skipped = true
		return res, nil
	}
	env := cfg.TargetEnv
	if env == "" {
		env = "dev"
	}
	if cfg.DatasetID == "" {
		return res, fmt.Errorf("selfevolve: capability %s has SelfEvolveConfig.DatasetID empty", capabilityID)
	}

	state, err := e.Repo.LoadSelfEvolveState(ctx, capabilityID, env)
	if err != nil {
		return res, fmt.Errorf("selfevolve: load state: %w", err)
	}
	if state == nil {
		state = &store.SelfEvolveState{
			CapabilityID: capabilityID,
			TargetEnv:    env,
			LastStatus:   StatusIdle,
		}
	}

	cooldown := time.Duration(cfg.CooldownSec) * time.Second
	if state.LastPromoteAt != nil && e.Now().Sub(*state.LastPromoteAt) < cooldown {
		e.Logger.Debug("selfevolve: cooldown active", "capability_id", capabilityID, "remaining", cooldown-e.Now().Sub(*state.LastPromoteAt))
		res.Skipped = true
		return res, nil
	}

	activeReleaseID, err := e.Repo.ActiveReleaseID(ctx, capabilityID, env)
	if err != nil {
		return res, fmt.Errorf("selfevolve: active release: %w", err)
	}
	if activeReleaseID == "" {
		e.Logger.Debug("selfevolve: no active release", "capability_id", capabilityID, "env", env)
		res.Skipped = true
		res.RejectReason = "no active release"
		return res, nil
	}

	lastRun, err := e.Repo.LastEvalRun(ctx, activeReleaseID)
	if err != nil {
		return res, fmt.Errorf("selfevolve: last eval run: %w", err)
	}
	if lastRun == nil {
		res.Skipped = true
		res.RejectReason = "no eval run yet"
		return res, nil
	}
	res.Score = lastRun.Score
	if lastRun.Score >= cfg.MinScore {
		res.Skipped = true
		return res, nil
	}

	if err := e.beginCycle(ctx, state, capabilityID, env, activeReleaseID, lastRun, cfg); err != nil {
		return res, err
	}

	failing, currentPrompt, modelPolicyHash, err := e.collectFailing(ctx, capabilityID, activeReleaseID, lastRun, state)
	if err != nil {
		return res, err
	}
	if len(failing) == 0 {
		e.Logger.Info("selfevolve: no failing cases to revise", "capability_id", capabilityID)
		res.Skipped = true
		state.LastStatus = StatusIdle
		_ = e.Repo.SaveSelfEvolveState(ctx, state)
		return res, nil
	}

	promoted, revisions := e.runRevisions(ctx, capabilityID, env, activeReleaseID, currentPrompt, modelPolicyHash, failing, state, cfg)
	res.Revisions = revisions
	if promoted {
		res.Promoted = true
		res.Score = state.LastScore
	} else {
		res.RejectReason = state.LastError
	}
	res.DurationMS = e.Now().Sub(start).Milliseconds()
	return res, nil
}

// beginCycle marks state as detecting and writes the
// detect audit row. Called once a tick when the
// conditions for a cycle are met (enabled, score below
// threshold, cooldown elapsed, active release with a
// run).
func (e *Evolver) beginCycle(ctx context.Context, state *store.SelfEvolveState, capabilityID, env, activeReleaseID string, lastRun *harness.EvalRun, cfg capability.SelfEvolveConfig) error {
	state.LastStatus = StatusDetected
	state.LastError = ""
	now := e.Now()
	state.CycleStartedAt = &now
	state.LastAttemptAt = &now
	state.LastRevisionIdx = 0
	state.RevisionIndex = 0
	if err := e.Repo.SaveSelfEvolveState(ctx, state); err != nil {
		return fmt.Errorf("selfevolve: save state: %w", err)
	}
	if e.Auditor != nil {
		e.Auditor.Audit(ctx, AuditDetect, "capability:"+capabilityID, map[string]any{
			"target_env":        env,
			"last_score":        lastRun.Score,
			"threshold":         cfg.MinScore,
			"dataset_id":        cfg.DatasetID,
			"active_release_id": activeReleaseID,
			"last_eval_run_id":  lastRun.ID,
		})
	}
	e.Logger.Info("selfevolve: cycle starting", "capability_id", capabilityID, "score", lastRun.Score, "threshold", cfg.MinScore)
	return nil
}

// collectFailing reads the failing cases for the run
// and the active release's current prompt + model
// policy hash. Returns (failing, currentPrompt,
// modelPolicyHash, err). On error the cycle state is
// stamped rejected.
func (e *Evolver) collectFailing(ctx context.Context, capabilityID, activeReleaseID string, lastRun *harness.EvalRun, state *store.SelfEvolveState) ([]FailingCase, []byte, string, error) {
	failing, err := e.failingCases(ctx, lastRun)
	if err != nil {
		state.LastError = err.Error()
		state.LastStatus = StatusRejected
		_ = e.Repo.SaveSelfEvolveState(ctx, state)
		return nil, nil, "", fmt.Errorf("selfevolve: collect failing cases: %w", err)
	}
	activeRel, err := e.Repo.GetRelease(ctx, activeReleaseID)
	if err != nil || activeRel == nil {
		state.LastError = "cannot load active release"
		state.LastStatus = StatusRejected
		_ = e.Repo.SaveSelfEvolveState(ctx, state)
		return nil, nil, "", fmt.Errorf("selfevolve: load active release: %w", err)
	}
	activeVer, err := e.Repo.GetVersionByNumber(ctx, capabilityID, activeRel.CapabilityVersion)
	if err != nil || activeVer == nil {
		state.LastError = "cannot load active version"
		state.LastStatus = StatusRejected
		_ = e.Repo.SaveSelfEvolveState(ctx, state)
		return nil, nil, "", fmt.Errorf("selfevolve: load active version: %w", err)
	}
	currentPromptBytes, err := e.Loader.LoadPrompt(ctx, activeVer.Manifest.Prompt.Hash)
	if err != nil {
		state.LastError = "cannot load current prompt: " + err.Error()
		state.LastStatus = StatusRejected
		_ = e.Repo.SaveSelfEvolveState(ctx, state)
		return nil, nil, "", fmt.Errorf("selfevolve: load current prompt: %w", err)
	}
	return failing, currentPromptBytes, activeVer.Manifest.ModelPolicy.Hash, nil
}

// runRevisions is the revise → validate → promote
// loop. Returns (promoted, revisions). On MaxRevisions
// without a promote the cycle state is marked rejected
// and a self_evolve.reject audit row is written.
func (e *Evolver) runRevisions(ctx context.Context, capabilityID, env, activeReleaseID string, currentPrompt []byte, modelPolicyHash string, failing []FailingCase, state *store.SelfEvolveState, cfg capability.SelfEvolveConfig) (bool, int) {
	for i := 0; i < cfg.MaxRevisions; i++ {
		state.LastStatus = StatusRevising
		state.LastRevisionIdx = i + 1
		_ = e.Repo.SaveSelfEvolveState(ctx, state)

		reviseResp, err := e.Revision.Revise(ctx, ReviseRequest{
			CapabilityID:    capabilityID,
			CurrentPrompt:   string(currentPrompt),
			CurrentHash:     "", // the current prompt hash is in the active version; the revision strategy reads it from the LLM's view
			ModelPolicyHash: modelPolicyHash,
			FailingCases:    failing,
		})
		if err != nil {
			state.LastError = "revise: " + err.Error()
			_ = e.Repo.SaveSelfEvolveState(ctx, state)
			e.Logger.Warn("selfevolve: revise failed", "capability_id", capabilityID, "iteration", i+1, "err", err)
			continue
		}
		if e.Auditor != nil {
			e.Auditor.Audit(ctx, AuditRevise, "capability:"+capabilityID, map[string]any{
				"iteration":        i + 1,
				"new_prompt_chars": len(reviseResp.NewPrompt),
				"rationale":        reviseResp.Rationale,
			})
		}

		state.LastStatus = StatusValidating
		_ = e.Repo.SaveSelfEvolveState(ctx, state)
		run, vErr := e.Validator.Validate(ctx, capabilityID, []byte(reviseResp.NewPrompt), cfg.DatasetID)
		if vErr != nil {
			state.LastError = "validate: " + vErr.Error()
			_ = e.Repo.SaveSelfEvolveState(ctx, state)
			e.Logger.Warn("selfevolve: validate failed", "capability_id", capabilityID, "iteration", i+1, "err", vErr)
			continue
		}
		if e.Auditor != nil {
			e.Auditor.Audit(ctx, AuditValidate, "capability:"+capabilityID, map[string]any{
				"iteration":   i + 1,
				"score":       run.Score,
				"passed":      run.Passed,
				"failed":      run.Failed,
				"total":       run.Total,
				"eval_run_id": run.ID,
			})
		}
		if run.Score < cfg.MinScore {
			e.Logger.Info("selfevolve: validate below threshold", "capability_id", capabilityID, "iteration", i+1, "score", run.Score, "threshold", cfg.MinScore)
			continue
		}

		promRes, pErr := e.Promoter.Promote(ctx, capabilityID, env, activeReleaseID, reviseResp.NewPrompt)
		if pErr != nil {
			state.LastError = "promote: " + pErr.Error()
			_ = e.Repo.SaveSelfEvolveState(ctx, state)
			e.Logger.Warn("selfevolve: promote failed", "capability_id", capabilityID, "iteration", i+1, "err", pErr)
			continue
		}

		now := e.Now()
		state.LastPromoteAt = &now
		state.LastScore = run.Score
		state.LastStatus = StatusPromoted
		state.LastError = ""
		state.CycleStartedAt = nil
		_ = e.Repo.SaveSelfEvolveState(ctx, state)
		e.Logger.Info("selfevolve: promoted", "capability_id", capabilityID, "iteration", i+1, "score", run.Score, "new_release_id", promRes.NewReleaseID)
		return true, i + 1
	}
	state.LastStatus = StatusRejected
	now := e.Now()
	state.LastPromoteAt = &now
	state.LastError = state.LastError + ";max revisions reached"
	_ = e.Repo.SaveSelfEvolveState(ctx, state)
	if e.Auditor != nil {
		e.Auditor.Audit(ctx, AuditReject, "capability:"+capabilityID, map[string]any{
			"max_revisions": cfg.MaxRevisions,
			"reason":        state.LastError,
		})
	}
	e.Logger.Warn("selfevolve: cycle rejected", "capability_id", capabilityID, "reason", state.LastError)
	return false, cfg.MaxRevisions
}

// failingCases reads the per-case results for the run,
// joins them with the dataset, and returns the failing
// (input, expected, actual) tuples sorted by seq. Cases
// without a matching dataset row are dropped (defensive —
// the runner should always insert with the dataset's
// seq).
func (e *Evolver) failingCases(ctx context.Context, run *harness.EvalRun) ([]FailingCase, error) {
	results, err := e.Repo.ListEvalResultsForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	cases, err := e.Repo.ListDatasetCases(ctx, run.DatasetID)
	if err != nil {
		return nil, fmt.Errorf("selfevolve: list cases: %w", err)
	}
	bySeq := make(map[int]harness.DatasetCase, len(cases))
	for _, c := range cases {
		bySeq[c.Seq] = c
	}
	var failing []FailingCase
	for _, r := range results {
		if r.Passed {
			continue
		}
		dc, ok := bySeq[r.Seq]
		if !ok {
			continue
		}
		var inputs map[string]any
		if len(dc.Inputs) > 0 {
			if uerr := json.Unmarshal(dc.Inputs, &inputs); uerr != nil {
				// Keep the case with empty inputs
				// rather than drop it.
				inputs = nil
			}
		}
		var expected string
		if len(dc.Expected) > 0 {
			expected = string(dc.Expected)
		}
		failing = append(failing, FailingCase{
			Seq:      r.Seq,
			Inputs:   inputs,
			Expected: expected,
			Actual:   string(r.Actual),
			Reason:   r.Error,
		})
	}
	sort.Slice(failing, func(i, j int) bool { return failing[i].Seq < failing[j].Seq })
	return failing, nil
}
