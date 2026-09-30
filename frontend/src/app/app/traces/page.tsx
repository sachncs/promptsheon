'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Activity, AlertTriangle, Coins, Cpu, Gauge, Layers, ShieldAlert } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { traceApi, type TraceRunSummary } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { ThemedSelect } from '@/components/brand/themed-select';
import { QueryError } from '@/components/brand/query-error';
import type { LucideIcon } from 'lucide-react';

export default function TracesPage() {
  const session = useRequireSession();
  const [status, setStatus] = useState<string>('');
  const [environment, setEnvironment] = useState<string>('');

  const traces = useQuery({
    queryKey: ['traces', { status, environment }],
    queryFn: () =>
      traceApi.list({
        page: 1,
        pageSize: 50,
        ...(status ? { status } : {}),
        ...(environment ? { environment } : {}),
      }),
    enabled: Boolean(session),
    refetchInterval: 10_000,
  });

  const rollup = useQuery({
    queryKey: ['traces', 'rollup', 7],
    queryFn: () => traceApi.rollup(7),
    enabled: Boolean(session),
    refetchInterval: 30_000,
  });

  const summary = useQuery({
    queryKey: ['traces', 'summary', 7],
    queryFn: () => traceApi.summary(7),
    enabled: Boolean(session),
    refetchInterval: 30_000,
  });

  const promptRisk = useQuery({
    queryKey: ['traces', 'prompt-risk', 30],
    queryFn: () => traceApi.promptRisk(30, 25),
    enabled: Boolean(session),
    refetchInterval: 30_000,
  });

  if (!session) return null;

  if (traces.isError) {
    return <QueryError message={traces.error} onRetry={() => void traces.refetch()} />;
  }

  if (rollup.isError) {
    return <QueryError message={rollup.error} onRetry={() => void rollup.refetch()} />;
  }

  if (summary.isError) {
    return <QueryError message={summary.error} onRetry={() => void summary.refetch()} />;
  }

  if (promptRisk.isError) {
    return <QueryError message={promptRisk.error} onRetry={() => void promptRisk.refetch()} />;
  }

  const runList = traces.data?.items ?? [];
  const total = traces.data?.total ?? 0;
  const sumTokens = rollup.data?.items.reduce((acc, r) => acc + r.tokens, 0) ?? 0;
  const sumCost = rollup.data?.items.reduce((acc, r) => acc + r.cost, 0) ?? 0;
  const sumRuns = rollup.data?.items.reduce((acc, r) => acc + r.runs, 0) ?? 0;
  const operational = summary.data?.summary;
  const errorRate = operational && operational.runs > 0 ? (operational.errors / operational.runs) * 100 : 0;

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/app"
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
        >
          <ArrowLeft className="h-3 w-3" /> Control plane
        </Link>
      </div>

      <PageHeader
        eyebrow="Observability"
        title="Traces"
        subtitle="Every manifest execution writes a span tree. Pick a row to inspect per-node latency, tokens, and cost."
      />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <SummaryTile
          label="Traces (7d)"
          value={String(sumRuns)}
          hint={`${rollup.data?.items.length ?? 0} active days`}
          Icon={Activity}
        />
        <SummaryTile
          label="Tokens (7d)"
          value={formatNumber(sumTokens)}
          hint="across all runs"
          Icon={Cpu}
        />
        <SummaryTile
          label="Cost (7d)"
          value={`$${sumCost.toFixed(4)}`}
          hint="per execution, raw-string SHA256 cost"
          Icon={Coins}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <SummaryTile
          label="Error rate (7d)"
          value={`${errorRate.toFixed(1)}%`}
          hint={`${operational?.errors ?? 0} failed runs`}
          Icon={AlertTriangle}
        />
        <SummaryTile
          label="Avg latency"
          value={`${Math.round(operational?.averageLatencyMs ?? 0)} ms`}
          hint="completed traces"
          Icon={Gauge}
        />
        <SummaryTile
          label="Models observed"
          value={String(operational?.models.length ?? 0)}
          hint="provider/model health"
          Icon={Layers}
        />
        <SummaryTile
          label="Evidence"
          value="Redacted"
          hint="before persistence"
          Icon={Cpu}
        />
      </div>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Prompt risk signals"
          description="Trace-backed signals for repeated failures, token burn, and unusual execution volume. Inputs remain redacted."
        />
        {promptRisk.isLoading ? (
          <div className="px-5 py-8 text-sm text-text-muted">Checking recent signals…</div>
        ) : promptRisk.data?.items.length ? (
          <ul className="divide-y divide-border-subtle">
            {promptRisk.data.items.map((item) => (
              <li key={item.promptKey} className="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
                <div className="flex min-w-0 items-start gap-3">
                  <div className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-amber-500/10 text-amber-500">
                    <ShieldAlert className="h-4 w-4" aria-hidden="true" />
                  </div>
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="max-w-[28rem] truncate font-mono text-sm text-text-strong">{item.promptKey}</span>
                      <StatusPill kind={item.risk === 'high' ? 'error' : 'review'} label={`${item.risk} risk`} />
                    </div>
                    <p className="mt-1 text-xs text-text-muted">
                      {item.runs} runs · {item.errors} errors ({(item.errorRate * 100).toFixed(0)}%) · {item.actors} actors · {formatNumber(item.tokens)} tokens
                    </p>
                  </div>
                </div>
                <div className="flex flex-wrap gap-1.5 text-[10px] uppercase tracking-wider text-text-subtle">
                  {item.signals.map((signal) => <span key={signal} className="rounded-full bg-surface-2 px-2 py-1">{signal}</span>)}
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState
            className="m-5 border-0 bg-transparent shadow-none p-8"
            icon={ShieldAlert}
            title="No risk signals"
            description="No prompt or agent has crossed the current repeat-failure, token-burn, or volume thresholds."
          />
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Recent runs"
          description="Newest first. Auto-refreshes every 10s."
          actions={
            <div className="flex items-center gap-2">
              <ThemedSelect
                value={status || 'all'}
                onValueChange={(v) => setStatus(v === 'all' ? '' : v)}
                options={[
                  { value: 'all', label: 'All statuses' },
                  { value: 'running', label: 'Running' },
                  { value: 'success', label: 'Success' },
                  { value: 'error', label: 'Error' },
                ]}
              />
              <ThemedSelect
                value={environment || 'all'}
                onValueChange={(v) => setEnvironment(v === 'all' ? '' : v)}
                options={[
                  { value: 'all', label: 'All envs' },
                  { value: 'dev', label: 'Dev' },
                  { value: 'staging', label: 'Staging' },
                  { value: 'prod', label: 'Production' },
                ]}
              />
            </div>
          }
        />
        {traces.isLoading ? (
          <div className="px-5 py-12 text-sm text-text-muted">Loading traces…</div>
        ) : runList.length === 0 ? (
          <EmptyState
            className="m-5 border-0 bg-transparent shadow-none p-12"
            icon={Layers}
            title="No traces yet"
            description="Once a manifest runs, the trace appears here. Traces also feed the cost dashboard."
          />
        ) : (
          <ul className="divide-y divide-border-subtle">
            {runList.map((r) => (
              <TraceRow key={r.id} run={r} />
            ))}
          </ul>
        )}
        {traces.data && traces.data.total > runList.length && (
          <div className="px-5 py-3 text-xs text-text-subtle">
            Showing {runList.length} of {total}.
          </div>
        )}
      </Surface>
    </div>
  );
}

function SummaryTile({
  label,
  value,
  hint,
  Icon,
}: {
  label: string;
  value: string;
  hint: string;
  Icon: LucideIcon;
}) {
  return (
    <Surface padded={false}>
      <div className="flex items-start justify-between gap-3 px-5 py-4">
        <div>
          <div className="text-xs uppercase tracking-wider text-text-subtle">{label}</div>
          <div className="mt-1 font-mono text-2xl text-text-strong">{value}</div>
          <div className="mt-0.5 text-xs text-text-muted">{hint}</div>
        </div>
        <div className="grid h-8 w-8 place-items-center rounded-lg bg-surface-2 text-text-muted">
          <Icon className="h-4 w-4" aria-hidden="true" />
        </div>
      </div>
    </Surface>
  );
}

function TraceRow({ run }: { run: TraceRunSummary }) {
  const startedMs = Date.parse(run.startTime);
  const durationMs = run.endTime
    ? Math.max(0, Date.parse(run.endTime) - startedMs)
    : null;
  return (
    <li>
      <Link
        href={`/app/traces/${run.id}`}
        className="flex items-start gap-3 px-5 py-3 hover:bg-surface-2/40"
      >
        <div className="grid h-8 w-8 place-items-center rounded-md bg-surface-2 text-text-subtle">
          <Activity className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <span className="truncate font-mono text-sm text-text-strong">{run.name}</span>
            <StatusPill
              kind={run.status === 'success' ? 'active' : run.status === 'error' ? 'error' : 'review'}
              label={run.status}
            />
            <span className="text-xs text-text-subtle">{run.environment}</span>
            {run.model && (
              <span className="rounded-full bg-surface-2 px-2 py-0.5 text-[10px] uppercase tracking-wider text-text-muted">
                {run.model}
              </span>
            )}
          </div>
          <div className="mt-0.5 flex items-center gap-3 text-xs text-text-muted">
            <span>{new Date(run.startTime).toLocaleString()}</span>
            <span>·</span>
            <span>{durationMs === null ? 'Running' : `${durationMs} ms`}</span>
            <span>·</span>
            <span>{run.totalTokens.toLocaleString()} tokens</span>
            <span>·</span>
            <span>${run.totalCostUsd.toFixed(4)}</span>
            {run.executionId && (
              <>
                <span>·</span>
                <HashChip hash={run.executionId} length={12} />
              </>
            )}
          </div>
        </div>
      </Link>
    </li>
  );
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}
