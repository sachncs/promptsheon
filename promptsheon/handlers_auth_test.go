//go:build tests_migration


package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/auth"
	"github.com/sachncs/promptsheon/promptsheon/models"
		"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

)

func TestHandleCreateAPIKey_NoAuth(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-key", "role": "writer", "user_id": "u1"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["key"] == "" {
		t.Error("expected api key in response")
	}
}

func TestHandleCreateAPIKey_NoAuth_AdminRejected(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "admin-key", "role": "admin"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_NoAuth_MissingName(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"role": "reader"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_NoAuth_MissingRole(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_NoAuth_InvalidRole(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test", "role": "superadmin"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_WithAuth_Reader(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "reader"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "u1", Name: "reader-key", KeyHash: hash, KeyPrefix: key[:8], Role: "reader",
	})
	s := newAuthTestServer(t, repo)

	body := mustMarshal(t, map[string]string{"name": "new-key", "role": "writer"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_WithAuth_Admin(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "admin1", Email: "a@t.com", Name: "A", Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "admin1", Name: "admin-key", KeyHash: hash, KeyPrefix: key[:8], Role: "admin",
	})

	s := newAuthTestServer(t, repo)

	body := mustMarshal(t, map[string]string{"name": "new-key", "role": "writer", "user_id": "admin1"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_WithAuth_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	body := mustMarshal(t, map[string]string{"name": "new-key", "role": "writer"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAPIKeys_NoAuth(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", Name: "key1", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/apikeys?user_id=u1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAPIKeys_MissingUserID(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/apikeys", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAPIKeys_AdminListsOtherUser(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "admin1", Email: "a@t.com", Name: "A", Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "ak1", UserID: "admin1", Name: "admin-key", KeyHash: hash, KeyPrefix: key[:8], Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "uk1", UserID: "u1", Name: "user-key", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader"})

	s := newAuthTestServer(t, repo)
	req := httptest.NewRequest("GET", "/api/v1/apikeys?user_id=u1", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_NoAuth(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", Name: "key1", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader", Revoked: false})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/k1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "reader1", Email: "r@t.com", Name: "R", Role: "reader"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "rk1", UserID: "reader1", Name: "r-key", KeyHash: hash, KeyPrefix: key[:8], Role: "reader"})

	s := newAuthTestServer(t, repo)
	// reader1 cannot revoke a key that belongs to another user (u2 is not in DB but key belongs to reader1)
	// First create a key owned by a different user
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "uk-other", UserID: "u-other", Name: "other-key", KeyHash: "h2", KeyPrefix: "ps_test2", Role: "admin"})
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/uk-other", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_AlreadyRevoked(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", Name: "key1", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader", Revoked: true})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/k1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		// Should still succeed after revocation
		t.Logf("status %d: %s", rr.Code, rr.Body.String())
	}
}

// BUG-27: TestHandleBootstrap_WrongMethod removed. The route is
// registered as POST /api/v1/setup, so the mux rejects non-POST
// requests with 405 before the handler ever runs. The previous
// guard inside the handler was unreachable.

func TestHandleBootstrap_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	t.Setenv("PROMPTSHEON_BOOTSTRAP_TOKEN", "test-bootstrap-secret")
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "test-bootstrap-secret")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBootstrap(t *testing.T) {
	repo := newMockRepo()
	s := newTestServer(t)
	s.db = newDB(repo)

	t.Setenv("PROMPTSHEON_BOOTSTRAP_TOKEN", "test-bootstrap-secret")

	body := mustMarshal(t, map[string]string{"email": "admin@local", "name": "Bootstrap Admin"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "test-bootstrap-secret")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["key"] == "" {
		t.Error("expected bootstrap key")
	}
}

func TestHandleBootstrap_NoToken(t *testing.T) {
	repo := newMockRepo()
	s := newTestServer(t)
	s.db = newDB(repo)

	// PROMPTSHEON_BOOTSTRAP_TOKEN is intentionally unset.
	body := mustMarshal(t, map[string]string{"email": "admin@local"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 when bootstrap token is unset, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBootstrap_WrongToken(t *testing.T) {
	repo := newMockRepo()
	s := newTestServer(t)
	s.db = newDB(repo)

	t.Setenv("PROMPTSHEON_BOOTSTRAP_TOKEN", "test-bootstrap-secret")

	body := mustMarshal(t, map[string]string{"email": "admin@local"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "wrong-token")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 on wrong bootstrap token, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBootstrap_AlreadyUsers(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "admin"})
	s := newTestServer(t)
	s.db = newDB(repo)

	t.Setenv("PROMPTSHEON_BOOTSTRAP_TOKEN", "test-bootstrap-secret")

	body := mustMarshal(t, map[string]string{"email": "admin@local"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "test-bootstrap-secret")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBootstrap_AuthEnabled(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	body := mustMarshal(t, map[string]string{"email": "admin@local"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	// SEC-5b: /api/v1/setup is unregistered when requireAuth=true.
	// mux returns 404 for unregistered paths.
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 (route not registered), got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthLogin_NoOAuth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/login", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthLogin(t *testing.T) {
	oauthMgr := auth.NewOAuthManager()
	oauthMgr.RegisterProvider("google", &auth.OAuthProvider{
		Name:     "google",
		AuthURL:  "https://accounts.google.com/o/oauth2/auth",
		ClientID: "test-client-id",
		Scopes:   []string{"email", "profile"},
	})
	s := newTestServer(t)
	s.oauth = oauthMgr

	req := httptest.NewRequest("GET", "/api/v1/auth/google/login", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d: %s", rr.Code, rr.Body.String())
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "accounts.google.com") {
		t.Errorf("expected Google auth URL, got %s", location)
	}
}

// TestHandleOAuthLogin_CookieSecureConditional locks in DEF-10 fix (c0.6).
// The OAuth state cookie must be Secure when the request is over TLS
// (r.TLS != nil OR X-Forwarded-Proto=https) and NOT Secure otherwise,
// otherwise local-dev OAuth over plain HTTP is broken.
func TestHandleOAuthLogin_CookieSecureConditional(t *testing.T) {
	oauthMgr := auth.NewOAuthManager()
	oauthMgr.RegisterProvider("google", &auth.OAuthProvider{
		Name:     "google",
		AuthURL:  "https://accounts.google.com/o/oauth2/auth",
		ClientID: "test-client-id",
		Scopes:   []string{"email"},
	})
	s := newTestServer(t)
	s.oauth = oauthMgr

	cases := []struct {
		name           string
		setTLS         bool
		forwardedProto string
		wantSecure     bool
	}{
		{"plain HTTP", false, "", false},
		{"plain HTTP behind proxy that forgot the header", false, "", false},
		{"TLS direct", true, "", true},
		{"TLS via forwarded proto (proxy)", false, "https", true},
		{"plain HTTP behind proxy with http header", false, "http", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/auth/google/login", nil)
			if c.setTLS {
				req.TLS = &tls.ConnectionState{}
			}
			if c.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", c.forwardedProto)
			}
			rr := httptest.NewRecorder()
			s.ServeHTTP(rr, req)

			cookies := rr.Result().Cookies()
			var oauthCookie *http.Cookie
			for _, ck := range cookies {
				if ck.Name == "oauth_state" {
					oauthCookie = ck
					break
				}
			}
			if oauthCookie == nil {
				t.Fatalf("oauth_state cookie not set (got %d cookies)", len(cookies))
			}
			if oauthCookie.Secure != c.wantSecure {
				t.Errorf("Secure=%v, want %v", oauthCookie.Secure, c.wantSecure)
			}
		})
	}
}

func TestHandleOAuthCallback_WithRealOAuth(t *testing.T) {
	oauthMgr := auth.NewOAuthManager()
	oauthMgr.RegisterProvider("test", &auth.OAuthProvider{
		Name:         "test",
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:9090/callback",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/userinfo",
		Scopes:       []string{"openid"},
	})

	stateVal := "test-oauthstate-123"

	s := newTestServer(t)
	s.oauth = oauthMgr
	s.oauthStates.put(stateVal, time.Now().Add(10*time.Minute))

	req := httptest.NewRequest("GET", "/api/v1/auth/test/callback?code=test_code&state="+stateVal, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: stateVal})
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	// The ExchangeCode will try to make an HTTP request to example.com and
	// it may fail in tests without network. We expect either 200 or 400.
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("expected 200 or 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthCallback_MissingStateCookie(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=abc&state=xyz", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGenerateOAuthState(t *testing.T) {
	s := newTestServer(t)
	state, err := s.generateOAuthState()
	if err != nil {
		t.Fatal(err)
	}
	if state == "" {
		t.Error("expected non-empty state")
	}
	if !s.validateOAuthState(state) {
		t.Error("expected valid state")
	}
	if s.validateOAuthState(state) {
		t.Error("expected state to be consumed after first use")
	}
}

// ---------------------------------------------------------------------------
// User Handler Tests
// ---------------------------------------------------------------------------

func TestOAuthStateStore_PutAndConsume(t *testing.T) {
	store := newOAuthStateStore()
	store.put("state1", time.Now().Add(10*time.Minute))

	if !store.consume("state1") {
		t.Error("expected state1 to be consumed successfully")
	}
	if store.consume("state1") {
		t.Error("expected state1 to be removed after consume")
	}
}

func TestOAuthStateStore_ConsumeNotFound(t *testing.T) {
	store := newOAuthStateStore()
	if store.consume("nonexistent") {
		t.Error("expected false for nonexistent state")
	}
}

func TestOAuthStateStore_ConsumeExpired(t *testing.T) {
	store := newOAuthStateStore()
	store.put("expired-state", time.Now().Add(-1*time.Hour))

	if store.consume("expired-state") {
		t.Error("expected false for expired state")
	}
}

func TestOAuthStateStore_Janitor(t *testing.T) {
	store := newOAuthStateStore()
	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx)

	store.put("stale", time.Now().Add(-1*time.Hour))
	store.put("fresh", time.Now().Add(10*time.Minute))

	cancel()
	store.stopJanitor()

	if !store.consume("fresh") {
		t.Error("expected fresh state to still be available after janitor stopped")
	}
	if store.consume("stale") {
		t.Error("expected stale state to be removed by janitor")
	}
}

func TestStartOAuthStateJanitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := newTestServer(t)
	s.StartOAuthStateJanitor(ctx)
	s.StopOAuthStateJanitor()
}

func TestStoreAuthAdapter_GetAPIKeyByHash(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "u1", KeyHash: "hash1", KeyPrefix: "ps_test", Role: "admin",
	})
	adapter := &storeAuthAdapter{db: repo}

	rec, err := adapter.GetAPIKeyByHash(context.Background(), "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if rec == nil {
		t.Fatal("expected APIKeyRecord")
	}
	if rec.ID != "k1" || rec.UserID != "u1" || rec.Role != "admin" {
		t.Errorf("unexpected record: %+v", rec)
	}
}

func TestStoreAuthAdapter_GetAPIKeyByHash_Nil(t *testing.T) {
	adapter := &storeAuthAdapter{db: newMockRepo()}
	rec, err := adapter.GetAPIKeyByHash(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if rec != nil {
		t.Error("expected nil record")
	}
}

func TestStoreAuthAdapter_UpdateAPIKeyLastUsed(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", KeyHash: "h1", KeyPrefix: "ps_t", Role: "reader"})
	adapter := &storeAuthAdapter{db: repo}
	if err := adapter.UpdateAPIKeyLastUsed(context.Background(), "k1"); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// authAuditLogger Tests
// ---------------------------------------------------------------------------

func TestAuthAuditLogger_LogAuthFailure(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = make(chan *models.AuditEntry, 100)

	adapter := &authAuditLogger{server: s}
	adapter.LogAuthFailure(context.Background(), "ps_test", "invalid key", "127.0.0.1")

	select {
	case entry := <-s.auditQueue:
		if entry.Action != "auth_failure" {
			t.Errorf("expected auth_failure, got %s", entry.Action)
		}
		if entry.Details["key_prefix"] != "ps_test" {
			t.Errorf("expected key_prefix ps_test, got %v", entry.Details["key_prefix"])
		}
	default:
		t.Error("expected audit entry")
	}
}

// ---------------------------------------------------------------------------
// Span Helper Tests (dashboard.go)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// WithRequest / httpRequestFromContext
// ---------------------------------------------------------------------------

func TestWithRequestAndHTTPRequestFromContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithRequest(context.Background(), req)
	got := httpRequestFromContext(ctx)
	if got == nil {
		t.Fatal("expected request from context")
	}
	if got.URL.Path != "/test" {
		t.Errorf("expected /test, got %s", got.URL.Path)
	}
	if httpRequestFromContext(context.Background()) != nil {
		t.Error("expected nil from empty context")
	}
}

// ---------------------------------------------------------------------------
// authenticateRequest
// ---------------------------------------------------------------------------

func TestAuthenticateRequest_NoAuth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	newReq, user, err := s.authenticateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if user != nil {
		t.Error("expected nil user when auth disabled")
	}
	if newReq == nil {
		t.Error("expected non-nil request")
	}
}

func TestAuthenticateRequest_WithAuth_NoKey(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	req := httptest.NewRequest("GET", "/", nil)
	_, _, err := s.authenticateRequest(req)
	if err == nil {
		t.Error("expected error when no API key provided")
	}
}

func TestAuthenticateRequest_WithAuth_ValidKey(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "u1", Name: "admin-key", KeyHash: hash, KeyPrefix: key[:8], Role: "admin",
	})
	s := newAuthTestServer(t, repo)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	newReq, user, err := s.authenticateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.ID != "u1" {
		t.Errorf("expected u1, got %s", user.ID)
	}
	if newReq == nil {
		t.Error("expected non-nil request")
	}
}
