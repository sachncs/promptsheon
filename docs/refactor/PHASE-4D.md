# PHASE-4D — Handler Tests: alerting/webhooks/vault/auth/contract/providers/observations/ratelimit/health

**9 commits.** Final test-hardening PR. Covers the remaining 9 handler files.

## Commits

```
c4.21 test(handlers): alerting — persisted + audit asserts
      Refs: PLAN-49/F1 F27
c4.22 test(handlers): webhooks — persisted + audit asserts
      Refs: PLAN-49/F1
c4.23 test(handlers): vault — persisted + audit asserts
      Refs: PLAN-49/F1 F39
c4.24 test(handlers): auth — persisted + audit asserts (incl. OAuth scenarios)
      Refs: PLAN-49/F1 F3b F4 F28
c4.25 test(handlers): contract — persisted + audit asserts
      Refs: PLAN-49/F1
c4.26 test(handlers): providers — persisted + audit asserts
      Refs: PLAN-49/F1
c4.27 test(handlers): observations — persisted + audit asserts
      Refs: PLAN-49/F1
c4.28 test(handlers): ratelimit — persisted + audit asserts
      Refs: PLAN-49/F1 F38
c4.29 test(handlers): health/ready — persisted + audit asserts (incl. closed-DB 503)
      Refs: PLAN-49/F1 F30
```

## Files touched

| File | Commits |
|---|---|
| `promptsheon/handlers_alerting_test.go` | c4.21 |
| `promptsheon/handlers_webhooks_test.go` | c4.22 |
| `promptsheon/handlers_vault_test.go` | c4.23 |
| `promptsheon/handlers_auth_test.go` | c4.24 |
| `promptsheon/handlers_contract_test.go` | c4.25 |
| `promptsheon/handlers_providers_test.go` | c4.26 |
| `promptsheon/handlers_observation_test.go` | c4.27 |
| `promptsheon/handlers_ratelimit_test.go` | c4.28 |
| `promptsheon/handlers_health_test.go` | c4.29 |

## Critical tests

### Vault encryption (c4.23 — F39 fix)

```go
func TestHandleSaveVaultKey_EncryptedAtRest(t *testing.T) {
    s := newTestServerWithSeam()

    body := `{"name": "openai-key", "secret": "plaintext-secret-value"}`
    req := httptest.NewRequest("POST", "/api/v1/vault/keys", strings.NewReader(body))
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)

    if rr.Code != http.StatusCreated {
        t.Fatalf("status %d", rr.Code)
    }

    // (b) persisted assertion: ciphertext != plaintext
    keys, _ := s.DB.ListProviderKeys(context.Background())
    if len(keys) != 1 {
        t.Fatalf("keys = %d, want 1", len(keys))
    }
    if string(keys[0].SecretCiphertext) == "plaintext-secret-value" {
        t.Error("ciphertext == plaintext; encryption not applied")
    }
    if keys[0].SecretCiphertext == nil || len(keys[0].SecretCiphertext) == 0 {
        t.Error("ciphertext empty")
    }
}
```

### OAuth callback (c4.24 — F4 fix)

```go
func TestHandleOAuthCallback_WithMockIdP_RoundTrip(t *testing.T) {
    s := newTestServerWithSeam()

    // Stand up mock IdP
    idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/token":
            w.Header().Set("Content-Type", "application/json")
            w.Write([]byte(`{"access_token":"test-token","token_type":"bearer"}`))
        case "/userinfo":
            w.Header().Set("Content-Type", "application/json")
            w.Write([]byte(`{"email":"alice@example.com","email_verified":true}`))
        default:
            w.WriteHeader(404)
        }
    }))
    defer idp.Close()

    // Configure provider with mock IdP
    s.OAuthManager.RegisterProvider("test", &auth.OAuthProvider{
        ClientID:     "cid",
        ClientSecret: "secret",
        AuthURL:      idp.URL + "/auth",
        TokenURL:     idp.URL + "/token",
        UserInfoURL:  idp.URL + "/userinfo",
        RedirectURL:  "http://localhost/callback",
        Scopes:       []string{"openid", "email"},
    })

    // Mint OAuth state
    state := generateOAuthStateForTest(t, s)

    // Hit callback
    req := httptest.NewRequest("GET", "/api/v1/auth/test/callback?code=test-code&state="+state, nil)
    req.AddCookie(&http.Cookie{Name: "oauth-state", Value: state})
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
    }

    // (b) persisted assertion: user created
    user, _ := s.DB.GetUserByEmail(context.Background(), "alice@example.com")
    if user == nil {
        t.Error("OAuth user not persisted")
    }
    if user.Role != string(auth.RoleReader) {
        t.Errorf("role = %q, want reader", user.Role)
    }
}
```

### Closed-DB health check (c4.29 — F30 fix)

```go
func TestHandleReady_ClosedDB_Returns503(t *testing.T) {
    s := newTestServerWithSeam()

    // Close the DB
    s.DB.Close()

    req := httptest.NewRequest("GET", "/ready", nil)
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)

    // /ready must return 503 when DB is closed
    if rr.Code != http.StatusServiceUnavailable {
        t.Errorf("/ready status = %d, want 503 when DB closed", rr.Code)
    }

    // /health may still return 200 (liveness vs readiness)
    req2 := httptest.NewRequest("GET", "/health", nil)
    rr2 := httptest.NewRecorder()
    s.ServeHTTP(rr2, req2)
    if rr2.Code != http.StatusOK {
        t.Errorf("/health status = %d, want 200 (liveness)", rr2.Code)
    }
}
```

## Per-handler matrix (abbreviated)

| Handler | Persisted assertion | Audit assertion | Special |
|---|---|---|---|
| `handleCreateAlertRule` | `repo.rules[id].Severity != ""` | `action=="create"` | – |
| `handleTriggerAlert` | `repo.alerts[id].Status == "active"` | `action=="alert"` | – |
| `handleResolveAlert` | `repo.alerts[id].Status == "resolved"` | `action=="resolve"` | – |
| `handleAddNotificationGroup` | `repo.groups[id].Name == "..."` | `action=="create"` | – |
| `handleLinkAlertRuleGroup` | `repo.ruleGroups[ruleID] contains groupID` | `action=="link"` | – |
| `handleCreateWebhook` | `dispatcher.Endpoints[id].URL == "https://..."` | `action=="create"` | – |
| `handleDeleteWebhook` | `GetWebhook(id)` returns `ErrStoreNotFound` | `action=="delete"` | – |
| `handleSaveVaultKey` | `repo.keys[id].SecretCiphertext != plaintext` | `action=="create"` | encryption verified |
| `handleDeleteVaultKey` | `repo.keys[id].Deleted == true` | `action=="delete"` | – |
| `handleCreateAPIKey` | `repo.apiKeys[id].Role == "writer"` | `action=="create"` | `key_hash` not in response |
| `handleRevokeAPIKey` | `repo.apiKeys[id].Revoked == true` | `action=="revoke"` | – |
| `handleOAuthLogin` | – | – | redirect URL contains state, scope |
| `handleOAuthCallback` | `repo.users[email]` created | `action=="login"` | – |
| `handleUpdateCapabilityContract` | `repo.capabilities[id].Contract.InputSchema != ""` | `action=="update"` | – |
| `handleGetCapabilityReputation` | – | – | returns score from `repo.reputations` |
| `handleCatalogSearch` | – | – | returns capabilities from `repo.searchIndex` |
| `handleTestProvider` | – | – | returns success/failure from `provider.Complete` |
| `handleGetWorkspaceObservation` | – | – | returns aggregated stats |
| `handleListReleases` | – | – | pagination |
| `handleListAlerts` | – | – | filtering by status |
| `handleReadinessCheck` | – | – | 503 when DB closed (covered above) |

## Exit criterion

```bash
go test -race -count=1 ./promptsheon/handlers_alerting_test.go ./promptsheon/handlers_webhooks_test.go ./promptsheon/handlers_vault_test.go ./promptsheon/handlers_auth_test.go ./promptsheon/handlers_contract_test.go ./promptsheon/handlers_providers_test.go ./promptsheon/handlers_observation_test.go ./promptsheon/handlers_ratelimit_test.go ./promptsheon/handlers_health_test.go
bash scripts/check-coverage.sh coverage.out  # global floor ≥ 60%, handlers ≥ 75%
```

## Parallelization

1 agent (single domain area). Could split 2-way:

| Agent | Files |
|---|---|
| 4D1 | handlers_alerting_test.go, handlers_webhooks_test.go, handlers_vault_test.go, handlers_auth_test.go |
| 4D2 | handlers_contract_test.go, handlers_providers_test.go, handlers_observation_test.go, handlers_ratelimit_test.go, handlers_health_test.go |