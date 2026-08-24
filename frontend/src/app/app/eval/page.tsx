'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { FlaskConical, Plus } from 'lucide-react';
import { evalApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';

export default function EvalListPage() {
  const router = useRouter();
  const evals = useQuery({
    queryKey: ['eval-runs'],
    queryFn: () => evalApi.list().then((r) => r.data).catch(() => []),
  });
  const rows = Array.isArray(evals.data) ? evals.data : [];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Quality"
        title="Evaluation engine"
        subtitle="Run datasets, score with deterministic or LLM-judge scorers, gate releases on thresholds, detect regressions."
        actions={
          <Link href="/app/capabilities">
            <Button><Plus className="mr-1.5 h-3.5 w-3.5" />New eval run</Button>
          </Link>
        }
      />

      {rows.length === 0 ? (
        <EmptyState
          icon={FlaskConical}
          title="No eval runs yet"
          description="Create a dataset and trigger a release-scoped run. Pass/fail thresholds gate activation."
          action={
            <Link href="/app/capabilities"><Button variant="outline">Open a release</Button></Link>
          }
        />
      ) : (
        <Surface padded={false}>
          <SurfaceHeader className="px-5 pt-5" title={`${rows.length} runs`} />
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r: Record<string, unknown>) => String(r['id'])}
            onRowClick={(r) => { router.push(`/app/eval/${String(r['id'])}`); }}
            columns={[
              { key: 'release', header: 'Release', render: (r) => String(r['releaseId'] ?? '—') },
              { key: 'dataset', header: 'Dataset', render: (r) => String(r['datasetId'] ?? '—') },
              { key: 'scorer', header: 'Scorer', render: (r) => String(r['scorer'] ?? '—') },
              { key: 'score', header: 'Score', render: (r) => r['score'] != null ? `${(Number(r['score']) * 100).toFixed(0)}%` : '—' },
              { key: 'state', header: 'Status', render: (r) => <StatusPill kind={(r['status'] as never) ?? 'pending'} /> },
              { key: 'started', header: 'Started', render: (r) => new Date(String(r['startedAt'] ?? r['createdAt'] ?? Date.now())).toLocaleString() },
            ]}
          />
        </Surface>
      )}
    </div>
  );
}
