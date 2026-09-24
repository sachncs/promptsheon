'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { ShieldCheck, ShieldAlert, Inbox } from 'lucide-react';
import { approvalApi, type PendingApprovalSummary } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { Button } from '@/components/ui/button';
import { QueryError } from '@/components/brand/query-error';

export default function ApprovalsPage() {
  const session = useRequireSession();
  const router = useRouter();
  const approvals = useQuery({
    queryKey: ['approvals', 'pending'],
    queryFn: () => approvalApi.listPending().then((r) => r.data.approvals),
  });

  if (!session) return null;
  if (approvals.isPending) {
    return (
      <div className="space-y-6" aria-busy="true" aria-live="polite">
        <PageHeader eyebrow="Quality" title="Approvals queue" subtitle="Loading releases that need review…" />
        <Surface className="h-72 animate-pulse bg-surface-2/40">
          <span className="sr-only">Loading approvals</span>
        </Surface>
      </div>
    );
  }
  if (approvals.isError) return <QueryError message={approvals.error} onRetry={() => void approvals.refetch()} />;

  const rows: PendingApprovalSummary[] = approvals.data ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Quality"
        title="Approvals queue"
        subtitle="Releases waiting for a second pair of eyes. Maker-checker enforcement: the creator cannot approve their own release."
      />

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Pending review"
          description={`${rows.length} release(s) need attention`}
        />
        {rows.length === 0 ? (
          <EmptyState
            icon={Inbox}
            title="No approvals queued"
            description="When a release enters review, it appears here for a second reviewer."
            action={
              <Link href="/app/releases">
                <Button>Browse releases</Button>
              </Link>
            }
            className="m-5 border-0 bg-transparent p-12 shadow-none"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => r.releaseId}
            onRowClick={(r) => { router.push(`/app/approvals/${r.releaseId}`); }}
            columns={[
              {
                key: 'release',
                header: 'Release',
                render: (r) => (
                  <div>
                    <div className="font-mono text-sm font-medium text-text-strong">{r.releaseId}</div>
                    <div className="text-xs text-text-subtle">Updated {new Date(r.updatedAt).toLocaleString()}</div>
                  </div>
                ),
              },
              {
                key: 'approvals',
                header: 'Votes',
                render: (r) => {
                  if (r.approvals.length === 0) return <span className="text-xs text-text-subtle">no votes yet</span>;
                  return (
                    <div className="flex flex-wrap items-center gap-1.5">
                      {r.approvals.map((vote) => (
                        <span
                          key={`${vote.userId}:${vote.createdAt}`}
                          className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs ${
                            vote.vote === 'approve'
                              ? 'bg-success/15 text-success'
                              : 'bg-destructive/15 text-destructive'
                          }`}
                          title={`${vote.userId} · ${new Date(vote.createdAt).toLocaleString()}`}
                        >
                          {vote.vote === 'approve' ? <ShieldCheck className="size-3" /> : <ShieldAlert className="size-3" />}
                          {vote.userId}
                        </span>
                      ))}
                    </div>
                  );
                },
              },
              {
                key: 'hash',
                header: 'Manifest',
                render: (r) => <HashChip hash={r.manifestHash} />,
              },
              {
                key: 'state',
                header: 'State',
                render: () => <StatusPill kind={statusKindOf('review')} />,
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
