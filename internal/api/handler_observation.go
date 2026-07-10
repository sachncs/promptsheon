package api

import (
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/rollups"
)

// handleGetWorkspaceObservation returns the per-Workspace rollup
// the architecture review board calls out as Tier 2.37 follow-on:
// the Budget / Quota consumption rollup at the current moment.
//
// The handler wires through rollups.Aggregator when the Server
// has one configured. Today the Server has no Aggregator (the
// rollup job is M3 follow-on), so the handler returns a
// placeholder that documents the contract.
func (s *Server) handleGetWorkspaceObservation(w http.ResponseWriter, r *http.Request) error {
	ws := r.PathValue("workspace_id")
	if ws == "" {
		return ErrBadRequest
	}
	now := time.Now().UTC()
	if s.rollupAgg == nil {
		// No aggregator configured (production wiring lands in M3
		// follow-on). Return an empty summary so the route is
		// observable while the rollup job is built.
		writeJSON(w, http.StatusOK, rollups.WorkspaceSummary{
			WorkspaceID:   ws,
			GeneratedAt:   now,
			OverallHealth: "ok",
		})
		return nil
	}
	got, err := s.rollupAgg.BuildWorkspaceSummary(r.Context(), ws, now)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, got)
	return nil
}
