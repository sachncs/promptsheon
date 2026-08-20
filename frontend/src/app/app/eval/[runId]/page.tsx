'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { ArrowLeft, FlaskConical, ListChecks, TrendingDown } from 'lucide-react';
import { evalApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';

export default function EvalRunPage() {
  const params = useParams<{ runId: string }>();
  const id = params.runId;

  const run = useQuery({
    queryKey: ['eval-run', id],
    queryFn: () => evalApi.get(id).then((r) => r.data).catch(() => null),
    enabled: Boolean(id),
  });

  const results = useQuery({
    queryKey: ['eval-run', id, 'results'],
    queryFn: () => evalApi.getResults(id).then((r) => r.data).catch(() => []),
    enabled: Boolean(id),
  });

  if (run.isLoading) return <div className="text-text-muted text-sm">Loading run…</div>;
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

  const r = run.data as Record<string, unknown>;
  const rows = (Array.isArray(results.data) ? results.data : []) as Array<Record<string, unknown>>;

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/eval" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Eval runs
        </Link>
        <PageHeader
          eyebrow="Evaluation"
          title={`Run ${String(r['id'] ?? '').slice(0, 8)}`}
          subtitle="Per-case results, scoring summary, regression detection, and threshold gate visualisation."
          actions={<StatusPill kind={(r['status'] as never) ?? 'pending'} />}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="rounded-xl border border-border-subtle bg-surface-1 p-5">
          <div className="text-xs uppercase tracking-wider text-text-subtle">Score</div>
          <div className="mt-3 text-3xl font-semibold text-text-strong">
            {r['score'] != null ? `${(Number(r['score']) * 100).toFixed(1)}%` : '—'}
          </div>
          <div className="mt-1 text-sm text-text-muted">Pass threshold {r['threshold'] != null ? `${(Number(r['threshold']) * 100).toFixed(0)}%` : '92%'}</div>
        </div>
        <div className="rounded-xl border border-border-subtle bg-surface-1 p-5">
          <div className="text-xs uppercase tracking-wider text-text-subtle">Cases</div>
          <div className="mt-3 text-3xl font-semibold text-text-strong">{rows.length}</div>
          <div className="mt-1 text-sm text-text-muted">{String(r['datasetName'] ?? 'dataset')}</div>
        </div>
        <div className="rounded-xl border border-border-subtle bg-surface-1 p-5">
          <div className="text-xs uppercase tracking-wider text-text-subtle">Regressions</div>
          <div className="mt-3 text-3xl font-semibold text-warning">
            {rows.filter((row) => row['regression']).length}
          </div>
          <div className="mt-1 text-sm text-text-muted">vs. baseline release</div>
        </div>
      </div>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Per-case results"
          description="Inputs, expected, and actual outputs. Decision column shows the scorer's verdict."
          actions={<ListChecks className="h-4 w-4 text-text-muted" />}
        />
        <DataTable
          className="rounded-none border-0 border-t border-border-subtle"
          rows={rows}
          rowKey={(row, idx) => String(row['caseId'] ?? row['id'] ?? `row-${idx ?? 0}`)}
          columns={[
            { key: 'name', header: 'Case', render: (row) => String(row['name'] ?? row['caseId'] ?? '—') },
            { key: 'input', header: 'Input', render: (row) => <span className="font-mono text-xs text-text-muted">{String(row['input'] ?? '').slice(0, 80)}</span> },
            { key: 'expected', header: 'Expected', render: (row) => <span className="font-mono text-xs text-text-muted">{String(row['expected'] ?? '').slice(0, 80)}</span> },
            { key: 'actual', header: 'Actual', render: (row) => <span className="font-mono text-xs text-text-default">{String(row['actual'] ?? '').slice(0, 80)}</span> },
            { key: 'score', header: 'Score', render: (row) => row['score'] != null ? `${(Number(row['score']) * 100).toFixed(0)}%` : '—' },
            { key: 'decision', header: 'Decision', render: (row) => <StatusPill kind={(row['passed'] === false ? 'rejected' : 'approved') as never} label={row['passed'] === false ? 'fail' : 'pass'} /> },
            { key: 'reg', header: '', render: (row) => row['regression'] ? <TrendingDown className="h-3.5 w-3.5 text-warning" /> : null },
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
