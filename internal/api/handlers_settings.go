package api

import (
	"encoding/json"
	"net/http"
	"strings"
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

// handleListSettings returns every key. Secret-shaped values
// are masked to "***" (see internal/settings/secret_keys.go).
func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.ListSystemConfig(r.Context())
	if err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, settingsResponse(r.Key, r.Value, r.UpdatedBy, r.UpdatedAt))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
	return nil
}

// handleGetSetting returns one key (or 404).
func (s *Server) handleGetSetting(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	if key == "" {
		return badRequest("key is required")
	}
	v, updatedAt, err := s.db.GetSystemConfig(r.Context(), key)
	if err != nil {
		return notFound("setting: " + key)
	}
	writeJSON(w, http.StatusOK, settingsResponse(key, v, "", updatedAt))
	return nil
}

// handleSetSetting upserts one key. Body is `{value: ...}`.
// Secret keys (`internal/settings.IsSecretKey`) are rejected
// at this layer — secrets go through the vault, not settings.
//
// The response is 200 only after the synchronous hot-reload
// notifier (commit A3 + A4) has finished. The notifier is
// currently empty; the PUT returns immediately.
func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) error {
	if s.settingsMode == "env-only" {
		return forbidden("settings: PROMPTSHEON_SETTINGS_MODE=env-only; writes disabled")
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
	if err := s.db.SetSystemConfig(r.Context(), key, req.Value, updatedBy); err != nil {
		return err
	}
	if s.settingsNotif != nil {
		s.settingsNotif.Publish(key, req.Value)
	}
	s.audit(r.Context(), "create", "setting:"+key, map[string]any{
		"key":  key,
		"by":   updatedBy,
	})
	writeJSON(w, http.StatusOK, settingsResponse(key, req.Value, updatedBy, timeNow()))
	return nil
}

// handleDeleteSetting removes one key. 404 on miss.
func (s *Server) handleDeleteSetting(w http.ResponseWriter, r *http.Request) error {
	if s.settingsMode == "env-only" {
		return forbidden("settings: PROMPTSHEON_SETTINGS_MODE=env-only; writes disabled")
	}
	key := r.PathValue("key")
	if key == "" {
		return badRequest("key is required")
	}
	if err := s.db.DeleteSystemConfig(r.Context(), key); err != nil {
		if strings.Contains(err.Error(), "sql: no rows") {
			return notFound("setting: " + key)
		}
		return err
	}
	if s.settingsNotif != nil {
		s.settingsNotif.Publish(key, "")
	}
	s.audit(r.Context(), "delete", "setting:"+key, map[string]any{
		"key": key,
		"by": settingsUpdatedBy(r),
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
		"key":        key,
		fieldSettingsValue: display,
		"updated_by": updatedBy,
		"updated_at": updatedAt,
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
