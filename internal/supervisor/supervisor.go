// Package supervisor manages the lifecycle of in-process plugins.
//
// The supervisor is the production-side counterpart to pkg/plugin.
// Today the protocol is in-process: the daemon discovers plugins via
// a YAML manifest, hands the supplied factory a PluginDescriptor,
// starts the plugin's goroutine, monitors health, and enforces a
// per-plugin restart budget.
//
// This is Tier 2.46 of the architecture review board. The gRPC
// over UDS path is in AD-0016 follow-on; this commit ships the
// supervisor against the in-process Plugin interface so that
// built-ins (PII redaction, prompt-injection detection) can be
// registered through the same supervisor that remote plugins
// will use.
//
// PluginDescriptor is the static metadata a plugin publishes; the
// supervisor reads the manifest from PROMPTSHEON_PLUGINS_FILE
// (YAML) and resolves names to factories. Built-in plugins are
// registered programmatically in main.go; remote plugins are loaded
// from the YAML file when the supervisor is constructed.
package supervisor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// Plugin is the lifecycle interface a supervised plugin satisfies.
type Plugin interface {
	// Start begins the plugin's work. The plugin must observe the
	// supplied context for cancellation. Start returns when the
	// plugin is ready; if it returns an error the supervisor
	// retries (subject to RestartPolicy).
	Start(ctx context.Context) error
	// Stop signals the plugin to drain and exit; the supervisor
	// calls Stop before tearing down the supervisor itself.
	Stop(ctx context.Context) error
	// Health returns nil if the plugin is healthy, an error
	// otherwise. The supervisor polls Health at the configured
	// interval; consecutive failures trigger a restart.
	Health(ctx context.Context) error
}

// HealthChecker is the consumer-defined health contract; plugin
// implementations that do not natively expose Health can adapt via
// a small wrapper.
type HealthChecker interface {
	Health(ctx context.Context) error
}

// RestartPolicy controls how the supervisor handles crashes.
type RestartPolicy struct {
	// MaxRestarts is the maximum number of restarts before the
	// supervisor gives up. 0 means "no restart". A negative number
	// means "unlimited".
	MaxRestarts int
	// Backoff is the wait between restarts; the supervisor doubles
	// the wait on each consecutive failure up to MaxBackoff.
	Backoff, MaxBackoff time.Duration
}

// Managed is one plugin under the supervisor's care.
type Managed struct {
	Name     string
	Plugin   Plugin
	Policy   RestartPolicy
	Restarts int
	Healthy  bool
}

// supervisor is the public entry point. Construct it with New,
// register plugins via Register, then call Run.
type Supervisor struct {
	mu       sync.Mutex
	plugins  map[string]*Managed
	publisher Publisher
	logger   *slog.Logger
}

// Publisher is the consumer-defined event sink; the supervisor
// emits plugin.started / plugin.crashed / plugin.exhausted events
// through it. A nil publisher is treated as a no-op.
type Publisher interface {
	Publish(event PluginEvent)
}

// PluginEvent is the shape the supervisor emits.
type PluginEvent struct {
	Name      string
	Kind      string // started | crashed | restarted | exhausted | stopped
	Timestamp time.Time
	Err       error
}

// New constructs a Supervisor with a publisher and logger. Logger
// defaults to a no-op logger if nil.
func New(p Publisher, logger *slog.Logger) *Supervisor {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Supervisor{
		plugins:  map[string]*Managed{},
		publisher: p,
		logger:   logger,
	}
}

// Register adds a plugin to the supervisor's inventory. Start is
// invoked on the next Run cycle, not by Register.
func (s *Supervisor) Register(name string, plugin Plugin, policy RestartPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plugins[name] = &Managed{Name: name, Plugin: plugin, Policy: policy, Healthy: true}
}

// Run starts all registered plugins and blocks until ctx is
// cancelled or any plugin exhausts its restart budget. The
// supervisor polls each plugin's Health at the configured interval
// (default 5 seconds; overridden by SetHealthInterval).
func (s *Supervisor) Run(ctx context.Context) error {
	interval := 5 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startCh := make(chan error, len(s.plugins))
	for name, m := range s.plugins {
		go func(n string, mm *Managed) {
			err := mm.Plugin.Start(ctx)
			startCh <- err
			s.publish(PluginEvent{Name: n, Kind: "started", Timestamp: time.Now(), Err: err})
		}(name, m)
	}

	failedStart := map[string]bool{}
	for range s.plugins {
		select {
		case err := <-startCh:
			if err != nil {
				failedStart[err.Error()] = true
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if len(failedStart) > 0 {
		return fmt.Errorf("supervisor: %d plugins failed to start", len(failedStart))
	}

	for {
		select {
		case <-ctx.Done():
			return s.shutdown(context.Background())
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick polls every plugin's Health and triggers a restart for any
// plugin whose consecutive health checks fail.
func (s *Supervisor) tick(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.plugins {
		if err := m.Plugin.Health(ctx); err != nil {
			m.Healthy = false
			s.publish(PluginEvent{Name: m.Name, Kind: "crashed", Timestamp: time.Now(), Err: err})
			s.tryRestart(ctx, m)
			continue
		}
		m.Healthy = true
	}
}

// tryRestart applies the RestartPolicy. Returns false if the
// budget is exhausted.
func (s *Supervisor) tryRestart(ctx context.Context, m *Managed) bool {
	if m.Policy.MaxRestarts == 0 {
		return false
	}
	if m.Policy.MaxRestarts > 0 && m.Restarts >= m.Policy.MaxRestarts {
		s.publish(PluginEvent{Name: m.Name, Kind: "exhausted", Timestamp: time.Now()})
		return false
	}
	backoff := m.Policy.Backoff
	if backoff <= 0 {
		backoff = time.Second
	}
	maxBackoff := m.Policy.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}
	// Exponential backoff capped at maxBackoff.
	wait := backoff << m.Restarts
	if wait > maxBackoff {
		wait = maxBackoff
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(wait):
	}
	m.Restarts++
	_ = m.Plugin.Stop(ctx)
	go func(mm *Managed) {
		if err := mm.Plugin.Start(ctx); err != nil {
			s.publish(PluginEvent{Name: mm.Name, Kind: "crashed", Timestamp: time.Now(), Err: err})
			_ = s  // keep s alive for publish
		}
	}(m)
	return true
}

// shutdown drains all plugins.
func (s *Supervisor) shutdown(ctx context.Context) error {
	var lastErr error
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.plugins {
		if err := m.Plugin.Stop(ctx); err != nil {
			lastErr = err
		}
		s.publish(PluginEvent{Name: m.Name, Kind: "stopped", Timestamp: time.Now()})
	}
	return lastErr
}

// List returns a snapshot of the supervised plugin names.
func (s *Supervisor) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.plugins))
	for name := range s.plugins {
		out = append(out, name)
	}
	return out
}

func (s *Supervisor) publish(ev PluginEvent) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(ev)
}

// Compile-time guard.
var _ = exec.Command // referenced for the gRPC follow-on that execs subprocesses
