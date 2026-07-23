package api

import (
	"net/http"

	"github.com/sachncs/promptsheon/internal/workflow"
)

// handleRunWorkflow accepts a workflow Definition and runs it
// through the configured workflow.Engine. The response is the
// workflow.Result, which contains per-step outcomes and a final
// status.
//
// Per the production-readiness review, the workflow.Engine is
// the only entry point for multi-step orchestration; the
// audit-chain wrapping is provided by audit() so a workflow
// run is recorded as a single "workflow.run" event with the
// workflow id.
func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) error {
	if s.workflowEngine == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "workflow engine not configured"}
	}
	var def workflow.Definition
	if err := readJSON(r, &def); err != nil {
		return badRequest("invalid request body")
	}
	// API-VAL-8: reject empty workflow definitions at the API
	// boundary. The engine would have rejected them anyway, but
	// emitting a structured 400 with "steps required" lets the
	// caller fix the request without reading the engine error.
	if err := validateNonEmpty("id", def.ID); err != nil {
		return err
	}
	if len(def.Steps) == 0 {
		return badRequest("steps is required and must be non-empty")
	}
	res, err := s.workflowEngine.Run(r.Context(), def, map[string]any{})
	if err != nil {
		s.audit(r.Context(), "workflow.fail", "workflow:"+def.ID, map[string]any{"error": err.Error()})
		writeJSON(w, http.StatusUnprocessableEntity, res)
		return nil
	}
	s.audit(r.Context(), "workflow.run", "workflow:"+def.ID, map[string]any{
		"status":  string(res.Status),
		"steps":   len(res.Steps),
		"outputs": len(res.Outputs),
	})
	writeJSON(w, http.StatusOK, res)
	return nil
}
