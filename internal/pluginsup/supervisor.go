// Package pluginsup ships the PluginSupervisor that ties the
// internal/manifest parser to the internal/supervisor lifecycle
// runner. The PluginSupervisor reads PROMPTSHEON_PLUGINS_FILE at
// boot, validates each entry, and registers one Plugin adapter per
// manifest row.
//
// Forward-only path: a manifest entry with a `binary:` line
// produces a real subprocess plugin (internal/subprocess.Binary)
// that the supervisor launches, supervises, and restarts. A
// manifest entry without a binary line is a pure registration
// stub (the manifest validation path) that the operator may
// extend in a later commit. gRPC codegen with .proto files is the
// M3.5 follow-on per ADR-0019; today's net/rpc over UDS is the
// v0.1.x production transport.
package pluginsup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/internal/manifest"
	"github.com/sachncs/promptsheon/internal/subprocess"
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
//
// A manifest entry that declares a `binary:` line produces a
// subprocess.Binary that the supervisor launches, supervises,
// and restarts. A manifest entry without a binary line is a
// pure registration stub.
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
		uds := e.UDS
		if uds == "" {
			uds = "/tmp/promptsheon/" + e.Name + ".sock"
		}
		var plugin supervisor.Plugin
		if e.Binary != "" {
			// Real subprocess: the supervisor launches the binary,
			// dials the UDS, and proxies the Plugin wire protocol
			// over net/rpc. Health() and Stop() are RPC calls.
			plugin = subprocess.New(e.Name, e.Binary, uds, e.Args, e.Env)
			p.log.Info("manifest: registered subprocess plugin",
				"name", e.Name, "binary", e.Binary, "uds", uds)
		} else {
			// Pure registration: no binary path. The entry
			// declares services the operator intends to fulfil
			// through some other mechanism (e.g. a future built-in).
			plugin = &Remote{Entry: e, Logger: p.log}
			p.log.Info("manifest: registered stub plugin (no binary)",
				"name", e.Name, "uds", uds)
		}
		p.sup.Register(e.Name, plugin, defaultPolicy())
	}
	return nil
}

// Remote is the Plugin adapter for a manifest entry that has no
// `binary:` line. It is a registration stub. The real
// subprocess path uses internal/subprocess.Binary which is
// constructed in LoadFromEnv.
type Remote struct {
	Entry  manifest.Entry
	Logger *slog.Logger
}

// Start is a no-op for the stub: the entry has no binary to
// launch. Production tenants extend the stub by implementing
// the Plugin interface.
func (r *Remote) Start(_ context.Context) error {
	if r.Logger != nil {
		r.Logger.Info("remote plugin: Start stub (no binary)",
			"name", r.Entry.Name)
	}
	return nil
}

// Stop is a no-op for the stub.
func (r *Remote) Stop(_ context.Context) error {
	if r.Logger != nil {
		r.Logger.Info("remote plugin: Stop stub (no binary)",
			"name", r.Entry.Name)
	}
	return nil
}

// Health is a no-op for the stub.
func (r *Remote) Health(_ context.Context) error { return nil }

var _ supervisor.Plugin = (*Remote)(nil)

// defaultPolicy returns the restart policy that the supervisor
// applies to every plugin registered through the manifest.
func defaultPolicy() supervisor.RestartPolicy {
	return supervisor.RestartPolicy{
		MaxRestarts: 5,
		Backoff:     1 * time.Second,
		MaxBackoff:  30 * time.Second,
	}
}
