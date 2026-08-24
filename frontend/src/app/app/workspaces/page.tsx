'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { FolderOpen, Plus, Trash2 } from 'lucide-react';
import { workspaceApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { useToast } from '@/components/brand/toast';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export default function WorkspacesPage() {
  const session = useRequireSession();
  const router = useRouter();
  const qc = useQueryClient();
  const { toast } = useToast();
  const [name, setName] = useState('');
  const [organization, setOrganization] = useState('');

  const ws = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data).catch(() => [] as Array<Record<string, unknown>>),
    enabled: Boolean(session),
  });
  const rows = (Array.isArray(ws.data) ? ws.data : []) as Array<Record<string, unknown>>;

  const create = useMutation({
    mutationFn: () => {
      const payload: { name: string; organization?: string } = { name: name.trim() };
      if (organization.trim()) payload.organization = organization.trim();
      return workspaceApi.create(payload);
    },
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['workspaces'] });
      setName('');
      setOrganization('');
      const created = data?.data as { id?: string } | undefined;
      if (created?.id) {
        toast({ title: 'Workspace created', variant: 'success', description: 'Open it to start adding projects.' });
        router.push(`/app/workspaces/${created.id}/projects`);
      } else {
        toast({ title: 'Workspace created', variant: 'success' });
      }
    },
    onError: (err) => toast({ title: 'Create failed', variant: 'destructive', description: (err as Error).message }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => workspaceApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] });
      toast({ title: 'Workspace deleted', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Delete failed', variant: 'destructive', description: (err as Error).message }),
  });

  if (!session) return null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Workspaces"
        subtitle="Workspaces group projects, capabilities, and release environments for one team."
      />

      <Surface>
        <SurfaceHeader
          title="New workspace"
          description="Workspaces are containers for projects and the capabilities they ship."
        />
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="sm:col-span-2">
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="refund-triage"
              className="mt-2"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Organisation label</label>
            <Input
              value={organization}
              onChange={(e) => setOrganization(e.target.value)}
              placeholder="Acme AI"
              className="mt-2"
            />
          </div>
        </div>
        <div className="mt-4 flex items-center justify-end">
          <Button onClick={() => create.mutate()} disabled={!name.trim() || create.isPending}>
            <Plus className="mr-1.5 size-3.5" />
            {create.isPending ? 'Creating…' : 'Create workspace'}
          </Button>
        </div>
        {create.isError && (
          <div className="mt-3 text-xs text-destructive">{(create.error as Error).message}</div>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Workspaces" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={FolderOpen}
            title="No workspaces yet"
            description="Create one above, or open a workspace and start adding projects."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => router.push(`/app/workspaces/${String(r['id'])}/projects`)}
            columns={[
              {
                key: 'name',
                header: 'Name',
                render: (r) => (
                  <Link href={`/app/workspaces/${String(r['id'])}/projects`} className="font-medium text-text-strong hover:underline">
                    {String(r['name'] ?? '—')}
                  </Link>
                ),
              },
              {
                key: 'org',
                header: 'Organisation',
                render: (r) => {
                  const org = (r['organization'] as string | undefined) ?? '';
                  return org ? <span className="text-text-muted">{org}</span> : <span className="text-text-subtle">—</span>;
                },
              },
              {
                key: 'id',
                header: 'Identifier',
                render: (r) => <HashChip hash={String(r['id'])} />,
              },
              {
                key: 'actions',
                header: '',
                render: (r) => (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={(e) => {
                      e.stopPropagation();
                      remove.mutate(String(r['id']));
                    }}
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