'use client';

import { useQuery } from '@tanstack/react-query';
import { Activity } from 'lucide-react';
import { useMemo } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { workspaceApi, costApi, type CostRollup } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { BarChart } from '@/components/brand/bar-chart';
import { QueryError } from '@/components/brand/query-error';

export default function CostPage() {
  const session = useRequireSession();
  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
    enabled: Boolean(session),
  });
  const wsFirst = Array.isArray(workspaces.data) ? workspaces.data[0] : undefined;
  const wsId = (wsFirst as { id?: string } | undefined)?.id;

  const costs = useQuery({
    queryKey: ['cost', wsId],
    queryFn: () => (wsId ? costApi.forOrg(wsId, 30).then((r) => r.data) : Promise.resolve([] as CostRollup[])),
    enabled: Boolean(wsId),
  });

  const rows = costs.data ?? [];
  const totalMicros = rows.reduce((acc, r) => acc + r.costMicros, 0);
  const totalExec = rows.reduce((acc, r) => acc + r.executions, 0);
  const capabilityIds = new Set(rows.map((r) => r.capabilityId));

  const byCapability = useMemo(() => {
    const acc = new Map<string, number>();
    for (const r of rows) {
      acc.set(r.capabilityId, (acc.get(r.capabilityId) ?? 0) + r.costMicros);
    }
    return [...acc.entries()]
      .map(([capabilityId, cost]) => ({ label: capabilityId.slice(0, 12), value: cost }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 8);
  }, [rows]);

  const byDay = useMemo(() => {
    const acc = new Map<string, number>();
    for (const r of rows) {
      acc.set(r.day, (acc.get(r.day) ?? 0) + r.costMicros);
    }
    return [...acc.entries()]
      .map(([day, cost]) => ({ label: day, value: cost }))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [rows]);

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={workspaces.error} onRetry={() => void workspaces.refetch()} />;
  if (costs.isError) return <QueryError message={costs.error} onRetry={() => void costs.refetch()} />;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Cost & analytics"
        subtitle="Token and execution rollups per capability, per day. Sink from any execution; surface from /app/admin/cost."
      />
      <div className="grid gap-4 md:grid-cols-3">
        <StatCard label="Rollup rows" value={String(rows.length)} hint="last 30 d" icon={Activity} />
        <StatCard
          label="Total cost"
          value={`$${(totalMicros / 1_000_000).toFixed(4)}`}
          hint="micros / 1_000_000"
        />
        <StatCard
          label="Executions"
          value={String(totalExec)}
          hint={`${capabilityIds.size} capability(s)`}
        />
      </div>

      <div className="grid gap-5 lg:grid-cols-2">
        <Surface>
          <SurfaceHeader title="Cost by capability" description="Top capabilities by cumulative cost (30 d)" />
          {byCapability.length === 0 ? (
            <EmptyState
              icon={Activity}
              title="No rollups yet"
              description="Ingest via POST /api/analytics/rollups to populate."
              className="border-0 bg-transparent p-8"
            />
          ) : (
            <BarChart
              data={byCapability}
              format={(n) => `$${(n / 1_000_000).toFixed(4)}`}
            />
          )}
        </Surface>
        <Surface>
          <SurfaceHeader title="Cost by day" description="Daily totals across all capabilities" />
          {byDay.length === 0 ? (
            <EmptyState
              icon={Activity}
              title="No rollups yet"
              description="Ingest via POST /api/analytics/rollups to populate."
              className="border-0 bg-transparent p-8"
            />
          ) : (
            <BarChart
              data={byDay}
              format={(n) => `$${(n / 1_000_000).toFixed(4)}`}
            />
          )}
        </Surface>
      </div>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Per-capability / per-day rollups" />
        {rows.length === 0 ? (
          <EmptyState
            icon={Activity}
            title="No rollups yet"
            description="Rollups are ingested by the executor on each invocation. Push synthetic numbers via POST /api/analytics/rollups to demo."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => `${r.capabilityId}-${r.day}`}
            columns={[
              { key: 'day', header: 'Day', render: (r) => r.day },
              {
                key: 'capability',
                header: 'Capability',
                render: (r) => <span className="font-mono text-xs">{r.capabilityId.slice(0, 12)}…</span>,
              },
              { key: 'exec', header: 'Executions', render: (r) => r.executions },
              {
                key: 'cost',
                header: 'Cost',
                render: (r) => `$${(r.costMicros / 1_000_000).toFixed(4)}`,
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
