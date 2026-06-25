package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"promptsheon/internal/auth"
	"promptsheon/internal/models"
)

// oauthStates holds in-flight OAuth state tokens. The previous
// implementation used a bare map read/written from arbitrary request
// goroutines — concurrent access would panic, and states that were
// issued but never validated leaked forever.
type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
	stop   chan struct{}
	done   chan struct{}
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{
		states: make(map[string]time.Time),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// start launches a janitor that removes expired entries every minute.
func (s *oauthStateStore) start(ctx context.Context) {
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case now := <-ticker.C:
				s.mu.Lock()
				for k, exp := range s.states {
					if now.After(exp) {
						delete(s.states, k)
					}
				}
				s.mu.Unlock()
			}
		}
	}()
}

func (s *oauthStateStore) stopJanitor() {
	select {
	case <-s.stop:
		// already stopped
	default:
		close(s.stop)
	}
	<-s.done
}

func (s *oauthStateStore) put(state string, exp time.Time) {
	s.mu.Lock()
	s.states[state] = exp
	s.mu.Unlock()
}

func (s *oauthStateStore) consume(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Now().Before(exp)
}

func (s *oauthStateStore) reset() {
	s.mu.Lock()
	s.states = make(map[string]time.Time)
	s.mu.Unlock()
}

var oauthStates = newOAuthStateStore()

// StartOAuthStateJanitor launches the cleanup goroutine.
func StartOAuthStateJanitor(ctx context.Context) {
	oauthStates.start(ctx)
}

// StopOAuthStateJanitor stops the cleanup goroutine.
func StopOAuthStateJanitor() { oauthStates.stopJanitor() }

// resetOAuthStates clears the store; test-only.
func resetOAuthStates() { oauthStates.reset() }

func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	oauthStates.put(state, time.Now().Add(10*time.Minute))
	return state, nil
}

func validateOAuthState(state string) bool {
	return oauthStates.consume(state)
}

// --- API Key Handlers ---

// authenticateRequest runs the configured authenticator on the request and
// attaches the resulting user to the context. Returns nil, false if auth
// is disabled.
func (s *Server) authenticateRequest(r *http.Request) (*http.Request, *auth.User, bool, error) {
	if !s.requireAuth || s.authn == nil {
		return r, nil, false, nil
	}
	user, err := s.authn.Authenticate(r)
	if err != nil {
		return r, nil, false, err
	}
	return r.WithContext(auth.WithUserContext(r.Context(), user)), user, true, nil
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	// Authenticate explicitly because the apikeys route is not wrapped
	// with requirePerm (the create-key route is the bootstrap path for
	// admin keys and is permitted for admin users; non-admins get a
	// self-only key).
	newCtx, caller, _, err := s.authenticateRequest(r)
	if err != nil {
		return unauthorized("authentication required")
	}
	r = newCtx

	var req struct {
		Name      string     `json:"name"`
		UserID    string     `json:"user_id"`
		Role      string     `json:"role"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest("invalid json")
	}
	if req.Name == "" {
		return badRequest("name is required")
	}
	if req.Role == "" {
		return badRequest("role is required")
	}
	if req.Role != string(auth.RoleAdmin) && req.Role != string(auth.RoleWriter) && req.Role != string(auth.RoleReader) {
		return badRequest("role must be admin, writer, or reader")
	}

	// Resolve target user + role. When auth is enabled, we ignore the
	// body for non-admin callers (you can only mint a key for yourself
	// with the role you already hold). When auth is disabled we honour
	// the body for backward compatibility — but refuse to mint
	// `admin`-role keys, which would give the holder full control
	// over the deployment. This closes H-1: previously, setting
	// PROMPTSHEON_AUTH=false (which .env.example and README both
	// suggest for local dev) let any anonymous caller POST
	// `{role:"admin"}` and walk away with an admin key.
	_, hasCaller := auth.UserFromContext(r.Context())
	var targetUserID, targetRole string
	if hasCaller {
		targetUserID = caller.ID
		targetRole = string(caller.Role)
		if auth.HasPermission(caller.Role, auth.PermAPIKeyManage) {
			if req.UserID != "" {
				targetUserID = req.UserID
			}
			if req.Role != "" {
				targetRole = req.Role
			}
		} else if req.UserID != "" && req.UserID != caller.ID {
			return forbidden("only admins may create keys for other users")
		} else if req.Role != "" && req.Role != string(caller.Role) {
			return forbidden("only admins may create keys with a different role")
		}
	} else if s.requireAuth {
		return unauthorized("authentication required")
	} else {
		// No-auth mode (PROMPTSHEON_AUTH=false). Admin keys are the
		// highest-trust credential, and minting them without
		// authentication is a privilege-escalation vector. Refuse the
		// request before it reaches the database. Reader/Writer keys
		// are still honoured for backward compatibility with the
		// local-development workflow.
		if req.Role == string(auth.RoleAdmin) {
			return forbidden("admin keys cannot be minted in no-auth mode (set PROMPTSHEON_AUTH=true)")
		}
		targetUserID = req.UserID
		targetRole = req.Role
	}

	// Ensure the target user actually exists. In legacy (no-auth) mode
	// we accept the user_id at face value for backward compatibility
	// with the test suite. In auth-enabled mode we reject unknown users
	// to prevent orphan keys.
	if targetUserID != "" && s.requireAuth {
		if _, err := s.db.GetUser(r.Context(), targetUserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return badRequest("user not found")
			}
			return err
		}
	}

	key, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return err
	}

	apiKey := &models.APIKey{
		ID:        generateID(),
		UserID:    targetUserID,
		Name:      req.Name,
		KeyHash:   hash,
		KeyPrefix: key[:8],
		Role:      targetRole,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.db.CreateAPIKey(r.Context(), apiKey); err != nil {
		return err
	}

	type response struct {
		*models.APIKey
		Key string `json:"key"`
	}
	writeJSON(w, http.StatusCreated, response{APIKey: apiKey, Key: key})
	return nil
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) error {
	newCtx, _, _, err := s.authenticateRequest(r)
	if err != nil {
		return unauthorized("authentication required")
	}
	r = newCtx
	caller, hasCaller := auth.UserFromContext(r.Context())
	userID := r.URL.Query().Get("user_id")
	if userID == "" && hasCaller {
		userID = caller.ID
	}
	if userID == "" {
		return badRequest("user_id is required")
	}
	if hasCaller && userID != caller.ID {
		if !auth.HasPermission(caller.Role, auth.PermAPIKeyManage) {
			return forbidden("only admins may list keys for other users")
		}
	}

	keys, err := s.db.ListAPIKeysByUser(r.Context(), userID)
	if err != nil {
		return err
	}
	if keys == nil {
		keys = []*models.APIKey{}
	}
	writeJSON(w, http.StatusOK, keys)
	return nil
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) error {
	newCtx, _, _, err := s.authenticateRequest(r)
	if err != nil {
		return unauthorized("authentication required")
	}
	r = newCtx
	caller, hasCaller := auth.UserFromContext(r.Context())
	if !hasCaller && s.requireAuth {
		return unauthorized("authentication required")
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	if id == "" {
		return badRequest("key id is required")
	}

	key, err := s.db.GetAPIKeyByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("api key")
		}
		return err
	}
	if hasCaller && key.UserID != caller.ID {
		if !auth.HasPermission(caller.Role, auth.PermAPIKeyManage) {
			return forbidden("only the owner or an admin may revoke this key")
		}
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

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

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

	token, err := s.oauth.ExchangeCode(r.Context(), providerName, code)
	if err != nil {
		return badRequest("token exchange failed: " + err.Error())
	}

	user, err := s.oauth.GetUserInfo(r.Context(), providerName, token)
	if err != nil {
		return badRequest("user info failed: " + err.Error())
	}

	existing, err := s.db.GetUserByEmail(r.Context(), user.Email)
	if err == sql.ErrNoRows {
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

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": existing,
		"key":  apiKey,
	})
	return nil
}
