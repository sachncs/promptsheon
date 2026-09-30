'use client';

import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Bot } from 'lucide-react';
import { use, useState } from 'react';
import { z } from 'zod';
import { useRequireSession } from '@/hooks/use-session';
import { agentSpecificationApi, executionJobApi, workspaceApi, type AgentSpecificationRecord, type ExecutionJob, type WorkspaceRow } from '@/lib/api';
import { HashChip } from '@/components/brand/hash-chip';
import { PageHeader } from '@/components/brand/page-header';
import { QueryError } from '@/components/brand/query-error';
import { Surface } from '@/components/brand/surface';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';

const SpecificationSummarySchema = z.object({ role: z.string().optional(), objective: z.string().optional() });

export default function AgentRevisionPage({ params }: { params: Promise<{ hash: string }> }) {
  const session = useRequireSession();
  const { hash } = use(params);
  const [inputJson, setInputJson] = useState('{\n  "input": ""\n}');
  const [inputError, setInputError] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const workspaces = useQuery<WorkspaceRow[]>({ queryKey: ['workspaces'], queryFn: () => workspaceApi.list(1, 100).then((response) => response.data), enabled: Boolean(session) });
  const workspaceId = workspaces.data?.[0]?.id;
  const revision = useQuery({ queryKey: ['agent-specification', workspaceId, hash], queryFn: () => agentSpecificationApi.get(workspaceId!, hash), enabled: Boolean(session && workspaceId) });
  const lineage = useQuery({ queryKey: ['agent-specification-lineage', workspaceId, hash], queryFn: () => agentSpecificationApi.lineage(workspaceId!, hash), enabled: Boolean(session && workspaceId) });
  const queryClient = useQueryClient();
  const publish = useMutation({
    mutationFn: () => agentSpecificationApi.publish(workspaceId!, hash),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['agent-specification', workspaceId, hash] }),
        queryClient.invalidateQueries({ queryKey: ['agent-specifications', workspaceId] }),
      ]);
    },
  });
  const enqueue = useMutation({
    mutationFn: (inputs: Record<string, unknown>) => executionJobApi.enqueue(workspaceId!, {
      agentHash: hash,
      inputs,
      idempotencyKey: `ui-${hash.slice(0, 12)}-${Date.now()}`,
    }),
    onSuccess: (response) => setJobId(response.data.id),
  });
  const job = useQuery({
    queryKey: ['execution-job', workspaceId, jobId],
    queryFn: () => executionJobApi.get(workspaceId!, jobId!),
    enabled: Boolean(session && workspaceId && jobId),
    refetchInterval: (query) => isTerminalJob(query.state.data?.data) ? false : 1_000,
  });
  const cancel = useMutation({
    mutationFn: () => executionJobApi.cancel(workspaceId!, jobId!),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['execution-job', workspaceId, jobId] });
    },
  });

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={workspaces.error} onRetry={() => void workspaces.refetch()} />;
  if (revision.isError) return <QueryError message={revision.error} onRetry={() => void revision.refetch()} />;
  if (lineage.isError) return <QueryError message={lineage.error} onRetry={() => void lineage.refetch()} />;
  const data: AgentSpecificationRecord | undefined = revision.data?.data;
  const specification = SpecificationSummarySchema.safeParse(data?.specification).data;
  function runRevision(): void {
    try {
      const parsed: unknown = JSON.parse(inputJson);
      if (!isJsonObject(parsed)) {
        setInputError('Inputs must be a JSON object.');
        return;
      }
      setInputError(null);
      enqueue.mutate(parsed);
    } catch {
      setInputError('Inputs must be valid JSON.');
    }
  }

  return <div className="space-y-6"><Link href="/app/agents" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"><ArrowLeft className="h-3 w-3" /> Agent specifications</Link><PageHeader eyebrow="Agent revision" title={specification?.role ?? 'Loading revision'} subtitle={specification?.objective} actions={data && <div className="flex items-center gap-2"><StatusPill kind={statusKindOf(data.status)} label={data.status} />{data.status !== 'published' && data.status !== 'retired' && <Button size="sm" onClick={() => publish.mutate()} disabled={publish.isPending}>{publish.isPending ? 'Publishing…' : 'Publish revision'}</Button>}</div>} /><Surface><div className="flex items-center gap-3"><Bot className="h-5 w-5 text-brand-highlight" /><div className="text-sm text-text-muted">{data?.author ?? '—'} · {data?.changeReason ?? '—'}</div></div>{publish.isError && <p role="alert" className="mt-3 text-sm text-destructive">Unable to publish this revision. Try again after reviewing its validation state.</p>}<pre className="mt-6 max-h-[32rem] overflow-auto rounded-xl border border-border-subtle bg-surface-2 p-4 text-xs text-text-muted">{JSON.stringify(data?.specification, null, 2)}</pre></Surface><Surface><div className="flex items-center justify-between gap-3"><div><h2 className="text-sm font-semibold text-text-strong">Run this revision</h2><p className="mt-1 text-xs text-text-muted">Queue a durable execution using the configured runtime and resource budgets.</p></div>{job.data?.data && <StatusPill kind={statusKindOf(job.data.data.state)} label={job.data.data.state} />}</div><div className="mt-4 grid gap-2"><Label htmlFor="agent-run-inputs">JSON inputs</Label><Textarea id="agent-run-inputs" value={inputJson} onChange={(event) => { setInputJson(event.target.value); setInputError(null); }} rows={6} className="font-mono text-xs" aria-invalid={Boolean(inputError)} />{inputError && <p role="alert" className="text-sm text-destructive">{inputError}</p>}{enqueue.isError && <p role="alert" className="text-sm text-destructive">Unable to queue this execution. Try again.</p>} {job.isError && <p role="alert" className="text-sm text-destructive">Unable to read execution status. Try again.</p>}<div className="mt-2 flex flex-wrap gap-2"><Button onClick={runRevision} disabled={enqueue.isPending || !workspaceId}>{enqueue.isPending ? 'Queueing…' : 'Run revision'}</Button>{job.data?.data && !isTerminalJob(job.data.data) && <Button variant="outline" onClick={() => cancel.mutate()} disabled={cancel.isPending}>{cancel.isPending ? 'Cancelling…' : 'Cancel execution'}</Button>}</div>{cancel.isError && <p role="alert" className="text-sm text-destructive">Unable to cancel this execution. Try again.</p>}{job.data?.data?.error && <p className="text-sm text-destructive">Execution failed: {job.data.data.error}</p>}{job.data?.data?.resultJson && <pre className="max-h-48 overflow-auto rounded-lg bg-surface-2 p-3 text-xs text-text-muted">{job.data.data.resultJson}</pre>}</div></Surface><Surface><h2 className="text-sm font-semibold text-text-strong">Revision lineage</h2><div className="mt-4 space-y-3">{(lineage.data?.data ?? []).map((revisionItem) => <div key={revisionItem.hash} className="flex flex-wrap items-center gap-2 border-l-2 border-border-subtle pl-3"><HashChip hash={revisionItem.hash} length={14} /><StatusPill kind={statusKindOf(revisionItem.status)} label={revisionItem.status} /><span className="text-xs text-text-subtle">{revisionItem.changeReason} · {formatDate(revisionItem.createdAt)}</span></div>)}</div></Surface></div>;
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

function isTerminalJob(job: ExecutionJob | undefined): boolean {
  return Boolean(job && ['completed', 'failed', 'cancelled', 'timed-out', 'partially-completed'].includes(job.state));
}

function isJsonObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value));
}
