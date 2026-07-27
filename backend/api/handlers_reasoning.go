// Reasoning API handler: POST /api/v1/reasoning/compile.
//
// The handler accepts an Intent and returns a CapabilityPlan
// (or a typed error: ErrNoMatch, ErrConstraintViolation).
// Production wiring supplies the catalog from the workspace's
// Capabilities filtered by reputation; tests pass an explicit
// catalog.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sachncs/promptsheon/backend/reasoning"
)

// handleReasoningCompile turns an Intent into a Plan.
//
// POST /api/v1/reasoning/compile
// Body: reasoning.Intent JSON
// 200 OK with reasoning.Plan
// 404 Not Found: ErrNoMatch
// 409 Conflict: ErrConstraintViolation
func (s *Server) handleReasoningCompile(w http.ResponseWriter, r *http.Request) error {
	var intent reasoning.Intent
	if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "invalid intent: " + err.Error()}
	}
	catalog, err := s.reasoningCatalog(r.Context())
	if err != nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "catalog: " + err.Error()}
	}
	plan, err := reasoning.NewCompiler(catalog).Compile(r.Context(), intent)
	if err != nil {
		switch {
		case errors.Is(err, reasoning.ErrNoMatch):
			return &HTTPError{Status: http.StatusNotFound, Message: err.Error()}
		case errors.Is(err, reasoning.ErrConstraintViolation):
			return &HTTPError{Status: http.StatusConflict, Message: err.Error()}
		default:
			return &HTTPError{Status: http.StatusBadRequest, Message: err.Error()}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(plan)
}

// reasoningCatalog returns the catalog the Compiler consumes.
// Production wiring populates this from the workspace's
// Capabilities (with reputation as the TrustScore); the
// default returns an empty catalog so the daemon boots even
// without capabilities.
func (s *Server) reasoningCatalog(ctx context.Context) ([]reasoning.CapabilityDescriptor, error) {
	caps, err := s.capabilityRepo().ListCapabilities(ctx, "")
	if err != nil || len(caps) == 0 {
		return nil, err
	}
	out := make([]reasoning.CapabilityDescriptor, 0, len(caps))
	for _, c := range caps {
		rep, _ := s.capabilityRepo().GetCapabilityReputation(ctx, c.ID)
		out = append(out, reasoning.CapabilityDescriptor{
			ID:         c.ID,
			Name:       c.Name,
			Tags:       c.Tags,
			TrustScore: rep.TrustScore,
			CostUSD:    0.001,
			LatencyMS:  500,
			Outputs:    []string{"result"},
		})
	}
	return out, nil
}
