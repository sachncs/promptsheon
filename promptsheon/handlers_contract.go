// Package api — Capability contract handlers. The contract is
// the typed promise a Capability makes to operators (input /
// output schema, SLO target, blast radius). Attaching a
// contract is required for the Recommendation engine's
// auto-promotion path; contract-less Capabilities fall back
// to the human-only approval workflow.
package promptsheon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sachncs/promptsheon/promptsheon/capability"
)

// capabilityRepo returns the typed CapabilityRepository the
// server holds. The accessor exists so handlers don't depend
// on the wider DB facade; they only see the surface
// the capability package owns.
func (s *Server) capabilityRepo() capability.Repository {
	return s.db.CapabilityRepository
}

// handleUpdateCapabilityContract attaches a contract to a
// Capability. The contract is the typed governance artifact:
// blast radius, SLO target, success rubric. A Capability
// without a contract cannot be auto-promoted.
//
// PUT /api/v1/capabilities/{id}/contract
// Body: CapabilityContract JSON
// 200 OK with the persisted contract.
// 400 Bad Request: contract.Validate failed.
// 404 Not Found: capability id does not exist.
// UpdateCapabilityContract updates the capabilityContract.
func (s *Server) handleUpdateCapabilityContract(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "missing capability id"}
	}
	var c capability.CapabilityContract
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "invalid contract body: " + err.Error()}
	}
	if err := c.Validate(); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if err := s.capabilityRepo().SetCapabilityContract(r.Context(), id, &c); err != nil {
		return translateDBError(err, "capability contract")
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(c)
}

// handleGetCapabilityContract returns the contract attached to
// a Capability. 404 if the Capability has no contract or does
// not exist.
// GetCapabilityContract returns the capabilityContract.
func (s *Server) handleGetCapabilityContract(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "missing capability id"}
	}
	c, err := s.capabilityRepo().GetCapabilityContract(r.Context(), id)
	if err != nil {
		return translateDBError(err, "capability contract")
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(c)
}

// handleDiffVersions returns the structural diff between two
// Versions of a Capability. The from / to query params name
// integer Version counters; the response is a JSON object
// listing the artifact references that changed between the
// two.
//
// GET /api/v1/capabilities/{id}/diff?from=1&to=2
// DiffVersions handles the request.
func (s *Server) handleDiffVersions(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "missing capability id"}
	}
	from, to, err := parseVersionDiffArgs(r)
	if err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	fromV, err := s.capabilityRepo().GetVersionByNumber(r.Context(), id, from)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &HTTPError{Status: http.StatusNotFound, Message: "from version not found"}
		}
		return &HTTPError{Status: http.StatusInternalServerError, Message: "internal error"}
	}
	toV, err := s.capabilityRepo().GetVersionByNumber(r.Context(), id, to)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &HTTPError{Status: http.StatusNotFound, Message: "to version not found"}
		}
		return &HTTPError{Status: http.StatusInternalServerError, Message: "internal error"}
	}
	diff := capability.DiffManifests(fromV.Manifest, toV.Manifest)
	diff.FromVersion = from
	diff.ToVersion = to
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(diff)
}

// handleGetCapabilityReputation returns the derived trust
// score for a Capability. The score is computed on demand from
// execution history, decision history, and SLO adherence.
//
// GET /api/v1/capabilities/{id}/reputation
// GetCapabilityReputation returns the capabilityReputation.
func (s *Server) handleGetCapabilityReputation(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "missing capability id"}
	}
	rep, err := s.capabilityRepo().GetCapabilityReputation(r.Context(), id)
	if err != nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "internal error"}
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(rep)
}

// parseVersionDiffArgs pulls from / to integer version
// counters from the request query string. Defaults: from=1,
// to=latest (caller must supply to).
func parseVersionDiffArgs(r *http.Request) (int, int, error) {
	q := r.URL.Query()
	from := 1
	to := 0
	if v := q.Get("from"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid from=%q: must be an integer", v)
		}
		from = n
	}
	if v := q.Get("to"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid to=%q: must be an integer", v)
		}
		to = n
	}
	if to == 0 {
		return 0, 0, fmt.Errorf("to is required")
	}
	if to < from {
		return 0, 0, fmt.Errorf("to must be >= from")
	}
	return from, to, nil
}

// handleCatalogSearch returns the Capabilities in the
// supplied workspace whose name matches the query string.
// Empty query returns every Capability in the workspace.
//
// GET /api/v1/catalog/capabilities?workspace_id=ws1&q=foo&limit=100
// CatalogSearch handles the request.
func (s *Server) handleCatalogSearch(w http.ResponseWriter, r *http.Request) error {
	ws := r.URL.Query().Get("workspace_id")
	if ws == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "workspace_id is required"}
	}
	q := r.URL.Query().Get("q")
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	caps, err := s.capabilityRepo().CatalogSearch(r.Context(), ws, q, limit)
	if err != nil {
		s.logger.Error("catalog search failed", "workspace_id", ws, "err", err)
		return &HTTPError{Status: http.StatusInternalServerError, Message: "internal error"}
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(caps)
}
