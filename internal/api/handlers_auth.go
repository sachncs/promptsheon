package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"promptsheon/internal/auth"
	"promptsheon/internal/models"
)

// OAuth state storage (in-memory for simplicity; use Redis in production)
var oauthStates = make(map[string]time.Time)

func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	oauthStates[state] = time.Now().Add(10 * time.Minute)
	return state, nil
}

func validateOAuthState(state string) bool {
	expiry, ok := oauthStates[state]
	if !ok {
		return false
	}
	delete(oauthStates, state)
	return time.Now().Before(expiry)
}

// --- API Key Handlers ---

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name      string     `json:"name"`
		UserID    string     `json:"user_id"`
		Role      string     `json:"role"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest("invalid json")
	}
	if req.Name == "" || req.UserID == "" || req.Role == "" {
		return badRequest("name, user_id, and role are required")
	}
	if req.Role != string(auth.RoleAdmin) && req.Role != string(auth.RoleWriter) && req.Role != string(auth.RoleReader) {
		return badRequest("role must be admin, writer, or reader")
	}

	key, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return err
	}

	apiKey := &models.APIKey{
		ID:        generateID(),
		UserID:    req.UserID,
		Name:      req.Name,
		KeyHash:   hash,
		KeyPrefix: key[:8],
		Role:      req.Role,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.db.CreateAPIKey(r.Context(), apiKey); err != nil {
		return err
	}

	// Return the key once — it won't be stored.
	type response struct {
		*models.APIKey
		Key string `json:"key"`
	}
	writeJSON(w, http.StatusCreated, response{APIKey: apiKey, Key: key})
	return nil
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) error {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		if u, ok := auth.UserFromContext(r.Context()); ok {
			userID = u.ID
		}
	}
	if userID == "" {
		return badRequest("user_id is required")
	}

	keys, err := s.db.ListAPIKeysByUser(r.Context(), userID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, keys)
	return nil
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) error {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	if id == "" {
		return badRequest("key id is required")
	}

	key, err := s.db.GetAPIKeyByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return notFound("api key")
		}
		return err
	}
	if key.Revoked {
		return badRequest("api key already revoked")
	}

	if err := s.db.DeleteAPIKey(r.Context(), id); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
	return nil
}

// --- OAuth Handlers ---

func (s *Server) handleOAuthLogin(w http.ResponseWriter, r *http.Request) error {
	providerName := r.PathValue("provider")
	if providerName == "" {
		return badRequest("provider is required")
	}

	state, err := generateOAuthState()
	if err != nil {
		return err
	}

	// Store state in session/cookie for verification
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   600,
	})

	// Get auth URL from provider
	if s.oauth == nil {
		return badRequest("OAuth not configured")
	}

	authURL, err := s.oauth.GetAuthURL(providerName, state)
	if err != nil {
		return badRequest(err.Error())
	}

	http.Redirect(w, r, authURL, http.StatusFound)
	return nil
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) error {
	providerName := r.PathValue("provider")
	if providerName == "" {
		return badRequest("provider is required")
	}

	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		return badRequest("missing OAuth state")
	}
	if stateCookie.Value != r.URL.Query().Get("state") {
		return badRequest("invalid OAuth state")
	}
	if !validateOAuthState(stateCookie.Value) {
		return badRequest("OAuth state expired")
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return badRequest("missing authorization code")
	}

	if s.oauth == nil {
		return badRequest("OAuth not configured")
	}

	// Exchange code for token
	token, err := s.oauth.ExchangeCode(r.Context(), providerName, code)
	if err != nil {
		return badRequest("token exchange failed: " + err.Error())
	}

	// Get user info
	user, err := s.oauth.GetUserInfo(r.Context(), providerName, token)
	if err != nil {
		return badRequest("user info failed: " + err.Error())
	}

	// Find or create user
	existing, err := s.db.GetUserByEmail(r.Context(), user.Email)
	if err == sql.ErrNoRows {
		// Create new user
		newUser := &models.User{
			ID:        generateID(),
			Email:     user.Email,
			Name:      user.Name,
			Role:      string(auth.RoleReader),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.db.CreateUser(r.Context(), newUser); err != nil {
			return err
		}
		existing = newUser
	} else if err != nil {
		return err
	}

	// Generate API key for the user
	apiKey, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return err
	}

	apiKeyModel := &models.APIKey{
		ID:        generateID(),
		UserID:    existing.ID,
		Name:      "oauth-login",
		KeyHash:   hash,
		KeyPrefix: apiKey[:8],
		Role:      existing.Role,
		CreatedAt: time.Now(),
	}

	if err := s.db.CreateAPIKey(r.Context(), apiKeyModel); err != nil {
		return err
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": existing,
		"key":  apiKey,
	})
	return nil
}
