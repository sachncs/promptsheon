'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, ShieldCheck, ShieldAlert } from 'lucide-react';
import { approvalApi, releaseApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill } from '@/components/brand/status-pill';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';

interface ReleaseDetail {
  id: string;
  capabilityId?: string;
  capabilityName?: string;
  capabilityVersion?: number;
  environment?: string;
  state?: string;
  manifestHash?: string;
  canaryPercent?: number;
  approvals?: Array<{ id: string; voter: string; decision: 'approve' | 'reject'; comment?: string; at: string }>;
  createdBy?: string;
  createdAt?: string;
  updatedAt?: string;
}

export default function ReleaseApprovalPage() {
  const session = useRequireSession();
  const params = useParams<{ releaseId: string }>();
  const releaseId = params.releaseId;
  const qc = useQueryClient();

  const release = useQuery({
    queryKey: ['release', releaseId],
    queryFn: () => releaseApi.get(releaseId).then((r) => r.data as ReleaseDetail),
    enabled: Boolean(releaseId),
    retry: false,
  });
  const approvals = useQuery({
    queryKey: ['approvals', releaseId],
    queryFn: () => approvalApi.list(releaseId).then((r) => r.data).catch(() => [] as Array<{ id: string; voter: string; decision: 'approve' | 'reject'; comment?: string; at: string }>),
    enabled: Boolean(releaseId),
  });

  const [decision, setDecision] = useState<'approve' | 'reject'>('approve');
  const [comment, setComment] = useState('');

  const vote = useMutation({
    mutationFn: () => {
      const trimmed = comment.trim();
      const payload: { decision: 'approve' | 'reject'; comment?: string } = { decision };
      if (trimmed) payload.comment = trimmed;
      return approvalApi.vote(releaseId, payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['approvals', releaseId] });
      qc.invalidateQueries({ queryKey: ['release', releaseId] });
      setComment('');
    },
  });

  if (!session) return null;

  const data = release.data;
  const approvalRows = ((approvals.data ?? []) as NonNullable<ReleaseDetail['approvals']>).concat(data?.approvals ?? []);
  const seen = new Set<string>();
  const dedup = approvalRows.filter((a) => {
    const key = `${a.voter}:${a.decision}:${a.at}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/releases" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="size-3" /> Back to releases
        </Link>
      </div>

      <PageHeader
        eyebrow="Approval"
        title={data?.capabilityName ? `${data.capabilityName} v${data.capabilityVersion ?? '?'}` : 'Release approval'}
        subtitle={data?.environment ? `${data.environment} · ${data.state ?? '—'}` : 'Approve or reject this release.'}
        actions={
          <div className="flex items-center gap-2">
            {data?.state ? <StatusPill kind={(data.state as never) ?? 'neutral'} /> : null}
            {data?.manifestHash ? <HashChip hash={data.manifestHash} /> : null}
          </div>
        }
      />

      {release.isError ? (
        <Surface>
          <div className="text-sm text-text-muted">Release not found.</div>
        </Surface>
      ) : !data ? (
        <Surface>
          <div className="text-sm text-text-muted">{release.isLoading ? 'Loading release…' : 'No release data.'}</div>
        </Surface>
      ) : (
        <div className="grid gap-5 lg:grid-cols-3">
          <Surface className="lg:col-span-2" padded={false}>
            <SurfaceHeader className="px-5 pt-5" title="Cast your vote" />
            <div className="space-y-3 px-5 pb-5">
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setDecision('approve')}
                  className={`flex-1 rounded-lg border p-3 text-sm font-medium transition-colors ${
                    decision === 'approve'
                      ? 'border-success bg-success/10 text-success'
                      : 'border-border-subtle bg-surface-1 text-text-muted hover:border-border-strong'
                  }`}
                >
                  <ShieldCheck className="mx-auto mb-1 size-5" />
                  Approve
                </button>
                <button
                  type="button"
                  onClick={() => setDecision('reject')}
                  className={`flex-1 rounded-lg border p-3 text-sm font-medium transition-colors ${
                    decision === 'reject'
                      ? 'border-destructive bg-destructive/10 text-destructive'
                      : 'border-border-subtle bg-surface-1 text-text-muted hover:border-border-strong'
                  }`}
                >
                  <ShieldAlert className="mx-auto mb-1 size-5" />
                  Reject
                </button>
              </div>
              <Textarea
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                placeholder={`Why ${decision === 'approve' ? 'approve' : 'reject'}? (optional, but recommended)`}
                className="min-h-24"
              />
              <div className="flex items-center justify-between">
                <div className="text-xs text-text-subtle">
                  The release creator cannot vote on their own release.
                </div>
                <Button onClick={() => vote.mutate()} disabled={vote.isPending}>
                  {vote.isPending ? 'Submitting…' : 'Submit vote'}
                </Button>
              </div>
              {vote.isError && (
                <div className="text-xs text-destructive">{(vote.error as Error).message}</div>
              )}
            </div>
          </Surface>

          <Surface padded={false}>
            <SurfaceHeader className="px-5 pt-5" title="Vote history" description={`${dedup.length} vote(s)`} />
            {dedup.length === 0 ? (
              <div className="px-5 pb-5 text-sm text-text-muted">No votes yet.</div>
            ) : (
              <ul className="divide-y divide-border-subtle">
                {dedup.map((a, i) => (
                  <li key={i} className="px-5 py-3 text-sm">
                    <div className="flex items-baseline justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <Badge className={a.decision === 'approve' ? 'bg-success/15 text-success' : 'bg-destructive/15 text-destructive'}>
                          {a.decision}
                        </Badge>
                        <span className="font-medium text-text-strong">{a.voter}</span>
                      </div>
                      <span className="text-xs text-text-subtle">{new Date(a.at).toLocaleString()}</span>
                    </div>
                    {a.comment && <p className="mt-1 text-text-muted">{a.comment}</p>}
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </div>
      )}
    </div>
  );
}