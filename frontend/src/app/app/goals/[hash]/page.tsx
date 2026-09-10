'use client';

import { useParams } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Target, History } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { client } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';

interface GoalHistoryEntry {
  iteration: number;
  score: number;
  cost: number;
  revised: boolean;
  timestamp: string;
}

interface GoalSnapshot {
  iteration: number;
  manifestHash: string;
  score: number;
  timestamp: string;
}

interface GoalDetail {
  manifestHash: string;
  bestScore: number;
  bestManifestHash: string;
  iterations: number;
  totalCost: number;
  snapshots: GoalSnapshot[];
  history: GoalHistoryEntry[];
}

export default function GoalDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ hash: string }>();
  const hash = params?.hash ?? '';

  const goal = useQuery<GoalDetail>({
    queryKey: ['goal', hash],
    queryFn: async () => {
      const res = await client.get(`/goals/${hash}`);
      return res.data as GoalDetail;
    },
    enabled: Boolean(session && hash),
  });

  if (!session) return null;

  const data = goal.data;

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/app/goals"
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
        >
          <ArrowLeft className="h-3 w-3" /> Back to goals
        </Link>
      </div>

      <PageHeader
        eyebrow="Goal"
        title={data ? `Evolution cycle · ${data.iterations} iterations` : 'Goal detail'}
        subtitle="Drill-down into one goal-based evolution run. The path of best score, the iteration history, and the manifests that mattered."
        actions={data?.bestManifestHash ? <HashChip hash={data.bestManifestHash} /> : undefined}
      />

      {goal.isError ? (
        <EmptyState
          icon={Target}
          title="Goal state not available"
          description={`No in-memory state for goal hash ${hash.slice(0, 16)}. Active goals are kept in memory only — restart cycles are not preserved.`}
          action={
            <Link href="/app/goals">
              <span className="text-sm text-brand-highlight hover:underline">Back to active goals</span>
            </Link>
          }
        />
      ) : !data ? (
        <Surface>
          <div className="text-sm text-text-muted">
            {goal.isLoading ? 'Loading goal state…' : 'No goal state for this hash.'}
          </div>
        </Surface>
      ) : (
        <>
          <Surface>
            <SurfaceHeader title="Best so far" />
            <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-4">
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Best score</dt>
                <dd className="mt-1 font-mono text-text-strong">{(data.bestScore * 100).toFixed(1)}%</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Iterations</dt>
                <dd className="mt-1 text-text-default">{data.iterations}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Total cost</dt>
                <dd className="mt-1 font-mono text-text-default">${data.totalCost.toFixed(4)}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Source manifest</dt>
                <dd className="mt-1">
                  <HashChip hash={data.manifestHash} length={16} />
                </dd>
              </div>
            </dl>
          </Surface>

          <Surface padded={false}>
            <SurfaceHeader
              className="px-5 pt-5"
              title="Iteration history"
              description="One row per cycle. Score is the post-execution eval pass rate."
            />
            {data.history.length === 0 ? (
              <div className="px-5 pb-5 text-sm text-text-muted">No iterations yet.</div>
            ) : (
              <ul className="divide-y divide-border-subtle">
                {data.history.map((h) => (
                  <li
                    key={h.iteration}
                    className="flex items-center gap-4 px-5 py-3 text-sm"
                  >
                    <span className="font-mono text-xs text-text-subtle">#{h.iteration}</span>
                    <StatusPill
                      kind={h.score >= 0.7 ? 'active' : 'review'}
                      label={`${(h.score * 100).toFixed(1)}%`}
                    />
                    <span className="text-text-muted">${h.cost.toFixed(4)}</span>
                    {h.revised && (
                      <span className="rounded-full bg-brand/10 px-2 py-0.5 text-[10px] uppercase tracking-wider text-brand-highlight">
                        revised
                      </span>
                    )}
                    <span className="ml-auto text-xs text-text-subtle">
                      {new Date(h.timestamp).toLocaleString()}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Surface>

          {data.snapshots.length > 0 && (
            <Surface padded={false}>
              <SurfaceHeader
                className="px-5 pt-5"
                title="Manifest snapshots"
                description="Up to 50 most recent iterations, oldest first."
              />
              <ul className="divide-y divide-border-subtle">
                {data.snapshots.map((s) => (
                  <li key={s.iteration} className="flex items-center gap-3 px-5 py-3 text-sm">
                    <History className="h-3.5 w-3.5 text-text-subtle" aria-hidden="true" />
                    <span className="font-mono text-xs text-text-subtle">#{s.iteration}</span>
                    <HashChip hash={s.manifestHash} length={20} />
                    <span className="ml-auto text-xs text-text-subtle">
                      {(s.score * 100).toFixed(1)}%
                    </span>
                  </li>
                ))}
              </ul>
            </Surface>
          )}
        </>
      )}
    </div>
  );
}
