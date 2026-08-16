package promptsheon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/approval"
	"github.com/sachncs/promptsheon/promptsheon/auth"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/executor"
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"github.com/sachncs/promptsheon/promptsheon/release"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// ---------------------------------------------------------------------------
// Release + Approval routes
// ---------------------------------------------------------------------------

func (s *Server) registerReleaseRoutes() {
	if s.releaseSvc == nil {
		return
	}
	s.mux.HandleFunc("GET /api/v1/capabilities/{capability_id}/releases", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListReleases)))
	s.mux.HandleFunc("POST /api/v1/versions/{version_id}/releases", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateRelease)))
	s.mux.HandleFunc("GET /api/v1/releases/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetRelease)))
	s.mux.HandleFunc("POST /api/v1/releases/{id}/votes", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleVoteOnRelease)))
	s.mux.HandleFunc("POST /api/v1/releases/{id}/activate", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleActivateRelease)))
	s.mux.HandleFunc("POST /api/v1/releases/{id}/rollback", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleRollbackRelease)))
	s.mux.HandleFunc("POST /api/v1/releases/{id}/invoke", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleInvokeRelease)))
	s.mux.HandleFunc("GET /api/v1/releases/{id}/approval", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetReleaseApproval)))
}

type createReleaseRequest struct {
	Environment string `json:"environment"`
}

// CreateRelease creates the release.
func (s *Server) handleCreateRelease(w http.ResponseWriter, r *http.Request) error {
	versionID := r.PathValue("version_id")
	v, err := s.db.GetVersion(r.Context(), versionID)
	if err != nil {
		return ErrNotFound
	}
	var req createReleaseRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	env := release.Environment(req.Environment)
	if !env.Valid() {
		return badRequest("environment: must be dev|staging|prod")
	}

	// Look up the parent Capability to compute capabilityVersion if not set.
	cap, err := s.db.GetCapability(r.Context(), v.CapabilityID)
	if err != nil {
		return ErrNotFound
	}

	rel, err := s.releaseSvc.Create(r.Context(), cap.ID, v.Version, v.Manifest, env, callerID(r))
	if err != nil {
		return badRequest(err.Error())
	}
	s.audit(r.Context(), "create", "release:"+rel.ID, map[string]any{
		"capability_id": cap.ID, "version_id": versionID, "environment": string(env),
	})
	writeJSON(w, http.StatusCreated, rel)
	return nil
}

// GetRelease returns the release.
func (s *Server) handleGetRelease(w http.ResponseWriter, r *http.Request) error {
	rel, err := s.releaseSvc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, rel)
	return nil
}

// ListReleases lists the releases.
func (s *Server) handleListReleases(w http.ResponseWriter, r *http.Request) error {
	rels, err := s.releaseSvc.ListForCapability(r.Context(), r.PathValue("capability_id"))
	if err != nil {
		return err
	}
	if rels == nil {
		rels = []*release.Release{}
	}
	writeJSON(w, http.StatusOK, rels)
	return nil
}

type voteRequest struct {
	Identity string `json:"identity"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// VoteOnRelease records a vote on the release.
func (s *Server) handleVoteOnRelease(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("id")
	var req voteRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	// SECURITY: vote identity is bound to the authenticated
	// principal. The previous implementation accepted an
	// arbitrary `identity` from the request body and defaulted to
	// the caller only when blank — which let the release creator
	// vote under another name to satisfy maker-checker quorum.
	// The authenticated principal is now the only allowed source.
	//
	// If delegated voting is ever required, expose it as a
	// separate admin-only operation that records both the
	// delegator and the represented identity in the audit log.
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user == nil || user.ID == "" {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "no authenticated user in context"}
	}
	if req.Identity != "" && req.Identity != user.ID {
		return &HTTPError{Status: http.StatusForbidden, Message: "vote identity must match the authenticated principal"}
	}
	req.Identity = user.ID
	decision := approval.Decision(req.Decision)
	switch decision {
	case approval.Approve, approval.Reject, approval.Abstain:
	default:
		return badRequest("decision: must be approve|reject|abstain")
	}
	vote := approval.Vote{
		Identity: req.Identity,
		Decision: decision,
		Reason:   req.Reason,
	}
	a, err := s.releaseSvc.Vote(r.Context(), releaseID, vote)
	if err != nil {
		return badRequest(err.Error())
	}
	s.audit(r.Context(), "vote", "release:"+releaseID, map[string]any{
		"identity": req.Identity, "decision": string(decision),
	})
	writeJSON(w, http.StatusOK, a)
	return nil
}

// ActivateRelease activates the release.
func (s *Server) handleActivateRelease(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("id")
	activated, err := s.releaseSvc.Activate(r.Context(), releaseID)
	if err != nil {
		if errors.Is(err, errs.ErrReleaseNotPending) {
			return &HTTPError{Status: http.StatusConflict, Message: err.Error()}
		}
		if errors.Is(err, errs.ErrSelfVote) || errors.Is(err, errs.ErrQuorum) {
			return &HTTPError{Status: http.StatusConflict, Message: err.Error()}
		}
		if errors.Is(err, errs.ErrApprovalNotFound) {
			return &HTTPError{Status: http.StatusConflict, Message: "no votes recorded; quorum not satisfied"}
		}
		if errors.Is(err, errs.ErrReleaseNotFound) {
			return ErrNotFound
		}
		if errors.Is(err, errs.ErrPrecondition) {
			var pe *harness.PreconditionError
			if errors.As(err, &pe) {
				return &HTTPError{
					Status:  http.StatusConflict,
					Message: pe.Error(),
					Details: map[string]any{"failures": pe.Failures},
				}
			}
			return &HTTPError{Status: http.StatusConflict, Message: err.Error()}
		}
		return badRequest(err.Error())
	}
	s.audit(r.Context(), "activate", "release:"+releaseID, nil)
	writeJSON(w, http.StatusOK, activated)
	return nil
}

// RollbackRelease rolls back the release.
func (s *Server) handleRollbackRelease(w http.ResponseWriter, r *http.Request) error {
	rolled, err := s.releaseSvc.Rollback(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, errs.ErrReleaseNotFound) {
			return ErrNotFound
		}
		return badRequest(err.Error())
	}
	s.audit(r.Context(), "rollback", "release:"+rolled.ID, nil)
	writeJSON(w, http.StatusOK, rolled)
	return nil
}

type invokeReleaseRequest struct {
	Inputs map[string]any `json:"inputs,omitempty"`
	// Model and Provider are intentionally NOT exposed on the
	// invoke-release request. The release runtime is the
	// authoritative source for both (via the Manifest's
	// ModelPolicy artifact). The previous design let the
	// caller pick either, which made the approval a fiction.
}

// InvokeRelease invokes the release.
func (s *Server) handleInvokeRelease(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("id")
	rel, err := s.releaseSvc.Get(r.Context(), releaseID)
	if err != nil {
		return ErrNotFound
	}
	if rel.Status != release.StatusActive {
		return &HTTPError{Status: http.StatusConflict, Message: "release is not active"}
	}

	// PR-6 (v0.4.0) Canary Release primitive: when this release is a
	// canary (CanaryPercent in [1, 99]), weighted-pick between it and
	// the stable counterpart. The stable is the most recently activated
	// release in the same (Capability, Environment) with CanaryPercent
	// == 0. When the stable counterpart does not exist (e.g. the canary
	// is the only active release), the canary receives 100% — the
	// weighted pick deterministically returns the canary.
	if rel.CanaryPercent > 0 {
		stable, stableErr := s.db.GetStableReleaseInEnv(r.Context(), rel.CapabilityID, string(rel.Environment), releaseID)
		if stableErr == nil && stable != nil {
			if pick := weightedPickCanaryTarget(rel, stable); pick != nil {
				rel = pick
			}
		}
	}
	var req invokeReleaseRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	mHash, err := manifestHashForRelease(rel)
	if err != nil {
		return errf.Errorf("manifest hash: %w", err)
	}
	plan, err := s.resolveRelease(r.Context(), rel)
	if err != nil {
		return &HTTPError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	// ponytail: previously the handler built
	//   CapabilityVersionID: rel.CapabilityID + "@" + version
	// which doesn't exist in the capability_versions table — the
	// FK constraint to capability_versions(id) blew up. Look up
	// the real version row by (capability_id, version number).
	ver, err := s.db.GetVersionByNumber(r.Context(), rel.CapabilityID, rel.CapabilityVersion)
	if err != nil {
		return errf.Errorf("lookup capability version: %w", err)
	}
	exec := &capability.Execution{
		ID:                  generateID(),
		CapabilityVersionID: ver.ID,
		Timestamp:           time.Now(),
		Inputs:              req.Inputs,
		Environment:         string(rel.Environment),
	}
	if plan != nil {
		exec.Model = plan.Model
		exec.Provider = plan.Provider
	}
	// Model and provider are now derived from the release, not
	// from the request. If the release has a Resolver wired, we
	// use the Resolver's plan; otherwise we use the placeholder
	// derived from the manifest hash, which surfaces as a 502
	// at the provider call site.
	result, invErr := s.invokeOneWithManifest(r, rel, req.Inputs, plan)
	rec := result.Record
	latency := result.Duration
	exec.LatencyMs = latency.Milliseconds()
	// BUG-21/30: even on a failed invoke, when the executor
	// returns a partial record (a record that has a token
	// count or a cost line before erroring), surface those
	// values on the execution and in the audit map. The
	// tokens_estimated flag tells audit consumers that the
	// numbers are real even if the call did not complete.
	if rec != nil {
		exec.PromptTokens = rec.PromptTokens
		exec.CompletionTokens = rec.OutputTokens
		exec.TotalTokens = rec.PromptTokens + rec.OutputTokens
		exec.Model = rec.Model
		exec.CostUSD = rec.CostUSD
		if len(rec.Output) > 0 {
			exec.Outputs = map[string]any{"content": string(rec.Output)}
		}
	}
	if invErr != nil {
		exec.Error = invErr.Error()
	}
	if err := s.db.CreateExecution(r.Context(), exec); err != nil {
		return err
	}
	s.audit(r.Context(), "invoke", "release:"+releaseID, map[string]any{
		"manifest_hash":    mHash,
		"tokens":           exec.TotalTokens,
		"cost_usd":         exec.CostUSD,
		"tokens_estimated": exec.TotalTokens > 0 || exec.CostUSD > 0,
		FieldError:         exec.Error,
	})
	if invErr != nil {
		return &HTTPError{Status: http.StatusBadGateway, Message: invErr.Error()}
	}
	writeJSON(w, http.StatusCreated, exec)
	return nil
}

// resolveRelease builds a ResolvedInvocation for a release. It is
// a no-op (returns nil plan) when no Resolver is configured; the
// invoke path then falls back to the placeholder model name.
func (s *Server) resolveRelease(ctx context.Context, rel *release.Release) (*release.ResolvedInvocation, error) {
	if s.releaseResolver == nil {
		return nil, nil
	}
	return s.releaseResolver.Resolve(ctx, rel.ID)
}

// invokeOneWithManifest is the release-side equivalent of invokeOne;
// it uses the Release's loaded Manifest to derive a stable manifest
// hash rather than the placeholder hash used by the existing
// /versions/{id}/executions route. Returns the ExecutionRecord (or nil
// when the invoker has nothing to record), the invocation error (or
// nil on success), and the wall-clock latency so the handler can
// populate the Execution row.
//
// Model and provider are taken from plan (the ResolvedInvocation),
// not from the HTTP request, so the request cannot override the
// approved release's runtime.
//
// Like invokeOne, requires s.invoker to be wired. A missing invoker
// returns an error rather than a silent no-op.
// invokeResult is the return tuple for invokeOneWithManifest.
// The struct is exported so callers can name the fields at the
// call site; the alternative (three return values, error in the
// middle) was flagged by staticcheck as ST1008.
type invokeResult struct {
	Record   *executor.ExecutionRecord
	Duration time.Duration
}

func (s *Server) invokeOneWithManifest(r *http.Request, rel *release.Release, inputs map[string]any, plan *release.ResolvedInvocation) (invokeResult, error) {
	if s.invoker == nil {
		return invokeResult{}, errors.New("api: invoke.Invoker not wired on this server")
	}
	input, err := json.Marshal(inputs)
	if err != nil {
		return invokeResult{}, err
	}
	mHash, err := capability.ComputeManifestHash(rel.Manifest)
	if err != nil {
		return invokeResult{}, err
	}
	model := ""
	provider := ""
	if plan != nil {
		model = plan.Model
		provider = plan.Provider
	}
	req := executor.InvokeRequest{
		WorkspaceID:   r.PathValue("workspace_id"),
		ReleaseID:     rel.ID,
		ManifestHash:  mHash,
		InputHash:     capability.InputHash(input),
		Input:         input,
		Model:         model,
		ModelRevision: capability.ModelRevision(model, provider),
		Provider:      provider,
	}
	// ponytail: same prompt-as-system fix as the harness path — the
	// live /releases/{id}/invoke route also dropped the manifest
	// prompt, so the model answered the user's raw input without
	// the system instruction.
	if plan != nil {
		req.SystemPrompt = plan.Prompt
	}
	start := time.Now()
	rec, err := s.invoker.Invoke(r.Context(), req)
	return invokeResult{Record: &rec, Duration: time.Since(start)}, err
}

// GetReleaseApproval returns the releaseApproval.
func (s *Server) handleGetReleaseApproval(w http.ResponseWriter, r *http.Request) error {
	a, err := s.releaseSvc.Approval(r.Context(), r.PathValue("id"))
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, a)
	return nil
}

func manifestHashForRelease(rel *release.Release) (string, error) {
	h, err := capability.ComputeManifestHash(rel.Manifest)
	if err != nil {
		// BUG-20: previously this returned "" and the caller
		// stored the empty hash in the audit row, silently
		// dropping tamper-evidence. Surface the error so the
		// handler returns 500 and the operator can see what
		// happened.
		slog.Error("manifest hash failed", "release_id", rel.ID, "err", err)
		return "", err
	}
	return h, nil
}

// weightedPickCanaryTarget is the canary routing primitive. Given a
// canary release and a stable release (both Active, same
// CapabilityID+Environment), it returns one of them based on
// canary.CanaryPercent. The pick uses math/rand so successive
// invocations are statistically independent. The traffic-routing
// decision does not require a CSPRNG — neither thread safety nor
// adversarial prediction are properties we need here.
//
// Edge cases:
//   - canary.CanaryPercent == 0 → stable (the caller treats 0 as
//     "no canary routing"). Defensive: if a 0 canary reaches here,
//     we still pick stable.
//   - canary.CanaryPercent >= 100 → canary.
//   - canary.CanaryPercent in [1, 99] → weighted pick.
func weightedPickCanaryTarget(canary, stable *release.Release) *release.Release {
	return weightedPickCanaryTargetWith(canary, stable, defaultCanaryRNG)
}

// canaryRNG abstracts the random number source used by the
// canary router. Tests substitute a deterministic source so the
// weighted pick is reproducible; production uses
// defaultCanaryRNG (math/rand) which is statistically uniform
// for the canary routing decision. P3.5 of the audit made this
// injectable so the routing decision is both reproducible in
// tests and not silently relying on a CSPRNG.
type canaryRNG interface {
	Intn(n int) int
}

// defaultCanaryRNG is the production source. It wraps the
// goroutine-safe math/rand top-level functions in a small
// interface so weightedPickCanaryTargetWith can be tested
// without exporting a private random handle.
type mathRand struct{}

// Intn returns the next int in [0, n) from the package-level
// math/rand source. The decision the canary router makes is a
// statistical A/B split, not a security decision: see the
// P3.5 audit note on weightedPickCanaryTargetWith. gosec
// flags every math/rand call as G404; this annotation covers
// the wrapper itself.
func (mathRand) Intn(n int) int { return rand.Intn(n) } // #nosec G404 -- A/B routing is statistical, not security-sensitive

var defaultCanaryRNG canaryRNG = mathRand{}

// weightedPickCanaryTargetWith is the testable core of the
// canary router. It takes a random source so the routing
// decision is reproducible; the canary/stable branch selection
// logic itself is unchanged.
//
// G404: math/rand is intentional here. The canary routing
// decision is a statistical A/B split, not a security
// decision: a CSPRNG would be a misuse of the API. The
// injectable canaryRNG interface lets tests substitute a
// deterministic source, which is the audit's P3.5
// recommendation (deterministic injectable selection for
// tests and production behaviour tests).
func weightedPickCanaryTargetWith(canary, stable *release.Release, rng canaryRNG) *release.Release { // #nosec G404 -- A/B routing is statistical, not security-sensitive
	if canary == nil {
		return stable
	}
	if stable == nil {
		return canary
	}
	pct := canary.CanaryPercent
	if pct <= 0 {
		return stable
	}
	if pct >= 100 {
		return canary
	}
	// rng.Intn(100) returns [0, 100). A value strictly less than
	// pct lands on the canary; the complement lands on the stable.
	if rng.Intn(100) < pct {
		return canary
	}
	return stable
}
