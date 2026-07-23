// Package schedule owns the Schedule aggregate.
//
// A Schedule triggers an Execution against a target Release. The
// trigger can be:
//
//   - cron:       a 5-field cron expression evaluated in the
//     daemon's local timezone (UTC by default).
//   - webhook:    an HTTP POST to /v1/schedules/{id}/fire authenticated
//     with the workspace API key.
//   - manual:     a human-initiated fire via the CLI.
//
// All three converge on a single tick that produces an Execution
// record referencing the Release; the ExecutionHash then closes
// against the Replay buffer for hash-stable round-trip
// reproducibility.
//
// Schedules are immutable identities. To change a schedule, create
// a new one and Supersede the previous.
//
// The Repository interface lives with the consumer-defined package;
// this file declares it.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Kind is how a Schedule decides when to fire.
type Kind string

const (
	KindCron    Kind = "cron"
	KindWebhook Kind = "webhook"
	KindManual  Kind = "manual"
)

// Schedule is the value-typed aggregate. Transitions return new
// values rather than mutating in place, in keeping with the
// immutability principle for domain objects.
type Schedule struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	ReleaseID   string     `json:"release_id"`
	Kind        Kind       `json:"kind"`
	Cron        string     `json:"cron,omitempty"`
	WebhookPath string     `json:"webhook_path,omitempty"`
	NextFireAt  time.Time  `json:"next_fire_at"`
	LastFireAt  *time.Time `json:"last_fire_at,omitempty"`
	FiredCount  int64      `json:"fired_count"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
}

// New constructs a Schedule. Returns ErrInvalidCron when the kind
// is cron and the expression is not a valid 5-field cron expression.
func New(workspaceID, releaseID string, kind Kind, cronExpr, webhookPath string) (Schedule, error) {
	if workspaceID == "" || releaseID == "" {
		return Schedule{}, errors.New("schedule: workspace_id and release_id required")
	}
	switch kind {
	case KindCron, KindWebhook, KindManual:
	default:
		return Schedule{}, fmt.Errorf("schedule: unknown kind %q", kind)
	}
	var next time.Time
	if kind == KindCron {
		t, err := nextCron(cronExpr, time.Now().UTC())
		if err != nil {
			return Schedule{}, err
		}
		next = t
	}
	return Schedule{
		WorkspaceID: workspaceID,
		ReleaseID:   releaseID,
		Kind:        kind,
		Cron:        cronExpr,
		WebhookPath: webhookPath,
		NextFireAt:  next,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// ErrInvalidCron signals an unparseable cron expression.
var ErrInvalidCron = errors.New("schedule: invalid cron expression")

// Validate returns ErrInvalidCron if cron is empty or unparseable.
func (s Schedule) Validate() error {
	if s.Kind != KindCron {
		return nil
	}
	if _, err := nextCron(s.Cron, time.Now().UTC()); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidCron, err)
	}
	return nil
}

// MarkFired advances LastFireAt and the fired count, and computes
// the next fire time. Returns a new value.
func (s Schedule) MarkFired(at time.Time) Schedule {
	t := at
	s.LastFireAt = &t
	s.FiredCount++
	if s.Kind == KindCron {
		if next, err := nextCron(s.Cron, at); err == nil {
			s.NextFireAt = next
		}
	}
	return s
}

// Disable returns a copy with Enabled=false. Immutability: the
// receiver is left untouched.
func (s Schedule) Disable() Schedule {
	s.Enabled = false
	return s
}

// Enable returns a copy with Enabled=true. Immutability: the
// receiver is left untouched.
func (s Schedule) Enable() Schedule {
	s.Enabled = true
	return s
}

// nextCron is a minimal 5-field cron parser that returns the next
// firing time strictly after `from`. It supports the spec the user
// is most likely to type in a config file:
//
//	field 1: minute        (0-59)
//	field 2: hour          (0-23)
//	field 3: day of month  (1-31)
//	field 4: month         (1-12)
//	field 5: day of week    (0-6, 0=Sun)
//
// Each field may be `*`, a single integer, or a comma list. Range
// lists (a-b) and step expressions (a-b/n) are NOT supported in M2
// and produce ErrInvalidCron. The parsing is deliberate: full cron
// parsers ship in M2 follow-on if real operators ask for them.
func nextCron(expr string, from time.Time) (time.Time, error) {
	if expr == "" {
		return time.Time{}, ErrInvalidCron
	}
	parts := splitWhitespace(expr)
	if len(parts) != 5 {
		return time.Time{}, ErrInvalidCron
	}
	parts2 := make([]string, 5)
	for i, p := range parts {
		parts2[i] = p
	}
	parts = parts2
	minute, err := parseField(parts[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseField(parts[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("hour: %w", err)
	}
	dom, err := parseField(parts[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("dom: %w", err)
	}
	month, err := parseField(parts[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("month: %w", err)
	}
	dow, dowWild, err := parseFieldWithWildcard(parts[4], 0, 6)
	if err != nil {
		return time.Time{}, fmt.Errorf("dow: %w", err)
	}
	dom, domWild, err := parseFieldWithWildcard(parts[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("dom: %w", err)
	}

	// Walk minute-by-minute from `from+1min` up to a 366-day cap.
	// POSIX cron DOM/DOW semantics: when EITHER field is a
	// literal wildcard ("*"), use the OTHER field as the only
	// day constraint. When BOTH are restricted, use OR. The
	// previous implementation always OR'd the two, which
	// meant "0 0 * * 1" matched every day whose weekday mask
	// was OR'd with the dom mask — in practice any cron
	// expression with one wildcard fired daily.
	t := from.UTC().Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 366*24*60; i++ {
		if minute[t.Minute()] &&
			hour[t.Hour()] &&
			month[int(t.Month())] {
			dayMatch := false
			switch {
			case domWild && dowWild:
				dayMatch = true
			case domWild:
				dayMatch = dow[int(t.Weekday())]
			case dowWild:
				dayMatch = dom[int(t.Day())]
			default:
				dayMatch = dom[int(t.Day())] || dow[int(t.Weekday())]
			}
			if dayMatch {
				return t, nil
			}
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, ErrInvalidCron
}

// parseField accepts "*", "n", "n,m,...", "a-b", and "*/n" step
// expressions. Returns a slice indexed by the field's natural
// integer range for fast membership tests. All errors wrap
// ErrInvalidCron so callers can errors.Is against the sentinel.
func parseField(s string, lo, hi int) ([]bool, error) {
	out, _, err := parseFieldWithWildcard(s, lo, hi)
	return out, err
}

// parseFieldWithWildcard returns the boolean mask plus a flag
// indicating whether the input was the literal "*". The flag is
// used by the DOM/DOW logic in nextCron to apply the correct
// POSIX "either-field-wildcard means the other field wins"
// rule. A comma list like "1,2,3" sets several bits to true
// and is NOT a wildcard, even though the resulting mask is
// similar to "*".
func parseFieldWithWildcard(s string, lo, hi int) ([]bool, bool, error) {
	out := make([]bool, hi+1)
	if s == "*" {
		for i := lo; i <= hi; i++ {
			out[i] = true
		}
		return out, true, nil
	}
	for _, raw := range splitCSV(s) {
		if raw == "" {
			return nil, false, fmt.Errorf("%w: empty token", ErrInvalidCron)
		}
		if strings.Contains(raw, "/") {
			if err := applyStep(out, raw, lo, hi); err != nil {
				return nil, false, fmt.Errorf("%w: %w", ErrInvalidCron, err)
			}
			continue
		}
		if strings.Contains(raw, "-") {
			if err := applyRange(out, raw, lo, hi); err != nil {
				return nil, false, fmt.Errorf("%w: %w", ErrInvalidCron, err)
			}
			continue
		}
		v, err := atoiStrict(raw, lo, hi)
		if err != nil {
			return nil, false, fmt.Errorf("%w: %w", ErrInvalidCron, err)
		}
		out[v] = true
	}
	return out, false, nil
}

// applyStep sets every Nth value from start (or lo) up to hi.
// Accepts "a/n" (start at a, every n) and "*/n" (every n from lo).
func applyStep(out []bool, raw string, lo, hi int) error {
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("malformed step %q", raw)
	}
	step, err := atoiStrict(parts[1], 1, hi-lo+1)
	if err != nil {
		return fmt.Errorf("step %q: %w", raw, err)
	}
	start := lo
	if parts[0] != "*" {
		start, err = atoiStrict(parts[0], lo, hi)
		if err != nil {
			return fmt.Errorf("start %q: %w", raw, err)
		}
	}
	for v := start; v <= hi; v += step {
		out[v] = true
	}
	return nil
}

// applyRange sets every value in [a, b].
func applyRange(out []bool, raw string, lo, hi int) error {
	parts := strings.SplitN(raw, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("malformed range %q", raw)
	}
	a, err := atoiStrict(parts[0], lo, hi)
	if err != nil {
		return fmt.Errorf("range start %q: %w", raw, err)
	}
	b, err := atoiStrict(parts[1], lo, hi)
	if err != nil {
		return fmt.Errorf("range end %q: %w", raw, err)
	}
	if a > b {
		return fmt.Errorf("range %q is descending", raw)
	}
	for v := a; v <= b; v++ {
		out[v] = true
	}
	return nil
}

func splitCSV(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// splitWhitespace splits on runs of ASCII whitespace.
func splitWhitespace(s string) []string {
	out := []string{}
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// splitCSVFields is retained for callers that want to enforce a
// field count from a comma-separated expression.
func splitCSVFields(s string, expected int) []string {
	parts := splitCSV(s)
	if len(parts) != expected {
		return nil
	}
	out := make([]string, expected)
	for i, p := range parts {
		out[i] = p
	}
	return out
}

func atoiStrict(s string, lo, hi int) (int, error) {
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid char %q in %q", c, s)
		}
		v = v*10 + int(c-'0')
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("%d out of range [%d,%d]", v, lo, hi)
	}
	return v, nil
}

// Repository is the consumer-defined persistence interface for
// Schedules.
type Repository interface {
	CreateSchedule(ctx context.Context, s *Schedule) error
	ListDueSchedules(ctx context.Context, now time.Time, limit int) ([]*Schedule, error)
	UpdateSchedule(ctx context.Context, s *Schedule) error
}
