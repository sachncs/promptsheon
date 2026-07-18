package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/release"
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

func (s *Server) handleGetRelease(w http.ResponseWriter, r *http.Request) error {
	rel, err := s.releaseSvc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, rel)
	return nil
}

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

func (s *Server) handleVoteOnRelease(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("id")
	var req voteRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Identity == "" {
		// Default to the authenticated caller when no explicit identity
		// is supplied. This matches the user/team mental model used
		// elsewhere in the API.
		req.Identity = callerID(r)
	}
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

func (s *Server) handleActivateRelease(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("id")
	activated, err := s.releaseSvc.Activate(r.Context(), releaseID)
	if err != nil {
		if errors.Is(err, release.ErrNotPending) {
			return &HTTPError{Status: http.StatusConflict, Message: err.Error()}
		}
		if errors.Is(err, approval.ErrCreatorVoted) || errors.Is(err, approval.ErrQuorumNotSatisfied) {
			return &HTTPError{Status: http.StatusConflict, Message: err.Error()}
		}
		if errors.Is(err, approval.ErrNotFound) {
			return &HTTPError{Status: http.StatusConflict, Message: "no votes recorded; quorum not satisfied"}
		}
		if errors.Is(err, release.ErrNotFound) {
			return ErrNotFound
		}
		return badRequest(err.Error())
	}
	s.audit(r.Context(), "activate", "release:"+releaseID, nil)
	writeJSON(w, http.StatusOK, activated)
	return nil
}

func (s *Server) handleRollbackRelease(w http.ResponseWriter, r *http.Request) error {
	rolled, err := s.releaseSvc.Rollback(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, release.ErrNotFound) {
			return ErrNotFound
		}
		return badRequest(err.Error())
	}
	s.audit(r.Context(), "rollback", "release:"+rolled.ID, nil)
	writeJSON(w, http.StatusOK, rolled)
	return nil
}

type invokeReleaseRequest struct {
	Inputs   map[string]any `json:"inputs,omitempty"`
	Model    string         `json:"model"`
	Provider string         `json:"provider"`
}

func (s *Server) handleInvokeRelease(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("id")
	rel, err := s.releaseSvc.Get(r.Context(), releaseID)
	if err != nil {
		return ErrNotFound
	}
	if rel.Status != release.StatusActive {
		return &HTTPError{Status: http.StatusConflict, Message: "release is not active"}
	}
	var req invokeReleaseRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	manifestHash := manifestHashForRelease(rel)
	exec := &capability.Execution{
		ID:                  generateID(),
		CapabilityVersionID: rel.CapabilityID + "@" + fmt.Sprintf("%d", rel.CapabilityVersion),
		Timestamp:           time.Now(),
		Inputs:              req.Inputs,
		Model:               req.Model,
		Provider:            req.Provider,
		Environment:         string(rel.Environment),
	}
	rec, invErr, latency := s.invokeOneWithManifest(r, rel, req.Inputs, req.Model, req.Provider)
	exec.LatencyMs = latency.Milliseconds()
	if invErr != nil {
		exec.Error = invErr.Error()
	} else if rec != nil {
		if len(rec.Output) > 0 {
			exec.Outputs = map[string]any{"content": string(rec.Output)}
		}
		exec.PromptTokens = rec.PromptTokens
		exec.CompletionTokens = rec.OutputTokens
		exec.TotalTokens = rec.PromptTokens + rec.OutputTokens
		exec.Model = rec.Model
		exec.CostUSD = rec.CostUSD
	}
	if err := s.db.CreateExecution(r.Context(), exec); err != nil {
		return err
	}
	s.audit(r.Context(), "invoke", "release:"+releaseID, map[string]any{
		"manifest_hash": manifestHash,
		"tokens":        exec.TotalTokens,
		"cost_usd":      exec.CostUSD,
	})
	writeJSON(w, http.StatusCreated, exec)
	return nil
}

// invokeOneWithManifest is the release-side equivalent of invokeOne;
// it uses the Release's loaded Manifest to derive a stable manifest
// hash rather than the placeholder hash used by the existing
// /versions/{id}/executions route. Returns the ExecutionRecord (or nil
// when no invoker is configured), the invocation error (or nil on
// success), and the wall-clock latency so the handler can populate
// the Execution row.
func (s *Server) invokeOneWithManifest(r *http.Request, rel *release.Release, inputs map[string]any, model, provider string) (*executor.ExecutionRecord, error, time.Duration) {
	if s.invoker == nil {
		return nil, nil, 0
	}
	input, err := marshalNoArgs(inputs)
	if err != nil {
		return nil, err, 0
	}
	manifestHash, _ := computeManifestHash(rel.Manifest)
	req := executor.InvokeRequest{
		WorkspaceID:   r.PathValue("workspace_id"),
		ReleaseID:     rel.ID,
		ManifestHash:  manifestHash,
		InputHash:     inputHash(input),
		Input:         input,
		Model:         model,
		ModelRevision: modelRevision(model, provider),
	}
	start := time.Now()
	rec, err := s.invoker.Invoke(r.Context(), req)
	return &rec, err, time.Since(start)
}

func (s *Server) handleGetReleaseApproval(w http.ResponseWriter, r *http.Request) error {
	a, err := s.releaseSvc.Approval(r.Context(), r.PathValue("id"))
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, a)
	return nil
}

func manifestHashForRelease(rel *release.Release) string {
	h, err := computeManifestHash(rel.Manifest)
	if err != nil {
		return ""
	}
	return h
}
