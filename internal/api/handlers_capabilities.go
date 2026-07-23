package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/invoke"
)

// translateGetError is retained as a thin wrapper over the
// shared translateDBError helper (API-4a) so existing call
// sites in this file don't have to be renamed. New code should
// call translateDBError directly.
func translateGetError(err error, resource string) error {
	return translateDBError(err, resource)
}

// computeManifestHash returns the canonical SHA-256 hex of a Manifest
// in its JSON serialisation. It is used to set Version.ManifestHash,
// which becomes the deduplication key on the CAS table that the
// content-addressable store will be backed by in a later milestone.
func computeManifestHash(m capability.Manifest) (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	workspaces, err := s.db.ListWorkspaces(r.Context())
	if err != nil {
		return err
	}
	if workspaces == nil {
		workspaces = []*capability.Workspace{}
	}
	paged := applyOffsetLimit(workspaces, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(workspaces), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name         string `json:"name"`
		Organization string `json:"organization,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if err := validateNonEmpty("name", req.Name); err != nil {
		return err
	}
	now := time.Now()
	wksp := &capability.Workspace{
		ID:           generateID(),
		Name:         req.Name,
		Organization: req.Organization,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.CreateWorkspace(r.Context(), wksp); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "workspace:"+wksp.ID, map[string]any{auditKeyName: wksp.Name})
	writeJSON(w, http.StatusCreated, wksp)
	return nil
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	wksp, err := s.db.GetWorkspace(r.Context(), id)
	if err != nil {
		return translateDBError(err, "workspace")
	}
	writeJSON(w, http.StatusOK, wksp)
	return nil
}

func (s *Server) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetWorkspace(r.Context(), id)
	if err != nil {
		return translateDBError(err, "workspace")
	}
	var req struct {
		Name         *string `json:"name"`
		Organization *string `json:"organization,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Organization != nil {
		existing.Organization = *req.Organization
	}
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateWorkspace(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "workspace:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteWorkspace(r.Context(), id); err != nil {
		return translateDBError(err, "workspace")
	}
	s.audit(r.Context(), "delete", "workspace:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	workspaceID := r.PathValue("workspace_id")
	projects, err := s.db.ListProjects(r.Context(), workspaceID)
	if err != nil {
		return err
	}
	if projects == nil {
		projects = []*capability.Project{}
	}
	paged := applyOffsetLimit(projects, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(projects), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) error {
	workspaceID := r.PathValue("workspace_id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if err := validateNonEmpty("name", req.Name); err != nil {
		return err
	}
	now := time.Now()
	proj := &capability.Project{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.CreateProject(r.Context(), proj); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "project:"+proj.ID, map[string]any{auditKeyName: proj.Name, "workspace_id": workspaceID})
	writeJSON(w, http.StatusCreated, proj)
	return nil
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	proj, err := s.db.GetProject(r.Context(), id)
	if err != nil {
		return translateDBError(err, "project")
	}
	writeJSON(w, http.StatusOK, proj)
	return nil
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetProject(r.Context(), id)
	if err != nil {
		return translateDBError(err, "project")
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateProject(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "project:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteProject(r.Context(), id); err != nil {
		return translateDBError(err, "project")
	}
	s.audit(r.Context(), "delete", "project:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListCapabilities(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	projectID := r.PathValue("project_id")
	caps, err := s.db.ListCapabilities(r.Context(), projectID)
	if err != nil {
		return err
	}
	if caps == nil {
		caps = []*capability.Capability{}
	}
	paged := applyOffsetLimit(caps, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(caps), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

func (s *Server) handleCreateCapability(w http.ResponseWriter, r *http.Request) error {
	projectID := r.PathValue("project_id")
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description,omitempty"`
		Owner       string   `json:"owner,omitempty"`
		Tags        []string `json:"tags,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if err := validateNonEmpty("name", req.Name); err != nil {
		return err
	}
	// API-VAL-3: Owner must reference an existing user when
	// supplied. An empty Owner is allowed (means "no owner").
	if req.Owner != "" {
		if _, err := s.db.GetUser(r.Context(), req.Owner); err != nil {
			return badRequest("owner: " + translateDBError(err, "user").Error())
		}
	}
	now := time.Now()
	capab := &capability.Capability{
		ID:          generateID(),
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		Owner:       req.Owner,
		Tags:        req.Tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.CreateCapability(r.Context(), capab); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "capability:"+capab.ID, map[string]any{auditKeyName: capab.Name, "project_id": projectID})
	writeJSON(w, http.StatusCreated, capab)
	return nil
}

func (s *Server) handleGetCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	c, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability")
	}
	writeJSON(w, http.StatusOK, c)
	return nil
}

func (s *Server) handleUpdateCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability")
	}
	var req struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description,omitempty"`
		Owner       *string   `json:"owner,omitempty"`
		Tags        *[]string `json:"tags,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Owner != nil {
		existing.Owner = *req.Owner
	}
	if req.Tags != nil {
		existing.Tags = *req.Tags
	}
	existing.UpdatedAt = time.Now()
	if err := s.db.UpdateCapability(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "capability:"+existing.ID, nil)
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteCapability(r.Context(), id); err != nil {
		return translateDBError(err, "capability")
	}
	s.audit(r.Context(), "delete", "capability:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	capabilityID := r.PathValue("capability_id")
	versions, err := s.db.ListVersions(r.Context(), capabilityID)
	if err != nil {
		return err
	}
	if versions == nil {
		versions = []*capability.Version{}
	}
	paged := applyOffsetLimit(versions, offset, limit)
	writePaginationHeaders(w, r, limit, offset, len(versions), len(paged))
	writeJSON(w, http.StatusOK, paged)
	return nil
}

func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	var req struct {
		Version  int                 `json:"version"`
		Manifest capability.Manifest `json:"manifest"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	// API-VAL-2: reject non-positive version numbers so the
	// caller can't insert a phantom "v0" or "v-1".
	if err := validatePositiveInt("version", req.Version); err != nil {
		return err
	}
	// Forward-only: every request MUST supply a Manifest. The
	// legacy synthesis helper is gone; clients that still pass
	// the old bundle shape get 400 with a manifest-required error.
	manifest := req.Manifest
	if err := manifest.Validate(); err != nil {
		return badRequest("manifest: " + err.Error())
	}
	hash, err := computeManifestHash(manifest)
	if err != nil {
		return fmt.Errorf("compute manifest hash: %w", err)
	}
	now := time.Now()
	v := &capability.Version{
		ID:           generateID(),
		CapabilityID: capabilityID,
		Version:      req.Version,
		Manifest:     manifest,
		ManifestHash: hash,
		CreatedAt:    now,
		CreatedBy:    callerID(r),
	}
	if err := s.db.CreateVersion(r.Context(), v); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "version:"+v.ID, map[string]any{"capability_id": capabilityID, auditKeyVersion: v.Version, "manifest_hash": hash})
	writeJSON(w, http.StatusCreated, v)
	return nil
}

// sha256Hex returns hex(sha256(b)).
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// legacyPlaceholderHash is used when marshalling fails. It is a
// recognisable 64-hex string that flags the row as having arrived via
// the migration rather than the canonical CAS path.
func legacyPlaceholderHash(kind capability.ArtifactKind) string {
	digest := sha256.Sum256([]byte("legacy:" + string(kind)))
	return hex.EncodeToString(digest[:])
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	v, err := s.db.GetVersion(r.Context(), id)
	if err != nil {
		return translateDBError(err, "version")
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) handleGetLatestVersion(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	v, err := s.db.GetLatestVersion(r.Context(), capabilityID)
	if err != nil {
		return translateDBError(err, "version")
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := parsePagination(r)
	if err != nil {
		return err
	}
	capabilityVersionID := r.PathValue("version_id")
	filter := capability.ExecutionFilter{
		CapabilityVersionID: capabilityVersionID,
		Limit:               limit,
		Offset:              offset,
	}
	execs, err := s.db.ListExecutions(r.Context(), filter)
	if err != nil {
		return err
	}
	if execs == nil {
		execs = []*capability.Execution{}
	}
	writeJSON(w, http.StatusOK, execs)
	return nil
}

// errProviderMissing is an alias for the executor's typed
// sentinel. We map the executor's ErrProviderMissing (returned
// by the daemon when no provider is registered for the
// requested model) to 502 Bad Gateway with a provider_missing
// detail so operators can distinguish "no provider" from
// "provider failed" without reading the daemon log. BUG-19.
var errProviderMissing = executor.ErrProviderMissing

func (s *Server) handleCreateExecution(w http.ResponseWriter, r *http.Request) error {
	capabilityVersionID := r.PathValue("version_id")
	var req struct {
		Inputs   map[string]any `json:"inputs,omitempty"`
		Model    string         `json:"model"`
		Provider string         `json:"provider"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	rec, invErr, latency := s.invokeOne(r, capabilityVersionID, req.Inputs, req.Model, req.Provider)
	exec := &capability.Execution{
		ID:                  generateID(),
		CapabilityVersionID: capabilityVersionID,
		Timestamp:           time.Now(),
		Inputs:              req.Inputs,
		Model:               req.Model,
		Provider:            req.Provider,
		LatencyMs:           latency.Milliseconds(),
	}
	// The previous implementation bailed on classifyInvokeError
	// before persisting the failed execution, so a 5xx-class
	// invoke was invisible in audit and the execution table.
	// The new contract: always persist (success or failure),
	// return 5xx on failure. A failed execution IS an event we
	// want in the audit chain.
	if rec != nil {
		if len(rec.Output) > 0 {
			exec.Outputs = map[string]any{"content": string(rec.Output)}
		}
		exec.PromptTokens = rec.PromptTokens
		exec.CompletionTokens = rec.OutputTokens
		exec.TotalTokens = rec.PromptTokens + rec.OutputTokens
		exec.Model = rec.Model
		exec.CostUSD = rec.CostUSD
	}
	if invErr != nil {
		exec.Error = invErr.Error()
	}
	if err := s.db.CreateExecution(r.Context(), exec); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "execution:"+exec.ID, map[string]any{
		"version_id":       capabilityVersionID,
		"tokens":           exec.TotalTokens,
		"cost_usd":         exec.CostUSD,
		"tokens_estimated": exec.TotalTokens > 0 || exec.CostUSD > 0,
		"error":            exec.Error,
	})
	if invErr != nil {
		// BUG-19: distinguish provider-missing from generic 5xx so
		// the operator can tell at a glance whether the LLM provider
		// was unregistered or the request simply failed.
		if errors.Is(invErr, errProviderMissing) {
			return &HTTPError{
				Status:  http.StatusBadGateway,
				Message: "no LLM provider configured for this invocation",
				Details: map[string]any{"provider_missing": true},
			}
		}
		if err := classifyInvokeError(invErr); err != nil {
			return err
		}
	}
	writeJSON(w, http.StatusCreated, exec)
	return nil
}

// classifyInvokeError maps an invoke.Invoker error to the appropriate
// HTTP status. Returns nil when the error is nil or not worth
// translating (the caller should still record the error in the
// Execution row).
//
// BUG-15: the previous version returned the raw err.Error() to
// the client on every 5xx. Upstream provider failures frequently
// embed the request URL, including the bearer-token query-string
// fallback (if an operator ever configured one), or internal
// stack traces. Sanitise by returning a generic message and
// relying on the audit log to preserve the full error.
func classifyInvokeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, invoke.ErrQuotaExceeded) {
		return &HTTPError{Status: http.StatusTooManyRequests, Message: "quota exceeded"}
	}
	if errors.Is(err, invoke.ErrBudgetExceeded) {
		return &HTTPError{Status: http.StatusPaymentRequired, Message: "budget exceeded"}
	}
	return &HTTPError{
		Status:  http.StatusBadGateway,
		Message: "invoke failed",
		Details: map[string]any{"error": err.Error()},
	}
}

// invokeOne is the per-request invocation entry point. It is
// introduced as a method on Server rather than a package-level
// function so that the production wiring can override the
// default Caller and AggregatorConsumer with a workspace-scoped
// Caller chain.
//
// When the versionID resolves to a Capability Version with a stored
// Manifest, that Manifest's canonical SHA-256 is used as the manifest
// hash. Otherwise the handler falls back to the placeholder hash so
// the route stays observable even for versions that pre-date the
// Manifest schema.
//
// Returns the ExecutionRecord (or nil when the invoker has nothing
// to record), the invocation error (or nil on success), and the
// wall-clock latency so callers can populate the Execution row.
//
// The function requires s.invoker to be set; tests and the daemon
// entry point must construct an invoke.Invoker. There is no
// "stub" path — a missing invoker is a programming error and
// returns a clear error rather than a silent no-op so misconfigured
// deployments fail loudly.
func (s *Server) invokeOne(r *http.Request, versionID string, inputs map[string]any, model, provider string) (*executor.ExecutionRecord, error, time.Duration) {
	if s.invoker == nil {
		return nil, errors.New("api: invoke.Invoker not wired on this server"), 0
	}
	input, err := marshalNoArgs(inputs)
	if err != nil {
		return nil, err, 0
	}
	mh := manifestHash(versionID, model, provider)
	if v, err := s.db.GetVersion(r.Context(), versionID); err == nil {
		if v.ManifestHash != "" {
			mh = v.ManifestHash
		}
	}
	req := executor.InvokeRequest{
		WorkspaceID:   r.PathValue("workspace_id"),
		ReleaseID:     versionID,
		ManifestHash:  mh,
		InputHash:     inputHash(input),
		Input:         input,
		Model:         model,
		ModelRevision: modelRevision(model, provider),
		Provider:      provider,
	}
	start := time.Now()
	rec, err := s.invoker.Invoke(r.Context(), req)
	return &rec, err, time.Since(start)
}

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	e, err := s.db.GetExecution(r.Context(), id)
	if err != nil {
		return translateDBError(err, "execution")
	}
	writeJSON(w, http.StatusOK, e)
	return nil
}

// manifestHash returns a stable SHA-256 hex of the inputs that
// uniquely identify a Release. The HTTP handler has only the
// release ID, model, and provider to work with; the active Release
// lookup in production fills in the full manifest hash.
func manifestHash(versionID, model, provider string) string {
	h := sha256.New()
	h.Write([]byte(versionID))
	h.Write([]byte{0x1f})
	h.Write([]byte(model))
	h.Write([]byte{0x1f})
	h.Write([]byte(provider))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// inputHash returns the SHA-256 hex of the JSON-encoded inputs.
func inputHash(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	h := sha256.Sum256(input)
	return "sha256:" + hex.EncodeToString(h[:])
}

// modelRevision returns the calendar-day revision label for the
// supplied model+provider pair. The active Release lookup fills in
// the precise revision from the manifest.
func modelRevision(model, provider string) string {
	return time.Now().UTC().Format("2006-01-02") + ":" + model + ":" + provider
}
