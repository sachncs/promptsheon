'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { GitMerge, ArrowRight } from 'lucide-react';
import { workspaceApi, repoApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

interface MergeRequestRow {
  id: string;
  repoId?: string;
  repoName?: string;
  title?: string;
  status?: 'open' | 'merged' | 'closed';
  sourceBranch?: string;
  targetBranch?: string;
  author?: string;
  createdAt?: string;
  updatedAt?: string;
}

export default function MergeRequestsIndex() {
  const session = useRequireSession();
  const router = useRouter();

  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const wsFirst = Array.isArray(workspaces.data) ? workspaces.data[0] as { id?: string } : undefined;
  const wsId = wsFirst?.id;

  const repos = useQuery({
    queryKey: ['repos-for-mrs', wsId],
    queryFn: () => (wsId ? repoApi.list(wsId) : Promise.resolve([] as Array<{ id: string; name: string }>)),
    enabled: Boolean(wsId),
  });

  const allMrs = useQuery({
    queryKey: ['merge-requests-all', (repos.data ?? []).map((r: { id: string }) => r.id).join(',')],
    queryFn: async (): Promise<MergeRequestRow[]> => {
      const out: MergeRequestRow[] = [];
      for (const r of repos.data ?? []) {
        try {
          const mrs = await repoApi.listMRs(r.id);
          for (const m of mrs as Array<Record<string, unknown>>) {
            const row: MergeRequestRow = {
              id: String(m['id'] ?? ''),
              repoId: r.id,
              repoName: r.name,
            };
            const title = m['title'];
            if (typeof title === 'string') row.title = title;
            const status = m['status'];
            if (status === 'open' || status === 'merged' || status === 'closed') row.status = status;
            const sourceBranch = m['sourceBranch'];
            if (typeof sourceBranch === 'string') row.sourceBranch = sourceBranch;
            const targetBranch = m['targetBranch'];
            if (typeof targetBranch === 'string') row.targetBranch = targetBranch;
            const author = m['authorId'];
            if (typeof author === 'string') row.author = author;
            const createdAt = m['createdAt'];
            if (typeof createdAt === 'string') row.createdAt = createdAt;
            const updatedAt = m['updatedAt'];
            if (typeof updatedAt === 'string') row.updatedAt = updatedAt;
            out.push(row);
          }
        } catch { /* skip */ }
      }
      return out;
    },
    enabled: (repos.data ?? []).length > 0,
  });

  if (!session) return null;

  const rows = allMrs.data ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Repositories"
        title="Merge requests"
        subtitle="Cross-repo queue of open MRs awaiting review or merge. Maker-checker enforcement on approvals."
      />

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="All merge requests"
          description={`${rows.length} across ${(repos.data ?? []).length} repository(s)`}
        />
        {rows.length === 0 ? (
          <EmptyState
            icon={GitMerge}
            title="No merge requests yet"
            description="Open an MR from a repository to populate this list."
            action={
              <Link href="/app/repos">
                <Button variant="outline">Open repositories</Button>
              </Link>
            }
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => { router.push(`/app/merge-requests/${String(r['id'])}`); }}
            columns={[
              {
                key: 'title',
                header: 'Title',
                render: (r) => (
                  <div>
                    <div className="font-medium text-text-strong">{String(r['title'] ?? '—')}</div>
                    {r['repoName'] ? <div className="text-xs text-text-subtle">{String(r['repoName'])}</div> : null}
                  </div>
                ),
              },
              {
                key: 'branches',
                header: 'Branches',
                render: (r) => (
                  <span className="font-mono text-xs text-text-muted">
                    {String(r['sourceBranch'] ?? '?')} <ArrowRight className="mx-1 inline size-3" /> {String(r['targetBranch'] ?? '?')}
                  </span>
                ),
              },
              {
                key: 'status',
                header: 'Status',
                render: (r) => {
                  const status = String(r['status'] ?? 'open');
                  const tone = status === 'merged' ? 'bg-success/15 text-success' : status === 'closed' ? 'bg-destructive/15 text-destructive' : 'bg-info/15 text-info';
                  return <Badge className={tone}>{status}</Badge>;
                },
              },
              {
                key: 'author',
                header: 'Author',
                render: (r) => r['author'] ? <span className="font-mono text-xs">{String(r['author']).slice(0, 12)}…</span> : '—',
              },
              {
                key: 'updated',
                header: 'Updated',
                render: (r) => r['updatedAt'] ? new Date(String(r['updatedAt'])).toLocaleString() : '—',
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}