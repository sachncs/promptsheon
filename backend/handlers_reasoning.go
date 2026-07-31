// Reasoning API handler: POST /api/v1/reasoning/compile.
//
// The handler accepts an Intent and returns a CapabilityPlan
// (or a typed error: ErrNoMatch, ErrConstraintViolation).
// Production wiring supplies the catalog from the workspace's
// Capabilities filtered by reputation; tests pass an explicit
// catalog.
package backend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sachncs/promptsheon/backend/errs"
)

// handleReasoningCompile turns an Intent into a Plan.
//
// POST /api/v1/reasoning/compile
// Body: Intent JSON
// 200 OK with Plan
// 404 Not Found: ErrNoMatch
// 409 Conflict: ErrConstraintViolation
// ReasoningCompile handles the request.
func (s *Server) handleReasoningCompile(w http.ResponseWriter, r *http.Request) error {
	var intent Intent
	if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "invalid intent: " + err.Error()}
	}
	catalog, err := s.reasoningCatalog(r.Context())
	if err != nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "catalog: " + err.Error()}
	}
	plan, err := NewCompiler(catalog).Compile(r.Context(), intent)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrorReasoningNoMatch):
			return &HTTPError{Status: http.StatusNotFound, Message: err.Error()}
		case errors.Is(err, errs.ErrorReasoningConstraintViolation):
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
func (s *Server) reasoningCatalog(ctx context.Context) ([]CapabilityDescriptor, error) {
	caps, err := s.capabilityRepo().ListCapabilities(ctx, "")
	if err != nil || len(caps) == 0 {
		return nil, err
	}
	out := make([]CapabilityDescriptor, 0, len(caps))
	for _, c := range caps {
		rep, _ := s.capabilityRepo().GetCapabilityReputation(ctx, c.ID)
		out = append(out, CapabilityDescriptor{
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
