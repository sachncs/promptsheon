'use client';

import { useQuery } from '@tanstack/react-query';
import { Activity } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { workspaceApi, costApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';

interface Rollup {
  capabilityId: string;
  day: string;
  costMicros: number;
  executions: number;
}

export default function CostPage() {
  const session = useRequireSession();
  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const wsFirst = Array.isArray(workspaces.data) ? workspaces.data[0] : undefined;
  const wsId = (wsFirst as { id?: string } | undefined)?.id;

  const costs = useQuery({
    queryKey: ['cost', wsId],
    queryFn: () => (wsId ? costApi.forOrg(wsId, 30) : Promise.resolve([] as Rollup[])),
    enabled: Boolean(wsId),
  });

  const rows = (costs.data ?? []) as Rollup[];
  const totalMicros = rows.reduce((acc, r) => acc + r.costMicros, 0);
  const totalExec = rows.reduce((acc, r) => acc + r.executions, 0);
  const capabilityIds = new Set(rows.map((r) => r.capabilityId));

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
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => `${String(r['capabilityId'])}-${String(r['day'])}`}
            columns={[
              { key: 'day', header: 'Day', render: (r) => String(r['day']) },
              {
                key: 'capability',
                header: 'Capability',
                render: (r) => <span className="font-mono text-xs">{String(r['capabilityId']).slice(0, 12)}…</span>,
              },
              { key: 'exec', header: 'Executions', render: (r) => String(r['executions']) },
              {
                key: 'cost',
                header: 'Cost',
                render: (r) => `$${(Number(r['costMicros']) / 1_000_000).toFixed(4)}`,
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
