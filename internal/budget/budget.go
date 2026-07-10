// Package budget owns the Budget aggregate.
//
// A Budget is a periodic USD cap on spending (Workspace-wide or
// Capability-scoped). When the period rolls over the counter
// resets; when the spend exceeds the cap, the budget emits a
// budget.alert event and prevents new invocations from being
// accepted at the invoke path.
//
// Periods are closed sets so we can switch on them in code:
//
//   - daily:  resets at 00:00 UTC
//   - weekly: resets at 00:00 UTC Monday
//   - monthly: resets at 00:00 UTC on the first of the month
//
// Budget is its own aggregate (separate from Policy/Quota) so that
// alerts, dashboards, and invoice rollups all read the same shape.
package budget

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Period is the closed set of budget periods.
type Period string

const (
	PeriodDaily   Period = "daily"
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
)

// Scope is what a Budget applies to.
type Scope string

const (
	ScopeWorkspace Scope = "workspace"
	ScopeCapability Scope = "capability"
)

// Budget is the value-typed aggregate.
type Budget struct {
	ID         string    `json:"id"`
	Scope      Scope     `json:"scope"`
	TargetID   string    `json:"target_id"`
	Period     Period    `json:"period"`
	CapUSD     float64   `json:"cap_usd"`
	PeriodStart time.Time `json:"period_start"`
	SpentUSD   float64   `json:"spent_usd"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  string    `json:"created_by"`
}

// ErrCapNotPositive is returned when constructing a Budget with a
// non-positive cap.
var ErrCapNotPositive = errors.New("budget: cap must be > 0")

// New constructs a Budget with the period_start set to the
// canonical start of the current period.
func New(scope Scope, targetID string, period Period, capUSD float64, now time.Time, createdBy string) (Budget, error) {
	if capUSD <= 0 {
		return Budget{}, ErrCapNotPositive
	}
	switch scope {
	case ScopeWorkspace, ScopeCapability:
	default:
		return Budget{}, fmt.Errorf("budget: unknown scope %q", scope)
	}
	switch period {
	case PeriodDaily, PeriodWeekly, PeriodMonthly:
	default:
		return Budget{}, fmt.Errorf("budget: unknown period %q", period)
	}
	if targetID == "" {
		return Budget{}, errors.New("budget: target_id is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Budget{
		Scope:       scope,
		TargetID:    targetID,
		Period:      period,
		CapUSD:      capUSD,
		PeriodStart: startOf(period, now),
		SpentUSD:    0,
		CreatedAt:   now,
		CreatedBy:   createdBy,
	}, nil
}

// Charge records spend against the budget. If the new spend would
// exceed the cap the charge is rejected and ErrCapExceeded is
// returned; otherwise the budget is advanced and the new total
// is included in the returned Budget.
func (b Budget) Charge(amount float64, now time.Time) (Budget, error) {
	if amount < 0 {
		return b, errors.New("budget: charge amount must be >= 0")
	}
	// Roll the period forward if `now` is past the current
	// PeriodStart.
	if now.UTC().After(b.PeriodEnd()) {
		b.PeriodStart = startOf(b.Period, now.UTC())
		b.SpentUSD = 0
	}
	if b.SpentUSD+amount > b.CapUSD {
		return b, ErrCapExceeded
	}
	b.SpentUSD += amount
	return b, nil
}

// ErrCapExceeded is returned when a Charge would push spent over
// the cap.
var ErrCapExceeded = errors.New("budget: cap exceeded")

// Remaining returns the headroom left for the period.
func (b Budget) Remaining() float64 {
	r := b.CapUSD - b.SpentUSD
	if r < 0 {
		return 0
	}
	return r
}

// PeriodEnd returns the exclusive end of the current period.
func (b Budget) PeriodEnd() time.Time {
	return endOf(b.Period, b.PeriodStart)
}

// startOf computes the inclusive start of the period containing t.
func startOf(p Period, t time.Time) time.Time {
	t = t.UTC()
	switch p {
	case PeriodDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case PeriodWeekly:
		day := int(t.Weekday())
		return time.Date(t.Year(), t.Month(), t.Day()-day, 0, 0, 0, 0, time.UTC)
	case PeriodMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return t
}

// endOf computes the exclusive end of the period beginning at t.
func endOf(p Period, t time.Time) time.Time {
	t = t.UTC()
	switch p {
	case PeriodDaily:
		return t.Add(24 * time.Hour)
	case PeriodWeekly:
		return t.Add(7 * 24 * time.Hour)
	case PeriodMonthly:
		return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	}
	return t
}

// Repository is the consumer-defined persistence interface.
type Repository interface {
	CreateBudget(ctx context.Context, b *Budget) error
	GetBudget(ctx context.Context, id string) (*Budget, error)
	ListBudgetsForTarget(ctx context.Context, targetID string) ([]*Budget, error)
	UpdateBudget(ctx context.Context, b *Budget) error
	DeleteBudget(ctx context.Context, id string) error
}
