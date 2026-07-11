package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	body := `plugins:
  - name: openai-gpt4
    version: "1.2.0"
    binary: /opt/promptsheon/plugins/openai-gpt4
    args: ["--bind=0.0.0.0:7700"]
    env:
      - "OPENAI_API_KEY_FILE=/etc/promptsheon/openai.key"
    services: ["Provider"]
    uds: /tmp/promptsheon/openai-gpt4.sock
    min_core_version: "0.1.0"
  - name: pii-redactor
    version: "0.1.0"
    binary: /opt/promptsheon/plugins/pii-redactor
    services: ["Guardrail"]
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(got.Plugins))
	}
	if got.Plugins[0].Name != "openai-gpt4" {
		t.Fatalf("expected openai-gpt4, got %s", got.Plugins[0].Name)
	}
	if got.Plugins[1].DefaultUDS() != "/tmp/promptsheon/pii-redactor.sock" {
		t.Fatalf("expected default UDS, got %s", got.Plugins[1].DefaultUDS())
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := Load("/nonexistent/plugins.yaml"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestLoadEmptyManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	if err := os.WriteFile(path, []byte("plugins: []\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err != ErrEmpty {
		t.Fatalf("expected ErrEmpty, got %v", err)
	}
}

func TestLoadRejectsBadName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	body := `plugins:
  - name: "bad name with spaces"
    binary: /opt/foo
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected error for bad name")
	}
}

func TestLoadRejectsEmptyBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	body := `plugins:
  - name: ok-name
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected error for empty binary")
	}
}

func TestDefaultUDSWhenSet(t *testing.T) {
	t.Parallel()
	e := Entry{Name: "x", UDS: "/custom/path.sock"}
	if got := e.DefaultUDS(); got != "/custom/path.sock" {
		t.Fatalf("expected /custom/path.sock, got %s", got)
	}
}
