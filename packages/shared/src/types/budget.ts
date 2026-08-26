export type BudgetPeriod = 'weekly' | 'monthly';

export interface CostBudget {
  id: string;
  organizationId: string;
  label: string;
  period: BudgetPeriod;
  /** Cap expressed in micros so small over-budgets are not lost. */
  limitMicros: number;
  /** Fraction in [0, 1]; alert webhooks fire when projected > limit * threshold. */
  alertThreshold: number;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  lastAlertedAt: string | null;
}

export interface CostForecastSnapshot {
  id: string;
  organizationId: string;
  periodStart: string;
  periodEnd: string;
  /** Actual spend so far in the period, in micros. */
  spendMicros: number;
  /** Projected total period spend (linear regression), in micros. */
  projectedMicros: number;
  /** 95% CI lower bound for projectedMicros. */
  bandLowMicros: number;
  /** 95% CI upper bound for projectedMicros. */
  bandHighMicros: number;
  /** Window used for the regression (number of days of history). */
  windowDays: number;
  computedAt: string;
}

export interface CostForecast {
  snapshot: CostForecastSnapshot;
  /** Per-budget alerts that would fire right now. */
  alerts: Array<{
    budgetId: string;
    label: string;
    projectedMicros: number;
    limitMicros: number;
    alertThreshold: number;
    fraction: number;
  }>;
}