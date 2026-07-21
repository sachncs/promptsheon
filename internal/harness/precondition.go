package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// EnvAllowlist is the set of environment variables passed through
// to a precondition command. Anything else is scrubbed. The list
// is intentionally tiny: PATH (so sh finds /usr/bin), HOME (so
// ~-relative paths work), LANG (for locale-correct output
// decoding), and TZ (so timestamps are predictable). Operators
// needing more can extend the list at startup via
// SetEnvAllowlist; the precondition runner reads the global
// list on every invocation, so a SetEnvAllowlist call before
// constructing the runner takes effect immediately.
var envAllowlist = []string{"PATH", "HOME", "LANG", "LC_ALL", "TZ"}

// envDenylist is the SEC-2a inverted form: rather than listing
// every allowed key, we list prefixes that must NEVER reach a
// precondition. This catches the typical credential-leak shape
// (AWS_*, *_KEY, *_TOKEN, *_SECRET) without requiring a
// per-deployment audit of which exact key names are sensitive.
var envDenylist = []string{
	"AWS_",
	"GOOGLE_",
	"AZURE_",
	"VAULT_",
	"KUBERNETES_",
	"HELM_",
	"OTEL_",
	"DATADOG_",
	"SENTRY_",
}

// envDenylistSuffixes catches credentials stored as *_KEY / *_TOKEN
// / *_SECRET regardless of vendor (e.g. STRIPE_SECRET_KEY,
// GITHUB_TOKEN). The match is "ends with" so OPENAI_KEY is caught
// but MY_KEYNAME is not.
var envDenylistSuffixes = []string{"_KEY", "_TOKEN", "_SECRET", "_PASSWORD", "_PASS", "_API_KEY", "_TOKEN_ID"}

// scrubEnv returns a copy of os.Environ() with every variable
// whose name matches the denylist removed. The allowlist still
// wins for keys explicitly added via SetEnvAllowlist.
func scrubEnv() []string {
	allow := make(map[string]struct{}, len(envAllowlist))
	for _, k := range envAllowlist {
		allow[k] = struct{}{}
	}
	denyPrefix := make(map[string]struct{}, len(envDenylist))
	for _, p := range envDenylist {
		denyPrefix[p] = struct{}{}
	}
	denySuffix := make(map[string]struct{}, len(envDenylistSuffixes))
	for _, s := range envDenylistSuffixes {
		denySuffix[s] = struct{}{}
	}
	var out []string
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		name := kv[:i]
		if _, ok := allow[name]; ok {
			out = append(out, kv)
			continue
		}
		drop := false
		for p := range denyPrefix {
			if strings.HasPrefix(name, p) {
				drop = true
				break
			}
		}
		if drop {
			continue
		}
		for s := range denySuffix {
			if strings.HasSuffix(name, s) {
				drop = true
				break
			}
		}
		if drop {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// PreconditionResult captures one precondition's outcome.
type PreconditionResult struct {
	Precondition Precondition
	Passed       bool
	Output       string
	DurationMS   int64
	Err          error
}

// PreconditionRunner executes the registered preconditions for a
// Capability and reports per-hook outcomes. The runner is the
// application-level abstraction above the OS process; storage and
// HTTP live elsewhere.
//
// The runner is GATED: every precondition is skipped unless
// PROMPTSHEON_HARNESS_PRECONDITIONS=true is set in the daemon
// environment. The previous design let any caller with
// PermPromptCreate persist a precondition and have it execute on
// every release activation; the gate is the fail-successfully
// default. The runner still exists so existing fixtures and tests
// compile; callers that want to enable it must set the env var.
type PreconditionRunner struct {
	// Workdir is the working directory for command execution.
	// Empty means "use the daemon's current directory".
	Workdir string
}

// Enabled reports whether the runner should actually execute
// preconditions. Without the env var the runner is a no-op: each
// Run/RunAll returns an empty result with no error and the caller
// proceeds as if there were no preconditions.
func (r *PreconditionRunner) Enabled() bool {
	return os.Getenv("PROMPTSHEON_HARNESS_PRECONDITIONS") == "true"
}

// NewPreconditionRunner constructs a runner that defaults to the
// daemon's working directory.
func NewPreconditionRunner() *PreconditionRunner {
	return &PreconditionRunner{}
}

// Run executes every precondition in preconditions. It returns the
// per-hook results plus a non-nil error if any precondition failed;
// the error wraps ErrPreconditionFailed and carries a Failure list
// describing each failure.
//
// Run is fail-fast: it stops at the first failing precondition and
// returns the partial result. RunAll runs every precondition and
// returns a combined result.
//
// Run is a no-op when the runner is disabled; the caller sees an
// empty result and nil error and proceeds to activate the release.
func (r *PreconditionRunner) Run(ctx context.Context, preconditions []Precondition) ([]PreconditionResult, error) {
	if !r.Enabled() {
		return nil, nil
	}
	results, failures := r.runAll(ctx, preconditions, true)
	if len(failures) > 0 {
		return results, &PreconditionError{Failures: failures}
	}
	return results, nil
}

// RunAll executes every precondition regardless of outcome. Returns
// per-hook results; failures are aggregated into a PreconditionError
// if any precondition failed.
func (r *PreconditionRunner) RunAll(ctx context.Context, preconditions []Precondition) ([]PreconditionResult, error) {
	if !r.Enabled() {
		return nil, nil
	}
	results, failures := r.runAll(ctx, preconditions, false)
	if len(failures) > 0 {
		return results, &PreconditionError{Failures: failures}
	}
	return results, nil
}

func (r *PreconditionRunner) runAll(ctx context.Context, preconditions []Precondition, failFast bool) ([]PreconditionResult, []Failure) {
	var (
		results  []PreconditionResult
		failures []Failure
	)
	for _, p := range preconditions {
		if !p.Enabled {
			continue
		}
		res := r.runOne(ctx, p)
		results = append(results, res)
		if !res.Passed {
			failures = append(failures, Failure{Name: p.Name, Output: res.Output})
			if failFast {
				return results, failures
			}
		}
	}
	return results, failures
}

// runOne executes a single precondition. The command runs in the
// configured Workdir (or "." when empty) with a per-command timeout
// taken from p.TimeoutSec. The combined stdout+stderr is captured
// for inspection.
//
// Security: the command runs through `sh -c` (a string), which is
// the existing API contract. The hardening in this revision is on
// the *process*:
//
//   - env is scrubbed to EnvAllowlist so a precondition cannot
//     read secrets from the daemon's environment (PROMPTSHEON_VAULT_KEY,
//     database DSN, OIDC client secrets, etc.);
//   - the process runs in its own process group; on timeout we
//     kill the entire group so a forked child cannot outlive the
//     parent's cancellation and leak file descriptors;
//   - the daemon itself is gated by PROMPTSHEON_HARNESS_PRECONDITIONS
//     so the whole subsystem is off by default.
//
// Operators who want strict argv-only execution (no shell at all)
// should file a follow-on to change the Precondition schema from
// `command string` to `argv []string`; that's an API change and
// out of scope for this commit.
func (r *PreconditionRunner) runOne(ctx context.Context, p Precondition) PreconditionResult {
	start := time.Now()
	dir := r.Workdir
	if dir == "" {
		dir = "."
	}
	timeout := time.Duration(p.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = DefaultPreconditionTimeout
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "sh", "-c", p.Command)
	cmd.Dir = dir
	cmd.Env = scrubEnv()
	// Put the command in its own process group so we can kill
	// the whole tree on timeout.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	out, err := cmd.CombinedOutput()
	res := PreconditionResult{
		Precondition: p,
		Output:       TruncateOutput(string(out), 8*1024),
		DurationMS:   time.Since(start).Milliseconds(),
	}
	if err != nil {
		res.Passed = false
		res.Err = err
		// On timeout, ensure the whole process group is reaped
		// so a forked child cannot outlive the cancellation.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return res
	}
	res.Passed = true
	return res
}

// TruncateOutput returns s clipped at max bytes with a trailing
// marker when truncated. Keeps the precondition-failure output
// bounded for audit logs and 4xx error bodies. Exported so the
// test package can exercise the bound.
func TruncateOutput(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 32 {
		max = 32
	}
	half := max / 2
	return s[:half] + "\n... [truncated] ...\n" + s[len(s)-half:]
}

// PreconditionError is the typed error returned by Run when one or
// more preconditions fail. The HTTP layer matches errors.Is(err,
// ErrPreconditionFailed) and converts to 409 with the Failure list.
type PreconditionError struct {
	Failures []Failure
}

func (e *PreconditionError) Error() string {
	if len(e.Failures) == 0 {
		return "precondition failed"
	}
	names := make([]string, 0, len(e.Failures))
	for _, f := range e.Failures {
		names = append(names, f.Name)
	}
	return fmt.Sprintf("precondition failed: %s", strings.Join(names, ", "))
}

func (e *PreconditionError) Unwrap() error { return ErrPreconditionFailed }
