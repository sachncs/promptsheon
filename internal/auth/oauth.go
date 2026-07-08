// Package auth provides OAuth/SSO authentication flows.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OAuthProvider represents an OAuth provider configuration.
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
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture"`
	Provider string `json:"provider"`
}

// OAuthManager handles OAuth flows for multiple providers.
type OAuthManager struct {
	providers map[string]*OAuthProvider
	client    *http.Client
}

// NewOAuthManager creates a new OAuth manager.
func NewOAuthManager() *OAuthManager {
	return &OAuthManager{
		providers: make(map[string]*OAuthProvider),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterProvider registers an OAuth provider.
func (m *OAuthManager) RegisterProvider(name string, provider *OAuthProvider) {
	m.providers[name] = provider
}

// GetAuthURL returns the authorization URL for a provider.
func (m *OAuthManager) GetAuthURL(providerName, state string) (string, error) {
	provider, ok := m.providers[providerName]
	if !ok {
		return "", fmt.Errorf("provider %s not registered", providerName)
	}

	params := map[string]string{
		"client_id":     provider.ClientID,
		"redirect_uri":  provider.RedirectURL,
		"response_type": "code",
		"scope":         strings.Join(provider.Scopes, " "),
		"state":         state,
	}

	var pairs []string
	for k, v := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}

	return provider.AuthURL + "?" + strings.Join(pairs, "&"), nil
}

// ExchangeCode exchanges an authorization code for tokens.
func (m *OAuthManager) ExchangeCode(ctx context.Context, providerName, code string) (*OAuthToken, error) {
	provider, ok := m.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider %s not registered", providerName)
	}

	data := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     provider.ClientID,
		"client_secret": provider.ClientSecret,
		"code":          code,
		"redirect_uri":  provider.RedirectURL,
	}

	var pairs []string
	for k, v := range data {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", provider.TokenURL,
		strings.NewReader(strings.Join(pairs, "&")))
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", body)
	}

	var token OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	if token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	return &token, nil
}

// GetUserInfo fetches user info from the provider.
func (m *OAuthManager) GetUserInfo(ctx context.Context, providerName string, token *OAuthToken) (*OAuthUser, error) {
	provider, ok := m.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider %s not registered", providerName)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info failed: %s", body)
	}

	var user OAuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	user.Provider = providerName
	return &user, nil
}

// DefaultGoogleProvider returns default Google OAuth configuration.
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
