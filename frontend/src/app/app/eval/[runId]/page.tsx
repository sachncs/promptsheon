'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { AlertTriangle, ArrowLeft, FlaskConical, ListChecks } from 'lucide-react';
import { evalApi, type EvalResult } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { QueryError } from '@/components/brand/query-error';

export default function EvalRunPage() {
  const session = useRequireSession();
  const params = useParams<{ runId: string }>();
  const id = params.runId;

  const run = useQuery({
    queryKey: ['eval-run', id],
    queryFn: () => evalApi.get(id).then((r) => r.data),
    enabled: Boolean(id) && Boolean(session),
  });

  const results = useQuery({
    queryKey: ['eval-run', id, 'results'],
    queryFn: () => evalApi.getResults(id).then((r) => r.data),
    enabled: Boolean(id) && Boolean(session),
  });

  if (run.isLoading) return <div className="text-text-muted text-sm">Loading run…</div>;
  if (run.isError) return <QueryError message={run.error} onRetry={() => void run.refetch()} />;
  if (!run.data) {
    return (
      <EmptyState
        icon={FlaskConical}
        title="Eval run not found"
        description="The run may have been removed or the link is stale."
        action={
          <Link href="/app/eval">
            <Button variant="outline"><ArrowLeft className="mr-1.5 h-3.5 w-3.5" />Back to runs</Button>
          </Link>
        }
      />
    );
  }
  if (results.isError) return <QueryError message={results.error} onRetry={() => void results.refetch()} />;

  const evalRun = run.data;
  const rows: EvalResult[] = results.data ?? [];

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/eval" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Eval runs
        </Link>
        <PageHeader
          eyebrow="Evaluation"
          title={`Run ${evalRun.id.slice(0, 8)}`}
          subtitle="Per-case results, scoring summary, and threshold gate visualisation."
          actions={<StatusPill kind={statusKindOf(evalRun.status, 'pending')} />}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="rounded-xl border border-border-subtle bg-surface-1 p-5">
          <div className="text-xs uppercase tracking-wider text-text-subtle">Score</div>
          <div className="mt-3 text-3xl font-semibold text-text-strong">
            {(evalRun.score * 100).toFixed(1)}%
          </div>
          <div className="mt-1 text-sm text-text-muted">Scored by {evalRun.scorer}</div>
        </div>
        <div className="rounded-xl border border-border-subtle bg-surface-1 p-5">
          <div className="text-xs uppercase tracking-wider text-text-subtle">Cases</div>
          <div className="mt-3 text-3xl font-semibold text-text-strong">{rows.length}</div>
          <div className="mt-1 text-sm text-text-muted">Dataset {evalRun.datasetId.slice(0, 12)}…</div>
        </div>
        <div className="rounded-xl border border-border-subtle bg-surface-1 p-5">
          <div className="text-xs uppercase tracking-wider text-text-subtle">Failed cases</div>
          <div className="mt-3 text-3xl font-semibold text-warning">
            {evalRun.failed}
          </div>
          <div className="mt-1 text-sm text-text-muted">of {evalRun.total} total cases</div>
        </div>
      </div>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Per-case results"
          description="Actual outputs, failures, and latency for each evaluated case."
          actions={<ListChecks className="h-4 w-4 text-text-muted" />}
        />
        <DataTable
          className="rounded-none border-0 border-t border-border-subtle"
          rows={rows}
          rowKey={(row) => row.id}
          columns={[
            { key: 'case', header: 'Case', render: (row) => row.caseId ?? `Case ${row.seq}` },
            { key: 'actual', header: 'Actual', render: (row) => <span className="font-mono text-xs text-text-default">{row.actual.slice(0, 80)}</span> },
            { key: 'error', header: 'Error', render: (row) => row.error ? <span className="inline-flex items-center gap-1 text-xs text-destructive"><AlertTriangle className="size-3" />{row.error.slice(0, 80)}</span> : '—' },
            { key: 'latency', header: 'Latency', render: (row) => `${row.latencyMs.toLocaleString()}ms` },
            { key: 'decision', header: 'Decision', render: (row) => <StatusPill kind={row.passed ? 'approved' : 'rejected'} label={row.passed ? 'pass' : 'fail'} /> },
          ]}
          empty={
            <EmptyState
              icon={FlaskConical}
              title="No results yet"
              description="The scorer is still running. Results stream in as cases complete."
            />
          }
        />
      </Surface>
    </div>
  );
}
