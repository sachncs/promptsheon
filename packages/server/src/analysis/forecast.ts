import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type {
  BudgetPeriod,
  CostBudget,
  CostForecast,
  CostForecastSnapshot,
} from '@promptsheon/shared';
import { CostRollupRepo } from '../repos/vault-extras.js';

/**
 * Linear-regression forecast: fits y = a + b*x on the last
 * `windowDays` days of per-day spend, then projects the period-end
 * total. Returns the regression coefficients + the 95% confidence
 * band on the projection.
 *
 * The maths are deliberately simple — ordinary least squares with
 * a fixed x = (dayIndex − mean(dayIndex)) so we can invert the
 * normal equations without a matrix library.
 */
export interface RegressionResult {
  /** y-intercept (spend at the centre of the window, in micros). */
  intercept: number;
  /** Slope (micros per day). */
  slope: number;
  /** r^2 of the fit, 0..1. */
  r2: number;
  /** Residual standard deviation of the fit, in micros. */
  residualStdDev: number;
}

export function linearRegression(
  points: ReadonlyArray<{ x: number; y: number }>,
): RegressionResult | null {
  const n = points.length;
  if (n < 2) return null;
  let sumX = 0;
  let sumY = 0;
  for (const p of points) {
    sumX += p.x;
    sumY += p.y;
  }
  const meanX = sumX / n;
  const meanY = sumY / n;
  let sxx = 0;
  let sxy = 0;
  let syy = 0;
  for (const p of points) {
    const dx = p.x - meanX;
    sxx += dx * dx;
    sxy += dx * (p.y - meanY);
    syy += (p.y - meanY) ** 2;
  }
  if (sxx === 0) return null;
  const slope = sxy / sxx;
  const intercept = meanY - slope * meanX;
  const r2 = syy === 0 ? 1 : (sxy * sxy) / (sxx * syy);
  // Residual sum of squares for σ̂.
  let rss = 0;
  for (const p of points) {
    const yhat = intercept + slope * p.x;
    const r = p.y - yhat;
    rss += r * r;
  }
  const denom = n - 2;
  if (denom <= 0) {
    return { intercept, slope, r2, residualStdDev: 0 };
  }
  const sigma = Math.sqrt(rss / denom);
  return { intercept, slope, r2, residualStdDev: sigma };
}

/**
 * Build the per-day time series for an organisation from the cost
 * rollups, filling missing days with zeros so the regression sees
 * a contiguous window.
 */
export function buildDailySeries(
  rollups: ReadonlyArray<{ day: string; costMicros: number }>,
  windowDays: number,
): Array<{ x: number; y: number; day: string }> {
  if (windowDays <= 0) return [];
  const byDay = new Map<string, number>();
  for (const r of rollups) byDay.set(r.day, r.costMicros);
  const series: Array<{ x: number; y: number; day: string }> = [];
  const today = new Date();
  for (let i = windowDays - 1; i >= 0; i -= 1) {
    const d = new Date(today);
    d.setUTCDate(d.getUTCDate() - i);
    const iso = d.toISOString().slice(0, 10);
    series.push({ x: windowDays - 1 - i, y: byDay.get(iso) ?? 0, day: iso });
  }
  return series;
}

export interface PeriodBounds {
  start: string;
  end: string;
  /** Number of days from `start` to `end`, inclusive. */
  days: number;
  /** Day index of "today" inside the period (0-based). */
  todayIndex: number;
}

/**
 * Compute the [start, end] window for a budget period containing
 * "today". `today` is anchored to midnight UTC so the math is
 * deterministic across operator timezones.
 */
export function periodBounds(
  period: BudgetPeriod,
  now: Date = new Date(),
): PeriodBounds {
  const todayIso = now.toISOString().slice(0, 10);
  if (period === 'monthly') {
    const start = isoForDate(now.getUTCFullYear(), now.getUTCMonth() + 1, 1);
    const nextMonth = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 1));
    const end = new Date(nextMonth.getTime() - 86_400_000);
    const endIso = end.toISOString().slice(0, 10);
    const days = daysBetween(start, endIso) + 1;
    const todayIndex = daysBetween(start, todayIso);
    return { start, end: endIso, days, todayIndex };
  }
  // weekly: ISO week starting Monday.
  const dow = now.getUTCDay(); // 0..6 with Sun=0
  const offsetToMonday = (dow + 6) % 7;
  const monday = new Date(now.getTime() - offsetToMonday * 86_400_000);
  const sunday = new Date(monday.getTime() + 6 * 86_400_000);
  const start = monday.toISOString().slice(0, 10);
  const end = sunday.toISOString().slice(0, 10);
  return {
    start,
    end,
    days: 7,
    todayIndex: daysBetween(start, todayIso),
  };
}

function daysBetween(startIso: string, endIso: string): number {
  const start = Date.parse(`${startIso}T00:00:00Z`);
  const end = Date.parse(`${endIso}T00:00:00Z`);
  return Math.round((end - start) / 86_400_000);
}

function isoForDate(year: number, month0: number, day: number): string {
  const d = new Date(Date.UTC(year, month0 - 1, day));
  return d.toISOString().slice(0, 10);
}

export interface ForecastOptions {
  /** Window of historical days to fit. Defaults to 30. */
  windowDays?: number;
}

export interface ForecastService {
  compute(orgId: string, options?: ForecastOptions): CostForecast | null;
  snapshot(orgId: string, options?: ForecastOptions): CostForecastSnapshot | null;
}

/**
 * Forecast service: reads cost rollups, runs a linear regression,
 * projects the period total with a 95% confidence band, and
 * caches the result for the dashboard. Returns null when the org
 * has no rollups in the window — that's a valid "no spend yet"
 * state, not an error.
 */
export class CostForecastService implements ForecastService {
  private rollups: CostRollupRepo;
  private snapshotter: (s: CostForecastSnapshot) => void;
  private lookups: {
    listBudgets(orgId: string): CostBudget[];
    updateLastAlerted(budgetId: string, ts: string): void;
  };

  constructor(
    private db: Database.Database,
    deps: {
      rollups?: CostRollupRepo;
      persistSnapshot?: (s: CostForecastSnapshot) => void;
      listBudgets?: (orgId: string) => CostBudget[];
      updateLastAlerted?: (budgetId: string, ts: string) => void;
    } = {},
  ) {
    this.rollups = deps.rollups ?? new CostRollupRepo(db);
    this.snapshotter = deps.persistSnapshot ?? (() => undefined);
    this.lookups = {
      listBudgets: deps.listBudgets ?? (() => []),
      updateLastAlerted: deps.updateLastAlerted ?? (() => undefined),
    };
  }

  compute(orgId: string, options: ForecastOptions = {}): CostForecast | null {
    const snapshot = this.snapshot(orgId, options);
    if (!snapshot) return null;
    const bounds = periodBounds('monthly'); // default; overridden by budgets
    const budgets = this.lookups.listBudgets(orgId);
    const alerts: CostForecast['alerts'] = [];
    const now = new Date().toISOString();
    for (const b of budgets) {
      if (!b.enabled) continue;
      const projected = snapshot.projectedMicros;
      const fraction = b.limitMicros === 0 ? 0 : projected / b.limitMicros;
      if (fraction >= b.alertThreshold) {
        alerts.push({
          budgetId: b.id,
          label: b.label,
          projectedMicros: projected,
          limitMicros: b.limitMicros,
          alertThreshold: b.alertThreshold,
          fraction,
        });
        // Cooldown: only stamp once per hour so a webhook storm
        // doesn't fire on every page load.
        const lastAlert = b.lastAlertedAt ? Date.parse(b.lastAlertedAt) : 0;
        if (Date.now() - lastAlert >= 60 * 60_000) {
          this.lookups.updateLastAlerted(b.id, now);
        }
      }
    }
    void bounds;
    return { snapshot, alerts };
  }

  snapshot(orgId: string, options: ForecastOptions = {}): CostForecastSnapshot | null {
    const windowDays = options.windowDays ?? 30;
    const rollups = this.rollups.rollupsForOrg(orgId, windowDays);
    if (rollups.length === 0) return null;
    const series = buildDailySeries(rollups, windowDays);
    const fit = linearRegression(series);
    if (!fit) return null;

    const bounds = periodBounds('monthly');
    const spendSoFar = sumMicros(rollups, bounds.start, isoToday());
    const totalDays = bounds.days;
    const daysElapsed = Math.max(bounds.todayIndex + 1, 1);

    // Projection: scale current spend by totalDays / daysElapsed,
    // blended with the linear-regression projection. Blending
    // avoids the early-period "we're at 10% of the budget so we'll
    // finish at 10%" trap.
    const naiveProjection = (spendSoFar / daysElapsed) * totalDays;
    const regressionProjection = fit.intercept + fit.slope * totalDays;
    const projected = Math.max(naiveProjection, regressionProjection);
    // 95% band: ±1.96 * residual_std * sqrt(1 + 1/n + (x − x̄)^2 / Σ(x − x̄)^2)
    const meanX = (windowDays - 1) / 2;
    let sxx = 0;
    for (const p of series) {
      const dx = p.x - meanX;
      sxx += dx * dx;
    }
    const se =
      sxx === 0
        ? fit.residualStdDev
        : fit.residualStdDev * Math.sqrt(1 + 1 / windowDays + ((totalDays - meanX) ** 2) / sxx);
    const margin = 1.96 * se;
    const bandLow = Math.max(0, projected - margin);
    const bandHigh = projected + margin;

    const id = randomUUID();
    const snapshot: CostForecastSnapshot = {
      id,
      organizationId: orgId,
      periodStart: bounds.start,
      periodEnd: bounds.end,
      spendMicros: spendSoFar,
      projectedMicros: Math.round(projected),
      bandLowMicros: Math.max(0, Math.round(bandLow)),
      bandHighMicros: Math.round(bandHigh),
      windowDays,
      computedAt: new Date().toISOString(),
    };
    this.snapshotter(snapshot);
    return snapshot;
  }
}

function isoToday(now: Date = new Date()): string {
  return now.toISOString().slice(0, 10);
}

function sumMicros(
  rollups: ReadonlyArray<{ day: string; costMicros: number }>,
  startIso: string,
  endIso: string,
): number {
  let total = 0;
  for (const r of rollups) {
    if (r.day >= startIso && r.day <= endIso) total += r.costMicros;
  }
  return total;
}

/**
 * Period-aware forecast: uses the *union* of every budget's period
 * so a workspace with both weekly and monthly budgets gets a
 * forecast for each. Returns one snapshot per period present.
 */
export function forecastByBudgets(
  service: CostForecastService,
  orgId: string,
  budgets: ReadonlyArray<CostBudget>,
): CostForecast[] {
  const periods = new Set<BudgetPeriod>();
  for (const b of budgets) if (b.enabled) periods.add(b.period);
  const out: CostForecast[] = [];
  for (const period of periods) {
    const bounds = periodBounds(period);
    const snap = service.snapshot(orgId);
    if (!snap) continue;
    out.push({
      snapshot: { ...snap, periodStart: bounds.start, periodEnd: bounds.end },
      alerts: [],
    });
  }
  return out;
}