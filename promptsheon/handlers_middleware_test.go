package promptsheon

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/auth"
)

func TestServer_ServeHTTP(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestNewServerOptions(t *testing.T) {
	_ = newTestServer(t)
	// Verify options-with-nil don't panic
	_ = newTestServer(t,
		WithTracing(nil, nil),
		WithWebhooks(nil),
		WithVault(nil),
		WithOAuth(nil),
		WithLogHub(nil),
		WithAlertingManager(nil),
		WithRateLimiter(nil),
	)
}

// ---------------------------------------------------------------------------
// Middleware Tests
// ---------------------------------------------------------------------------

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mw := Logging(logger)

	var handled bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !handled {
		t.Error("inner handler not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(buf.String(), "http request") {
		t.Error("expected log output")
	}

	// Test with X-Request-ID header
	buf.Reset()
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Request-ID", "req-123")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if !strings.Contains(buf.String(), "req-123") {
		t.Error("expected X-Request-ID in log output")
	}

	// Test with X-Trace-ID header
	buf.Reset()
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("X-Trace-ID", "trace-456")
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if !strings.Contains(buf.String(), "trace-456") {
		t.Error("expected X-Trace-ID in log output")
	}

	// Test with user in context
	buf.Reset()
	req4 := httptest.NewRequest("GET", "/test", nil)
	ctx := auth.WithUserContext(req4.Context(), &auth.User{ID: "u42", Role: auth.RoleAdmin})
	req4 = req4.WithContext(ctx)
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)
	if !strings.Contains(buf.String(), "u42") {
		t.Error("expected user_id in log output")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	mw := Recovery(logger)

	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/panic", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Error("expected panic log output")
	}
	if !strings.Contains(rr.Body.String(), "internal server error") {
		t.Error("expected error message in body")
	}
}

func TestCORSNoOrigins(t *testing.T) {
	mw := CORS()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS headers when no origins configured")
	}
}

func TestCORSWithOrigin(t *testing.T) {
	mw := CORS("https://example.com")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected CORS origin header, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS methods header")
	}

	// OPTIONS preflight
	req2 := httptest.NewRequest("OPTIONS", "/test", nil)
	req2.Header.Set("Origin", "https://example.com")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rr2.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options header")
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected X-Frame-Options header")
	}
	// BUG-24: HSTS is now TLS-only. Plain HTTP requests must NOT
	// receive the header because it would train the browser to
	// expect HTTPS on a connection that isn't using it.
	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("did not expect HSTS on plain HTTP, got %q", got)
	}
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Error("expected Content-Security-Policy header")
	}
}

func TestSecurityHeadersHSTSOverTLS(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1) r.TLS set (real TLS connection).
	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS when r.TLS is set")
	}

	// 2) X-Forwarded-Proto: https (TLS-terminating proxy).
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Forwarded-Proto", "https")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS when X-Forwarded-Proto=https")
	}
}

func TestMaxBytesReader(t *testing.T) {
	mw := MaxBytesReader(10)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 20)
		n, _ := r.Body.Read(buf)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf[:n])
	}))

	body := strings.NewReader("hello world is long")
	req := httptest.NewRequest("POST", "/test", body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestChainHTTP(t *testing.T) {
	var order []string
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, r)
		})
	}

	handler := ChainHTTP(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		order = append(order, "inner")
	}), mw1, mw2)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if len(order) != 3 || order[0] != "mw1" || order[1] != "mw2" || order[2] != "inner" {
		t.Errorf("unexpected order: %v", order)
	}
}

func TestStatusWriter(t *testing.T) {
	sw := &statusWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	sw.WriteHeader(http.StatusNotFound)
	if sw.status != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, sw.status)
	}
}

func TestSlogContext(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	ctx = WithSlogContext(ctx, logger)
	got := SlogFromContext(ctx)
	if got == nil {
		t.Fatal("expected logger from context")
	}
	defaultLogger := SlogFromContext(context.Background())
	if defaultLogger == slog.Default() {
		t.Log("default logger fallback works")
	}
}

// ---------------------------------------------------------------------------
// Error / Helper Tests
// ---------------------------------------------------------------------------

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", w.Header().Get("Content-Type"))
	}

	var result map[string]string
	readJSONBody(t, w.Body.Bytes(), &result)
	if result["key"] != "value" {
		t.Errorf("expected value, got %s", result["key"])
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "internal error", err: errors.New("oops"), wantStatus: http.StatusInternalServerError},
		{name: "not found", err: ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "bad request", err: ErrBadRequest, wantStatus: http.StatusBadRequest},
		{name: "conflict", err: ErrConflict, wantStatus: http.StatusConflict},
		{name: "http error", err: &HTTPError{Status: http.StatusTeapot, Message: "teapot"}, wantStatus: http.StatusTeapot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.err)
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
			var result map[string]string
			readJSONBody(t, w.Body.Bytes(), &result)
			if result["error"] == "" {
				t.Error("expected error message in body")
			}
		})
	}
}

func TestReadJSON(t *testing.T) {
	body := mustMarshal(t, map[string]string{"hello": "world"})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))

	var target map[string]string
	if err := readJSON(req, &target); err != nil {
		t.Fatal(err)
	}
	if target["hello"] != "world" {
		t.Errorf("expected world, got %s", target["hello"])
	}
}

func TestGenerateID(t *testing.T) {
	id := generateID()
	if !strings.HasPrefix(id, "api-") {
		t.Errorf("expected api- prefix, got %s", id)
	}
	time.Sleep(time.Nanosecond)
	id2 := generateID()
	if id == id2 {
		t.Error("expected different IDs")
	}
}

func TestCallerID(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if id := callerID(req); id != "api" {
		t.Errorf("expected api, got %s", id)
	}

	ctx := auth.WithUserContext(context.Background(), &auth.User{ID: "u1", Role: auth.RoleAdmin})
	req2 := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	if id := callerID(req2); id != "u1" {
		t.Errorf("expected u1, got %s", id)
	}

	// With nil user
	ctx2 := auth.WithUserContext(context.Background(), nil)
	req3 := httptest.NewRequest("GET", "/", nil).WithContext(ctx2)
	if id := callerID(req3); id != "api" {
		t.Errorf("expected api for nil user, got %s", id)
	}
}

func TestHelperErrors(t *testing.T) {
	if badRequest("bad").Error() != "bad" {
		t.Error("badRequest message mismatch")
	}
	if notFound("nf").Error() != "nf" {
		t.Error("notFound message mismatch")
	}
	if unauthorized().Error() != "authentication required" {
		t.Error("unauthorized message mismatch")
	}
	if forbidden("forb").Error() != "forb" {
		t.Error("forbidden message mismatch")
	}
}

func TestHTTPError(t *testing.T) {
	e := &HTTPError{Status: http.StatusBadRequest, Message: "bad"}
	var httpErr *HTTPError
	if !errors.As(e, &httpErr) {
		t.Error("expected HTTPError to match HTTPError")
	}
}

// ---------------------------------------------------------------------------
// Health Handler Tests
// ---------------------------------------------------------------------------

func TestRequirePerm_NoAuth(t *testing.T) {
	s := newTestServer(t)
	handler := s.requirePerm(auth.PermPromptRead)(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequirePerm_WithAuth_Unauthenticated(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	handler := s.requirePerm(auth.PermPromptRead)(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err == nil {
		t.Fatal("expected error for unauthenticated request")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusUnauthorized {
		t.Errorf("expected 401 error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Audit Worker Tests
// ---------------------------------------------------------------------------
