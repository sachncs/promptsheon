package server_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/api/server"
	"github.com/sachncs/promptsheon/internal/store"
)

func TestMain(m *testing.M) {
	// Migration 008 is destructive; the test DBs need this flag.
	t := &testing.T{}
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	_ = t
	m.Run()
}

func newRepos(t *testing.T) *store.Repositories {
	t.Helper()
	s, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return store.NewRepositories(s)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewServer_HealthEndpoint(t *testing.T) {
	srv := server.New(newRepos(t), discardLogger())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "healthy") {
		t.Errorf("body missing 'healthy': %q", rec.Body.String())
	}
}

func TestNewServer_VersionEndpoint(t *testing.T) {
	srv := server.New(newRepos(t), discardLogger())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
