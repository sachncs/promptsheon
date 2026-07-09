package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/config"
	"github.com/sachncs/promptsheon/internal/store"
)

func TestServerHelpText(t *testing.T) {
	text := serverHelpText()
	if text == "" {
		t.Fatal("expected non-empty help text")
	}
	for _, want := range []string{
		"promptsheond",
		"--version",
		"--help",
		"PROMPTSHEON_ADDR",
		"PROMPTSHEON_AUTH",
		"PROMPTSHEON_VAULT_KEY",
		"/health",
		"/api/v1/version",
		"/api/v1/setup",
		"SECURITY",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected help text to contain %q", want)
		}
	}
}

func TestConfigureShellToolEmptyAllowlistDisables(t *testing.T) {
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "true")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "")
	configureShellTool(&config.Config{})
}

func TestConfigureShellToolWithAllowlist(t *testing.T) {
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "true")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "ls, cat, head")
	configureShellTool(&config.Config{})
}

func TestConfigureShellToolDisabledByDefault(t *testing.T) {
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "")
	configureShellTool(&config.Config{})
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
		{"default", ""},
		{"unknown", "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{LogLevel: tt.level}
			logger := setupLogger(cfg)
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
		})
	}
}

func TestOpenDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "promptsheon_test.db")
	cfg := &config.Config{DBPath: dbPath}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	db := openDB(cfg, logger)
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	defer db.Close()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestOpenDB_InMemory(t *testing.T) {
	cfg := &config.Config{DBPath: ":memory:"}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	db := openDB(cfg, logger)
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	defer db.Close()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestBuildServer_Minimal(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.OTelEndpoint = ""

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, spans, collector := buildServer(ctx, &cfg, db, logger)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if limiter == nil {
		t.Error("expected non-nil rate limiter")
	}
	if spans == nil {
		t.Error("expected non-nil trace store")
	}
	if collector == nil {
		t.Error("expected non-nil metrics collector")
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health returned %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestBuildServer_WithAuth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, spans, collector := buildServer(ctx, &cfg, db, logger)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if limiter == nil {
		t.Error("expected non-nil rate limiter")
	}
	if spans == nil {
		t.Error("expected non-nil trace store")
	}
	if collector == nil {
		t.Error("expected non-nil metrics collector")
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health returned %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestBuildServer_WithVault(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Setenv("PROMPTSHEON_VAULT_KEY", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, _, _, _ := buildServer(ctx, &cfg, db, logger)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health returned %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestBuildServer_WithWebhookEndpointsInDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Insert a webhook endpoint so the adapter's ListWebhookEndpoints
	// loop body is exercised during buildServer.
	now := time.Now()
	err := db.SaveWebhookEndpoint(ctx, testWebhookEndpoint("wh-1", now))
	if err != nil {
		t.Fatal(err)
	}
	err = db.SaveWebhookEndpoint(ctx, testWebhookEndpoint("wh-2", now.Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.OTelEndpoint = ""

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srvCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, _, _, _ := buildServer(srvCtx, &cfg, db, logger)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health returned %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestBuildServer_WithOTelEndpoint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.OTelEndpoint = "localhost:19999"
	cfg.OTelInsecure = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, spans, collector := buildServer(ctx, &cfg, db, logger)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if limiter == nil {
		t.Error("expected non-nil rate limiter")
	}
	if collector == nil {
		t.Error("expected non-nil metrics collector")
	}
	// spans may be nil if OTel init failed, that's OK
	_ = spans

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health returned %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestStartHTTPServerAndWait_Subprocess(t *testing.T) {
	if os.Getenv("GO_TEST_SIGNAL_SUBPROCESS") == "1" {
		runServerSubprocess()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestStartHTTPServerAndWait_Subprocess")
	cmd.Env = append(os.Environ(), "GO_TEST_SIGNAL_SUBPROCESS=1")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("subprocess exited with: %v", err)
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("subprocess timed out")
	}
}

func TestOpenDB_InvalidPath(t *testing.T) {
	if os.Getenv("GO_TEST_OPENDB_SUBPROCESS") == "1" {
		runOpenDBSubprocess()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestOpenDB_InvalidPath")
	cmd.Env = append(os.Environ(), "GO_TEST_OPENDB_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess to exit with non-zero status")
	}
	t.Logf("subprocess output: %s", string(out))
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func setupTestDB(t *testing.T) *store.SQLite {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "promptsheon_test.db")
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func testWebhookEndpoint(id string, now time.Time) *store.WebhookEndpointRecord {
	return &store.WebhookEndpointRecord{
		ID:        id,
		URL:       "https://example.com/webhook",
		Secret:    "test-secret",
		Events:    []string{"prompt.created", "prompt.updated"},
		Active:    true,
		CreatedAt: now,
	}
}

func runServerSubprocess() {
	dbPath := filepath.Join(os.TempDir(), "promptsheon_signal_test.db")
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		os.Exit(1)
	}
	defer db.Close()
	defer os.Remove(dbPath)

	cfg := config.DefaultConfig()
	cfg.DBPath = dbPath
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.Addr = ":0"

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, spans, collector := buildServer(ctx, &cfg, db, logger)
	if srv == nil {
		os.Exit(1)
	}

	startHTTPServerAndWait(ctx, cancel, &cfg, srv, logger, limiter, spans, collector)
}

func runOpenDBSubprocess() {
	cfg := &config.Config{DBPath: "/nonexistent_dir_xyzzy/promptsheon.db"}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	openDB(cfg, logger)
}
