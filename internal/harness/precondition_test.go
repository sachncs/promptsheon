package harness_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/harness"
)

func TestPreconditionRunnerPasses(t *testing.T) {
	r := harness.NewPreconditionRunner()
	precs := []harness.Precondition{
		{ID: "p1", CapabilityID: "c1", Name: "true", Command: "true", TimeoutSec: 5, Enabled: true},
	}
	results, err := r.Run(context.Background(), precs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Passed {
		t.Fatalf("expected one passing result, got %+v", results)
	}
	if results[0].DurationMS < 0 {
		t.Fatalf("duration_ms should be non-negative, got %d", results[0].DurationMS)
	}
}

func TestPreconditionRunnerFails(t *testing.T) {
	r := harness.NewPreconditionRunner()
	precs := []harness.Precondition{
		{ID: "p1", CapabilityID: "c1", Name: "exit1", Command: "exit 1", TimeoutSec: 5, Enabled: true},
	}
	results, err := r.Run(context.Background(), precs)
	if err == nil {
		t.Fatal("expected error from failing precondition")
	}
	if !errors.Is(err, harness.ErrPreconditionFailed) {
		t.Fatalf("expected harness.ErrPreconditionFailed, got %v", err)
	}
	if len(results) != 1 || results[0].Passed {
		t.Fatalf("expected one failing result, got %+v", results)
	}
	if results[0].Err == nil {
		t.Fatal("expected inner error")
	}
	// Output is allowed to be empty for commands that exit without
	// writing (e.g. `exit 1`). The harness guarantees Output is
	// truncated, not non-empty.
}

func TestPreconditionRunnerSkipDisabled(t *testing.T) {
	r := harness.NewPreconditionRunner()
	precs := []harness.Precondition{
		{ID: "p1", CapabilityID: "c1", Name: "skip", Command: "exit 1", TimeoutSec: 5, Enabled: false},
	}
	results, err := r.Run(context.Background(), precs)
	if err != nil {
		t.Fatalf("disabled precondition should be skipped, got %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results for skipped preconditions, got %+v", results)
	}
}

func TestPreconditionRunnerFailFastStopsAtFirst(t *testing.T) {
	r := harness.NewPreconditionRunner()
	precs := []harness.Precondition{
		{ID: "p1", CapabilityID: "c1", Name: "exit1", Command: "exit 1", TimeoutSec: 5, Enabled: true},
		{ID: "p2", CapabilityID: "c1", Name: "echo2", Command: "echo two", TimeoutSec: 5, Enabled: true},
	}
	results, err := r.Run(context.Background(), precs)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(results) != 1 {
		t.Fatalf("expected fail-fast to stop at 1 result, got %d", len(results))
	}
}

func TestPreconditionRunnerRunAllContinuesPastFailure(t *testing.T) {
	r := harness.NewPreconditionRunner()
	precs := []harness.Precondition{
		{ID: "p1", CapabilityID: "c1", Name: "exit1", Command: "exit 1", TimeoutSec: 5, Enabled: true},
		{ID: "p2", CapabilityID: "c1", Name: "echo2", Command: "echo two", TimeoutSec: 5, Enabled: true},
	}
	results, err := r.RunAll(context.Background(), precs)
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if len(results) != 2 {
		t.Fatalf("expected RunAll to run every precondition, got %d", len(results))
	}
	if !results[1].Passed {
		t.Fatalf("second precondition should still pass, got %+v", results[1])
	}
}

func TestPreconditionRunnerTimeout(t *testing.T) {
	r := harness.NewPreconditionRunner()
	// Use a sleep that's much longer than the timeout so the test
	// proves the runner killed the process rather than the sleep
	// returning on its own. CI runs with shared / loaded runners and
	// can fork+exec a few hundred ms slower than a dev laptop, so
	// the upper bound is 1s of timeout + 4s of CI slack.
	precs := []harness.Precondition{
		{ID: "p1", CapabilityID: "c1", Name: "sleep", Command: "sleep 30", TimeoutSec: 1, Enabled: true},
	}
	start := time.Now()
	results, err := r.Run(context.Background(), precs)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	// 1s timeout + 4s slack. If the timeout didn't fire, the sleep
	// alone would take 30s and the assertion catches it.
	if elapsed > 5*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
	if results[0].Passed {
		t.Fatal("expected timed-out precondition to be marked failed")
	}
}

func TestTruncate(t *testing.T) {
	if got := harness.TruncateOutput("hello", 1024); got != "hello" {
		t.Fatalf("short input should pass through: %q", got)
	}
	long := ""
	for i := 0; i < 100; i++ {
		long += "abcdefgh"
	}
	got := harness.TruncateOutput(long, 64)
	if len(got) > 64+32 {
		t.Fatalf("truncate did not bound length: %d", len(got))
	}
	if !contains(got, "[truncated]") {
		t.Fatalf("expected truncated marker in output, got %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
