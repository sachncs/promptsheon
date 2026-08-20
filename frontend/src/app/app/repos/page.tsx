'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { Plus, GitBranch, Search, Lock } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { workspaceApi, repoApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill } from '@/components/brand/status-pill';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export default function RepositoriesPage() {
  const session = useRequireSession();
  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const wsFirst = Array.isArray(workspaces.data) ? workspaces.data[0] : undefined;
  const wsId = (wsFirst as { id?: string } | undefined)?.id;

  const [query, setQuery] = useState('');

  const repos = useQuery({
    queryKey: ['repos', wsId],
    queryFn: () => (wsId ? repoApi.list(wsId) : Promise.resolve([] as Array<Record<string, unknown>>)),
    enabled: Boolean(wsId),
  });

  const filtered = useMemo(() => {
    const list = (Array.isArray(repos.data) ? repos.data : []) as Array<Record<string, unknown>>;
    if (!query.trim()) return list;
    const q = query.toLowerCase();
    return list.filter((r) =>
      String(r['name'] ?? '').toLowerCase().includes(q) ||
      String(r['slug'] ?? '').toLowerCase().includes(q),
    );
  }, [repos.data, query]);

  if (!session) return null;

  if (!wsId && workspaces.isFetched) {
    return (
      <EmptyState
        icon={Lock}
        title="Create a workspace first"
        description="A workspace is the top-level container for repositories. Once it's set up, repositories will appear here."
        action={<Link href="/app/workspaces"><Button>Open workspaces</Button></Link>}
      />
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Repositories"
        title="All repositories"
        subtitle="Git-native version control for your capabilities. Branches, tags, commits, merge requests, all content-addressed."
      />

      <Surface padded={false}>
        <div className="flex items-center gap-3 border-b border-border-subtle px-5 py-4">
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-subtle" />
            <Input
              placeholder="Filter by name…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9 h-9 bg-surface-1 border-border-subtle"
            />
          </div>
          <Button>
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            New repository
          </Button>
        </div>
        {filtered.length === 0 ? (
          <EmptyState
            icon={GitBranch}
            title="No repositories yet"
            description="Create a repository to start committing capability manifests."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={filtered}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => { window.location.href = `/app/repos/${String(r['id'])}`; }}
            columns={[
              {
                key: 'name',
                header: 'Name',
                render: (r) => (
                  <Link href={`/app/repos/${String(r['id'])}`} className="font-medium text-text-strong hover:underline">
                    {String(r['name'])}
                  </Link>
                ),
              },
              {
                key: 'slug',
                header: 'Slug',
                render: (r) => <span className="font-mono text-xs text-text-muted">{String(r['slug'])}</span>,
              },
              {
                key: 'visibility',
                header: 'Visibility',
                render: (r) => <StatusPill kind="neutral" label={String(r['visibility'] ?? 'private')} />,
              },
              {
                key: 'approvers',
                header: 'Approvers',
                render: (r) => `${Number(r['minApprovers'] ?? 1)}+`,
              },
              {
                key: 'signed',
                header: 'Signed releases',
                render: (r) => (r['requireSignedReleases'] ? 'required' : '—'),
              },
              {
                key: 'updated',
                header: 'Updated',
                render: (r) => new Date(String(r['updatedAt'] ?? Date.now())).toLocaleString(),
              },
              {
                key: 'default',
                header: 'Default',
                render: (r) => <HashChip hash={String(r['defaultBranch'] ?? 'main')} length={16} />,
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
