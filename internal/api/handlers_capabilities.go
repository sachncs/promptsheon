package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
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
		Version         int                          `json:"version"`
		Manifest        capability.Manifest          `json:"manifest"`
		Prompt          capability.Prompt            `json:"prompt"`
		ModelPolicy     capability.ModelPolicy       `json:"model_policy"`
		ContextContract capability.ContextContract   `json:"context_contract"`
		Knowledge       []capability.KnowledgeSource `json:"knowledge,omitempty"`
		Memory          capability.MemoryConfig      `json:"memory"`
		Guardrails      []capability.Guardrail       `json:"guardrails,omitempty"`
		Tools           []capability.Tool            `json:"tools,omitempty"`
		MCPServers      []capability.MCPServer       `json:"mcp_servers,omitempty"`
		RuntimePolicy   capability.RuntimePolicy     `json:"runtime_policy"`
		EvaluationSuite capability.EvaluationSuite   `json:"evaluation_suite"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	// During the transition window (migration 023) the API still
	// accepts the legacy bundle shape. When the request carries no
	// manifest we synthesise one from the bundle so the persisted
	// row is consistent. Once the legacy columns are dropped in a
	// later migration this synthesis block will be removed and
	// every request must supply a manifest.
	manifest := req.Manifest
	if manifestIsEmpty(manifest) {
		manifest = manifestFromLegacy(
			req.Prompt, req.ModelPolicy, req.ContextContract,
			req.Memory, req.Knowledge, req.Guardrails,
			req.Tools, req.MCPServers,
		)
	}
	if err := manifest.Validate(); err != nil {
		return badRequest("manifest: " + err.Error())
	}
	hash, err := computeManifestHash(manifest)
	if err != nil {
		return fmt.Errorf("compute manifest hash: %w", err)
	}
	now := time.Now()
	v := &capability.Version{
		ID:              generateID(),
		CapabilityID:    capabilityID,
		Version:         req.Version,
		Manifest:        manifest,
		ManifestHash:    hash,
		Prompt:          req.Prompt,
		ModelPolicy:     req.ModelPolicy,
		ContextContract: req.ContextContract,
		Knowledge:       req.Knowledge,
		Memory:          req.Memory,
		Guardrails:      req.Guardrails,
		Tools:           req.Tools,
		MCPServers:      req.MCPServers,
		RuntimePolicy:   req.RuntimePolicy,
		EvaluationSuite: req.EvaluationSuite,
		CreatedAt:       now,
		CreatedBy:       callerID(r),
	}
	if err := s.db.CreateVersion(r.Context(), v); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "version:"+v.ID, map[string]any{"capability_id": capabilityID, fieldVersion: v.Version, "manifest_hash": hash})
	writeJSON(w, http.StatusCreated, v)
	return nil
}

// manifestIsEmpty reports whether m has no populated artifact references.
// An empty Manifest with default Kind values would otherwise Validate
// as ErrEmptyManifest; this helper decides whether to synthesise from
// legacy fields first.
func manifestIsEmpty(m capability.Manifest) bool {
	return m.Prompt.Hash == "" &&
		m.ModelPolicy.Hash == "" &&
		m.RuntimePolicy.Hash == "" &&
		m.Context.Hash == "" &&
		m.Memory.Hash == ""
}

// manifestFromLegacy builds a content-addressed Manifest from the
// legacy embedded-bundle fields by hashing the canonical JSON of
// each value. The hashes are deterministic; the same legacy bundle
// always produces the same manifest hash. Synthetic placeholder
// hashes for empty values satisfy the kind requirement while
// leaving the obvious "this came from a legacy row" footprint in
// the manifest_hash column during the transition window.
func manifestFromLegacy(
	p capability.Prompt, mp capability.ModelPolicy, cc capability.ContextContract,
	mem capability.MemoryConfig, ks []capability.KnowledgeSource, gs []capability.Guardrail,
	ts []capability.Tool, ms []capability.MCPServer,
) capability.Manifest {
	refFrom := func(kind capability.ArtifactKind, v any) capability.ArtifactRef {
		b, err := json.Marshal(v)
		if err != nil {
			return capability.ArtifactRef{Kind: kind, Hash: legacyPlaceholderHash(kind)}
		}
		return capability.ArtifactRef{Kind: kind, Hash: sha256Hex(b)}
	}
	m := capability.Manifest{
		Prompt:        refFrom(capability.ArtifactPrompt, p),
		ModelPolicy:   refFrom(capability.ArtifactModelPolicy, mp),
		RuntimePolicy: refFrom(capability.ArtifactRuntimePolicy, capability.RuntimePolicy{}),
		Context:       refFrom(capability.ArtifactContext, cc),
		Memory:        refFrom(capability.ArtifactMemory, mem),
	}
	for _, k := range ks {
		m.Knowledge = append(m.Knowledge, refFrom(capability.ArtifactKnowledge, k))
	}
	for _, g := range gs {
		m.Guardrails = append(m.Guardrails, refFrom(capability.ArtifactGuardrail, g))
	}
	for _, t := range ts {
		m.Tools = append(m.Tools, refFrom(capability.ArtifactTool, t))
	}
	for _, sv := range ms {
		m.MCPServers = append(m.MCPServers, refFrom(capability.ArtifactMCPServer, sv))
	}
	return m
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

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	e, err := s.db.GetExecution(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, e)
	return nil
}
