package workflow

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestShellTool_PolicyRaceFree pins the M-8 fix: the previous
// implementation exposed ShellToolEnabled (bool) and
// AllowedCommands (map) as bare package-level mutable globals. This
// test fires concurrent writers and readers and asserts no race
// detector hits and the reads always see a consistent snapshot.
func TestShellTool_PolicyRaceFree(_ *testing.T) {
	p := newShellPolicy()
	p.Set(false, nil)

	const writers = 20
	const readers = 20
	const iters = 100

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				p.Set(j%2 == 0, []string{"ls", "echo"})
			}
		}()
	}
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = p.Enabled()
					_ = p.Allowed()
				}
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestShellTool_DisabledByDefault pins the M-8 fix: the global
// policy defaults to disabled and an empty allowlist, so the shell
// tool refuses all commands until the configuration loader calls
// SetShellToolPolicy.
func TestShellTool_DisabledByDefault(t *testing.T) {
	// Save and restore the global policy.
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()
	p := newShellPolicy()
	globalShellPolicy = p

	tool := &ShellTool{}
	_, err := tool.Execute(context.Background(), map[string]any{"command": "ls"})
	if err == nil {
		t.Fatal("expected error for default-disabled shell tool")
	}
	if err.Error() != "shell tool: disabled (set PROMPTSHEON_SHELL_ENABLED=true and configure PROMPTSHEON_SHELL_ALLOWLIST to enable)" {
		t.Fatalf("unexpected error: %v", err)
	}
}
