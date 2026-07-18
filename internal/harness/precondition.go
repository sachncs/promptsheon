package harness

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

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
type PreconditionRunner struct {
	// Workdir is the working directory for command execution.
	// Empty means "use the daemon's current directory".
	Workdir string
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
func (r *PreconditionRunner) Run(ctx context.Context, preconditions []Precondition) ([]PreconditionResult, error) {
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
func (r *PreconditionRunner) runOne(ctx context.Context, p Precondition) PreconditionResult {
	start := time.Now()
	dir := r.Workdir
	if dir == "" {
		dir = "."
	}
	timeout := time.Duration(p.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "sh", "-c", p.Command)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	res := PreconditionResult{
		Precondition: p,
		Output:       TruncateOutput(string(out), 8*1024),
		DurationMS:   time.Since(start).Milliseconds(),
	}
	if err != nil {
		res.Passed = false
		res.Err = err
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
