// Package quota owns the Quota aggregate.
//
// A Quota is a rate cap on a per-Workspace or per-Provider basis
// over a sliding window. Where Budget enforces USD ceilings over
// periods (daily/weekly/monthly), Quota enforces request/execution
// rates per second and per minute so the platform does not run
// away with a hot loop.
//
// Quota is intentionally distinct from the existing
// internal/ratelimit which is process-local and used for HTTP-edge
// throttling. Quota persists across processes so a tenant-level
// limit holds even when the daemon is restarted.
package quota

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Scope identifies what a Quota applies to.
type Scope string

const (
	ScopeWorkspace Scope = "workspace"
	ScopeProvider  Scope = "provider"
)

// Window is the sliding window a Quota is measured over.
type Window string

const (
	WindowSecond Window = "second"
	WindowMinute Window = "minute"
	WindowHour   Window = "hour"
)

// Quota is the value-typed aggregate.
type Quota struct {
	ID        string    `json:"id"`
	Scope     Scope     `json:"scope"`
	TargetID  string    `json:"target_id"`
	Window    Window    `json:"window"`
	Limit     int64     `json:"limit"`
	Used      int64     `json:"used"`
	WindowEnd time.Time `json:"window_end"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// ErrLimitNotPositive is returned when constructing a Quota with
// a non-positive limit.
var ErrLimitNotPositive = errors.New("quota: limit must be > 0")

// ErrOverLimit is returned by Charge when the window's used
// counter would exceed the limit.
var ErrOverLimit = errors.New("quota: over limit")

// New constructs a Quota at the start of the supplied window.
func New(scope Scope, targetID string, window Window, limit int64, now time.Time, createdBy string) (Quota, error) {
	if limit <= 0 {
		return Quota{}, ErrLimitNotPositive
	}
	switch scope {
	case ScopeWorkspace, ScopeProvider:
	default:
		return Quota{}, fmt.Errorf("quota: unknown scope %q", scope)
	}
	switch window {
	case WindowSecond, WindowMinute, WindowHour:
	default:
		return Quota{}, fmt.Errorf("quota: unknown window %q", window)
	}
	if targetID == "" {
		return Quota{}, errors.New("quota: target_id is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Quota{
		Scope:     scope,
		TargetID:  targetID,
		Window:    window,
		Limit:     limit,
		WindowEnd: nextBoundary(window, now),
		CreatedAt: now,
		CreatedBy: createdBy,
	}, nil
}

// Charge increments the window counter; returns ErrOverLimit when
// the limit is reached.
func (q Quota) Charge(now time.Time) (Quota, error) {
	if now.UTC().After(q.WindowEnd) || now.UTC().Equal(q.WindowEnd) {
		q.WindowEnd = nextBoundary(q.Window, now.UTC())
		q.Used = 0
	}
	if q.Used >= q.Limit {
		return q, ErrOverLimit
	}
	q.Used++
	return q, nil
}

// Remaining returns the headroom left in the current window.
func (q Quota) Remaining() int64 {
	r := q.Limit - q.Used
	if r < 0 {
		return 0
	}
	return r
}

// nextBoundary computes the end of the window containing t
// (exclusive).
func nextBoundary(w Window, t time.Time) time.Time {
	t = t.UTC()
	switch w {
	case WindowSecond:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()+1, 0, time.UTC)
	case WindowMinute:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()+1, 0, 0, time.UTC)
	case WindowHour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, time.UTC)
	}
	return t
}

// Repository is the consumer-defined persistence interface.
type Repository interface {
	CreateQuota(ctx context.Context, q *Quota) error
	GetQuota(ctx context.Context, id string) (*Quota, error)
	ListQuotasForTarget(ctx context.Context, targetID string) ([]*Quota, error)
	UpdateQuota(ctx context.Context, q *Quota) error
	DeleteQuota(ctx context.Context, id string) error
}
