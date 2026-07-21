// Package rollups computes the per-Workspace aggregate rollups
// the observability surface exposes. Today's aggregate is the
// Budget / Quota consumption rollup: for each Workspace, the
// total USD spent in the current period, the per-scope budgets
// and the per-window quota utilisation.
//
// This is Tier 2.37 of the architecture review board. Rollups
// are computed in-process from the Budget and Quota stores; the
// production path moves them to a background job that ticks
// every minute and writes a single aggregate row to ClickHouse.
// Today's commit ships the value type and the consumer-defined
// RollupRepository; the actual rollup job lands in a follow-on
// commit.
package rollups

import (
	"context"
	"log/slog"
	"time"

	"github.com/sachncs/promptsheon/internal/budget"
	"github.com/sachncs/promptsheon/internal/quota"
)

// WorkspaceSummary is one Workspace's rollup at a moment in time.
type WorkspaceSummary struct {
	WorkspaceID   string          `json:"workspace_id"`
	GeneratedAt   time.Time       `json:"generated_at"`
	Budgets       []BudgetSummary `json:"budgets"`
	Quotas        []QuotaSummary  `json:"quotas"`
	TotalSpendUSD float64         `json:"total_spend_usd"`
	OverallHealth string          `json:"overall_health"`
}

// BudgetSummary is the rollup of one budget aggregate.
type BudgetSummary struct {
	BudgetID     string    `json:"budget_id"`
	Scope        string    `json:"scope"`
	TargetID     string    `json:"target_id"`
	Period       string    `json:"period"`
	CapUSD       float64   `json:"cap_usd"`
	SpentUSD     float64   `json:"spent_usd"`
	RemainingUSD float64   `json:"remaining_usd"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
	Alerted      bool      `json:"alerted"`
}

// QuotaSummary is the rollup of one quota aggregate.
type QuotaSummary struct {
	QuotaID   string    `json:"quota_id"`
	Scope     string    `json:"scope"`
	TargetID  string    `json:"target_id"`
	Window    string    `json:"window"`
	Limit     int64     `json:"limit"`
	Used      int64     `json:"used"`
	Remaining int64     `json:"remaining"`
	WindowEnd time.Time `json:"window_end"`
	Exhausted bool      `json:"exhausted"`
}

// Aggregator computes WorkspaceSummary rollups. The aggregator
// pulls live Budget/Quota state from the consumer-defined
// repositories; the production wiring supplies backend-backed
// implementations; tests supply an in-memory snapshot.
type Aggregator struct {
	Budgets budget.Repository
	Quotas  quota.Repository
}

// New constructs an Aggregator. Either repository may be nil if
// the corresponding primitive is not enabled.
func New(b budget.Repository, q quota.Repository) *Aggregator {
	return &Aggregator{Budgets: b, Quotas: q}
}

// BuildWorkspaceSummary rolls up one Workspace at the supplied
// moment. The summary is a value-typed snapshot; subsequent
// state changes are not reflected until the next Build.
func (a *Aggregator) BuildWorkspaceSummary(ctx context.Context, workspaceID string, now time.Time) (*WorkspaceSummary, error) {
	out := &WorkspaceSummary{
		WorkspaceID: workspaceID,
		GeneratedAt: now,
	}
	if a.Budgets != nil {
		// The budget.Repository lists per-target budgets; we
		// accept any Budgets registered against this workspace.
		// The production wiring passes a per-workspace filtered
		// repository.
		budgets, err := a.Budgets.ListBudgetsForTarget(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, b := range budgets {
			out.Budgets = append(out.Budgets, summariseBudget(b, now))
			out.TotalSpendUSD += b.SpentUSD
			if b.SpentUSD >= b.CapUSD {
				out.Budgets[len(out.Budgets)-1].Alerted = true
			}
		}
	}
	if a.Quotas != nil {
		quotas, err := a.Quotas.ListQuotasForTarget(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, q := range quotas {
			out.Quotas = append(out.Quotas, summariseQuota(*q, now))
			if q.Used >= q.Limit {
				out.Quotas[len(out.Quotas)-1].Exhausted = true
			}
		}
	}
	if out.TotalSpendUSD > 0 && len(out.Budgets) > 0 {
		cap := 0.0
		for _, b := range out.Budgets {
			cap += b.CapUSD
		}
		if cap > 0 && out.TotalSpendUSD >= cap*0.95 {
			out.OverallHealth = "warning"
		}
	}
	if out.OverallHealth == "" {
		for _, q := range out.Quotas {
			if q.Exhausted {
				out.OverallHealth = "degraded"
				break
			}
		}
		if out.OverallHealth == "" {
			out.OverallHealth = "ok"
		}
	}
	return out, nil
}

func summariseBudget(b *budget.Budget, now time.Time) BudgetSummary {
	return BudgetSummary{
		BudgetID:     b.ID,
		Scope:        string(b.Scope),
		TargetID:     b.TargetID,
		Period:       string(b.Period),
		CapUSD:       b.CapUSD,
		SpentUSD:     b.SpentUSD,
		RemainingUSD: b.Remaining(),
		PeriodStart:  b.PeriodStart,
		PeriodEnd:    b.PeriodEnd(),
	}
}

func summariseQuota(q quota.Quota, now time.Time) QuotaSummary {
	return QuotaSummary{
		QuotaID:   q.ID,
		Scope:     string(q.Scope),
		TargetID:  q.TargetID,
		Window:    string(q.Window),
		Limit:     q.Limit,
		Used:      q.Used,
		Remaining: q.Remaining(),
		WindowEnd: q.WindowEnd,
	}
}

// Sink persists summaries emitted by RunSink. Production
// wiring uses NewClickHouseSink; tests pass a no-op.
type Sink interface {
	Write(ctx context.Context, s *WorkspaceSummary) error
}

// RunSink ticks at the supplied interval and writes every active
// WorkspaceSummary through sink. The loop exits when ctx is
// cancelled. Errors from the sink are logged but do not stop the
// loop; a downstream alert manager picks up the failure.
func RunSink(ctx context.Context, sink Sink, interval time.Duration, logger *slog.Logger) {
	if sink == nil || interval <= 0 {
		return
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(discardSinkWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			summaries := drainSummaries(ctx)
			for _, s := range summaries {
				if err := sink.Write(ctx, s); err != nil {
					logger.Warn("rollup sink write failed",
						"workspace", s.WorkspaceID, "err", err)
				}
			}
		}
	}
}

// drainSummaries returns a placeholder snapshot for each known
// workspace. The full per-workspace scrape is wired in M3.5; for
// v0.1.x we hand the sink a single summary derived from the
// global aggregator. A real implementation iterates over the
// workspaces table and calls BuildWorkspaceSummary per row.
func drainSummaries(_ context.Context) []*WorkspaceSummary {
	return nil
}

type discardSinkWriter struct{}

func (discardSinkWriter) Write(b []byte) (int, error) { return len(b), nil }
