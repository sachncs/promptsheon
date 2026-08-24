'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Activity, AlertTriangle, GitMerge, ShieldAlert } from 'lucide-react';
import { releaseApi, workspaceApi, projectApi, evalApi, alertApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { HashChip } from '@/components/brand/hash-chip';

interface Release {
  id: string;
  capabilityId?: string;
  capabilityName?: string;
  capabilityVersion?: number;
  environment?: string;
  state?: string;
  canaryPercent?: number;
  manifestHash?: string;
  updatedAt?: string;
}

interface EvalRun {
  id: string;
  releaseId?: string;
  score?: number;
  passed?: number;
  total?: number;
  startedAt?: string;
  completedAt?: string;
}

interface AlertItem {
  id: string;
  ruleName?: string;
  severity?: string;
  message?: string;
  at?: string;
  acknowledged?: boolean;
}

export default function OperationsPage() {
  const session = useRequireSession();

  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const wsFirst = Array.isArray(workspaces.data) ? workspaces.data[0] as { id?: string; name?: string } : undefined;
  const wsId = wsFirst?.id;

  const projects = useQuery({
    queryKey: ['projects', wsId],
    queryFn: () => (wsId ? projectApi.list(wsId).then((r) => r.data) : Promise.resolve([])),
    enabled: Boolean(wsId),
  });
  const projectList = Array.isArray(projects.data) ? projects.data as Array<{ id: string; name?: string }> : [];

  const allReleases = useQuery({
    queryKey: ['operations', 'releases', projectList.map((p) => p.id)],
    queryFn: async () => {
      const out: Release[] = [];
      for (const p of projectList) {
        try {
          const r = await releaseApi.list(p.id).then((res) => res.data);
          if (Array.isArray(r)) {
            for (const rel of r) {
              const base = rel as Release;
              const merged: Release = {
                ...base,
                capabilityId: base.capabilityId ?? p.id,
              };
              if (p.name !== undefined) merged.capabilityName = p.name;
              out.push(merged);
            }
          }
        } catch {
          /* skip */
        }
      }
      return out;
    },
    enabled: projectList.length > 0,
  });

  const recentEvals = useQuery({
    queryKey: ['eval-runs', 'recent'],
    queryFn: () => evalApi.list().then((r) => r.data as EvalRun[]),
  });

  const alerts = useQuery({
    queryKey: ['alerts'],
    queryFn: () => alertApi.listAlerts().then((r) => r.data).catch(() => [] as AlertItem[]),
  });

  if (!session) return null;

  const releases = (allReleases.data ?? []) as Release[];
  const activeReleases = releases.filter((r) => r.state === 'active');
  const canaryReleases = releases.filter((r) => r.state === 'canary');
  const draftReleases = releases.filter((r) => r.state === 'draft' || r.state === 'review');
  const evals = (recentEvals.data ?? []) as EvalRun[];
  const unackAlerts = ((alerts.data ?? []) as AlertItem[]).filter((a) => !a.acknowledged);

  const last24h = evals.filter((e) => {
    if (!e.startedAt) return false;
    const t = new Date(e.startedAt).getTime();
    return Date.now() - t < 24 * 60 * 60 * 1000;
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
              rows={canaryReleases as unknown as Array<Record<string, unknown>>}
              rowKey={(r) => String(r['id'])}
              onRowClick={(r) => { window.location.href = `/app/releases/${String(r['id'])}`; }}
              columns={[
                {
                  key: 'cap',
                  header: 'Capability',
                  render: (r) => (
                    <div>
                      <div className="font-medium text-text-strong">{String(r['capabilityName'] ?? '—')}</div>
                      <div className="text-xs text-text-subtle">v{String(r['capabilityVersion'] ?? '?')} · {String(r['environment'] ?? '—')}</div>
                    </div>
                  ),
                },
                {
                  key: 'canary',
                  header: 'Canary',
                  render: (r) => {
                    const pct = Number(r['canaryPercent'] ?? 0);
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
                  key: 'hash',
                  header: 'Hash',
                  render: (r) => r['manifestHash'] ? <HashChip hash={String(r['manifestHash'])} /> : <span className="text-text-muted">—</span>,
                },
                { key: 'state', header: 'State', render: (r) => <StatusPill kind={(r['state'] as never) ?? 'neutral'} /> },
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
              rows={evals.slice(0, 8) as unknown as Array<Record<string, unknown>>}
              rowKey={(r) => String(r['id'])}
              onRowClick={(r) => { window.location.href = `/app/eval/${String(r['id'])}`; }}
              columns={[
                { key: 'run', header: 'Run', render: (r) => <span className="font-mono text-xs">{String(r['id']).slice(0, 12)}…</span> },
                {
                  key: 'score',
                  header: 'Score',
                  render: (r) => {
                    const s = r['score'] as number | undefined;
                    return s !== undefined ? `${(s * 100).toFixed(0)}%` : '—';
                  },
                },
                {
                  key: 'cases',
                  header: 'Cases',
                  render: (r) => `${String(r['passed'] ?? '—')}/${String(r['total'] ?? '—')}`,
                },
                {
                  key: 'when',
                  header: 'When',
                  render: (r) => r['startedAt'] ? new Date(String(r['startedAt'])).toLocaleString() : '—',
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
                    <span className="font-medium text-text-strong">{a.ruleName ?? a.message ?? a.id}</span>
                    <span className="text-xs text-text-subtle">{a.at ? new Date(a.at).toLocaleString() : '—'}</span>
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