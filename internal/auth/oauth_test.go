package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOAuthManager_GetAuthURL(t *testing.T) {
	mgr := NewOAuthManager()
	mgr.RegisterProvider("google", DefaultGoogleProvider("client-id", "client-secret", "http://localhost:8080/callback"))

	url, err := mgr.GetAuthURL("google", "test-state")
	if err != nil {
		t.Fatal(err)
	}
	if url == "" {
		t.Fatal("expected non-empty auth URL")
	}
	if !contains(url, "client_id=client-id") {
		t.Fatal("expected client_id in URL")
	}
	if !contains(url, "state=test-state") {
		t.Fatal("expected state in URL")
	}
}

func TestOAuthManager_GetAuthURL_UnknownProvider(t *testing.T) {
	mgr := NewOAuthManager()
	_, err := mgr.GetAuthURL("unknown", "state")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestOAuthManager_ExchangeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			token := OAuthToken{
				AccessToken:  "test-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "refresh-token",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(token)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mgr := NewOAuthManager()
	mgr.RegisterProvider("test", &OAuthProvider{
		Name:         "test",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/callback",
		TokenURL:     server.URL + "/token",
	})

	token, err := mgr.ExchangeCode(context.Background(), "test", "auth-code")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "test-token" {
		t.Fatalf("expected test-token, got %s", token.AccessToken)
	}
	if token.ExpiresIn != 3600 {
		t.Fatalf("expected 3600, got %d", token.ExpiresIn)
	}
}

func TestOAuthManager_GetUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/userinfo" {
			user := OAuthUser{
				ID:    "123",
				Email: "test@example.com",
				Name:  "Test User",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mgr := NewOAuthManager()
	mgr.RegisterProvider("test", &OAuthProvider{
		Name:        "test",
		UserInfoURL: server.URL + "/userinfo",
	})

	token := &OAuthToken{AccessToken: "test-token"}
	user, err := mgr.GetUserInfo(context.Background(), "test", token)
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "test@example.com" {
		t.Fatalf("expected test@example.com, got %s", user.Email)
	}
	if user.Provider != "test" {
		t.Fatalf("expected test provider, got %s", user.Provider)
	}
}

func TestDefaultProviders(t *testing.T) {
	google := DefaultGoogleProvider("id", "secret", "http://localhost/callback")
	if google.Name != "google" {
		t.Fatalf("expected google, got %s", google.Name)
	}
	if google.AuthURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Fatal("unexpected Google auth URL")
	}

	github := DefaultGitHubProvider("id", "secret", "http://localhost/callback")
	if github.Name != "github" {
		t.Fatalf("expected github, got %s", github.Name)
	}
	if github.AuthURL != "https://github.com/login/oauth/authorize" {
		t.Fatal("unexpected GitHub auth URL")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
