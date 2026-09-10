'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMemo } from 'react';
import { Boxes, Workflow, Plus } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { workspaceApi, projectApi, capabilityApi, releaseApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';

export default function CapabilitiesRegistryPage() {
  const session = useRequireSession();
  const router = useRouter();
  const workspaces = useQuery({ queryKey: ['workspaces'], queryFn: () => workspaceApi.list(1).then((r) => r.data) });
  const wsFirst = unwrapFirst<{ id: string; name: string }>(workspaces.data);
  const projects = useQuery({
    queryKey: ['projects', wsFirst?.id],
    queryFn: () => projectApi.list(wsFirst!.id).then((r) => r.data),
    enabled: Boolean(wsFirst?.id),
  });

  const allProjects = unwrapArray<{ id: string; name: string }>(projects.data);

  const capabilities = useQuery({
    queryKey: ['capabilities', 'all', allProjects.map((p) => p.id)],
    queryFn: async () => {
      const out: Array<Record<string, unknown> & { projectName: string }> = [];
      for (const p of allProjects) {
        const list = await capabilityApi.list(p.id).then((r) => r.data).catch(() => []);
        if (Array.isArray(list)) out.push(...list.map((c: Record<string, unknown>) => ({ ...c, projectName: p.name })));
      }
      return out;
    },
    enabled: allProjects.length > 0,
  });

  const rows = useMemo(() => {
    const arr = Array.isArray(capabilities.data) ? capabilities.data : [];
    return arr;
  }, [capabilities.data]);

  if (!session) return null;

  const empty = rows.length === 0 && !capabilities.isLoading;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capabilities"
        title="Registry"
        subtitle="Every capability is a multi-agent DAG. Versions are content-addressed. Releases are governed."
        actions={
          <Link href="/app/editor">
            <Button><Plus className="mr-1.5 h-3.5 w-3.5" />Author capability</Button>
          </Link>
        }
      />

      {empty ? (
        <EmptyState
          icon={Boxes}
          title="No capabilities yet"
          description="Author a capability by composing prompts, policies, tools, memory, and guardrails into a DAG."
          action={
            <Link href="/app/editor">
              <Button><Plus className="mr-1.5 h-3.5 w-3.5" />Author first capability</Button>
            </Link>
          }
        />
      ) : (
        <Surface padded={false}>
          <SurfaceHeader
            className="px-5 pt-5"
            title={`${rows.length} capabilities`}
            description="Browse, compare, and inspect versions of every capability in this organisation."
          />
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => { router.push(`/app/capabilities/${String(r['id'])}`); }}
            columns={[
              {
                key: 'name',
                header: 'Capability',
                render: (r) => (
                  <Link href={`/app/capabilities/${String(r['id'])}`} className="text-text-strong font-medium hover:underline">
                    {String(r['name'] ?? '—')}
                  </Link>
                ),
              },
              { key: 'project', header: 'Project', render: (r) => <span className="text-text-muted">{String(r['projectName'])}</span> },
              {
                key: 'shape',
                header: 'Shape',
                render: () => (
                  <span className="inline-flex items-center gap-1.5 text-xs text-text-muted">
                    <Workflow className="h-3 w-3" /> multi-agent DAG
                  </span>
                ),
              },
              {
                key: 'latest',
                header: 'Latest',
                render: (r) => <span className="font-mono text-xs text-text-muted">v{String(r['latestVersion'] ?? r['version'] ?? '1')}</span>,
              },
              {
                key: 'hash',
                header: 'Content',
                render: (r) => <HashChip hash={String(r['manifestHash'] ?? r['id'])} />,
              },
              { key: 'state', header: 'State', render: (r) => <StatusPill kind={(r['state'] as never) ?? 'neutral'} /> },
            ]}
          />
        </Surface>
      )}
    </div>
  );
}

function unwrapArray<T = unknown>(data: unknown): T[] {
  if (Array.isArray(data)) return data as T[];
  if (data && typeof data === 'object' && 'items' in data) {
    const items = (data as { items?: unknown }).items;
    if (Array.isArray(items)) return items as T[];
  }
  return [];
}

function unwrapFirst<T = unknown>(data: unknown): T | undefined {
  return unwrapArray<T>(data)[0];
}
