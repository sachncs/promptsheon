package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/models"
)

const defaultAdminEmail = "admin@local"
const oauthStateCookie = "oauth_state"
const fieldAPIKey = "key"

// oauthStateStore holds in-flight OAuth state tokens. The previous
// implementation used a package-level `var` shared across all Server
// instances and tests, which made it impossible to run multiple
// servers in the same test binary without state leakage. The fix
// moves the store onto Server; helpers below remain package-level
// and dispatch to the active server, so existing call sites do not
// need to change.
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

// activeOAuthStates is the package-level reference that helpers
// (generateOAuthState, validateOAuthState, StartOAuthStateJanitor)
// consult. It is set on Server construction and reset to nil on
// shutdown. Tests that need a per-test store should set
// activeOAuthStates to a fresh instance; the default points to the
// most recently constructed server's store.
var activeOAuthStates = newOAuthStateStore()

// StartOAuthStateJanitor launches the cleanup goroutine for the
// active server's state store.
func StartOAuthStateJanitor(ctx context.Context) {
	activeOAuthStates.start(ctx)
}

// StopOAuthStateJanitor stops the cleanup goroutine.
func StopOAuthStateJanitor() { activeOAuthStates.stopJanitor() }

func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	activeOAuthStates.put(state, time.Now().Add(10*time.Minute))
	return state, nil
}

func validateOAuthState(state string) bool {
	return activeOAuthStates.consume(state)
}

// --- API Key Handlers ---

// authenticateRequest runs the configured authenticator on the request and
// attaches the resulting user to the context. Returns nil, false if auth
// is disabled.
func (s *Server) authenticateRequest(r *http.Request) (*http.Request, *auth.User, error) {
	if !s.requireAuth || s.authn == nil {
		return r, nil, nil
	}
	user, err := s.authn.Authenticate(r)
	if err != nil {
		return r, nil, err
	}
	return r.WithContext(auth.WithUserContext(r.Context(), user)), user, nil
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	// Authenticate explicitly because the apikeys route is not wrapped
	// with requirePerm (the create-key route is the bootstrap path for
	// admin keys and is permitted for admin users; non-admins get a
	// self-only key).
	newCtx, caller, err := s.authenticateRequest(r)
	if err != nil {
		return unauthorized()
	}
	r = newCtx

	var req struct {
		Name      string     `json:"name"`
		UserID    string     `json:"user_id"`
		Role      string     `json:"role"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
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
	switch {
	case hasCaller:
		targetUserID = caller.ID
		targetRole = string(caller.Role)
		if auth.HasPermission(caller.Role, auth.PermAPIKeyManage) {
			if req.UserID != "" {
				targetUserID = req.UserID
			}
			if req.Role != "" {
				targetRole = req.Role
			}
		} else {
			switch {
			case req.UserID != "" && req.UserID != caller.ID:
				return forbidden("only admins may create keys for other users")
			case req.Role != "" && req.Role != string(caller.Role):
				return forbidden("only admins may create keys with a different role")
			}
		}
	case s.requireAuth:
		return unauthorized()
	default:
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
		if _, e := s.db.GetUser(r.Context(), targetUserID); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
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

	s.audit(r.Context(), "apikey_create", "api_key:"+apiKey.ID, map[string]any{
		"key_prefix":  apiKey.KeyPrefix,
		"target_user": apiKey.UserID,
		"role":        apiKey.Role,
		"name":        apiKey.Name,
	})

	type response struct {
		*models.APIKey
		Key string `json:"key"`
	}
	writeJSON(w, http.StatusCreated, response{APIKey: apiKey, Key: key})
	return nil
}

// handleBootstrap is the first-run endpoint that mints an admin
// user and an admin API key when the deployment is brand-new and
// running with authentication disabled. It is the only place in
// the system where a plaintext admin key is ever returned to a
// caller.
//
// The endpoint is intentionally tiny and fails closed: it
// returns 404 the moment any user record exists, it 403s when
// auth is enabled, and the audit log records every call. The
// documentation in docs/configuration.md and the in-binary
// --help text both warn operators that PROMPTSHEON_AUTH=true
// is the recommended setup and that the bootstrap endpoint is
// for local development only.
//
// SECURITY: the endpoint is unauthenticated and gates entirely
// on "no users exist yet". An attacker who can reach the
// server before the operator does gets a free admin key. This
// is acceptable for the documented use case (local dev with
// PROMPTSHEON_AUTH=false) and unacceptable for any production
// deployment — operators must set PROMPTSHEON_AUTH=true before
// exposing the port.
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return badRequest("POST required")
	}
	// Bootstrap is the documented "first caller wins" path. The
	// standard configuration is PROMPTSHEON_AUTH=false so the
	// first caller can mint an admin key without credentials. The
	// e2e harness and a small number of human operators want the
	// same first-call behaviour even with auth=true: the
	// PROMPTSHEON_BOOTSTRAP_TOKEN env var gates the path. When
	// the env var is set we honour the token; otherwise we
	// require auth=false (the standard production path).
	if s.requireAuth && os.Getenv("PROMPTSHEON_BOOTSTRAP_TOKEN") == "" {
		return forbidden("bootstrap is disabled when authentication is enabled and no PROMPTSHEON_BOOTSTRAP_TOKEN is set")
	}

	// Bootstrap token: when PROMPTSHEON_BOOTSTRAP_TOKEN is set the
	// caller must present it via X-Bootstrap-Token. Without this
	// gate, a misconfigured deployment with auth=false would mint
	// an admin key to the first network-adjacent caller. The token
	// is a deliberate operational safety net, not a replacement
	// for proper authentication.
	if token, want := r.Header.Get("X-Bootstrap-Token"), os.Getenv("PROMPTSHEON_BOOTSTRAP_TOKEN"); want != "" {
		if subtle.ConstantTimeCompare([]byte(token), []byte(want)) != 1 {
			return forbidden("invalid or missing X-Bootstrap-Token")
		}
	}

	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
		return badRequest("invalid json")
	}
	if req.Email == "" {
		req.Email = defaultAdminEmail
	}
	if req.Name == "" {
		req.Name = "Bootstrap Admin"
	}

	now := time.Now()
	admin := &models.User{
		ID:        generateID(),
		Email:     req.Email,
		Name:      req.Name,
		Role:      string(auth.RoleAdmin),
		CreatedAt: now,
		UpdatedAt: now,
	}

	key, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return fmt.Errorf("bootstrap: generate key: %w", err)
	}
	apiKey := &models.APIKey{
		ID:        generateID(),
		UserID:    admin.ID,
		Name:      "bootstrap-admin",
		KeyHash:   hash,
		KeyPrefix: key[:8],
		Role:      string(auth.RoleAdmin),
		CreatedAt: now,
	}

	// Atomic bootstrap: the empty-users check + user insert +
	// API-key insert run in one transaction. SQLite serialises
	// the writes, so two concurrent callers cannot both win:
	// the second one sees ErrConflict and returns 409.
	if err := s.db.BootstrapAdmin(r.Context(), admin, apiKey); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return &HTTPError{Status: http.StatusConflict, Message: "bootstrap is no longer available; the server already has users"}
		}
		return fmt.Errorf("bootstrap: %w", err)
	}

	// Log loudly. The warning is the operator's signal that the
	// bootstrap endpoint was used and that they should now
	// reconfigure with PROMPTSHEON_AUTH=true.
	if s.logger != nil {
		s.logger.Warn("bootstrap endpoint used; admin key minted",
			"user_id", admin.ID,
			"key_prefix", apiKey.KeyPrefix,
			"action", "rotate this key and enable PROMPTSHEON_AUTH=true before exposing the server")
	}

	type response struct {
		User    *models.User   `json:"user"`
		APIKey  *models.APIKey `json:"api_key"`
		Key     string         `json:"key"`
		Message string         `json:"message"`
	}
	writeJSON(w, http.StatusCreated, response{
		User:    admin,
		APIKey:  apiKey,
		Key:     key,
		Message: "Bootstrap admin user created. Store the api_key securely —it is the only time it is returned. Set PROMPTSHEON_AUTH=true and rotate the key before exposing this server.",
	})
	return nil
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) error {
	newCtx, _, err := s.authenticateRequest(r)
	if err != nil {
		return unauthorized()
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
	newCtx, _, err := s.authenticateRequest(r)
	if err != nil {
		return unauthorized()
	}
	r = newCtx
	caller, hasCaller := auth.UserFromContext(r.Context())
	if !hasCaller && s.requireAuth {
		return unauthorized()
	}
	id := r.PathValue("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	}
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

	s.audit(r.Context(), "apikey_revoke", "api_key:"+id, map[string]any{
		"key_prefix":  key.KeyPrefix,
		"target_user": key.UserID,
	})

	writeJSON(w, http.StatusOK, map[string]string{auditKeyStatus: "revoked"})
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
		Name:     oauthStateCookie,
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

	parsed, err := url.Parse(authURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return badRequest("invalid redirect URL")
	}

	// #nosec G710 -- authURL is constructed by oauth.GetAuthURL which returns
	// a URL from the configured OAuth provider (not user input).
	http.Redirect(w, r, authURL, http.StatusFound)
	return nil
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) error {
	providerName := r.PathValue("provider")
	if providerName == "" {
		return badRequest("provider is required")
	}

	stateCookie, err := r.Cookie(oauthStateCookie)
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
		// Do NOT echo err.Error() to the unauthenticated client. The
		// underlying error already wraps the upstream provider's
		// response body, which may contain HTML, internal stack
		// traces, or session-bound information. Log the detail and
		// return a generic message.
		if s.logger != nil {
			s.logger.Error("oauth: token exchange failed",
				"provider", providerName, "err", err)
		}
		return badRequest("oauth exchange failed")
	}

	user, err := s.oauth.GetUserInfo(r.Context(), providerName, token)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("oauth: user info lookup failed",
				"provider", providerName, "err", err)
		}
		return badRequest("oauth user lookup failed")
	}

	existing, err := s.db.GetUserByEmail(r.Context(), user.Email)
	if err == sql.ErrNoRows {
		// OAuth auto-provision is gated behind an explicit env var
		// (PROMPTSHEON_OAUTH_AUTO_PROVISION=true, default false). With
		// auto-provision off, unknown emails are rejected: the
		// caller is told an admin must approve before they can
		// retry. This stops an IdP with self-service signup from
		// polluting the user table; it also stops a misconfigured
		// IdP client_id from authenticating arbitrary external
		// users into a corporate deployment.
		if os.Getenv("PROMPTSHEON_OAUTH_AUTO_PROVISION") != "true" {
			return forbidden("OAuth auto-provision is disabled; an admin must create your account first")
		}
		newUser := &models.User{
			ID:        generateID(),
			Email:     user.Email,
			Name:      user.Name,
			Role:      string(auth.RoleReader),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if e := s.db.CreateUser(r.Context(), newUser); e != nil {
			return e
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
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user":      existing,
		fieldAPIKey: apiKey,
	})
	return nil
}
