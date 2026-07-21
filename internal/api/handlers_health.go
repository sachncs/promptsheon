package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/sachncs/promptsheon/internal/buildinfo"
)

const dbStatusOK = "ok"

var startTime = time.Now()

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) error {
	info := buildinfo.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		auditKeyStatus:  "healthy",
		auditKeyVersion: info.Version,
		"uptime":        time.Since(startTime).String(),
	})
	return nil
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) error {
	ready := map[string]any{
		auditKeyStatus: "ready",
		"go":           runtime.Version(),
	}
	if s.db != nil {
		if err := s.db.Ping(r.Context()); err != nil {
			ready[auditKeyStatus] = "not_ready"
			ready["database"] = "unreachable"
			writeJSON(w, http.StatusServiceUnavailable, ready)
			return nil
		}
		ready["database"] = dbStatusOK
	}
	// Leader election: report the current holder so operators
	// can tell which replica is the writer.
	if s.elector != nil {
		leader, _ := s.elector.Current(r.Context())
		ready["leader"] = leader.Identity
		ready["leader_expires_at"] = leader.ExpiresAt.UTC().Format(time.RFC3339)
		if leader.IsLeader {
			ready["role"] = "leader"
		} else {
			ready["role"] = "follower"
		}
	}
	writeJSON(w, http.StatusOK, ready)
	return nil
}

// handleVersion returns the structured build info. The endpoint
// is intentionally unauthenticated so external uptime probes and
// load balancers can read the running version without an API key.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) error {
	writeJSON(w, http.StatusOK, buildinfo.Get())
	return nil
}
