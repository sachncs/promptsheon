
package promptsheon

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListProviders(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/providers", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetProvider_Found(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/providers/openai", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetProvider_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/providers/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleTestProvider_NotAvailable(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"model": "gpt-3.5-turbo"})
	req := httptest.NewRequest("POST", "/api/v1/providers/nonexistent/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleCreateVersionWithManifest verifies that POST /v1/capabilities/
// {c}/versions accepts an explicit Manifest in the request body and
// rejects an invalid manifest with HTTP 400.
