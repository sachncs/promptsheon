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
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/sachncs/promptsheon/internal/pluginmanifest"
	pluginv1 "github.com/sachncs/promptsheon/internal/pluginproto/pluginv1"
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
// `binary:` line. The previous form was a no-op stub that
// silently stayed "healthy" — operators couldn't tell from
// health checks that the entry wasn't actually serving. Remote
// now fail-closes: Start returns an error explaining the
// configuration is incomplete, and Health surfaces the same
// error on every poll so the supervisor can record the failure
// in its metrics and restart the entry.
//
// To make a Remote entry actually do work, the manifest row
// must declare a `binary:` line (which produces a
// internal/subprocess.Binary) or the entry must be replaced
// with a built-in. The supervisor keeps the failed Remote
// registered so the operator sees the failure in
// /metrics, not in silent absence.
type Remote struct {
	Entry  manifest.Entry
	Logger *slog.Logger
}

// errRemoteNotConfigured is the sentinel Remote returns from
// Start/Health/Stop. The supervisor treats it as a restartable
// failure: the entry stays registered, every Health poll
// fails, the operator sees the gap in /metrics, and the
// restart budget gives the entry time to recover (e.g. the
// operator adding a binary: line and reloading the manifest).
var errRemoteNotConfigured = fmt.Errorf("pluginsup: manifest entry has no binary line")

// GRPCPlugin is the supervisor.Plugin adapter for a remote
// plugin served over gRPC (typically over a UDS socket). The
// plugin binary implements the pluginv1.PluginServer contract;
// the supervisor is the client.
//
// Each supervisor-Plugin call (Start/Health/Stop) dials the
// UDS if not already connected, then invokes the corresponding
// gRPC method. On any transport error the connection is reset
// so the next call re-dials. The dial uses an insecure
// credentials bundle because UDS traffic is kernel-local.
type GRPCPlugin struct {
	Addr string
	Name string
	Log  *slog.Logger

	mu   sync.Mutex
	conn *grpc.ClientConn
}

// Start dials the gRPC endpoint. The supervisor's RestartPolicy
// drives retries on failure.
func (g *GRPCPlugin) Start(ctx context.Context) error {
	return g.dial(ctx)
}

// Stop closes the underlying gRPC connection.
func (g *GRPCPlugin) Stop(_ context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.conn != nil {
		err := g.conn.Close()
		g.conn = nil
		return err
	}
	return nil
}

// Health calls gRPC pluginv1.PluginServer.Health. Any transport
// or gRPC error is returned to the supervisor; the supervisor
// records the failure and triggers a restart.
func (g *GRPCPlugin) Health(ctx context.Context) error {
	if err := g.dial(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("pluginsup: gRPC client not connected")
	}
	client := pluginv1.NewPluginClient(conn)
	resp, err := client.Health(ctx, &pluginv1.HealthRequest{})
	if err != nil {
		// Reset the connection so the next call re-dials.
		g.resetConn()
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("pluginsup: plugin %q reports not OK", g.Name)
	}
	_ = status.Code(err)
	return nil
}

// Ping is exposed for operator diagnostics (not part of the
// supervisor.Plugin surface). It calls the gRPC Ping method
// and returns the plugin's reported identity.
func (g *GRPCPlugin) Ping(ctx context.Context) (name, version string, err error) {
	if err := g.dial(ctx); err != nil {
		return "", "", err
	}
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()
	if conn == nil {
		return "", "", fmt.Errorf("pluginsup: gRPC client not connected")
	}
	client := pluginv1.NewPluginClient(conn)
	resp, err := client.Ping(ctx, &pluginv1.PingRequest{})
	if err != nil {
		g.resetConn()
		return "", "", err
	}
	return resp.Name, resp.Version, nil
}

func (g *GRPCPlugin) dial(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.conn != nil {
		return nil
	}
	if g.Addr == "" {
		return fmt.Errorf("pluginsup: gRPC plugin %q has empty addr", g.Name)
	}
	conn, err := grpc.NewClient(
		"unix://"+g.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("pluginsup: gRPC dial %s: %w", g.Addr, err)
	}
	g.conn = conn
	if g.Log != nil {
		g.Log.Info("grpc plugin: dial ok", "name", g.Name, "addr", g.Addr)
	}
	_ = ctx
	_ = net.Conn(nil)
	return nil
}

func (g *GRPCPlugin) resetConn() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.conn != nil {
		_ = g.conn.Close()
		g.conn = nil
	}
}

var _ supervisor.Plugin = (*GRPCPlugin)(nil)

// Start returns errRemoteNotConfigured; the supervisor treats
// this as a restartable failure (subject to RestartPolicy).
func (r *Remote) Start(_ context.Context) error {
	if r.Logger != nil {
		r.Logger.Warn("remote plugin: manifest entry has no binary line",
			"name", r.Entry.Name,
			"hint", "add a binary: line to the manifest entry or implement the service as a built-in")
	}
	return errRemoteNotConfigured
}

// Stop is a no-op: there is nothing running to stop.
func (r *Remote) Stop(_ context.Context) error {
	if r.Logger != nil {
		r.Logger.Info("remote plugin: Stop (no binary to stop)",
			"name", r.Entry.Name)
	}
	return nil
}

// Health surfaces errRemoteNotConfigured so the supervisor
// records a Health failure on every poll.
func (r *Remote) Health(_ context.Context) error {
	return errRemoteNotConfigured
}

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
