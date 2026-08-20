/**
 * Pareto-frontier analysis across experiment variants. The
 * evolver stores a set of (pass_rate, resistance) points; the
 * frontier is the maximal set under both axes. Each frontier
 * point carries a "retune next" hint describing which axes need
 * movement to leave the frontier.
 *
 * Pure function — no DB, no async. The routes / evolver read
 * this and surface the result in /app/admin/cost etc.
 */

export interface ExperimentPoint {
  label: string;
  passRate: number;
  resistance: number;
  costMicros: number;
}

export interface ParetoPoint {
  label: string;
  passRate: number;
  resistance: number;
  costMicros: number;
  isFrontier: boolean;
  dominatedBy: string[];
  retuneHint: string;
}

function dominates(a: ExperimentPoint, b: ExperimentPoint): boolean {
  // Higher pass rate and higher resistance are both better.
  // Cost is a constraint — lower is better, but we don't filter
  // by it here; cost is reported on the frontier.
  return (
    a.passRate >= b.passRate &&
    a.resistance >= b.resistance &&
    (a.passRate > b.passRate || a.resistance > b.resistance)
  );
}

export function paretoFrontier(points: ExperimentPoint[]): ParetoPoint[] {
  const frontier: ParetoPoint[] = [];
  for (const p of points) {
    const dominatedBy: string[] = [];
    for (const q of points) {
      if (q === p) continue;
      if (dominates(q, p)) dominatedBy.push(q.label);
    }
    const isFrontier = dominatedBy.length === 0;
    const retuneHint = !isFrontier
      ? suggestRetune(p, points.filter((q) => dominates(q, p)))
      : 'frontier: no change suggested';
    frontier.push({
      label: p.label,
      passRate: p.passRate,
      resistance: p.resistance,
      costMicros: p.costMicros,
      isFrontier,
      dominatedBy,
      retuneHint,
    });
  }
  return frontier;
}

function suggestRetune(p: ExperimentPoint, dominators: ExperimentPoint[]): string {
  if (dominators.length === 0) return 'no change suggested';
  const best = dominators.sort((a, b) => {
    const sa = a.passRate + a.resistance;
    const sb = b.passRate + b.resistance;
    return sb - sa;
  })[0];
  if (!best) return 'no change suggested';
  const dPR = (best.passRate - p.passRate).toFixed(3);
  const dRE = (best.resistance - p.resistance).toFixed(3);
  return `dominated by "${best.label}": pass_rate +${dPR}, resistance +${dRE}`;
}

/**
 * Pick the cheapest frontier point at or above a pass-rate
 * threshold. Used by the CI gate to choose a release.
 */
export function pickCheapestFrontier(
  frontier: ParetoPoint[],
  passRateThreshold: number,
): ParetoPoint | null {
  const eligible = frontier
    .filter((p) => p.isFrontier && p.passRate >= passRateThreshold)
    .sort((a, b) => a.costMicros - b.costMicros);
  return eligible[0] ?? null;
}
