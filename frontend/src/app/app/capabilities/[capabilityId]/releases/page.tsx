'use client';

import { useParams, useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Play } from 'lucide-react';
import { releaseApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';

export default function ReleasesPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const session = useRequireSession();
  const router = useRouter();

  const { data, isLoading } = useQuery({
    queryKey: ['releases', capabilityId],
    queryFn: () => releaseApi.list(capabilityId!).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });

  const rows = (Array.isArray(data) ? data : []) as Array<{ id: string; environment: string; status: string; createdAt: string }>;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capability"
        title="Releases"
        subtitle="One row per environment. Click into a release to activate, route canary, or rollback."
      />

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Releases" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Play}
            title="No releases yet"
            description={isLoading ? 'Loading…' : 'Create a capability version and publish it as a release.'}
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => router.push(`/app/releases/${String(r['id'])}`)}
            columns={[
              { key: 'env', header: 'Environment', render: (r) => <span className="font-medium">{String(r['environment'])}</span> },
              {
                key: 'state',
                header: 'Status',
                render: (r) => (
                  <StatusPill
                    kind={r['status'] === 'active' ? 'active' : r['status'] === 'canary' ? 'review' : 'neutral'}
                    label={String(r['status'])}
                  />
                ),
              },
              {
                key: 'created',
                header: 'Created',
                render: (r) => r['createdAt'] ? new Date(String(r['createdAt'])).toLocaleString() : '—',
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}