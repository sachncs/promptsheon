'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, ArrowLeft, Gauge, Plus, Trash2 } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { budgetApi, type CostBudget } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { Field, FieldGroup } from '@/components/brand/field';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { EmptyState } from '@/components/brand/empty-state';
import { QueryError } from '@/components/brand/query-error';
import { StatusPill } from '@/components/brand/status-pill';

function dollars(micros: number): string {
  return `$${(micros / 1_000_000).toFixed(2)}`;
}

function microsFromDollars(value: string): number {
  const amount = Number(value);
  if (!Number.isFinite(amount) || amount <= 0) throw new Error('Enter a budget greater than $0.');
  return Math.round(amount * 1_000_000);
}

export default function BudgetsPage() {
  const session = useRequireSession();
  const queryClient = useQueryClient();
  const [label, setLabel] = useState('Monthly LLM spend');
  const [period, setPeriod] = useState<CostBudget['period']>('monthly');
  const [limit, setLimit] = useState('100');
  const [threshold, setThreshold] = useState('80');
  const [formError, setFormError] = useState<string | null>(null);

  const budgets = useQuery({
    queryKey: ['budgets', session?.orgId],
    queryFn: () => budgetApi.list(session!.orgId),
    enabled: Boolean(session),
  });
  const forecast = useQuery({
    queryKey: ['cost-forecast', session?.orgId],
    queryFn: () => budgetApi.forecast(session!.orgId),
    enabled: Boolean(session),
  });

  const refresh = async (): Promise<void> => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['budgets', session?.orgId] }),
      queryClient.invalidateQueries({ queryKey: ['cost-forecast', session?.orgId] }),
    ]);
  };

  const create = useMutation({
    mutationFn: () => budgetApi.create({
      organizationId: session!.orgId,
      label: label.trim(),
      period,
      limitMicros: microsFromDollars(limit),
      alertThreshold: Number(threshold) / 100,
      enabled: true,
    }),
    onSuccess: async () => {
      setFormError(null);
      await refresh();
    },
    onError: (error: Error) => setFormError(error.message),
  });
  const update = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => budgetApi.update(id, { enabled }),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (id: string) => budgetApi.remove(id),
    onSuccess: refresh,
  });

  if (!session) return null;
  if (budgets.isError) return <QueryError message={budgets.error} onRetry={() => void budgets.refetch()} />;
  if (forecast.isError) return <QueryError message={forecast.error} onRetry={() => void forecast.refetch()} />;

  const snapshot = forecast.data?.snapshot;
  const alerts = forecast.data?.alerts ?? [];

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/admin/cost" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" /> Cost & analytics
        </Link>
      </div>
      <PageHeader
        eyebrow="Admin"
        title="Spend budgets"
        subtitle="Set durable organization spend limits and see when the current cost trajectory is likely to cross them."
      />

      {snapshot && (
        <div className="grid gap-4 md:grid-cols-3">
          <Metric label="Spend so far" value={dollars(snapshot.spendMicros)} hint={`last ${snapshot.windowDays} days`} />
          <Metric label="Projected" value={dollars(snapshot.projectedMicros)} hint={`through ${snapshot.periodEnd}`} />
          <Metric label="Forecast band" value={`${dollars(snapshot.bandLowMicros)}–${dollars(snapshot.bandHighMicros)}`} hint="95% confidence range" />
        </div>
      )}

      {alerts.length > 0 && (
        <Surface className="border-warning/40 bg-warning/5">
          <SurfaceHeader title="Budget attention needed" description="These budgets are projected to cross their alert threshold." />
          <ul className="space-y-2 px-5 pb-5">
            {alerts.map((alert) => (
              <li key={alert.budgetId} className="flex flex-wrap items-center gap-2 text-sm">
                <AlertTriangle className="h-4 w-4 text-warning" aria-hidden="true" />
                <span className="font-medium text-text-strong">{alert.label}</span>
                <span className="text-text-muted">{dollars(alert.projectedMicros)} projected against {dollars(alert.limitMicros)}</span>
              </li>
            ))}
          </ul>
        </Surface>
      )}

      <Surface>
        <SurfaceHeader title="Add a budget" description="Limits are stored in micro-dollars to keep small spend precise." />
        <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); setFormError(null); create.mutate(); }}>
          <FieldGroup cols={4}>
            <Field label="Label" htmlFor="budget-label" required>
              <Input id="budget-label" value={label} onChange={(event) => setLabel(event.target.value)} maxLength={120} />
            </Field>
            <Field label="Limit (USD)" htmlFor="budget-limit" required>
              <Input id="budget-limit" type="number" min="0.01" step="0.01" value={limit} onChange={(event) => setLimit(event.target.value)} />
            </Field>
            <Field label="Period" htmlFor="budget-period">
              <select id="budget-period" value={period} onChange={(event) => setPeriod(event.target.value as CostBudget['period'])} className="h-10 w-full rounded-md border border-border-subtle bg-surface-1 px-3 text-sm text-text-default">
                <option value="monthly">Monthly</option>
                <option value="weekly">Weekly</option>
              </select>
            </Field>
            <Field label="Alert at (%)" htmlFor="budget-threshold">
              <Input id="budget-threshold" type="number" min="1" max="100" step="1" value={threshold} onChange={(event) => setThreshold(event.target.value)} />
            </Field>
          </FieldGroup>
          {formError && <p role="alert" className="text-sm text-destructive">{formError}</p>}
          <Button type="submit" disabled={create.isPending || !label.trim()}>
            <Plus className="mr-1.5 h-4 w-4" aria-hidden="true" />
            {create.isPending ? 'Adding…' : 'Add budget'}
          </Button>
        </form>
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Organization budgets" description="Only enabled budgets participate in forecast alerts." />
        {(budgets.data?.items.length ?? 0) === 0 ? (
          <div className="px-5 pb-5"><EmptyState icon={Gauge} title="No budgets yet" description="Add a budget to make spend thresholds visible to the team." /></div>
        ) : (
          <ul className="divide-y divide-border-subtle">
            {budgets.data?.items.map((budget) => (
              <li key={budget.id} className="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium text-text-strong">{budget.label}</span>
                    <StatusPill kind={budget.enabled ? 'active' : 'neutral'} label={budget.enabled ? 'enabled' : 'disabled'} />
                  </div>
                  <p className="mt-1 text-xs text-text-muted">{dollars(budget.limitMicros)} / {budget.period} · alert at {Math.round(budget.alertThreshold * 100)}%</p>
                </div>
                <div className="flex items-center gap-2">
                  <Button size="sm" variant="outline" onClick={() => update.mutate({ id: budget.id, enabled: !budget.enabled })} disabled={update.isPending}>
                    {budget.enabled ? 'Disable' : 'Enable'}
                  </Button>
                  <Button size="sm" variant="ghost" aria-label={`Delete ${budget.label}`} onClick={() => { if (window.confirm(`Delete budget “${budget.label}”?`)) remove.mutate(budget.id); }} disabled={remove.isPending}>
                    <Trash2 className="h-4 w-4 text-destructive" aria-hidden="true" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Surface>
    </div>
  );
}

function Metric({ label, value, hint }: { label: string; value: string; hint: string }) {
  return <div className="rounded-xl border border-border-subtle bg-surface-1 p-5"><div className="text-xs font-medium uppercase tracking-wider text-text-muted">{label}</div><div className="mt-3 text-2xl font-semibold tracking-tight text-text-strong">{value}</div><div className="mt-1 text-sm text-text-muted">{hint}</div></div>;
}
