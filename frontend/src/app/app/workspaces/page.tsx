'use client';

import { FolderOpen } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { workspaceApi } from '@/lib/api';
import { StubPage } from '@/components/brand/stub-page';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';

export default function WorkspacesPage() {
  const w = useQuery({ queryKey: ['workspaces'], queryFn: () => workspaceApi.list(1).then((r) => r.data).catch(() => []) });
  const rows = (Array.isArray(w.data) ? w.data : []) as Array<Record<string, unknown>>;

  return (
    <div className="space-y-6">
      <div>
        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">Admin</div>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-text-strong">Workspaces</h1>
        <p className="mt-1.5 text-sm text-text-muted">Workspaces group projects, capabilities, and release environments for one team.</p>
      </div>
      {rows.length === 0 ? (
        <StubPage
          eyebrow="Admin"
          title="Workspaces"
          description="No workspaces yet. Create one to start housing projects and capabilities."
          icon={FolderOpen}
          primary={{
            title: 'No workspaces yet',
            description: 'Create your first workspace to start organising capabilities.',
            action: { label: 'Create workspace', href: '#' },
          }}
        />
      ) : (
        <Surface padded={false}>
          <SurfaceHeader className="px-5 pt-5" title={`${rows.length} workspaces`} />
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => String(r['id'])}
            columns={[
              { key: 'name', header: 'Name', render: (r) => String(r['name'] ?? '—') },
              { key: 'org', header: 'Organisation', render: (r) => String((r['organization'] as string | undefined) ?? '—') },
              { key: 'id', header: 'Identifier', render: (r) => <HashChip hash={String(r['id'])} /> },
            ]}
            empty={
              <EmptyState
                icon={FolderOpen}
                title="No workspaces"
                description="Create one to get started."
                className="m-5 border-0 bg-transparent shadow-none p-12"
              />
            }
          />
        </Surface>
      )}
    </div>
  );
}
