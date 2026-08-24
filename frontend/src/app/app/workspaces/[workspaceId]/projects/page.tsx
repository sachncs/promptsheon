'use client';

import * as React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import Link from 'next/link';
import { projectApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { useToast } from '@/components/brand/toast';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

interface ProjectRow {
  id: string;
  name: string;
  description?: string;
}

export default function WorkspaceProjectsPage() {
  const params = useParams<{ workspaceId: string }>();
  const workspaceId = params.workspaceId;
  const session = useRequireSession();
  const router = useRouter();
  const qc = useQueryClient();
  const { toast } = useToast();

  const projects = useQuery({
    queryKey: ['projects', workspaceId],
    queryFn: () => projectApi.list(workspaceId!).then((r) => r.data).catch(() => [] as ProjectRow[]),
    enabled: Boolean(workspaceId) && Boolean(session),
  });

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const create = useMutation({
    mutationFn: () => {
      const payload: { workspaceId: string; name: string; description?: string } = {
        workspaceId: workspaceId!,
        name: name.trim(),
      };
      if (description.trim()) payload.description = description.trim();
      return projectApi.create(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', workspaceId] });
      setName('');
      setDescription('');
      toast({ title: 'Project created', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Create failed', variant: 'destructive', description: (err as Error).message }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => projectApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', workspaceId] });
      toast({ title: 'Project deleted', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Delete failed', variant: 'destructive', description: (err as Error).message }),
  });

  const rows = (Array.isArray(projects.data) ? projects.data : []) as ProjectRow[];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Workspace"
        title="Projects"
        subtitle="Projects group capabilities with shared eval suites and release policies."
      />

      <Surface>
        <SurfaceHeader title="New project" description="A workspace can host many projects." />
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="refund-triage"
              className="mt-2"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Description</label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What lives in this project?"
              className="mt-2 min-h-10"
            />
          </div>
        </div>
        <div className="mt-4 flex items-center justify-end">
          <Button onClick={() => create.mutate()} disabled={!name.trim() || create.isPending}>
            <Plus className="mr-1.5 size-3.5" />
            {create.isPending ? 'Creating…' : 'Create project'}
          </Button>
        </div>
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Projects" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Plus}
            title="No projects yet"
            description="Create one above, then click into it to add capabilities."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => router.push(`/app/projects/${String(r['id'])}/capabilities`)}
            columns={[
              {
                key: 'name',
                header: 'Project',
                render: (r) => (
                  <Link href={`/app/projects/${String(r['id'])}/capabilities`} className="font-medium text-text-strong hover:underline">
                    {String(r['name'])}
                  </Link>
                ),
              },
              {
                key: 'description',
                header: 'Description',
                render: (r) => r['description'] ? <span className="text-text-muted">{String(r['description'])}</span> : <span className="text-text-subtle">—</span>,
              },
              {
                key: 'actions',
                header: '',
                render: (r) => (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={(e) => { e.stopPropagation(); remove.mutate(String(r['id'])); }}
                  >
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