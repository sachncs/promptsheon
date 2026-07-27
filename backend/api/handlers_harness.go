package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/eval"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/store"
)

// ---------------------------------------------------------------------------
// Datasets
// ---------------------------------------------------------------------------

func (s *Server) registerHarnessRoutes() {
	if s.harnessSvc == nil {
		return
	}
	s.mux.HandleFunc("POST /api/v1/capabilities/{capability_id}/datasets", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateDataset)))
	s.mux.HandleFunc("GET /api/v1/capabilities/{capability_id}/datasets", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListDatasets)))
	s.mux.HandleFunc("GET /api/v1/datasets/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetDataset)))
	s.mux.HandleFunc("PUT /api/v1/datasets/{id}/cases", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handlePutDatasetCases)))
	s.mux.HandleFunc("DELETE /api/v1/datasets/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeleteDataset)))

	s.mux.HandleFunc("POST /api/v1/capabilities/{capability_id}/preconditions", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreatePrecondition)))
	s.mux.HandleFunc("GET /api/v1/capabilities/{capability_id}/preconditions", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListPreconditions)))
	// API-SDK-1: PUT /api/v1/preconditions/{id} was missing; the
	// SDK's UpdatePrecondition now hits a real route. The
	// handler validates the same invariants as create so a
	// PUT cannot weaken a precondition below the floor
	// (e.g. timeout_sec < 0).
	s.mux.HandleFunc("PUT /api/v1/preconditions/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdatePrecondition)))
	s.mux.HandleFunc("DELETE /api/v1/preconditions/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeletePrecondition)))

	s.mux.HandleFunc("POST /api/v1/releases/{release_id}/evals", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleRunEval)))
	s.mux.HandleFunc("GET /api/v1/releases/{release_id}/evals", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListEvals)))
	s.mux.HandleFunc("GET /api/v1/evals/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetEval)))
}

type createDatasetRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Cases       []harness.DatasetCase `json:"cases,omitempty"`
}

func (s *Server) handleCreateDataset(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	var req createDatasetRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" {
		return badRequest("name is required")
	}
	now := time.Now().UTC()
	d := &harness.Dataset{
		ID:           generateID(),
		CapabilityID: capabilityID,
		Name:         req.Name,
		Description:  req.Description,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.CreateDataset(r.Context(), d); err != nil {
		return err
	}
	if len(req.Cases) > 0 {
		for i := range req.Cases {
			if req.Cases[i].ID == "" {
				req.Cases[i].ID = generateID()
			}
			if req.Cases[i].DatasetID == "" {
				req.Cases[i].DatasetID = d.ID
			}
			req.Cases[i].Seq = i
		}
		if err := s.db.UpsertDatasetCases(r.Context(), d.ID, req.Cases); err != nil {
			return err
		}
	}
	s.audit(r.Context(), "create", "dataset:"+d.ID, map[string]any{auditKeyName: d.Name, "capability_id": capabilityID})
	writeJSON(w, http.StatusCreated, d)
	return nil
}

func (s *Server) handleListDatasets(w http.ResponseWriter, r *http.Request) error {
	ds, err := s.db.ListDatasetsForCapability(r.Context(), r.PathValue("capability_id"))
	if err != nil {
		return err
	}
	if ds == nil {
		ds = []*harness.Dataset{}
	}
	writeJSON(w, http.StatusOK, ds)
	return nil
}

func (s *Server) handleGetDataset(w http.ResponseWriter, r *http.Request) error {
	d, err := s.db.GetDataset(r.Context(), r.PathValue("id"))
	if err != nil {
		return ErrNotFound
	}
	cases, err := s.db.ListDatasetCases(r.Context(), d.ID)
	if err != nil {
		return err
	}
	d.Cases = cases
	writeJSON(w, http.StatusOK, d)
	return nil
}

type putCasesRequest struct {
	Cases []harness.DatasetCase `json:"cases"`
}

func (s *Server) handlePutDatasetCases(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	var req putCasesRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	for i := range req.Cases {
		if req.Cases[i].ID == "" {
			req.Cases[i].ID = generateID()
		}
		if req.Cases[i].DatasetID == "" {
			req.Cases[i].DatasetID = id
		}
		req.Cases[i].Seq = i
	}
	if err := s.db.UpsertDatasetCases(r.Context(), id, req.Cases); err != nil {
		return err
	}
	cases, err := s.db.ListDatasetCases(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"cases": cases})
	return nil
}

func (s *Server) handleDeleteDataset(w http.ResponseWriter, r *http.Request) error {
	if err := s.db.DeleteDataset(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---------------------------------------------------------------------------
// Preconditions
// ---------------------------------------------------------------------------

type createPreconditionRequest struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

func (s *Server) handleCreatePrecondition(w http.ResponseWriter, r *http.Request) error {
	capabilityID := r.PathValue("capability_id")
	var req createPreconditionRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	timeout := req.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}
	p := &harness.Precondition{
		ID:           generateID(),
		CapabilityID: capabilityID,
		Name:         req.Name,
		Command:      req.Command,
		TimeoutSec:   timeout,
		Enabled:      enabled,
		CreatedAt:    time.Now().UTC(),
	}
	if err := p.Validate(); err != nil {
		return badRequest(err.Error())
	}
	if err := s.db.CreatePrecondition(r.Context(), p); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "precondition:"+p.ID, map[string]any{auditKeyName: p.Name, "capability_id": capabilityID})
	writeJSON(w, http.StatusCreated, p)
	return nil
}

func (s *Server) handleListPreconditions(w http.ResponseWriter, r *http.Request) error {
	ps, err := s.db.ListPreconditionsForCapability(r.Context(), r.PathValue("capability_id"))
	if err != nil {
		return err
	}
	if ps == nil {
		ps = []*harness.Precondition{}
	}
	writeJSON(w, http.StatusOK, ps)
	return nil
}

func (s *Server) handleDeletePrecondition(w http.ResponseWriter, r *http.Request) error {
	if err := s.db.DeletePrecondition(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// handleUpdatePrecondition applies a partial mutation to an
// existing precondition. Only the fields present in the body
// are touched; missing fields keep their previous value. The
// (id, capability_id, created_at) tuple is immutable: the
// handler ignores any attempt to change those via the body.
//
// API-SDK-1: this route completes the precondition lifecycle
// (create / read / update / delete) that the OpenAPI spec
// already advertises.
func (s *Server) handleUpdatePrecondition(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetPrecondition(r.Context(), id)
	if err != nil {
		return translateDBError(err, "precondition")
	}
	var req struct {
		Name       *string `json:"name,omitempty"`
		Command    *string `json:"command,omitempty"`
		TimeoutSec *int    `json:"timeout_sec,omitempty"`
		Enabled    *bool   `json:"enabled,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name != nil {
		if err := validateNonEmpty("name", *req.Name); err != nil {
			return err
		}
		existing.Name = *req.Name
	}
	if req.Command != nil {
		if err := validateNonEmpty("command", *req.Command); err != nil {
			return err
		}
		existing.Command = *req.Command
	}
	if req.TimeoutSec != nil {
		if *req.TimeoutSec < 0 {
			return badRequest("timeout_sec must be non-negative (0 means use the default)")
		}
		existing.TimeoutSec = *req.TimeoutSec
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := existing.Validate(); err != nil {
		return badRequest(err.Error())
	}
	if err := s.db.UpdatePrecondition(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "precondition:"+existing.ID, map[string]any{auditKeyName: existing.Name})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

// ---------------------------------------------------------------------------
// Evals
// ---------------------------------------------------------------------------

type runEvalRequest struct {
	DatasetID string `json:"dataset_id"`
	Scorer    string `json:"scorer"`
}

func (s *Server) handleRunEval(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("release_id")
	var req runEvalRequest
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.DatasetID == "" {
		return badRequest("dataset_id is required")
	}
	scorer := eval.Scorer(req.Scorer)
	if scorer == "" {
		scorer = eval.ScorerExactMatch
	}
	if !eval.ValidScorers(scorer) {
		return badRequest("scorer: unknown")
	}

	run, err := s.harnessSvc.Run(r.Context(), harness.EvalRunOptions{
		ReleaseID:  releaseID,
		DatasetID:  req.DatasetID,
		ScorerName: scorer,
	})
	if err != nil {
		return err
	}
	status := http.StatusOK
	if run.Status == harness.RunFailed || run.Status == harness.RunError {
		status = http.StatusUnprocessableEntity
	}
	s.audit(r.Context(), "run", "eval:"+run.ID, map[string]any{
		"release_id": releaseID,
		"dataset_id": req.DatasetID,
		"scorer":     string(scorer),
		"score":      run.Score,
	})
	writeJSON(w, status, run)
	return nil
}

func (s *Server) handleListEvals(w http.ResponseWriter, r *http.Request) error {
	releaseID := r.PathValue("release_id")
	rs, err := s.db.ListEvalRunsForRelease(r.Context(), releaseID)
	if err != nil {
		return err
	}
	if rs == nil {
		rs = []*harness.EvalRun{}
	}
	writeJSON(w, http.StatusOK, rs)
	return nil
}

func (s *Server) handleGetEval(w http.ResponseWriter, r *http.Request) error {
	// BUG-26: distinguish 404 from 500 on DB failure. The
	// previous form returned ErrNotFound for any error,
	// masking DB outages as "run not found". The repo
	// translates sql.ErrNoRows into store.ErrNotFound, so
	// match that sentinel here; anything else is a real
	// failure and gets a 500.
	run, err := s.db.GetEvalRun(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotFound
		}
		return &HTTPError{Status: http.StatusInternalServerError, Message: "eval run lookup failed"}
	}
	results, err := s.db.ListEvalResultsForRun(r.Context(), run.ID)
	if err != nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "eval results lookup failed"}
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "results": results})
	return nil
}
