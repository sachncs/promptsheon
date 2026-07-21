package main

import (
	"bytes"
	"context"
	"flag"
	"io"
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
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/webhook"
)

func init() {
	// Tests in this package exercise the full migration set from a
	// fresh DB, including the destructive 025 cleanup migration.
	// Production refuses by default; the test binary opts in.
	os.Setenv(store.DestructiveMigrationEnv, "true")
}

// ---------------------------------------------------------------------------
// serverHelpText
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// configureShellTool
// ---------------------------------------------------------------------------

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

func TestConfigureShellToolEnabledWithValidAllowlist(t *testing.T) {
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "true")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "ls,cat")
	configureShellTool(&config.Config{})
}

func TestConfigureShellToolOnlySpaces(t *testing.T) {
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "true")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "  ,  ,  ")
	configureShellTool(&config.Config{})
}

func TestConfigureShellToolNotEnabled(t *testing.T) {
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "false")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "ls,cat")
	configureShellTool(&config.Config{})
}

// ---------------------------------------------------------------------------
// setupLogger
// ---------------------------------------------------------------------------

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
			logger := setupLogger(cfg, nil)
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// openDB
// ---------------------------------------------------------------------------

func TestOpenDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "promptsheon_test.db")
	cfg := &config.Config{DBPath: dbPath}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	db := openDB(cfg, logger)
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()
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
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
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
// buildServer
// ---------------------------------------------------------------------------

func TestBuildServer_Minimal(t *testing.T) {
	db := setupTestDB(t)

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.OTelEndpoint = ""

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if limiter == nil {
		t.Error("expected non-nil rate limiter")
	}
	if tracer == nil {
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

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if limiter == nil {
		t.Error("expected non-nil rate limiter")
	}
	if tracer == nil {
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

	t.Setenv("PROMPTSHEON_VAULT_KEY", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)
	_ = limiter
	_ = tracer
	_ = collector
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

func TestBuildServer_WithInvalidVaultKey(t *testing.T) {
	db := setupTestDB(t)

	t.Setenv("PROMPTSHEON_VAULT_KEY", "not-a-valid-hex-key")

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)
	_ = limiter
	_ = tracer
	_ = collector
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

	ctx := context.Background()

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

	srv, limiter, tracer, collector := buildServer(srvCtx, &cfg, db, logger, nil, nil, nil)
	_ = limiter
	_ = tracer
	_ = collector
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

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.OTelEndpoint = "localhost:19999"
	cfg.OTelInsecure = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if limiter == nil {
		t.Error("expected non-nil rate limiter")
	}
	if collector == nil {
		t.Error("expected non-nil metrics collector")
	}
	_ = tracer

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health returned %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestBuildServer_ClosedDB(t *testing.T) {
	db := setupTestDB(t)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.DefaultConfig()
	cfg.LogLevel = "warn"
	cfg.Auth = false

	_ = db.Close()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if limiter == nil {
		t.Error("expected non-nil rate limiter")
	}
	if collector == nil {
		t.Error("expected non-nil metrics collector")
	}
	if tracer != nil {
		// OBS-2 follow-up: when SQLite tracer construction fails
		// (closed DB), buildServer returns a no-op Multi tracer so
		// the daemon can still start; tracing endpoints then return
		// 503 because spanStore is also nil. Accept either.
		t.Logf("tracer is non-nil but no-op; ok for closed-DB path")
	}
}

// ---------------------------------------------------------------------------
// startHTTPServerAndWait
// ---------------------------------------------------------------------------

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

func TestStartHTTPServerAndWait_InProcess(t *testing.T) {
	db := setupTestDB(t)

	cfg := config.DefaultConfig()
	cfg.Addr = ":0"
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.WriteTimeout = 5
	cfg.ReadTimeout = 5
	cfg.ReadHeaderTimeout = 5
	cfg.IdleTimeout = 10

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)
	srv.StartAuditWorkers(ctx, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		startHTTPServerAndWait(ctx, cancel, &cfg, srv, logger, limiter, tracer, collector)
	}()

	time.Sleep(2 * time.Second)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("startHTTPServerAndWait did not return in time")
	}
}

func TestStartHTTPServerAndWait_WithCORS(t *testing.T) {
	db := setupTestDB(t)

	cfg := config.DefaultConfig()
	cfg.Addr = ":0"
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.CORSOrigins = []string{"https://example.com"}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)
	srv.StartAuditWorkers(ctx, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		startHTTPServerAndWait(ctx, cancel, &cfg, srv, logger, limiter, tracer, collector)
	}()

	time.Sleep(2 * time.Second)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("startHTTPServerAndWait did not return in time")
	}
}

// ---------------------------------------------------------------------------
// main() via flag manipulation
// ---------------------------------------------------------------------------

func TestMain_VersionFlag(t *testing.T) {
	origArgs := os.Args
	origCL := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCL
	}()

	os.Args = []string{"promptsheond", "-version"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if !strings.Contains(output, "promptsheond") {
		t.Errorf("expected version output, got: %s", output)
	}
}

func TestMain_HelpFlag(t *testing.T) {
	origArgs := os.Args
	origCL := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCL
	}()

	os.Args = []string{"promptsheond", "-help"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if !strings.Contains(output, "promptsheond") {
		t.Errorf("expected help output, got: %s", output)
	}
	if !strings.Contains(output, "PROMPTSHEON_ADDR") {
		t.Errorf("expected help text to mention PROMPTSHEON_ADDR")
	}
}

func TestMain_FullServer(t *testing.T) {
	origArgs := os.Args
	origCL := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCL
	}()

	os.Args = []string{"promptsheond"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	dbPath := filepath.Join(t.TempDir(), "main_test.db")
	t.Setenv("PROMPTSHEON_ADDR", ":0")
	t.Setenv("PROMPTSHEON_DB_PATH", dbPath)
	t.Setenv("PROMPTSHEON_AUTH", "false")
	t.Setenv("PROMPTSHEON_LOG_LEVEL", "warn")

	done := make(chan struct{})
	go func() {
		defer close(done)
		main()
	}()

	time.Sleep(3 * time.Second)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not exit in time")
	}
}

func TestMain_FullServerWithAuth(t *testing.T) {
	origArgs := os.Args
	origCL := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCL
	}()

	os.Args = []string{"promptsheond"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	dbPath := filepath.Join(t.TempDir(), "main_auth_test.db")
	t.Setenv("PROMPTSHEON_ADDR", ":0")
	t.Setenv("PROMPTSHEON_DB_PATH", dbPath)
	t.Setenv("PROMPTSHEON_AUTH", "true")
	t.Setenv("PROMPTSHEON_LOG_LEVEL", "warn")

	done := make(chan struct{})
	go func() {
		defer close(done)
		main()
	}()

	time.Sleep(3 * time.Second)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not exit in time")
	}
}

func TestMain_WithShellToolEnabled(t *testing.T) {
	origArgs := os.Args
	origCL := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCL
	}()

	os.Args = []string{"promptsheond"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	dbPath := filepath.Join(t.TempDir(), "main_shell_test.db")
	t.Setenv("PROMPTSHEON_ADDR", ":0")
	t.Setenv("PROMPTSHEON_DB_PATH", dbPath)
	t.Setenv("PROMPTSHEON_AUTH", "false")
	t.Setenv("PROMPTSHEON_LOG_LEVEL", "warn")
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "true")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "ls,cat")

	done := make(chan struct{})
	go func() {
		defer close(done)
		main()
	}()

	time.Sleep(3 * time.Second)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not exit in time")
	}
}

func TestMain_WithVaultKey(t *testing.T) {
	origArgs := os.Args
	origCL := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCL
	}()

	os.Args = []string{"promptsheond"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	dbPath := filepath.Join(t.TempDir(), "main_vault_test.db")
	t.Setenv("PROMPTSHEON_ADDR", ":0")
	t.Setenv("PROMPTSHEON_DB_PATH", dbPath)
	t.Setenv("PROMPTSHEON_AUTH", "false")
	t.Setenv("PROMPTSHEON_LOG_LEVEL", "warn")
	t.Setenv("PROMPTSHEON_VAULT_KEY", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")

	done := make(chan struct{})
	go func() {
		defer close(done)
		main()
	}()

	time.Sleep(3 * time.Second)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not exit in time")
	}
}

func TestMain_WithCORSOrigins(t *testing.T) {
	origArgs := os.Args
	origCL := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCL
	}()

	os.Args = []string{"promptsheond"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	dbPath := filepath.Join(t.TempDir(), "main_cors_test.db")
	t.Setenv("PROMPTSHEON_ADDR", ":0")
	t.Setenv("PROMPTSHEON_DB_PATH", dbPath)
	t.Setenv("PROMPTSHEON_AUTH", "false")
	t.Setenv("PROMPTSHEON_LOG_LEVEL", "warn")
	t.Setenv("PROMPTSHEON_CORS_ORIGINS", "https://example.com,https://test.com")

	done := make(chan struct{})
	go func() {
		defer close(done)
		main()
	}()

	time.Sleep(3 * time.Second)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not exit in time")
	}
}

// ---------------------------------------------------------------------------
// webhookStoreAdapter
// ---------------------------------------------------------------------------

func TestWebhookStoreAdapter_SaveWebhookEndpoint(t *testing.T) {
	db := setupTestDB(t)

	adapter := &webhookStoreAdapter{db: db}
	ctx := context.Background()

	ep := &webhook.Endpoint{
		ID:        "test-1",
		URL:       "https://example.com/hook",
		Secret:    "secret123",
		Events:    []webhook.EventType{"prompt.created", "prompt.updated"},
		Active:    true,
		CreatedAt: time.Now(),
	}

	if err := adapter.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatal(err)
	}

	list, err := db.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(list))
	}
	if list[0].ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", list[0].ID)
	}
	if list[0].URL != "https://example.com/hook" {
		t.Errorf("expected URL https://example.com/hook, got %s", list[0].URL)
	}
	// SEC-7: the plaintext secret must not be persisted; the
	// adapter stores only ciphertext. The plaintext form lives
	// only on the wire.
	if list[0].Secret != "" {
		t.Errorf("expected empty plaintext secret, got %q (must be ciphertext-only)", list[0].Secret)
	}
	if len(list[0].Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(list[0].Events))
	}
}

func TestWebhookStoreAdapter_SaveWebhookEndpoint_Overwrite(t *testing.T) {
	db := setupTestDB(t)

	adapter := &webhookStoreAdapter{db: db}
	ctx := context.Background()

	ep := &webhook.Endpoint{
		ID:        "test-1",
		URL:       "https://example.com/hook",
		Events:    []webhook.EventType{"prompt.created"},
		Active:    true,
		CreatedAt: time.Now(),
	}

	if err := adapter.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatal(err)
	}

	ep.URL = "https://example.com/hook-v2"
	if err := adapter.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatal(err)
	}

	list, err := db.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(list))
	}
	if list[0].URL != "https://example.com/hook-v2" {
		t.Errorf("expected URL https://example.com/hook-v2, got %s", list[0].URL)
	}
}

func TestWebhookStoreAdapter_DeleteWebhookEndpoint(t *testing.T) {
	db := setupTestDB(t)

	adapter := &webhookStoreAdapter{db: db}
	ctx := context.Background()

	ep := &webhook.Endpoint{
		ID:        "test-1",
		URL:       "https://example.com/hook",
		Events:    []webhook.EventType{"prompt.created"},
		Active:    true,
		CreatedAt: time.Now(),
	}
	if err := adapter.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatal(err)
	}

	if err := adapter.DeleteWebhookEndpoint(ctx, "test-1"); err != nil {
		t.Fatal(err)
	}

	list, err := db.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 endpoints, got %d", len(list))
	}
}

func TestWebhookStoreAdapter_DeleteWebhookEndpoint_NotFound(t *testing.T) {
	db := setupTestDB(t)

	adapter := &webhookStoreAdapter{db: db}
	ctx := context.Background()

	if err := adapter.DeleteWebhookEndpoint(ctx, "nonexistent"); err != nil {
		t.Fatal(err)
	}
}

func TestWebhookStoreAdapter_ListWebhookEndpoints(t *testing.T) {
	db := setupTestDB(t)

	adapter := &webhookStoreAdapter{db: db}
	ctx := context.Background()

	ep1 := &webhook.Endpoint{
		ID:        "wh-1",
		URL:       "https://example.com/hook1",
		Secret:    "s1",
		Events:    []webhook.EventType{"prompt.created"},
		Active:    true,
		CreatedAt: time.Now(),
	}
	ep2 := &webhook.Endpoint{
		ID:        "wh-2",
		URL:       "https://example.com/hook2",
		Secret:    "s2",
		Events:    []webhook.EventType{"prompt.updated", "prompt.deleted"},
		Active:    false,
		CreatedAt: time.Now().Add(-time.Hour),
	}

	if err := adapter.SaveWebhookEndpoint(ctx, ep1); err != nil {
		t.Fatal(err)
	}
	if err := adapter.SaveWebhookEndpoint(ctx, ep2); err != nil {
		t.Fatal(err)
	}

	eps, err := adapter.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}

	byID := make(map[string]*webhook.Endpoint)
	for _, e := range eps {
		byID[e.ID] = e
	}
	if byID["wh-1"] == nil || byID["wh-2"] == nil {
		t.Fatal("expected both endpoints")
	}
	if byID["wh-1"].URL != "https://example.com/hook1" {
		t.Errorf("wrong URL for wh-1: %s", byID["wh-1"].URL)
	}
	if byID["wh-2"].URL != "https://example.com/hook2" {
		t.Errorf("wrong URL for wh-2: %s", byID["wh-2"].URL)
	}
	if len(byID["wh-2"].Events) != 2 {
		t.Errorf("expected 2 events for wh-2, got %d", len(byID["wh-2"].Events))
	}
}

func TestWebhookStoreAdapter_ListWebhookEndpoints_Empty(t *testing.T) {
	db := setupTestDB(t)

	adapter := &webhookStoreAdapter{db: db}
	ctx := context.Background()

	eps, err := adapter.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 0 {
		t.Fatalf("expected 0 endpoints, got %d", len(eps))
	}
}

func TestWebhookStoreAdapter_ListWebhookEndpoints_Error(t *testing.T) {
	db := setupTestDB(t)
	adapter := &webhookStoreAdapter{db: db}

	_ = db.Close()

	_, err := adapter.ListWebhookEndpoints(context.Background())
	if err == nil {
		t.Fatal("expected error from closed DB")
	}
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
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	})
	return db
}

func testWebhookEndpoint(id string, now time.Time) *models.WebhookEndpointRecord {
	return &models.WebhookEndpointRecord{
		ID:        id,
		URL:       "https://example.com/webhook-" + id,
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
	_ = db.Close()
	defer func() { _ = os.Remove(dbPath) }()

	cfg := config.DefaultConfig()
	cfg.DBPath = dbPath
	cfg.LogLevel = "warn"
	cfg.Auth = false
	cfg.Addr = ":0"

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, limiter, tracer, collector := buildServer(ctx, &cfg, db, logger, nil, nil, nil)
	if srv == nil {
		os.Exit(1)
	}

	startHTTPServerAndWait(ctx, cancel, &cfg, srv, logger, limiter, tracer, collector)
}

func runOpenDBSubprocess() {
	cfg := &config.Config{DBPath: "/nonexistent_dir_xyzzy/promptsheon.db"}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	openDB(cfg, logger)
}
