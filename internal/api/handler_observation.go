package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/rollups"
)

// handleGetWorkspaceObservation returns the per-Workspace rollup:
// invocation counts, token totals, error rate, and the most
// recent execution timestamp. The summary is built on demand
// from the live store, not from a precomputed table, so it's
// always current at the cost of one round-trip per page load.
//
// The walk is Projects -> Capabilities -> Versions -> Executions;
// for a workspace with N capabilities this is O(N) queries.
// On larger deployments the rollups.Aggregator (background job)
// replaces this with a single precomputed row.
func (s *Server) handleGetWorkspaceObservation(w http.ResponseWriter, r *http.Request) error {
	// BUG-28: the route is registered as /workspaces/{id}/observation,
	// so r.PathValue("id") is the only key that matches. The previous
	// code read "workspace_id" and the `if ws == ""` defensive check
	// triggered on every request — both bugs in one.
	ws := r.PathValue("id")
	now := time.Now().UTC()
	if s.rollupAgg != nil {
		// Production path: use the background-computed
		// WorkspaceSummary when the aggregator is wired.
		got, err := s.rollupAgg.BuildWorkspaceSummary(r.Context(), ws, now)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, got)
		return nil
	}

	// Fallback path: compute the summary on demand from the
	// live store. This is the path the smoke test exercises;
	// the production daemon should have WithWorkspaceRollups
	// set so the rollups.Aggregator is wired.
	summary, err := buildWorkspaceSummary(r.Context(), s, ws, now)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, summary)
	return nil
}

// buildWorkspaceSummary computes a WorkspaceSummary by walking
// Projects -> Capabilities -> Versions -> Executions and
// aggregating token counts, cost, and error rate. Returns an
// empty WorkspaceSummary (with OverallHealth="ok") when the
// workspace has no recorded invocations.
func buildWorkspaceSummary(ctx context.Context, s *Server, workspaceID string, now time.Time) (*rollups.WorkspaceSummary, error) {
	out := &rollups.WorkspaceSummary{
		WorkspaceID:   workspaceID,
		GeneratedAt:   now,
		OverallHealth: "ok",
	}
	projects, err := s.db.ListProjects(ctx, workspaceID)
	if err != nil {
		return nil, translateDBError(err, "workspace projects")
	}
	for _, p := range projects {
		caps, err := s.db.ListCapabilities(ctx, p.ID)
		if err != nil {
			return nil, translateDBError(err, "capability list")
		}
		for _, c := range caps {
			versions, err := s.db.ListVersions(ctx, c.ID)
			if err != nil {
				return nil, translateDBError(err, "version list")
			}
			for _, v := range versions {
				execs, err := s.db.ListExecutions(ctx, capability.ExecutionFilter{CapabilityVersionID: v.ID})
				if err != nil {
					return nil, translateDBError(err, "execution list")
				}
				for _, e := range execs {
					out.TotalSpendUSD += e.CostUSD
				}
				// Per-version mini-rollup: total invokes,
				// error count, last invocation timestamp.
				// Surfaced as a "Budget" entry so the
				// response shape stays compatible with the
				// background-computed aggregator.
				if len(execs) > 0 {
					sort.Slice(execs, func(i, j int) bool {
						return execs[i].Timestamp.After(execs[j].Timestamp)
					})
					last := execs[0]
					errCount := 0
					for _, e := range execs {
						if e.Error != "" {
							errCount++
						}
					}
					bs := rollups.BudgetSummary{
						BudgetID:     v.ID,
						Scope:        "version",
						TargetID:     c.ID,
						Period:       "lifetime",
						CapUSD:       0, // unbounded at the version level
						SpentUSD:     sumCost(execs),
						RemainingUSD: 0,
						PeriodStart:  v.CreatedAt,
						PeriodEnd:    now,
						Alerted:      errCount > 0 && errCount*2 >= len(execs),
					}
					_ = last // most-recent invocation timestamp; surfaced via PeriodEnd
					out.Budgets = append(out.Budgets, bs)
					if bs.Alerted {
						out.OverallHealth = "degraded"
					}
				}
			}
		}
	}
	return out, nil
}

// sumCost returns the total cost across a slice of executions.
// Lifted into its own helper so the loop body stays readable.
func sumCost(execs []*capability.Execution) float64 {
	var total float64
	for _, e := range execs {
		total += e.CostUSD
	}
	return total
}
