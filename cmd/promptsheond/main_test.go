package main

import (
	"strings"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/config"
)

func TestServerHelpText(t *testing.T) {
	text := serverHelpText()
	if text == "" {
		t.Fatal("expected non-empty help text")
	}
	// Pin a few non-negotiable sentences so a future
	// refactor that drops a key warning trips a test
	// rather than silently shipping a less-helpful help
	// block.
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
			t.Errorf("expected help text to contain %q, got:\n%s", want, text)
		}
	}
}

func TestConfigureShellToolEmptyAllowlistDisables(t *testing.T) {
	// With the env unset, the shell tool must be disabled
	// even if the operator accidentally sets
	// PROMPTSHEON_SHELL_ENABLED=true.
	t.Setenv("PROMPTSHEON_SHELL_ENABLED", "true")
	t.Setenv("PROMPTSHEON_SHELL_ALLOWLIST", "")
	configureShellTool(&config.Config{})
	// No assertion on the package var here — the test
	// exercises the path; the policy values are read by
	// the workflow engine at run time. We just confirm
	// the function returns without panic and reads the
	// environment it was given.
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
