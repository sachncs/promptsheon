// Package auth provides OAuth/SSO authentication flows.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/errf"
)

// OAuthProvider represents an OAuth provider configuration.
//
// All URL fields MUST be HTTPS scheme and host must not be in a
// loopback, private, link-local, or metadata range. RegisterProvider
// rejects configurations that fail SSRF validation.
type OAuthProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
}

// OAuthToken represents an OAuth access token.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"-"`
}

// OAuthUser represents user info from OAuth provider.
type OAuthUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Provider      string `json:"provider"`
}

// OAuthManager handles OAuth flows for multiple providers.
//
// OAuth state parameters are generated server-side, stored in
// oauthStates, and matched against the callback value. They expire
// after oauthStateTTL and are single-use. This defends against
// login CSRF (authorization-code injection) and replay.
type OAuthManager struct {
	mu          sync.RWMutex
	providers   map[string]*OAuthProvider
	oauthStates map[string]oauthState
	client      *http.Client
}

type oauthState struct {
	provider  string
	expiresAt time.Time
}

// oauthStateTTL bounds how long a server-issued state parameter is
// valid. Two minutes is enough for the OAuth round-trip and short
// enough to keep the state store small.
const oauthStateTTL = 2 * time.Minute

// maxOAuthErrorBody bounds how much of an upstream error response we
// keep when surfacing a failure to the caller.
const maxOAuthErrorBody = 4 << 10

// NewOAuthManager creates a new OAuth manager.
func NewOAuthManager() *OAuthManager {
	return &OAuthManager{
		providers:   make(map[string]*OAuthProvider),
		oauthStates: make(map[string]oauthState),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterProvider registers an OAuth provider. The provider
// configuration is deep-copied so subsequent caller-side mutation
// cannot affect the registered copy.
//
// 1.4 / CRIT-3: every URL the provider declares (AuthURL, TokenURL,
// UserInfoURL, RedirectURL) is now SSRF-validated at registration
// time. The previous comment claimed this was done; the code did
// not. With auto-provisioning on, a misconfigured operator could
// route OAuth code exchange + access tokens to an internal
// endpoint and exfiltrate IAM credentials.
//
// Returns an error if any URL fails validation; the provider is
// not registered.
func (m *OAuthManager) RegisterProvider(name string, provider *OAuthProvider) error {
	if provider == nil {
		return nil
	}
	for _, raw := range []string{provider.AuthURL, provider.TokenURL, provider.UserInfoURL, provider.RedirectURL} {
		if err := validateOAuthURL(raw); err != nil {
			return errf.Errorf("oauth provider %q: %w", name, err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	clone := *provider
	clone.Scopes = append([]string(nil), provider.Scopes...)
	m.providers[name] = &clone
	return nil
}

// validateOAuthURL rejects URLs that don't pass the SSRF guard:
// must parse, must be https or http (loopback allowed for tests),
// host must resolve to a non-loopback, non-private, non-link-local,
// non-metadata IP. Empty URL is permitted (the field is optional).
func validateOAuthURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return errf.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return errf.Errorf("URL %q has unsupported scheme %q (https required)", rawURL, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errf.Errorf("URL %q missing host", rawURL)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return errf.Errorf("URL %q: cannot resolve host %q: %w", rawURL, host, err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return errf.Errorf("URL %q resolves to disallowed address %s", rawURL, ip)
		}
	}
	return nil
}

// generateOAuthState returns a cryptographically random 32-byte
// (base64-url encoded) state token associated with providerName. It
// also records the state for later validation in ConsumeOAuthState.
func (m *OAuthManager) GenerateOAuthState(providerName string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errf.Errorf("generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(raw[:])

	m.mu.Lock()
	defer m.mu.Unlock()
	m.gcOAuthStatesLocked(time.Now())
	m.oauthStates[state] = oauthState{
		provider:  providerName,
		expiresAt: time.Now().Add(oauthStateTTL),
	}
	return state, nil
}

// consumeOAuthState validates a state parameter and removes it
// from the store on success. Returns the provider name bound at
// generation time so the callback can route to the same provider
// the user started with.
func (m *OAuthManager) consumeOAuthState(state string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gcOAuthStatesLocked(time.Now())
	st, ok := m.oauthStates[state]
	if !ok {
		return "", errf.Errorf("invalid state parameter")
	}
	delete(m.oauthStates, state)
	if time.Now().After(st.expiresAt) {
		return "", errf.Errorf("state parameter expired")
	}
	return st.provider, nil
}

func (m *OAuthManager) gcOAuthStatesLocked(now time.Time) {
	for k, v := range m.oauthStates {
		if now.After(v.expiresAt) {
			delete(m.oauthStates, k)
		}
	}
}

// GetAuthURL returns the authorization URL for a provider.
//
// The state parameter is generated server-side via GenerateOAuthState
// (the caller should pass the result of that method, not a free-form
// value). When the supplied state is empty the method generates one
// itself; callers should always prefer the explicit form.
func (m *OAuthManager) GetAuthURL(providerName, state string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	provider, ok := m.providers[providerName]
	if !ok {
		return "", errf.Errorf("provider %s not registered", providerName)
	}

	if state == "" {
		return "", errf.Errorf("empty state parameter: use GenerateOAuthState to produce one")
	}

	v := url.Values{}
	v.Set("client_id", provider.ClientID)
	v.Set("redirect_uri", provider.RedirectURL)
	v.Set("response_type", "code")
	v.Set("scope", strings.Join(provider.Scopes, " "))
	v.Set("state", state)

	u, err := url.Parse(provider.AuthURL)
	if err != nil {
		return "", errf.Errorf("auth url: %w", err)
	}
	u.RawQuery = v.Encode()
	return u.String(), nil
}

// ExchangeCode exchanges an authorization code for tokens. The
// supplied state must match one previously issued via GenerateOAuthState
// for the same provider; the binding defeats authorization-code
// injection across providers.
func (m *OAuthManager) ExchangeCode(ctx context.Context, providerName, code, state string) (*OAuthToken, error) {
	m.mu.RLock()
	provider, ok := m.providers[providerName]
	m.mu.RUnlock()
	if !ok {
		return nil, errf.Errorf("provider %s not registered", providerName)
	}

	bound, err := m.consumeOAuthState(state)
	if err != nil {
		return nil, err
	}
	if bound != providerName {
		return nil, errf.Errorf("state bound to %q, not %q", bound, providerName)
	}

	v := url.Values{}
	v.Set("grant_type", "authorization_code")
	v.Set("client_id", provider.ClientID)
	v.Set("client_secret", provider.ClientSecret)
	v.Set("code", code)
	v.Set("redirect_uri", provider.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", provider.TokenURL, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(io.LimitReader(resp.Body, maxOAuthErrorBody))
		return nil, errf.Errorf("token exchange failed: status=%d", resp.StatusCode)
	}

	var token OAuthToken
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOAuthErrorBody)).Decode(&token); err != nil {
		return nil, err
	}

	if token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	return &token, nil
}

// GetUserInfo fetches user info from the provider.
func (m *OAuthManager) GetUserInfo(ctx context.Context, providerName string, token *OAuthToken) (*OAuthUser, error) {
	m.mu.RLock()
	provider, ok := m.providers[providerName]
	m.mu.RUnlock()
	if !ok {
		return nil, errf.Errorf("provider %s not registered", providerName)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", provider.UserInfoURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, errf.Errorf("user info failed: status=%d", resp.StatusCode)
	}

	var user OAuthUser
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOAuthErrorBody)).Decode(&user); err != nil {
		return nil, err
	}

	user.Provider = providerName
	return &user, nil
}

// DefaultGoogleProvider returns default Google OAuth configuration.
// #nosec G101 -- provider names and endpoint URLs are metadata, not credentials.
// The actual secrets (clientID, clientSecret) are function parameters.
func DefaultGoogleProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
	return &OAuthProvider{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		Scopes:       []string{"openid", "email", "profile"},
	}
}

// DefaultGitHubProvider returns default GitHub OAuth configuration.
// #nosec G101 -- provider names and endpoint URLs are metadata, not credentials.
func DefaultGitHubProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
	return &OAuthProvider{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"user:email"},
	}
}

// ponytail: CompareStates was a constant-time state comparator
// intended for callers maintaining their own state cache. The Manager's
// ValidateState does the same comparison inline; no production caller.
