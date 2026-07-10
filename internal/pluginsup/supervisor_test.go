package pluginsup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestLoadFromEnvUnsetIsNoop(t *testing.T) {
	t.Parallel()
	_ = filepath.Join
	sup := newTestSupervisor()
	ps := New(sup, nil)
	if err := ps.LoadFromEnv(context.Background()); err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if got := sup.List(); len(got) != 0 {
		t.Fatalf("expected empty supervisor, got %v", got)
	}
}

func TestLoadFromEnvRegistersEachEntry(t *testing.T) {
	body := `plugins:
  - name: openai-gpt4
    version: "1.0.0"
    binary: /opt/openai
    services: ["Provider"]
  - name: pii-redactor
    version: "0.1.0"
    binary: /opt/pii
    services: ["Guardrail"]
`
	path := writeManifest(t, body)
	t.Setenv(EnvKey, path)
	sup := newTestSupervisor()
	ps := New(sup, nil)
	if err := ps.LoadFromEnv(context.Background()); err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	got := sup.List()
	if len(got) != 2 {
		t.Fatalf("expected 2 registered, got %d (%v)", len(got), got)
	}
}

func TestLoadFromEnvPropagatesParseErrors(t *testing.T) {
	body := "plugins:\n  - name: \"bad name\"\n    binary: /opt/x"
	path := writeManifest(t, body)
	t.Setenv(EnvKey, path)
	sup := newTestSupervisor()
	ps := New(sup, nil)
	if err := ps.LoadFromEnv(context.Background()); err == nil {
		t.Fatalf("expected error for bad name")
	}
}

func TestRemotePluginLifecycle(t *testing.T) {
	t.Parallel()
	r := &Remote{Entry: manifestEntry(t, "alpha")}
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := r.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got := r.DefaultUDS(); got != "/tmp/promptsheon/alpha.sock" {
		t.Fatalf("expected default UDS, got %s", got)
	}
}

func TestRemotePluginUDSOverride(t *testing.T) {
	t.Parallel()
	r := &Remote{Entry: manifestEntryWithUDS(t, "beta", "/custom/path.sock")}
	if got := r.DefaultUDS(); got != "/custom/path.sock" {
		t.Fatalf("expected /custom/path.sock, got %s", got)
	}
}
