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

const fieldVersion = "version"

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

var _ = fmt.Sprintf

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) error {
	workspaces, err := s.db.ListWorkspaces(r.Context())
	if err != nil {
		return err
	}
	if workspaces == nil {
		workspaces = []*capability.Workspace{}
	}
	writeJSON(w, http.StatusOK, workspaces)
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
	if req.Name == "" {
		return ErrBadRequest
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
	s.audit(r.Context(), "create", "workspace:"+wksp.ID, map[string]any{keyName: wksp.Name})
	writeJSON(w, http.StatusCreated, wksp)
	return nil
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	wksp, err := s.db.GetWorkspace(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, wksp)
	return nil
}

func (s *Server) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetWorkspace(r.Context(), id)
	if err != nil {
		return ErrNotFound
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
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "workspace:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) error {
	workspaceID := r.PathValue("workspace_id")
	projects, err := s.db.ListProjects(r.Context(), workspaceID)
	if err != nil {
		return err
	}
	if projects == nil {
		projects = []*capability.Project{}
	}
	writeJSON(w, http.StatusOK, projects)
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
	if req.Name == "" {
		return ErrBadRequest
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
	s.audit(r.Context(), "create", "project:"+proj.ID, map[string]any{keyName: proj.Name, "workspace_id": workspaceID})
	writeJSON(w, http.StatusCreated, proj)
	return nil
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	proj, err := s.db.GetProject(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, proj)
	return nil
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetProject(r.Context(), id)
	if err != nil {
		return ErrNotFound
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
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "project:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListCapabilities(w http.ResponseWriter, r *http.Request) error {
	projectID := r.PathValue("project_id")
	caps, err := s.db.ListCapabilities(r.Context(), projectID)
	if err != nil {
		return err
	}
	if caps == nil {
		caps = []*capability.Capability{}
	}
	writeJSON(w, http.StatusOK, caps)
	return nil
}

func (s *Server) handleCreateCapability(w http.ResponseWriter, r *http.Request) error {
	projectID := r.PathValue("project_id")
	var req struct {
		Name        string           `json:"name"`
		Description string           `json:"description,omitempty"`
		Owner       string           `json:"owner,omitempty"`
		Tags        []string         `json:"tags,omitempty"`
		State       capability.State `json:"state,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" {
		return ErrBadRequest
	}
	if req.State == "" {
		req.State = capability.StateDraft
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
	s.audit(r.Context(), "create", "capability:"+capab.ID, map[string]any{keyName: capab.Name, "project_id": projectID})
	writeJSON(w, http.StatusCreated, capab)
	return nil
}

func (s *Server) handleGetCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	c, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, c)
	return nil
}

func (s *Server) handleUpdateCapability(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetCapability(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	var req struct {
		Name        *string           `json:"name"`
		Description *string           `json:"description,omitempty"`
		Owner       *string           `json:"owner,omitempty"`
		Tags        *[]string         `json:"tags,omitempty"`
		State       *capability.State `json:"state,omitempty"`
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
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "capability:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	versions, err := s.db.ListVersions(r.Context(), capabilityID)
	if err != nil {
		return err
	}
	if versions == nil {
		versions = []*capability.Version{}
	}
	writeJSON(w, http.StatusOK, versions)
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
	s.audit(r.Context(), "create", "version:"+v.ID, map[string]any{"capability_id": capabilityID, fieldVersion: v.Version, "manifest_hash": hash})
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
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) handleGetLatestVersion(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	v, err := s.db.GetLatestVersion(r.Context(), capabilityID)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) error {
	capabilityVersionID := r.PathValue("version_id")
	filter := capability.ExecutionFilter{
		CapabilityVersionID: capabilityVersionID,
		Limit:               100,
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
	// Tier 2.36 / Tier 2.44 follow-on: the production wiring
	// constructs an invoke.Invoker per request, runs it through
	// the Budget/Quota enforcer, and maps the resulting errors to
	// HTTP 402 (Payment Required) and 429 (Too Many Requests).
	// Today's commit ships the API surface; the Invoker call is
	// M3 follow-on (see internal/invoke/invoke.go's NewInvoker +
	// DefaultEnforcer wiring).
	if err := s.invokeOne(r, capabilityVersionID, req.Inputs, req.Model, req.Provider); err != nil {
		if errors.Is(err, invoke.ErrQuotaExceeded) {
			return &HTTPError{Status: http.StatusTooManyRequests, Message: err.Error()}
		}
		if errors.Is(err, invoke.ErrBudgetExceeded) {
			return &HTTPError{Status: http.StatusPaymentRequired, Message: err.Error()}
		}
		return err
	}
	now := time.Now()
	exec := &capability.Execution{
		ID:                  generateID(),
		CapabilityVersionID: capabilityVersionID,
		Timestamp:           now,
		Inputs:              req.Inputs,
		Model:               req.Model,
		Provider:            req.Provider,
	}
	if err := s.db.CreateExecution(r.Context(), exec); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "execution:"+exec.ID, map[string]any{"version_id": capabilityVersionID})
	writeJSON(w, http.StatusCreated, exec)
	return nil
}

// invokeOne is the per-request invocation entry point. It is
// introduced as a method on Server rather than a package-level
// function so that the production wiring can override the
// default Caller and AggregatorConsumer with a workspace-scoped
// Caller chain. Today's commit ships the wrapper; the production
// wiring lands in M3 follow-on.
func (s *Server) invokeOne(r *http.Request, versionID string, inputs map[string]any, model, provider string) error {
	if s.invoker == nil {
		// No Invoker configured (today's build). The handler
		// records the stub execution and returns nil; the route
		// is observable while the M3 follow-on wires the
		// production Caller chain.
		return nil
	}
	input, err := marshalNoArgs(inputs)
	if err != nil {
		return err
	}
	req := executor.InvokeRequest{
		WorkspaceID:   r.PathValue("workspace_id"),
		ReleaseID:     versionID,
		ManifestHash:  manifestHash(versionID, model, provider),
		InputHash:     inputHash(input),
		Input:         input,
		Model:         model,
		ModelRevision: modelRevision(model, provider),
	}
	_, err = s.invoker.Invoke(r.Context(), req)
	return err
}

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	e, err := s.db.GetExecution(r.Context(), id)
	if err != nil {
		return ErrNotFound
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
