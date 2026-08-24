'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { GitBranch, Search, Lock } from 'lucide-react';
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
import { NewRepositoryDialog } from '@/components/brand/new-repository-dialog';

interface WorkspaceRow {
  id: string;
  name: string;
}
interface RepoRow {
  id: string;
  name: string;
  slug: string;
  visibility: string;
  minApprovers: number;
  requireSignedReleases: boolean;
  updatedAt: string;
  defaultBranch: string;
}

export default function RepositoriesPage() {
  const session = useRequireSession();
  const router = useRouter();
  const workspaces = useQuery<{ workspaces?: WorkspaceRow[] }>({
    queryKey: ['workspaces'],
    queryFn: async () => {
      const r = await workspaceApi.list(1);
      return r.data as { workspaces?: WorkspaceRow[] };
    },
  });
  const wsList: WorkspaceRow[] = workspaces.data?.workspaces ?? [];
  const wsFirst = wsList[0];

  const [query, setQuery] = useState('');

  const repos = useQuery<RepoRow[]>({
    queryKey: ['repos', wsFirst?.id ?? ''],
    queryFn: async () => {
      if (!wsFirst) return [] as RepoRow[];
      const list = await repoApi.list(wsFirst.id);
      return list as unknown as RepoRow[];
    },
    enabled: Boolean(wsFirst),
  });

  const filtered = useMemo(() => {
    const list = repos.data ?? [];
    if (!query.trim()) return list;
    const q = query.toLowerCase();
    return list.filter(
      (r) => r.name.toLowerCase().includes(q) || r.slug.toLowerCase().includes(q),
    );
  }, [repos.data, query]);

  if (!session) return null;

  if (!wsFirst && workspaces.isFetched) {
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
              aria-label="Filter repositories"
            />
          </div>
          {wsFirst && (
            <NewRepositoryDialog
              workspaceId={wsFirst.id}
              workspaces={wsList}
              disabled={repos.isLoading}
            />
          )}
        </div>
        {filtered.length === 0 ? (
          <EmptyState
            icon={GitBranch}
            title="No repositories yet"
            description="Create a repository to start committing capability manifests."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable<RepoRow>
            className="rounded-none border-0 border-t border-border-subtle"
            rows={filtered}
            caption="All repositories in the active workspace"
            rowKey={(r) => r.id}
            onRowClick={(r) => router.push(`/app/repos/${r.id}`)}
            columns={[
              {
                key: 'name',
                header: 'Name',
                render: (r) => (
                  <Link href={`/app/repos/${r.id}`} className="font-medium text-text-strong hover:underline">
                    {r.name}
                  </Link>
                ),
                sortable: true,
                sortKey: 'name',
              },
              {
                key: 'slug',
                header: 'Slug',
                render: (r) => <span className="font-mono text-xs text-text-muted">{r.slug}</span>,
              },
              {
                key: 'visibility',
                header: 'Visibility',
                render: (r) => <StatusPill kind="neutral" label={r.visibility || 'private'} />,
              },
              {
                key: 'approvers',
                header: 'Approvers',
                render: (r) => `${r.minApprovers}+`,
              },
              {
                key: 'signed',
                header: 'Signed releases',
                render: (r) => (r.requireSignedReleases ? 'required' : '—'),
              },
              {
                key: 'updated',
                header: 'Updated',
                render: (r) => new Date(r.updatedAt ?? Date.now()).toLocaleString(),
              },
              {
                key: 'default',
                header: 'Default',
                render: (r) => <HashChip hash={r.defaultBranch || 'main'} length={16} />,
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
