'use client';

import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Boxes } from 'lucide-react';
import { capabilityApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';

export default function ProjectCapabilitiesPage() {
  const params = useParams<{ projectId: string }>();
  const projectId = params.projectId;
  const session = useRequireSession();
  const router = useRouter();

  const capabilities = useQuery({
    queryKey: ['capabilities', projectId],
    queryFn: () => capabilityApi.list(projectId!).then((r) => r.data).catch(() => []),
    enabled: Boolean(projectId) && Boolean(session),
  });

  const rows = (Array.isArray(capabilities.data) ? capabilities.data : []) as Array<{ id: string; name: string; description?: string }>;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Project"
        title="Capabilities"
        subtitle="Capabilities in this project. Each one is a multi-agent DAG with versions, releases, evals, and audit history."
      />

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Capabilities" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Boxes}
            title="No capabilities in this project"
            description="Capabilities are the unit of version control here. Create one to start authoring a DAG."
            action={
              <Link href="/app/editor">
                <Button>Author capability</Button>
              </Link>
            }
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => router.push(`/app/capabilities/${String(r['id'])}`)}
            columns={[
              {
                key: 'name',
                header: 'Capability',
                render: (r) => (
                  <Link href={`/app/capabilities/${String(r['id'])}`} className="font-medium text-text-strong hover:underline">
                    {String(r['name'])}
                  </Link>
                ),
              },
              {
                key: 'description',
                header: 'Description',
                render: (r) => <span className="text-text-muted">{String(r['description'] ?? '—')}</span>,
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}