package auth

import (
	"strings"
	"testing"
)

// TestRegisterProvider_RejectsSSRF locks in 1.4 / CRIT-3 fix (c0.16).
// Before the fix, RegisterProvider accepted any URL string. Now
// SSRF validation rejects loopback / private / link-local / metadata
// targets on every URL field.
func TestRegisterProvider_RejectsSSRF(t *testing.T) {
	mgr := NewOAuthManager()

	cases := []struct {
		name      string
		authURL   string
		tokenURL  string
		userURL   string
		redirURL  string
		wantError bool
	}{
		{
			name:      "all https external",
			authURL:   "https://accounts.google.com/o/oauth2/auth",
			tokenURL:  "https://oauth2.googleapis.com/token",
			userURL:   "https://openidconnect.googleapis.com/v3/userinfo",
			redirURL:  "https://www.googleapis.com/callback",
			wantError: false,
		},
		{
			name:      "loopback in auth URL",
			authURL:   "http://127.0.0.1:9999/admin",
			tokenURL:  "https://oauth2.googleapis.com/token",
			userURL:   "https://openidconnect.googleapis.com/v3/userinfo",
			redirURL:  "https://www.googleapis.com/callback",
			wantError: true,
		},
		{
			name:      "private IP in token URL",
			authURL:   "https://accounts.google.com/o/oauth2/auth",
			tokenURL:  "http://10.0.0.5/token",
			userURL:   "https://openidconnect.googleapis.com/v3/userinfo",
			redirURL:  "https://www.googleapis.com/callback",
			wantError: true,
		},
		{
			name:      "cloud metadata in user info URL",
			authURL:   "https://accounts.google.com/o/oauth2/auth",
			tokenURL:  "https://oauth2.googleapis.com/token",
			userURL:   "http://169.254.169.254/latest/meta-data/iam/security-credentials/",
			redirURL:  "https://www.googleapis.com/callback",
			wantError: true,
		},
		{
			name:      "loopback in redirect URL",
			authURL:   "https://accounts.google.com/o/oauth2/auth",
			tokenURL:  "https://oauth2.googleapis.com/token",
			userURL:   "https://openidconnect.googleapis.com/v3/userinfo",
			redirURL:  "http://localhost:8080/callback",
			wantError: true,
		},
		{
			name:      "ftp scheme rejected",
			authURL:   "ftp://accounts.google.com/auth",
			tokenURL:  "https://oauth2.googleapis.com/token",
			userURL:   "https://openidconnect.googleapis.com/v3/userinfo",
			redirURL:  "https://app.example.com/callback",
			wantError: true,
		},
		{
			name:      "empty URLs permitted",
			authURL:   "",
			tokenURL:  "",
			userURL:   "",
			redirURL:  "",
			wantError: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prov := &OAuthProvider{
				Name:        "test",
				ClientID:    "cid",
				AuthURL:     c.authURL,
				TokenURL:    c.tokenURL,
				UserInfoURL: c.userURL,
				RedirectURL: c.redirURL,
			}
			err := mgr.RegisterProvider("test", prov)
			if (err != nil) != c.wantError {
				t.Errorf("err=%v, wantError=%v", err, c.wantError)
			}
			if c.wantError && err != nil && !strings.Contains(err.Error(), "test") {
				t.Errorf("error should mention provider name: %v", err)
			}
		})
	}
}

// TestRegisterProvider_NilNoop ensures the documented behaviour
// (RegisterProvider(nil) is a no-op) still holds after the signature
// change to return error.
func TestRegisterProvider_NilNoop(t *testing.T) {
	mgr := NewOAuthManager()
	if err := mgr.RegisterProvider("ignored", nil); err != nil {
		t.Errorf("nil provider should be no-op, got %v", err)
	}
}
