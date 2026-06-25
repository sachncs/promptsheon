package api

import (
	"net/http"

	"github.com/sachn-cs/promptsheon/internal/models"
)

func (s *Server) handleListAgentExecutions(w http.ResponseWriter, r *http.Request) error {
	agentID := r.PathValue("id")
	if _, err := s.db.GetAgent(r.Context(), agentID); err != nil {
		return ErrNotFound
	}

	limit := 50
	offset := 0
	execs, err := s.db.ListAgentExecutions(r.Context(), agentID, limit, offset)
	if err != nil {
		return err
	}
	if execs == nil {
		execs = []*models.AgentExecution{}
	}

	writeJSON(w, http.StatusOK, execs)
	return nil
}

func (s *Server) handleGetAgentExecution(w http.ResponseWriter, r *http.Request) error {
	execID := r.PathValue("exec_id")
	exec, err := s.db.GetAgentExecution(r.Context(), execID)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, exec)
	return nil
}
