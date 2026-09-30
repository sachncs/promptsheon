'use client';

import * as React from 'react';
import Link from 'next/link';
import { Check, GitPullRequest, X } from 'lucide-react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getErrorMessage, mutationProposalApi, type MutationProposal, type ReleaseEnvironment } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { EmptyState } from '@/components/brand/empty-state';
import { QueryError } from '@/components/brand/query-error';
import { StatusPill, statusKindOf, type StatusKind } from '@/components/brand/status-pill';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';

function riskLabel(risk: MutationProposal['risk']): StatusKind {
  return risk === 'critical' || risk === 'high' ? 'error' : risk === 'medium' ? 'review' : 'active';
}

export default function MutationProposalsPage() {
  const session = useRequireSession();
  const queryClient = useQueryClient();
  const [reasonById, setReasonById] = React.useState<Record<string, string>>({});
  const [environment, setEnvironment] = React.useState<ReleaseEnvironment>('dev');
  const proposals = useQuery({
    queryKey: ['mutation-proposals'],
    queryFn: () => mutationProposalApi.list(),
    enabled: Boolean(session),
  });
  const decide = useMutation({
    mutationFn: ({ id, decision }: { id: string; decision: 'approve' | 'reject' | 'abandon' }) => {
      const reason = reasonById[id]?.trim();
      if (!reason) throw new Error('Add a decision reason before continuing.');
      return mutationProposalApi.decide(id, decision, reason);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['mutation-proposals'] }),
  });
  const validate = useMutation({
    mutationFn: (id: string) => mutationProposalApi.validate(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['mutation-proposals'] }),
  });
  const promote = useMutation({
    mutationFn: (id: string) => mutationProposalApi.promote(id, environment),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['mutation-proposals'] }),
  });

  if (!session) return null;
  if (proposals.isPending) {
    return <div className="space-y-6" aria-busy="true"><PageHeader eyebrow="Build" title="Mutation proposals" subtitle="Loading candidate changes…" /><Surface className="h-72 animate-pulse bg-surface-2/40"><span className="sr-only">Loading proposals</span></Surface></div>;
  }
  if (proposals.isError) return <QueryError message={proposals.error} onRetry={() => void proposals.refetch()} />;

  const rows = proposals.data ?? [];
  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Build"
        title="Mutation proposals"
        subtitle="Review evidence-backed candidates before they can enter the release workflow. Proposals never activate themselves."
      />
      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Candidate changes" description={`${rows.length} proposal${rows.length === 1 ? '' : 's'} in this organisation`} />
        {rows.length === 0 ? (
          <EmptyState icon={GitPullRequest} title="No mutation proposals" description="When the evolution loop finds a candidate, its rationale and risk will appear here for review." className="m-5 border-0 bg-transparent p-12 shadow-none" />
        ) : (
          <div className="divide-y divide-border-subtle border-t border-border-subtle">
            {rows.map((proposal) => {
              const actionable = proposal.status === 'validated' && proposal.evaluationStatus === 'passed';
              const reviewable = proposal.status === 'proposed' || proposal.status === 'validated';
              return (
                <article key={proposal.id} className="space-y-4 p-5">
                  <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-xs text-text-subtle">{proposal.id}</span>
                        <StatusPill kind={statusKindOf(proposal.status, 'pending')} label={proposal.status} />
                        <StatusPill kind={riskLabel(proposal.risk)} label={`${proposal.risk} risk`} />
                        <span className="rounded-full bg-surface-2 px-2 py-0.5 text-xs text-text-muted">{proposal.mutationKind}</span>
                      </div>
                      <h2 className="mt-3 text-sm font-semibold text-text-strong">{proposal.expectedOutcome}</h2>
                      <p className="mt-1 text-sm leading-relaxed text-text-muted">{proposal.rationale}</p>
                    </div>
                    <div className="shrink-0 text-left text-xs text-text-subtle lg:text-right">
                      <div>Confidence {Math.round(proposal.confidence * 100)}%</div>
                      <div className="mt-1">Source <span className="font-mono">{proposal.sourceHash.slice(0, 12)}</span></div>
                      <div className="mt-1">Candidate <span className="font-mono">{proposal.candidateHash?.slice(0, 12) ?? 'not materialised'}</span></div>
                      <div className="mt-1">
                        Evaluation {proposal.evaluationStatus}
                        {proposal.baselineScore !== null && proposal.candidateScore !== null
                          ? ` · ${Math.round(proposal.baselineScore * 100)}% → ${Math.round(proposal.candidateScore * 100)}%`
                          : ''}
                      </div>
                      <div className="mt-1">Trace {proposal.evaluationRunId ? 'attached' : 'not attached'}</div>
                    </div>
                  </div>
                  <details className="rounded-lg border border-border-subtle bg-surface-2/50 px-3 py-2 text-xs">
                    <summary className="cursor-pointer font-medium text-text-muted">View proposed changes</summary>
                    <pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-text-subtle">{JSON.stringify(proposal.changes, null, 2)}</pre>
                  </details>
                  {reviewable ? (
                    <div className="grid gap-3 lg:grid-cols-[1fr_auto] lg:items-end">
                      <Textarea
                        value={reasonById[proposal.id] ?? ''}
                        onChange={(event) => setReasonById((current) => ({ ...current, [proposal.id]: event.target.value }))}
                        placeholder="Decision reason (required for auditability)"
                        rows={2}
                        aria-label={`Decision reason for ${proposal.id}`}
                      />
                      <div className="flex flex-wrap gap-2">
                        {proposal.status === 'proposed' ? (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => validate.mutate(proposal.id)}
                            disabled={validate.isPending || !proposal.candidateHash}
                            title={proposal.candidateHash ? 'Validate the immutable candidate manifest' : 'Validation requires a materialised candidate'}
                          >
                            <Check /> Validate candidate
                          </Button>
                        ) : null}
                        {actionable ? (
                          <Button
                            size="sm"
                            onClick={() => decide.mutate({ id: proposal.id, decision: 'approve' })}
                            disabled={decide.isPending || !proposal.candidateHash}
                            title={proposal.candidateHash ? 'Approve this validated immutable candidate' : 'Approval requires a materialised candidate'}
                          >
                            <Check /> Approve
                          </Button>
                        ) : null}
                        <Button size="sm" variant="destructive" onClick={() => decide.mutate({ id: proposal.id, decision: 'reject' })} disabled={decide.isPending}>
                          <X /> Reject
                        </Button>
                      </div>
                    </div>
                  ) : proposal.status === 'approved' && !proposal.promotedReleaseId ? (
                    <div className="flex flex-wrap items-center gap-2">
                      <label className="text-xs text-text-muted" htmlFor={`environment-${proposal.id}`}>Create draft for</label>
                      <select
                        id={`environment-${proposal.id}`}
                        value={environment}
                        onChange={(event) => setEnvironment(event.target.value as ReleaseEnvironment)}
                        className="h-8 rounded-lg border border-border-subtle bg-surface-1 px-2 text-xs text-text-default"
                      >
                        <option value="dev">Development</option>
                        <option value="staging">Staging</option>
                        <option value="prod">Production</option>
                      </select>
                      <Button size="sm" variant="outline" onClick={() => promote.mutate(proposal.id)} disabled={promote.isPending}>
                        {promote.isPending ? 'Creating draft…' : 'Create draft release'}
                      </Button>
                    </div>
                  ) : proposal.promotedReleaseId ? (
                    <p className="text-xs text-text-subtle">Draft release created: <Link className="text-brand hover:underline" href={`/app/releases/${proposal.promotedReleaseId}`}>{proposal.promotedReleaseId}</Link></p>
                  ) : proposal.decisionReason ? (
                    <p className="text-xs text-text-subtle">Decision: {proposal.decisionReason}</p>
                  ) : null}
                  {reviewable && !proposal.candidateHash && (
                    <p className="text-xs text-text-muted">This proposal cannot be approved until its immutable candidate is materialised.</p>
                  )}
                  {proposal.status === 'validated' && proposal.evaluationStatus !== 'passed' && (
                    <p className="text-xs text-text-muted">Approval is disabled until the candidate passes evaluation.</p>
                  )}
                  {(validate.isError || decide.isError || promote.isError) && (
                    <p role="alert" className="text-xs text-destructive">
                      {getErrorMessage(validate.error ?? decide.error ?? promote.error, 'The proposal action failed. Try again.')}
                    </p>
                  )}
                </article>
              );
            })}
          </div>
        )}
      </Surface>
    </div>
  );
}
