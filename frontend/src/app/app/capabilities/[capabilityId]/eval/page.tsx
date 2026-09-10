'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { evalApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { Progress } from '@/components/ui/progress';
import { EmptyState } from '@/components/brand/empty-state';
import { FlaskConical } from 'lucide-react';

export default function EvalRunsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const session = useRequireSession();

  const { data, isLoading } = useQuery({
    queryKey: ['eval-runs', capabilityId],
    queryFn: () => evalApi.list().then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });

  const rows = (Array.isArray(data) ? data : []) as Array<{
    id: string;
    scorer: string;
    score: number;
    status: string;
    startedAt: string;
  }>;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capability"
        title="Eval runs"
        subtitle="Run-on-save evals against this capability. Score is the per-case mean across the suite."
      />

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Recent runs" description={`${rows.length} recorded`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={FlaskConical}
            title="No eval runs yet"
            description={isLoading ? 'Loading…' : 'Activate a release to start collecting eval runs.'}
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              { key: 'id', header: 'Run', render: (r) => <span className="font-mono text-xs">{String(r['id']).slice(0, 12)}…</span> },
              { key: 'scorer', header: 'Scorer', render: (r) => String(r['scorer'] ?? '—') },
              {
                key: 'score',
                header: 'Score',
                render: (r) => {
                  const s = Number(r['score'] ?? 0);
                  return (
                    <div className="flex items-center gap-2 w-40">
                      <Progress value={s * 100} />
                      <span className="text-xs text-text-muted">{(s * 100).toFixed(0)}%</span>
                    </div>
                  );
                },
              },
              {
                key: 'status',
                header: 'Status',
                render: (r) => (
                  <StatusPill
                    kind={r['status'] === 'passed' ? 'active' : r['status'] === 'failed' ? 'rejected' : 'review'}
                    label={String(r['status'])}
                  />
                ),
              },
              {
                key: 'started',
                header: 'Started',
                render: (r) => r['startedAt'] ? new Date(String(r['startedAt'])).toLocaleString() : '—',
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}