package promptsheon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIdempotencyMiddleware_PreservesHeaders verifies the live
// 2xx response carries the handler-set headers (Content-Type,
// Link, Set-Cookie, Location, custom headers). Before the fix
// to recordingResponseWriter.Header(), the live response was
// shipped without any handler-set headers because they were
// written to a separate map that was never forwarded to the
// underlying writer.
func TestIdempotencyMiddleware_PreservesHeaders(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "abc")
		w.Header().Set("Link", `</next>; rel="next"`)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}

	mw := IdempotencyMiddleware(nil)
	rec := httptest.NewRecorder()

	body := strings.NewReader(`{"a":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things", body)
	req.Header.Set("Idempotency-Key", "test-key-1")
	req.Header.Set("Content-Type", "application/json")

	mw(http.HandlerFunc(handler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Header().Get("X-Custom"); got != "abc" {
		t.Errorf("X-Custom = %q, want abc", got)
	}
	if got := rec.Header().Get("Link"); got != `</next>; rel="next"` {
		t.Errorf("Link = %q, want pagination link", got)
	}
}

// TestIdempotencyMiddleware_ReplaysHeaders verifies the cached
// response (on replay) carries the same headers as the original.
// This was the only path working before the fix; included to
// lock in regression coverage.
func TestIdempotencyMiddleware_ReplaysHeaders(t *testing.T) {
	handlerCalls := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "abc")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}

	mw := IdempotencyMiddleware(nil)

	body1 := strings.NewReader(`{"a":1}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/things", body1)
	req1.Header.Set("Idempotency-Key", "test-key-2")
	req1.Header.Set("Content-Type", "application/json")

	body2 := strings.NewReader(`{"a":1}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/things", body2)
	req2.Header.Set("Idempotency-Key", "test-key-2")
	req2.Header.Set("Content-Type", "application/json")

	rec1 := httptest.NewRecorder()
	mw(http.HandlerFunc(handler)).ServeHTTP(rec1, req1)

	rec2 := httptest.NewRecorder()
	mw(http.HandlerFunc(handler)).ServeHTTP(rec2, req2)

	if handlerCalls != 1 {
		t.Errorf("handler called %d times, want 1 (replay)", handlerCalls)
	}
	if got := rec2.Header().Get("Idempotent-Replayed"); got != "true" {
		t.Errorf("Idempotent-Replayed = %q, want true", got)
	}
	if got := rec2.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("replay Content-Type = %q, want application/json", got)
	}
	if got := rec2.Header().Get("X-Custom"); got != "abc" {
		t.Errorf("replay X-Custom = %q, want abc", got)
	}
}

// TestIdempotencyMiddleware_PreservesStatusCode locks in DEF-11 fix (c0.19).
// Before the fix, recordingResponseWriter auto-stamped status 200 on the
// first Write even when the handler intended a different status. On
// replay, the cached 200 was returned even if the original was 502.
func TestIdempotencyMiddleware_PreservesStatusCode(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"err":"upstream"}`))
	}

	mw := IdempotencyMiddleware(nil)

	body1 := strings.NewReader(`{"a":1}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/things", body1)
	req1.Header.Set("Idempotency-Key", "test-status-key")
	req1.Header.Set("Content-Type", "application/json")

	body2 := strings.NewReader(`{"a":1}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/things", body2)
	req2.Header.Set("Idempotency-Key", "test-status-key")
	req2.Header.Set("Content-Type", "application/json")

	// Note: handler is called twice because 5xx is not cached, so
	// replays re-run the handler. The replayed status must still be 502.
	rec1 := httptest.NewRecorder()
	mw(http.HandlerFunc(handler)).ServeHTTP(rec1, req1)

	rec2 := httptest.NewRecorder()
	mw(http.HandlerFunc(handler)).ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusBadGateway {
		t.Errorf("first response: status=%d, want 502", rec1.Code)
	}
	if rec2.Code != http.StatusBadGateway {
		t.Errorf("replayed response: status=%d, want 502 (not auto-stamped 200)", rec2.Code)
	}
}
