'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, GitMerge, ShieldCheck } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { repoApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { QueryError } from '@/components/brand/query-error';

export default function MergeRequestDetail() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params.id;
  const [comment, setComment] = useState('');

  const mr = useQuery({
    queryKey: ['mr-detail', id],
    queryFn: () => repoApi.getMR(id),
    enabled: Boolean(id),
  });

  if (!session) return null;
  if (mr.isError) return <QueryError message={mr.error} onRetry={() => void mr.refetch()} />;
  if (mr.isLoading) return <div className="text-text-muted text-sm">Loading…</div>;
  if (!mr.data) return <div className="text-text-muted text-sm">Merge request not found.</div>;

  const detail = mr.data;
  const m = detail.mr;
  const status = m.status;

  async function decide(decision: 'approve' | 'request_changes') {
    await repoApi.decideMR(id, decision, comment || undefined);
    setComment('');
    await mr.refetch();
  }

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/repos" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Repositories
        </Link>
        <PageHeader
          eyebrow={`Merge request #${m.number}`}
          title={m.title}
          subtitle={`${m.sourceBranch} → ${m.targetBranch} · ${m.authorId}`}
          actions={<StatusPill kind={status === 'open' ? 'review' : status === 'merged' ? 'active' : 'rolled-back'} />}
        />
      </div>

      <div className="grid gap-5 lg:grid-cols-3">
        <Surface className="lg:col-span-2">
          <SurfaceHeader title="Description" />
          {m.description ? (
            <p className="whitespace-pre-wrap text-sm text-text-default">{m.description}</p>
          ) : (
            <p className="text-text-muted text-sm">No description provided.</p>
          )}

          <SurfaceHeader title="Inline comments" className="mt-6" />
          {detail.comments.length === 0 ? (
            <p className="text-text-muted text-sm">No comments yet.</p>
          ) : (
            <ul className="space-y-3">
              {detail.comments.map((c) => (
                <li key={c.id} className="rounded-lg border border-border-subtle bg-surface-2/40 p-3">
                  <div className="text-xs text-text-subtle">
                    {c.authorId} · {new Date(c.createdAt).toLocaleString()}
                    {c.path ? <span className="ml-2 font-mono">[{c.path}]</span> : null}
                  </div>
                  <div className="mt-1 text-sm text-text-default whitespace-pre-wrap">{c.body}</div>
                </li>
              ))}
            </ul>
          )}
        </Surface>

        <Surface>
          <SurfaceHeader title="Your decision" description="Approval must come from a different user than the author." />
          <Textarea
            placeholder="Optional comment…"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            rows={4}
          />
          <div className="mt-3 flex gap-2">
            <Button onClick={() => decide('approve')} className="bg-success/20 text-success hover:bg-success/30">
              <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />Approve
            </Button>
            <Button variant="outline" onClick={() => decide('request_changes')}>
              <GitMerge className="mr-1.5 h-3.5 w-3.5" />Request changes
            </Button>
          </div>

          <SurfaceHeader title="Approvals" className="mt-6" />
          {detail.approvals.length === 0 ? (
            <p className="text-text-muted text-sm">No decisions recorded.</p>
          ) : (
            <ul className="space-y-2">
              {detail.approvals.map((a) => (
                <li key={`${a.userId}-${a.createdAt}`} className="flex items-center gap-2">
                  <StatusPill kind={a.decision === 'approve' ? 'approved' : 'rejected'} label={String(a.decision)} />
                  <span className="text-xs text-text-muted">{a.userId}</span>
                </li>
              ))}
            </ul>
          )}

          <SurfaceHeader title="Source commit" className="mt-6" />
          <HashChip hash={m.sourceCommitOid} length={32} />
        </Surface>
      </div>
    </div>
  );
}
