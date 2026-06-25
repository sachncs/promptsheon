package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
)

func (s *Server) handleListDatasets(w http.ResponseWriter, r *http.Request) error {
	datasets, err := s.db.ListDatasets(r.Context())
	if err != nil {
		return err
	}
	if datasets == nil {
		datasets = []*models.TestDataset{}
	}
	writeJSON(w, http.StatusOK, datasets)
	return nil
}

func (s *Server) handleCreateDataset(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name  string            `json:"name"`
		Cases []models.TestCase `json:"cases"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" {
		return ErrBadRequest
	}

	d := &models.TestDataset{
		ID:        generateID(),
		Name:      req.Name,
		Cases:     req.Cases,
		CreatedBy: callerID(r),
		CreatedAt: time.Now(),
	}

	if err := s.db.CreateDataset(r.Context(), d); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "dataset:"+d.ID, map[string]any{"name": d.Name, "cases": len(d.Cases)})
	writeJSON(w, http.StatusCreated, d)
	return nil
}

func (s *Server) handleGetDataset(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	d, err := s.db.GetDataset(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) handleUpdateDataset(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	existing, err := s.db.GetDataset(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		Name  *string           `json:"name"`
		Cases []models.TestCase `json:"cases"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Cases != nil {
		existing.Cases = req.Cases
	}

	if err := s.db.UpdateDataset(r.Context(), existing); err != nil {
		return err
	}
	s.audit(r.Context(), "update", "dataset:"+existing.ID, map[string]any{"name": existing.Name, "cases": len(existing.Cases)})
	writeJSON(w, http.StatusOK, existing)
	return nil
}

func (s *Server) handleDeleteDataset(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteDataset(r.Context(), id); err != nil {
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "dataset:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) handleExportDataset(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	d, err := s.db.GetDataset(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	for _, tc := range d.Cases {
		if err := enc.Encode(tc); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleImportDataset(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name  string            `json:"name"`
		Cases []models.TestCase `json:"cases"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Name == "" {
		return ErrBadRequest
	}

	d := &models.TestDataset{
		ID:        generateID(),
		Name:      req.Name,
		Cases:     req.Cases,
		CreatedBy: callerID(r),
		CreatedAt: time.Now(),
	}

	if err := s.db.CreateDataset(r.Context(), d); err != nil {
		return err
	}
	s.audit(r.Context(), "import", "dataset:"+d.ID, map[string]any{"name": d.Name, "cases": len(d.Cases)})
	writeJSON(w, http.StatusCreated, d)
	return nil
}

func (s *Server) handleImportCSVDataset(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	d, err := s.db.GetDataset(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		CSV string `json:"csv"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.CSV == "" {
		return ErrBadRequest
	}

	reader := csv.NewReader(strings.NewReader(req.CSV))
	records, err := reader.ReadAll()
	if err != nil {
		return badRequestf("invalid CSV: %v", err)
	}
	if len(records) < 2 {
		return badRequest("CSV must have a header row and at least one data row")
	}

	header := records[0]
	var cases []models.TestCase
	for i, row := range records[1:] {
		if len(row) != len(header) {
			return badRequestf("row %d has %d columns, expected %d", i+1, len(row), len(header))
		}

		tc := models.TestCase{
			ID:    generateID(),
			Input: make(map[string]any),
		}

		for j, col := range header {
			col = strings.TrimSpace(col)
			val := strings.TrimSpace(row[j])

			switch {
			case strings.HasPrefix(col, "expected:"):
				// expected:output or expected:contains:X
				suffix := strings.TrimPrefix(col, "expected:")
				if suffix == "output" {
					tc.ExpectedOutput = val
				} else {
					tc.ExpectedContains = append(tc.ExpectedContains, val)
				}
			default:
				tc.Input[col] = val
			}
		}

		cases = append(cases, tc)
	}

	d.Cases = append(d.Cases, cases...)
	if err := s.db.UpdateDataset(r.Context(), d); err != nil {
		return err
	}
	s.audit(r.Context(), "import_csv", "dataset:"+d.ID, map[string]any{
		"name":        d.Name,
		"cases_added": len(cases),
		"total_cases": len(d.Cases),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"cases_added": len(cases),
		"total_cases": len(d.Cases),
	})
	return nil
}
