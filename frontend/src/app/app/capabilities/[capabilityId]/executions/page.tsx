'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { executionApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { Play } from 'lucide-react';
import Link from 'next/link';

export default function ExecutionsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const session = useRequireSession();

  const { data, isLoading } = useQuery({
    queryKey: ['executions', capabilityId],
    queryFn: () => executionApi.list(capabilityId!).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });

  const rows = (Array.isArray(data) ? data : []) as Array<{
    id: string;
    status: string;
    startedAt: string;
    totalCost: number;
    totalLatencyMs: number;
  }>;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capability"
        title="Executions"
        subtitle="Live and historical runs against this capability. Click a row for the full trace."
      />

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Executions" description={`${rows.length} run(s)`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Play}
            title="No executions yet"
            description={isLoading ? 'Loading…' : 'Activate a release to start collecting execution history.'}
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              {
                key: 'id',
                header: 'Run',
                render: (r) => (
                  <Link href={`/app/executions/${String(r['id'])}`} className="font-mono text-xs text-brand-highlight hover:underline">
                    {String(r['id']).slice(0, 12)}…
                  </Link>
                ),
              },
              {
                key: 'status',
                header: 'Status',
                render: (r) => (
                  <StatusPill
                    kind={r['status'] === 'completed' ? 'active' : r['status'] === 'failed' ? 'rejected' : 'review'}
                    label={String(r['status'])}
                  />
                ),
              },
              {
                key: 'started',
                header: 'Started',
                render: (r) => r['startedAt'] ? new Date(String(r['startedAt'])).toLocaleString() : '—',
              },
              {
                key: 'cost',
                header: 'Cost',
                render: (r) => {
                  const c = Number(r['totalCost'] ?? 0);
                  return `$${(c / 1_000_000).toFixed(4)}`;
                },
              },
              {
                key: 'latency',
                header: 'Latency',
                render: (r) => {
                  const ms = Number(r['totalLatencyMs'] ?? 0);
                  return `${ms.toLocaleString()}ms`;
                },
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}