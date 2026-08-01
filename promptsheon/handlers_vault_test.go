package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/models"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

)

func TestHandleSaveVaultKey(t *testing.T) {
	s := newTestServer(t)
	s.vault = newVault(t)

	body := mustMarshal(t, map[string]string{"provider_name": "openai", "key_name": "default", "key": "sk-abc123"})
	req := httptest.NewRequest("POST", "/api/v1/vault/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["id"] == "" {
		t.Error("expected key id in response")
	}
}

func TestHandleSaveVaultKey_NoVault(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"provider_name": "openai", "key_name": "default", "key": "sk-abc123"})
	req := httptest.NewRequest("POST", "/api/v1/vault/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSaveVaultKey_MissingFields(t *testing.T) {
	s := newTestServer(t)
	s.vault = newVault(t)

	body := mustMarshal(t, map[string]string{"provider_name": "openai"})
	req := httptest.NewRequest("POST", "/api/v1/vault/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListVaultKeys(t *testing.T) {
	repo := newMockRepo()
	_ = repo.SaveProviderKey(context.Background(), &models.ProviderKey{ID: "pk1", ProviderName: "openai", KeyName: "default", EncryptedKey: "enc"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/vault/keys", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteVaultKey(t *testing.T) {
	repo := newMockRepo()
	_ = repo.SaveProviderKey(context.Background(), &models.ProviderKey{ID: "pk1", ProviderName: "openai", KeyName: "default", EncryptedKey: "enc"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/vault/keys/pk1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteVaultKey_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/vault/keys/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Alert Handler Tests
// ---------------------------------------------------------------------------
