'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { Activity, AlertTriangle, GitMerge, ShieldAlert } from 'lucide-react';
import { releaseApi, evalApi, alertApi, type Alert } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { QueryError } from '@/components/brand/query-error';

export default function OperationsPage() {
  const session = useRequireSession();
  const router = useRouter();
  const [now] = useState(() => Date.now());

  const allReleases = useQuery({
    queryKey: ['operations', 'releases'],
    queryFn: () => releaseApi.listAll(1, 100).then((res) => res.data.items),
  });

  const recentEvals = useQuery({
    queryKey: ['eval-runs', 'recent'],
    queryFn: () => evalApi.list().then((r) => r.data),
  });

  const alerts = useQuery({
    queryKey: ['alerts'],
    queryFn: () => alertApi.listAlerts().then((r) => r.data),
  });

  if (!session) return null;
  if (
    allReleases.isPending || recentEvals.isPending || alerts.isPending
  ) {
    return (
      <div className="space-y-6" aria-busy="true" aria-live="polite">
        <PageHeader eyebrow="Release" title="Operations hub" subtitle="Loading fleet health…" />
        <div className="grid gap-5 lg:grid-cols-2">
          <Surface className="h-80 animate-pulse bg-surface-2/40"><span className="sr-only">Loading release health</span></Surface>
          <Surface className="h-80 animate-pulse bg-surface-2/40"><span className="sr-only">Loading evaluation health</span></Surface>
        </div>
      </div>
    );
  }
  if (allReleases.isError) return <QueryError message={allReleases.error} onRetry={() => void allReleases.refetch()} />;
  if (recentEvals.isError) return <QueryError message={recentEvals.error} onRetry={() => void recentEvals.refetch()} />;
  if (alerts.isError) return <QueryError message={alerts.error} onRetry={() => void alerts.refetch()} />;

  const releases = allReleases.data ?? [];
  const activeReleases = releases.filter((r) => r.status === 'active');
  const canaryReleases = releases.filter((r) => r.status === 'canary');
  const draftReleases = releases.filter((r) => r.status === 'draft' || r.status === 'review');
  const evals = recentEvals.data ?? [];
  const unackAlerts: Alert[] = (alerts.data ?? []).filter((a) => a.status === 'active' && a.acknowledgedAt === null);

  const last24h = evals.filter((e) => {
    if (!e.startedAt) return false;
    const t = new Date(e.startedAt).getTime();
    return now - t < 24 * 60 * 60 * 1000;
  });
  const passRate =
    last24h.length > 0
      ? last24h.reduce((acc, e) => acc + (e.score ?? (e.total ? (e.passed ?? 0) / e.total : 0)), 0) / last24h.length
      : null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Release"
        title="Operations hub"
        subtitle="Live health of the fleet — active releases, canary progress, recent eval outcomes, and unacknowledged alerts."
      />

      <div className="grid gap-4 md:grid-cols-4">
        <StatCard
          label="Active releases"
          value={String(activeReleases.length)}
          hint={`${canaryReleases.length} canary`}
          icon={GitMerge}
        />
        <StatCard
          label="In flight"
          value={String(draftReleases.length)}
          hint="draft + review"
          icon={Activity}
        />
        <StatCard
          label="Eval pass rate (24h)"
          value={passRate !== null ? `${(passRate * 100).toFixed(0)}%` : '—'}
          hint={`${last24h.length} runs`}
          icon={ShieldAlert}
        />
        <StatCard
          label="Unack alerts"
          value={String(unackAlerts.length)}
          hint={unackAlerts.length > 0 ? 'investigate' : 'all clear'}
          icon={AlertTriangle}
        />
      </div>

      <div className="grid gap-5 lg:grid-cols-2">
        <Surface padded={false}>
          <SurfaceHeader
            className="px-5 pt-5"
            title="Canary in progress"
            description="Releases rolling out by weighted traffic split."
          />
          {canaryReleases.length === 0 ? (
            <EmptyState
              icon={GitMerge}
              title="No canary releases"
              description="Promote a release to canary from the releases page."
              className="m-5 border-0 bg-transparent shadow-none p-12"
            />
          ) : (
            <DataTable
              className="rounded-none border-0 border-t border-border-subtle"
              rows={canaryReleases}
              rowKey={(r) => r.id}
              onRowClick={(r) => { router.push(`/app/releases/${r.id}`); }}
              columns={[
                {
                  key: 'cap',
                  header: 'Capability',
                  render: (r) => (
                    <div>
                      <div className="font-medium text-text-strong">{r.capabilityId}</div>
                      <div className="text-xs text-text-subtle">v{r.capabilityVersion} · {r.environment}</div>
                    </div>
                  ),
                },
                {
                  key: 'canary',
                  header: 'Canary',
                  render: (r) => {
                    const pct = r.canaryPercent;
                    return (
                      <div className="flex items-center gap-2">
                        <div className="h-1.5 w-24 overflow-hidden rounded-full bg-surface-2">
                          <div className="h-full bg-brand" style={{ width: `${pct}%` }} />
                        </div>
                        <span className="font-mono text-xs text-text-muted">{pct}%</span>
                      </div>
                    );
                  },
                },
                {
                  key: 'id',
                  header: 'Release ID',
                  render: (r) => <span className="font-mono text-xs text-text-muted">{r.id}</span>,
                },
                { key: 'state', header: 'State', render: (r) => <StatusPill kind={statusKindOf(r.status)} /> },
              ]}
            />
          )}
        </Surface>

        <Surface padded={false}>
          <SurfaceHeader
            className="px-5 pt-5"
            title="Recent eval runs"
            description="Last eval outcomes across all releases."
          />
          {evals.length === 0 ? (
            <EmptyState
              icon={Activity}
              title="No eval runs yet"
              description="Trigger an eval run from a release to populate this list."
              className="m-5 border-0 bg-transparent shadow-none p-12"
            />
          ) : (
            <DataTable
              className="rounded-none border-0 border-t border-border-subtle"
              rows={evals.slice(0, 8)}
              rowKey={(r) => r.id}
              onRowClick={(r) => { router.push(`/app/eval/${r.id}`); }}
              columns={[
                { key: 'run', header: 'Run', render: (r) => <span className="font-mono text-xs">{r.id.slice(0, 12)}…</span> },
                {
                  key: 'score',
                  header: 'Score',
                  render: (r) => {
                    const s = r.score;
                    return `${(s * 100).toFixed(0)}%`;
                  },
                },
                {
                  key: 'cases',
                  header: 'Cases',
                  render: (r) => `${r.passed}/${r.total}`,
                },
                {
                  key: 'when',
                  header: 'When',
                  render: (r) => new Date(r.startedAt).toLocaleString(),
                },
              ]}
            />
          )}
        </Surface>
      </div>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Unacknowledged alerts"
          description={unackAlerts.length === 0 ? 'All clear.' : `${unackAlerts.length} need attention.`}
        />
        {unackAlerts.length === 0 ? (
          <EmptyState
            icon={AlertTriangle}
            title="No alerts"
            description="Define alert rules to surface regressions, drift, or canary anomalies."
            className="m-5 border-0 bg-transparent shadow-none p-12"
            action={<Link href="/app/alerts/rules"><span className="text-text-muted text-sm">Configure alert rules →</span></Link>}
          />
        ) : (
          <ul className="divide-y divide-border-subtle">
            {unackAlerts.slice(0, 10).map((a) => (
              <li key={a.id} className="flex items-start gap-3 px-5 py-3 text-sm">
                <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warning" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="font-medium text-text-strong">{a.ruleName || a.id}</span>
                    <span className="text-xs text-text-subtle">{new Date(a.triggeredAt).toLocaleString()}</span>
                  </div>
                  {a.message && <p className="mt-1 text-text-muted">{a.message}</p>}
                </div>
              </li>
            ))}
          </ul>
        )}
      </Surface>
    </div>
  );
}
