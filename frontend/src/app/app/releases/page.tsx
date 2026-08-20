'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import {
  GitBranch, GitMerge, Plus,
} from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { workspaceApi, projectApi, capabilityApi, releaseApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';

export default function ReleasesPage() {
  const session = useRequireSession();
  const workspaces = useQuery({ queryKey: ['workspaces'], queryFn: () => workspaceApi.list(1).then((r) => r.data) });
  const wsFirst = unwrapFirst<{ id: string }>(workspaces.data);

  const projects = useQuery({
    queryKey: ['projects', wsFirst?.id],
    queryFn: () => projectApi.list(wsFirst!.id).then((r) => r.data),
    enabled: Boolean(wsFirst?.id),
  });
  const allProjects = unwrapArray<{ id: string }>(projects.data);

  const capabilities = useQuery({
    queryKey: ['capabilities', 'all', allProjects.map((p) => p.id)],
    queryFn: async () => {
      const out: Array<Record<string, unknown>> = [];
      for (const p of allProjects) {
        const list = await capabilityApi.list(p.id).then((r) => r.data).catch(() => []);
        if (Array.isArray(list)) out.push(...(list as Array<Record<string, unknown>>));
      }
      return out;
    },
    enabled: allProjects.length > 0,
  });

  const releases = useQuery({
    queryKey: ['releases', 'all', capabilities.data],
    queryFn: async () => {
      const out: Array<Record<string, unknown>> = [];
      const caps = capabilities.data ?? [];
      for (const c of caps) {
        const list = await releaseApi.list(String(c['id'])).then((r) => r.data).catch(() => []);
        if (Array.isArray(list)) out.push(...list.map((rel: Record<string, unknown>) => ({ ...rel, capabilityName: String(c['name'] ?? '—'), capabilityId: String(c['id']) })));
      }
      return out.sort((a, b) => String(b['createdAt'] ?? '').localeCompare(String(a['createdAt'] ?? '')));
    },
    enabled: Array.isArray(capabilities.data) && capabilities.data.length > 0,
  });

  const rows = Array.isArray(releases.data) ? releases.data : [];

  if (!session) return null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Release"
        title="Releases"
        subtitle="Every promotion of a capability is a first-class artifact. Draft → review → approved → canary → active → rolled back."
        actions={
          <Link href="/app/capabilities">
            <Button><Plus className="mr-1.5 h-3.5 w-3.5" />New release</Button>
          </Link>
        }
      />

      {rows.length === 0 ? (
        <EmptyState
          icon={GitBranch}
          title="No releases yet"
          description="Author a capability and create its first release. Releases inherit the deterministic state machine."
          action={
            <Link href="/app/editor">
              <Button variant="outline">Open the DAG editor</Button>
            </Link>
          }
        />
      ) : (
        <Surface padded={false}>
          <SurfaceHeader className="px-5 pt-5" title={`${rows.length} releases`} />
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => { window.location.href = `/app/releases/${String(r['id'])}`; }}
            columns={[
              { key: 'cap', header: 'Capability', render: (r) => String(r['capabilityName'] ?? '—') },
              { key: 'v', header: 'Version', render: (r) => `v${String(r['capabilityVersion'] ?? '?')}` },
              { key: 'env', header: 'Env', render: (r) => <span className="font-mono text-xs text-text-muted">{String(r['environment'] ?? 'production')}</span> },
              { key: 'state', header: 'State', render: (r) => <StatusPill kind={(r['state'] as never) ?? 'neutral'} /> },
              { key: 'canary', header: 'Canary', render: (r) => r['canaryPercent'] != null ? `${String(r['canaryPercent'])}%` : '—' },
              { key: 'hash', header: 'Content', render: (r) => <HashChip hash={String(r['manifestHash'] ?? r['id'])} /> },
              { key: 'updated', header: 'Updated', render: (r) => new Date(String(r['updatedAt'] ?? r['createdAt'] ?? Date.now())).toLocaleString() },
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
