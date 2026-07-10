package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// mockStore implements APIKeyStore for testing.
type mockStore struct {
	keys         map[string]*APIKeyRecord
	updateCalls  atomic.Int64
	updateBlocks chan struct{} // closed to unblock all blocked calls
}

func (m *mockStore) GetAPIKeyByHash(_ context.Context, keyHash string) (*APIKeyRecord, error) {
	if r, ok := m.keys[keyHash]; ok {
		return r, nil
	}
	return nil, nil
}

func (m *mockStore) UpdateAPIKeyLastUsed(_ context.Context, _ string) error {
	m.updateCalls.Add(1)
	if m.updateBlocks != nil {
		<-m.updateBlocks // wait until test closes the channel
	}
	return nil
}

func TestGenerateAPIKey(t *testing.T) {
	key, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if len(key) != 67 { // "ps_" + 64 hex chars
		t.Errorf("key length = %d, want 67", len(key))
	}
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	if key == hash {
		t.Error("key and hash should differ")
	}
}

func TestHashAPIKey(t *testing.T) {
	h1 := HashAPIKey("ps_abc123")
	h2 := HashAPIKey("ps_abc123")
	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64", len(h1))
	}
}

func TestValidateAPIKeyFormat(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"ps_" + strings.Repeat("a", 64), true},
		{"ps_short", false},
		{"not_ps_abcdefghijklmnopqrstuvwxyz1234567890abcdefghij", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ValidateAPIKeyFormat(tt.key); got != tt.want {
			t.Errorf("ValidateAPIKeyFormat(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestHasPermission(t *testing.T) {
	if !HasPermission(RoleAdmin, PermPromptCreate) {
		t.Error("admin should have prompt:create")
	}
	if !HasPermission(RoleAdmin, PermUserManage) {
		t.Error("admin should have user:manage")
	}
	if HasPermission(RoleReader, PermPromptCreate) {
		t.Error("reader should not have prompt:create")
	}
	if !HasPermission(RoleWriter, PermPromptCreate) {
		t.Error("writer should have prompt:create")
	}
	if HasPermission(RoleWriter, PermUserManage) {
		t.Error("writer should not have user:manage")
	}
	if !HasPermission(RoleReader, PermPromptRead) {
		t.Error("reader should have prompt:read")
	}
}

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "bearer header",
			req:  httptest.NewRequest("GET", "/", nil),
			want: "ps_abc123",
		},
		{
			name: "query param ignored",
			req:  httptest.NewRequest("GET", "/?api_key=ps_xyz789", nil),
			want: "",
		},
		{
			name: "no auth",
			req:  httptest.NewRequest("GET", "/", nil),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "bearer header" {
				tt.req.Header.Set("Authorization", "Bearer ps_abc123")
			}
			if got := extractAPIKey(tt.req); got != tt.want {
				t.Errorf("extractAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUserContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), userContextKey, &User{ID: "u1", Role: RoleAdmin})
	u, ok := UserFromContext(ctx)
	if !ok {
		t.Fatal("expected user in context")
	}
	if u.ID != "u1" {
		t.Errorf("user ID = %q, want %q", u.ID, "u1")
	}
	if u.Role != RoleAdmin {
		t.Errorf("user role = %q, want %q", u.Role, RoleAdmin)
	}

	_, ok = UserFromContext(context.Background())
	if ok {
		t.Error("expected no user in empty context")
	}
}

func TestAuthenticator_MissingKey(t *testing.T) {
	s := &mockStore{keys: make(map[string]*APIKeyRecord)}
	a := NewAuthenticator(s)
	req := httptest.NewRequest("GET", "/", nil)
	_, err := a.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestAuthenticator_InvalidKey(t *testing.T) {
	s := &mockStore{keys: make(map[string]*APIKeyRecord)}
	a := NewAuthenticator(s)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ps_invalidkey")
	_, err := a.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestAuthenticator_ValidKey(t *testing.T) {
	key, hash, _ := GenerateAPIKey()
	s := &mockStore{
		keys: map[string]*APIKeyRecord{
			hash: {ID: "k1", UserID: "u1", Role: string(RoleWriter)},
		},
	}
	a := NewAuthenticator(s)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	user, err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "u1" {
		t.Errorf("user ID = %q, want %q", user.ID, "u1")
	}
	if user.Role != RoleWriter {
		t.Errorf("user role = %q, want %q", user.Role, RoleWriter)
	}
}

func TestAuthenticator_ExpiredKey(t *testing.T) {
	key, hash, _ := GenerateAPIKey()
	past := time.Now().Add(-1 * time.Hour)
	s := &mockStore{
		keys: map[string]*APIKeyRecord{
			hash: {ID: "k1", UserID: "u1", Role: string(RoleAdmin), ExpiresAt: &past},
		},
	}
	a := NewAuthenticator(s)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	_, err := a.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for expired key")
	}
}

func TestAuthenticatorMiddleware(t *testing.T) {
	key, hash, _ := GenerateAPIKey()
	s := &mockStore{
		keys: map[string]*APIKeyRecord{
			hash: {ID: "k1", UserID: "u1", Role: string(RoleAdmin)},
		},
	}
	a := NewAuthenticator(s)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "no user", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("user:" + u.ID))
	})

	handler := a.AuthenticateMiddleware(inner)

	// Request without key.
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no key: code = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	// Request with valid key.
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("valid key: code = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "user:u1" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "user:u1")
	}
}

// TestAuthenticate_UpdateLastUsedIsNonBlocking pins the H-5 fix: a
// synchronous UpdateAPIKeyLastUsed that blocks the request goroutine
// is unsafe because a slow DB can stall the request path. The fix
// moves the call to a background goroutine; the request hot path
// returns promptly even if the store is wedged.
func TestAuthenticate_UpdateLastUsedIsNonBlocking(t *testing.T) {
	key, hash, _ := GenerateAPIKey()
	store := &mockStore{
		keys: map[string]*APIKeyRecord{
			hash: {ID: "k1", UserID: "u1", Role: string(RoleAdmin)},
		},
		updateBlocks: make(chan struct{}),
	}
	auth := NewAuthenticator(store)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)

	// Bound Authenticate to 500ms. The pre-fix implementation would
	// block for as long as the store's updateBlocks channel stays
	// open. The post-fix implementation returns immediately and
	// the update happens on the background goroutine.
	done := make(chan struct{})
	var authErr error
	go func() {
		_, authErr = auth.Authenticate(req)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Authenticate blocked the request goroutine on UpdateAPIKeyLastUsed")
	}
	if authErr != nil {
		t.Fatalf("unexpected error: %v", authErr)
	}
	// Give the worker a moment to pick up the queued update.
	time.Sleep(100 * time.Millisecond)
	if store.updateCalls.Load() == 0 {
		t.Fatal("expected background worker to invoke UpdateAPIKeyLastUsed")
	}
	// Unblock the worker and stop.
	close(store.updateBlocks)
	auth.Stop()
}

func TestAuthorizer_Require(t *testing.T) {
	az := NewAuthorizer()
	handler := az.Require(PermPromptCreate)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No user in context.
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no user: code = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	// User with wrong role.
	ctx := context.WithValue(context.Background(), userContextKey, &User{ID: "u1", Role: RoleReader})
	req = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("wrong role: code = %d, want %d", rr.Code, http.StatusForbidden)
	}

	// User with correct role.
	ctx = context.WithValue(context.Background(), userContextKey, &User{ID: "u1", Role: RoleWriter})
	req = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("correct role: code = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWithUserContext(t *testing.T) {
	u := &User{ID: "u42", Role: RoleAdmin}
	ctx := WithUserContext(context.Background(), u)
	got, ok := UserFromContext(ctx)
	if !ok {
		t.Fatal("expected user in context")
	}
	if got.ID != "u42" || got.Role != RoleAdmin {
		t.Errorf("got {ID: %q, Role: %q}, want {ID: u42, Role: admin}", got.ID, got.Role)
	}
}

func TestHasPermissionUnknownRole(t *testing.T) {
	if HasPermission("superadmin", PermPromptRead) {
		t.Error("expected false for unknown role")
	}
}

func TestAuthenticator_RevokedKey(t *testing.T) {
	key, hash, _ := GenerateAPIKey()
	s := &mockStore{
		keys: map[string]*APIKeyRecord{
			hash: {ID: "k1", UserID: "u1", Role: string(RoleReader), Revoked: true},
		},
	}
	a := NewAuthenticator(s)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	_, err := a.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for revoked key")
	}
}

// mockLogger records auth failures for testing.
type mockLogger struct {
	failures []struct{ keyPrefix, reason, remoteAddr string }
}

func (m *mockLogger) LogAuthFailure(_ context.Context, keyPrefix, reason, remoteAddr string) {
	m.failures = append(m.failures, struct{ keyPrefix, reason, remoteAddr string }{keyPrefix, reason, remoteAddr})
}

func TestNewAuthenticatorWithLogger(t *testing.T) {
	logger := &mockLogger{}
	s := &mockStore{keys: make(map[string]*APIKeyRecord)}
	a := NewAuthenticatorWithLogger(s, logger)

	req := httptest.NewRequest("GET", "/", nil)
	_, err := a.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if len(logger.failures) != 1 {
		t.Fatalf("expected 1 logged failure, got %d", len(logger.failures))
	}
	if logger.failures[0].reason != "missing api key" {
		t.Errorf("reason = %q, want %q", logger.failures[0].reason, "missing api key")
	}
	a.Stop()
}

func TestAuthenticator_StopMultipleTimes(_ *testing.T) {
	s := &mockStore{keys: make(map[string]*APIKeyRecord)}
	a := NewAuthenticator(s)
	a.Stop()
	a.Stop() // must not panic or hang
}
