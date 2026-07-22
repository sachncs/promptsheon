package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sachncs/promptsheon/internal/models"
)

const fieldUserID = "user_id"

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) error {
	filter := models.AuditFilter{
		UserID:   r.URL.Query().Get("user_id"),
		Resource: r.URL.Query().Get("resource"),
		Action:   r.URL.Query().Get("action"),
		Limit:    50,
		Offset:   0,
	}

	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return badRequest("invalid since format, use RFC3339")
		}
		filter.Since = &t
	}
	if v := r.URL.Query().Get("until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return badRequest("invalid until format, use RFC3339")
		}
		filter.Until = &t
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return badRequest("invalid limit: must be an integer")
		}
		if n < 1 || n > 1000 {
			return badRequest("invalid limit: must be between 1 and 1000")
		}
		filter.Limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return badRequest("invalid offset: must be an integer")
		}
		if n < 0 {
			return badRequest("invalid offset: must be non-negative")
		}
		filter.Offset = n
	}

	entries, err := s.db.ListAudit(r.Context(), &filter)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []*models.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
	return nil
}

func (s *Server) handleExportAudit(w http.ResponseWriter, r *http.Request) error {
	filter := models.AuditFilter{
		UserID:   r.URL.Query().Get("user_id"),
		Resource: r.URL.Query().Get("resource"),
		Action:   r.URL.Query().Get("action"),
	}

	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return badRequest("invalid since format, use RFC3339")
		}
		filter.Since = &t
	}
	if v := r.URL.Query().Get("until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return badRequest("invalid until format, use RFC3339")
		}
		filter.Until = &t
	}

	entries, err := s.db.ExportAudit(r.Context(), &filter)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []*models.AuditEntry{}
	}

	format := r.URL.Query().Get("format")
	if format == "csv" {
		return s.writeAuditCSV(w, entries)
	}
	writeJSON(w, http.StatusOK, entries)
	return nil
}

func (s *Server) writeAuditCSV(w http.ResponseWriter, entries []*models.AuditEntry) error {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=audit_%s.csv", time.Now().Format("20060102_150405")))

	writer := csv.NewWriter(w)

	// Header
	if err := writer.Write([]string{"id", fieldUserID, "action", "resource", "details", "timestamp", "previous_hash", "entry_hash"}); err != nil {
		return fmt.Errorf("csv write header: %w", err)
	}

	// Data
	for _, e := range entries {
		details, err := json.Marshal(e.Details)
		if err != nil {
			details = []byte("{}")
		}
		if err := writer.Write([]string{
			e.ID,
			e.UserID,
			e.Action,
			e.Resource,
			string(details),
			e.Timestamp.Format(time.RFC3339),
			e.PreviousHash,
			e.EntryHash,
		}); err != nil {
			return fmt.Errorf("csv write row %s: %w", e.ID, err)
		}
	}

	writer.Flush()
	return writer.Error()
}

func (s *Server) handleVerifyAuditChain(w http.ResponseWriter, r *http.Request) error {
	res, err := s.db.VerifyAuditChain(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            res.Ok,
		"tail_mismatch": res.TailMismatch,
		"last_row_id":   res.LastRowID,
		"last_hash":     res.LastHash,
		"reason":        res.Reason,
	})
	return nil
}
