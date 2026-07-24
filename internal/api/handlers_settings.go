package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/settings"
)

const fieldSettingsValue = "value"

// registerSettingsRoutes wires the four /api/v1/settings
// routes. GET is PermSettingsRead (every role); PUT and
// DELETE are PermSettingsWrite (admin-only by default).
//
// The PUT and DELETE handlers also gate on the env-only mode:
// if Config.SettingsMode == "env-only" the write returns 403
// even for an admin key (operator set the locked-baseline
// story at boot and a runtime PUT can't override it).
func (s *Server) registerSettingsRoutes() {
	s.mux.HandleFunc("GET /api/v1/settings",
		s.wrapHandler(s.requirePerm(auth.PermSettingsRead)(s.handleListSettings)))
	s.mux.HandleFunc("GET /api/v1/settings/{key}",
		s.wrapHandler(s.requirePerm(auth.PermSettingsRead)(s.handleGetSetting)))
	s.mux.HandleFunc("PUT /api/v1/settings/{key}",
		s.wrapHandler(s.requirePerm(auth.PermSettingsWrite)(s.handleSetSetting)))
	s.mux.HandleFunc("DELETE /api/v1/settings/{key}",
		s.wrapHandler(s.requirePerm(auth.PermSettingsWrite)(s.handleDeleteSetting)))
}

// settingsResolver is the per-Request view of the settings
// layer. The Server owns the canonical *settings.Resolver;
// handlers get a thin wrapper that knows the per-process
// replica id and points at the right Store. Constructed
// lazily so legacy tests that don't wire a settings layer
// fall back to a no-op (handlers return 503).
func (s *Server) settingsResolver() (*settings.Resolver, error) {
	if s.settingsNotif == nil {
		return nil, errors.New("settings: notifier not configured")
	}
	if s.settingsReplicaID == "" {
		return nil, errors.New("settings: replica id not configured")
	}
	return settings.NewResolver(s.db, s.settingsNotif, nil, s.settingsReplicaID), nil
}

// handleListSettings returns every non-tombstoned key.
// Secret-shaped values are masked to "***" (see
// internal/settings/secret_keys.go).
func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) error {
	res, err := s.settingsResolver()
	if err != nil {
		return err
	}
	rows, err := res.List(r.Context())
	if err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, settingsResponse(row.Key, row.Value, row.UpdatedBy, row.UpdatedAt))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
	return nil
}

// handleGetSetting returns the effective value for one key (or 404).
// Unlike list, which contains persisted live rows only, this endpoint applies
// environment-over-database precedence. Tombstones remain distinguishable
// from live empty values and are treated as missing.
func (s *Server) handleGetSetting(w http.ResponseWriter, r *http.Request) error {
	res, err := s.settingsResolver()
	if err != nil {
		return err
	}
	key := r.PathValue("key")
	if key == "" {
		return badRequest("key is required")
	}
	rec, found, err := res.Lookup(r.Context(), key)
	if err != nil {
		return err
	}
	if !found || rec.Tombstone {
		return notFound("setting: " + key)
	}
	writeJSON(w, http.StatusOK, settingsResponse(key, rec.Value, rec.UpdatedBy, rec.UpdatedAt))
	return nil
}

// handleSetSetting upserts one key. Body is `{value: ...}`.
// Secret keys (`internal/settings.IsSecretKey`) are rejected
// at this layer — secrets go through the vault, not settings.
//
// The response is 200 only after the synchronous hot-reload
// notifier has finished. If any subscriber returns an error,
// the handler propagates it as a 500 so the operator sees the
// failed hot-reload.
func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) error {
	if s.settingsMode == "env-only" {
		return forbidden("settings: PROMPTSHEON_SETTINGS_MODE=env-only; writes disabled")
	}
	res, err := s.settingsResolver()
	if err != nil {
		return err
	}
	key := r.PathValue("key")
	if key == "" {
		return badRequest("key is required")
	}
	if settings.IsSecretKey(key) {
		return badRequest("settings: key is registered as a secret; use the vault")
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest("invalid json body")
	}
	updatedBy := settingsUpdatedBy(r)
	if err := res.Set(r.Context(), key, req.Value, updatedBy); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "setting:"+key, map[string]any{
		fieldAPIKey: key,
		"by":        updatedBy,
	})
	writeJSON(w, http.StatusOK, settingsResponse(key, req.Value, updatedBy, timeNow()))
	return nil
}

// handleDeleteSetting removes one key. 404 on miss. The
// resolver writes a tombstone row (so a concurrent replica's
// Set cannot resurrect the key); the GET surface treats the
// tombstone as missing.
func (s *Server) handleDeleteSetting(w http.ResponseWriter, r *http.Request) error {
	if s.settingsMode == "env-only" {
		return forbidden("settings: PROMPTSHEON_SETTINGS_MODE=env-only; writes disabled")
	}
	res, err := s.settingsResolver()
	if err != nil {
		return err
	}
	key := r.PathValue("key")
	if key == "" {
		return badRequest("key is required")
	}
	// Resolver.Delete handles both "row missing" and "row
	// present": a missing row still gets a tombstone so the
	// eventual merge sees a coherent state.
	if err := res.Delete(r.Context(), key); err != nil {
		return err
	}
	s.audit(r.Context(), "delete", "setting:"+key, map[string]any{
		fieldAPIKey: key,
		"by":        settingsUpdatedBy(r),
	})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// settingsResponse formats a single SystemConfig row for the
// API response. Secret-shaped values are masked; the audit
// chain still records the plaintext write so the operator
// can audit-but-not-display secrets.
func settingsResponse(key, value, updatedBy string, updatedAt time.Time) map[string]any {
	display := value
	if settings.IsSecretKey(key) {
		display = "***"
	}
	return map[string]any{
		fieldAPIKey:        key,
		fieldSettingsValue: display,
		"updated_by":       updatedBy,
		"updated_at":       updatedAt,
	}
}

// settingsUpdatedBy pulls the authenticated user id from the
// request context (matches the audit-chain convention). Falls
// back to "system" when there's no authenticated user.
func settingsUpdatedBy(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil && u.ID != "" {
		return u.ID
	}
	return "system"
}

// timeNow is a tiny wrapper so handlers can mock the clock in
// tests; it defaults to time.Now().
func timeNow() time.Time { return time.Now() }
