'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { executionApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Play } from 'lucide-react';
import Link from 'next/link';
import { QueryError } from '@/components/brand/query-error';

export default function ExecutionsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const session = useRequireSession();

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['executions', capabilityId],
    queryFn: () => executionApi.list(capabilityId!).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });

  const rows = data?.items ?? [];

  if (!session) return null;

  if (isError) {
    return <QueryError message={error} onRetry={() => void refetch()} />;
  }

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
            rows={rows}
            rowKey={(r) => r.id}
            columns={[
              {
                key: 'id',
                header: 'Run',
                render: (r) => (
                  <Link href={`/app/executions/${r.id}`} className="font-mono text-xs text-brand-highlight hover:underline">
                    {r.id.slice(0, 12)}…
                  </Link>
                ),
              },
              {
                key: 'status',
                header: 'Status',
                render: (r) => (
                  <StatusPill
                    kind={r.error ? 'rejected' : 'active'}
                    label={r.error ? 'error' : 'completed'}
                  />
                ),
              },
              {
                key: 'started',
                header: 'Started',
                render: (r) => new Date(r.timestamp).toLocaleString(),
              },
              {
                key: 'cost',
                header: 'Cost',
                render: (r) => {
                  return `$${r.costUsd.toFixed(4)}`;
                },
              },
              {
                key: 'latency',
                header: 'Latency',
                render: (r) => {
                  return `${r.latencyMs.toLocaleString()}ms`;
                },
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
