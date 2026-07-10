// Package pluginsup ships the PluginSupervisor that ties the
// internal/manifest parser to the internal/supervisor lifecycle
// runner. The PluginSupervisor reads PROMPTSHEON_PLUGINS_FILE at
// boot, validates each entry, and registers one Plugin adapter per
// manifest row.
//
// The in-process Plugin interface (internal/supervisor.Plugin) is
// the production interface today; today's commit implements the
// `Remote` Plugin adapter that would, in the M3 follow-on, exec
// the manifest Entry's Binary, connect to its UDS, and proxy the
// Lifecycle. For now the Remote adapter is a stub that records the
// configured binary and env, satisfies the Plugin interface
// (Start/Stop/Health no-op), and is the integration point the
// subprocess supervisor will replace.
//
// This is Tier 2.32 follow-on: the manifest parser shipped in the
// previous commit; today's commit is the consumer that registers
// the parsed entries with the lifecycle runner.
package pluginsup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/internal/manifest"
	"github.com/sachncs/promptsheon/internal/supervisor"
)

// EnvKey is the environment variable that holds the manifest path.
const EnvKey = "PROMPTSHEON_PLUGINS_FILE"

// PluginSupervisor ties the manifest parser to the
// supervisor.Supervisor lifecycle. It is the production wiring
// that consumes the YAML file the operator authors.
type PluginSupervisor struct {
	sup *supervisor.Supervisor
	log *slog.Logger
}

// New constructs a PluginSupervisor that wraps a *supervisor.Supervisor.
// The caller is responsible for invoking the supervisor's Run
// method on a goroutine with a context that observes shutdown.
func New(sup *supervisor.Supervisor, log *slog.Logger) *PluginSupervisor {
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return &PluginSupervisor{sup: sup, log: log}
}

// LoadFromEnv reads PROMPTSHEON_PLUGINS_FILE and registers every
// plugin descriptor in the supervisor. When the env var is unset
// or empty, the supervisor runs with only the in-process built-ins
// (the same path as before this commit).
func (p *PluginSupervisor) LoadFromEnv(ctx context.Context) error {
	path := strings.TrimSpace(os.Getenv(EnvKey))
	if path == "" {
		p.log.Info("manifest: env var not set, supervisor runs in-process built-ins only")
		return nil
	}
	f, err := manifest.Load(path)
	if err != nil {
		return fmt.Errorf("pluginsup: %w", err)
	}
	for i := range f.Plugins {
		e := f.Plugins[i]
		adapter := &Remote{
			Entry:  e,
			Logger: p.log,
		}
		p.sup.Register(e.Name, adapter, defaultPolicy())
		p.log.Info("manifest: registered plugin",
			"name", e.Name, "binary", e.Binary, "uds", adapter.DefaultUDS())
	}
	return nil
}

// Remote is the Plugin adapter for one manifest entry. M3 follow-on
// replaces the body with the subprocess-execution path: fork
// exec.Command, dial the UDS, run a gRPC client. Today's adapter
// is a no-op that satisfies the interface and provides a clean
// integration point.
type Remote struct {
	Entry  manifest.Entry
	Logger *slog.Logger
}

// Start is a no-op today; M3 follow-on execs the binary.
func (r *Remote) Start(_ context.Context) error {
	if r.Logger != nil {
		r.Logger.Info("remote plugin: Start placeholder (M3 follow-on)",
			"name", r.Entry.Name, "binary", r.Entry.Binary)
	}
	return nil
}

// Stop is a no-op today; M3 follow-on sends a graceful stop signal
// to the subprocess.
func (r *Remote) Stop(_ context.Context) error {
	if r.Logger != nil {
		r.Logger.Info("remote plugin: Stop placeholder (M3 follow-on)",
			"name", r.Entry.Name)
	}
	return nil
}

// Health is a no-op today; M3 follow-on dials the UDS and calls
// the plugin's Health RPC.
func (r *Remote) Health(_ context.Context) error { return nil }

// DefaultUDS returns the UDS path the manifest declares, falling
// back to the canonical /tmp/promptsheon/<name>.sock path when
// the manifest entry does not specify one.
func (r *Remote) DefaultUDS() string {
	if r.Entry.UDS != "" {
		return r.Entry.UDS
	}
	return "/tmp/promptsheon/" + r.Entry.Name + ".sock"
}

// defaultPolicy returns the restart policy that the supervisor
// applies to every plugin registered through the manifest.
func defaultPolicy() supervisor.RestartPolicy {
	return supervisor.RestartPolicy{
		MaxRestarts: 5,
		Backoff:     1 * time.Second,
		MaxBackoff:  30 * time.Second,
	}
}

var _ supervisor.Plugin = (*Remote)(nil)
