'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Boxes, Plus, Trash2 } from 'lucide-react';
import { projectApi, workspaceApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

interface ProjectItem {
  id: string;
  workspaceId?: string;
  name?: string;
  description?: string;
  capabilityCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

export default function ProjectsPage() {
  const session = useRequireSession();
  const router = useRouter();
  const qc = useQueryClient();

  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const wsFirst = Array.isArray(workspaces.data) ? workspaces.data[0] as { id?: string } : undefined;
  const wsId = wsFirst?.id;

  const projects = useQuery({
    queryKey: ['projects', wsId],
    queryFn: () => (wsId ? projectApi.list(wsId).then((r) => r.data) : Promise.resolve([])),
    enabled: Boolean(wsId),
  });
  const rows = (projects.data ?? []) as ProjectItem[];

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const create = useMutation({
    mutationFn: () => {
      if (!wsId) throw new Error('No active workspace');
      const payload: { workspaceId: string; name: string; description?: string } = { workspaceId: wsId, name };
      if (description.trim()) payload.description = description.trim();
      return projectApi.create(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', wsId] });
      setName('');
      setDescription('');
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => projectApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects', wsId] }),
  });

  if (!session) return null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Projects"
        subtitle="Projects sit inside workspaces. They group capabilities with shared eval suites and release policies."
      />

      <Surface>
        <SurfaceHeader title="New project" description={wsId ? `In workspace ${wsId.slice(0, 8)}…` : 'No workspace available'} />
        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="refund-triage"
              className="mt-2"
            />
          </div>
          <div className="sm:col-span-2">
            <label className="text-xs uppercase tracking-wider text-text-subtle">Description</label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What lives in this project? What does it ship?"
              className="mt-2 min-h-10"
            />
          </div>
        </div>
        <div className="mt-3 flex items-center justify-end">
          <Button onClick={() => create.mutate()} disabled={!name || create.isPending}>
            <Plus className="mr-1.5 size-3.5" />
            {create.isPending ? 'Creating…' : 'Create project'}
          </Button>
        </div>
        {create.isError && (
          <div className="mt-3 text-xs text-destructive">{(create.error as Error).message}</div>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Projects" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Boxes}
            title="No projects to show"
            description="Create one above, or open a workspace and group capabilities under a project."
            action={
              <Link href="/app/workspaces">
                <Button variant="outline">Open workspaces</Button>
              </Link>
            }
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => { router.push(`/app/workspaces/${String(r['workspaceId'] ?? wsId)}/projects`); }}
            columns={[
              {
                key: 'name',
                header: 'Project',
                render: (r) => (
                  <div>
                    <div className="font-medium text-text-strong">{String(r['name'] ?? '—')}</div>
                    {r['description'] ? <div className="text-xs text-text-subtle">{String(r['description'])}</div> : null}
                  </div>
                ),
              },
              {
                key: 'capabilities',
                header: 'Capabilities',
                render: (r) => {
                  const n = r['capabilityCount'];
                  return n !== undefined ? <span className="font-mono text-xs">{String(n)}</span> : '—';
                },
              },
              {
                key: 'updated',
                header: 'Updated',
                render: (r) => r['updatedAt'] ? new Date(String(r['updatedAt'])).toLocaleDateString() : '—',
              },
              {
                key: 'actions',
                header: '',
                render: (r) => (
                  <Button size="sm" variant="outline" onClick={(e) => { e.stopPropagation(); remove.mutate(String(r['id'])); }}>
                    <Trash2 className="mr-1 size-3" />
                    Delete
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}