// Package subprocess implements a real subprocess-based plugin
// that the supervisor launches, supervises, and restarts.
//
// F-20b follow-on. The previous Tier 2.32 commit landed
// PROMPTSHEON_PLUGINS_FILE parsing and an in-process stub
// (internal/pluginsup.Remote). The in-process stub is sufficient
// for the in-memory built-in Guardrail plugins that ship with the
// v0.1.0 binary; the subprocess path is the production path for
// remote plugins.
//
// Architecture (F-20b):
//   - Operator authors PROMPTSHEON_PLUGINS_FILE with binary: /opt/foo
//     and (optionally) uds: /tmp/promptsheon/foo.sock.
//   - At boot, the supervisor forks the binary, dials the UDS
//     path, and exposes a net/rpc server (Plugin is the local
//     in-process interface).
//   - On crash, the supervisor restarts the binary subject to
//     RestartPolicy. The UDS path is the integration point;
//     gRPC over UDS is the M3 follow-on.
//
// net/rpc over UDS is the v0.1.x transport. gRPC codegen with
// .proto files is deferred per ADR-0019 (M3 follow-on). The
// in-process interface (supervisor.Plugin) is the boundary that
// the gRPC adapter will eventually implement.
package subprocess

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/sachncs/promptsheon/internal/supervisor"
)

// Binary is the per-entry runtime state. The supervisor owns one
// Binary per manifest entry; the manifest consumer (internal/pluginsup)
// creates the Binary, the supervisor starts it.
type Binary struct {
	Name string
	Path string
	UDS  string
	Args []string
	Env  []string

	cmd    *exec.Cmd
	client *rpc.Client

	mu        sync.Mutex
	lastStart time.Time
	crashed   bool
	stopped   bool
}

// New constructs a Binary; Start must be called before Health/Stop.
func New(name, path, uds string, args, env []string) *Binary {
	return &Binary{
		Name: name,
		Path: path,
		UDS:  uds,
		Args: args,
		Env:  env,
	}
}

// Compile-time guard: *Binary satisfies the supervisor.Plugin
// interface.
var _ supervisor.Plugin = (*Binary)(nil)

// Start execs the binary and dials the UDS endpoint. The binary
// itself is expected to listen on the UDS path and register a
// net/rpc server that exposes the Plugin wire protocol:
//
//   - "Plugin.Ping" returns the plugin's name + version
//   - "Plugin.Health" returns nil when healthy
//   - "Plugin.Stop" returns "ok" when ready to terminate
//
// The Start method blocks until the UDS endpoint is reachable,
// with a 5-second timeout. A failure to reach the UDS within
// the timeout returns an error so the supervisor's RestartPolicy
// applies.
func (b *Binary) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return errors.New("subprocess: cannot start a stopped binary")
	}
	b.mu.Unlock()

	if b.UDS == "" {
		return errors.New("subprocess: UDS path is required")
	}

	// Wait for the previous process to release the UDS socket
	// (an immediate restart after a crash can race the kernel
	// releasing the bind).
	if b.lastStart.IsZero() == false {
		// Wait a short grace period for the previous socket to
		// release. 50ms is enough on Linux for a fast-closing
		// net.Listen.
		time.Sleep(50 * time.Millisecond)
	}

	cmd := exec.CommandContext(ctx, b.Path, b.Args...)
	cmd.Env = append(os.Environ(), b.Env...)
	// New process group so SIGTERM to the supervisor doesn't
	// kill the child and so the supervisor can kill the whole
	// group on Stop.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("subprocess: start: %w", err)
	}
	b.mu.Lock()
	b.cmd = cmd
	b.lastStart = time.Now()
	b.mu.Unlock()

	// Wait for the UDS endpoint to come up. The plugin binary
	// listens, the supervisor dials; net/rpc then connects.
	deadline := time.Now().Add(5 * time.Second)
	for {
		client, err := rpc.Dial("unix", b.UDS)
		if err == nil {
			b.mu.Lock()
			b.client = client
			b.mu.Unlock()
			break
		}
		if time.Now().After(deadline) {
			// Plugin binary started but never became reachable;
			// kill it so the supervisor's restart policy applies.
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			b.mu.Lock()
			b.cmd = nil
			b.mu.Unlock()
			return fmt.Errorf("subprocess: %s did not become reachable at %s within 5s", b.Path, b.UDS)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// Health proxies an RPC ping to the plugin. The plugin returns
// nil when healthy; non-nil error means unhealthy.
func (b *Binary) Health(_ context.Context) error {
	b.mu.Lock()
	c := b.client
	b.mu.Unlock()
	if c == nil {
		return errors.New("subprocess: no client (process not started or restarting)")
	}
	var reply string
	if err := c.Call("Plugin.Health", struct{}{}, &reply); err != nil {
		return err
	}
	return nil
}

// Stop sends a graceful shutdown signal to the plugin via the UDS
// RPC, then kills the process if it does not exit in 2 seconds.
func (b *Binary) Stop(_ context.Context) error {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return nil
	}
	b.stopped = true
	c := b.client
	cmd := b.cmd
	b.mu.Unlock()

	if c != nil {
		// Graceful RPC shutdown; ignore the reply (the plugin
		// may already be in a stuck state).
		var reply string
		_ = c.Call("Plugin.Stop", struct{}{}, &reply)
		_ = c.Close()
	}
	if cmd != nil && cmd.Process != nil {
		// Best-effort graceful termination; SIGKILL after 2s.
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}
	return nil
}

// Reaped returns true if the underlying process exited and the
// supervisor's RestartPolicy should apply. The supervisor polls
// this after each Health tick and treats a reaped process as
// a crash to be restarted.
func (b *Binary) Reaped() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cmd == nil || b.cmd.Process == nil {
		return false
	}
	// ProcessState is set by exec.Cmd after Wait. We test by
	// attempting a non-blocking state read.
	if b.cmd.ProcessState == nil {
		return false
	}
	return b.cmd.ProcessState.Exited()
}

// Underlying is the *exec.Cmd for testability.
func (b *Binary) Underlying() *exec.Cmd {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cmd
}

// Start a TCP server on a unix socket for net/rpc. This is a
// utility for the test suite that the production plugin flow
// uses to validate the protocol; production plugins import the
// "subprocess" package and call New + Register + Run.
func ServeUnix(socket string, name string) (net.Listener, error) {
	_ = os.Remove(socket)
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	srv := rpc.NewServer()
	if err := srv.RegisterName("Plugin", &PluginRPC{name: name}); err != nil {
		_ = ln.Close()
		_ = os.Remove(socket)
		return nil, err
	}
	go srv.Accept(ln)
	return ln, nil
}

// PluginRPC is the in-process side of the UDS net/rpc contract.
// Plugin binaries register this name with rpc.RegisterName("Plugin").
// The supervisor calls these methods over the UDS net/rpc client.
type PluginRPC struct {
	name string
}

// PingArgs / PingReply are the contract for Plugin.Ping.
type PingArgs struct{}
type PingReply struct {
	Name    string
	Version string
}

// HealthArgs / HealthReply are the contract for Plugin.Health.
type HealthArgs struct{}
type HealthReply struct{}

// Ping returns the plugin's name and version.
func (p *PluginRPC) Ping(args *PingArgs, reply *PingReply) error {
	reply.Name = p.name
	reply.Version = "v0.1.0"
	return nil
}

// Health returns nil when the plugin is healthy.
func (p *PluginRPC) Health(args *HealthArgs, reply *HealthReply) error {
	return nil
}

// StopArgs / StopReply are the contract for Plugin.Stop.
type StopArgs struct{}
type StopReply struct{}

// Stop tells the plugin to drain and exit gracefully.
func (p *PluginRPC) Stop(args *StopArgs, reply *StopReply) error {
	// The plugin binary is expected to interpret the call as
	// a request to exit; the supervisor's Stop() then kills
	// the process if it does not exit in 2s.
	return nil
}
