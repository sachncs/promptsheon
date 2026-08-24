'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Activity, Play, RefreshCw, TrendingUp } from 'lucide-react';
import { selfEvolveApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

interface SelfEvolveState {
  capabilityId?: string;
  iteration?: number;
  bestScore?: number;
  status?: 'idle' | 'running' | 'cooling-down' | 'error';
  lastRunAt?: string | null;
  nextRunAt?: string | null;
  history?: Array<{ iteration: number; score: number; at: string }>;
  cooldownSeconds?: number;
}

export default function SelfEvolvePage() {
  const session = useRequireSession();
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const qc = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const state = useQuery({
    queryKey: ['self-evolve', capabilityId],
    queryFn: () => selfEvolveApi.getState(capabilityId).then((r) => r.data as SelfEvolveState),
    enabled: Boolean(capabilityId),
    refetchInterval: 5000,
  });

  const runCycle = useMutation({
    mutationFn: () => selfEvolveApi.runCycle(capabilityId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['self-evolve', capabilityId] });
      setError(null);
    },
    onError: (err) => setError((err as Error).message),
  });

  if (!session) return null;

  const s = state.data;
  const isLoading = state.isLoading;
  const isError = state.isError && !s;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Self-evolve"
        title="Self-evolution loop"
        subtitle="Monitors live eval scores of the active release; on regression, re-plans and re-releases with cooldown."
        actions={
          <Button
            onClick={() => runCycle.mutate()}
            disabled={runCycle.isPending || s?.status === 'running' || s?.status === 'cooling-down'}
          >
            <Play className="mr-1.5 size-3.5" />
            {runCycle.isPending ? 'Running…' : 'Run cycle'}
          </Button>
        }
      />

      {isError ? (
        <EmptyState
          icon={Activity}
          title="Capability not found"
          description={`No self-evolve state for ${capabilityId.slice(0, 16)}. Activate a release before running cycles.`}
          className="border-0 bg-transparent p-12"
        />
      ) : isLoading && !s ? (
        <Surface>
          <div className="text-sm text-text-muted">Loading self-evolve state…</div>
        </Surface>
      ) : (
        <>
          <div className="grid gap-4 md:grid-cols-4">
            <StatCard
              label="Iteration"
              value={String(s?.iteration ?? 0)}
              hint={s?.lastRunAt ? `last ${new Date(s.lastRunAt).toLocaleString()}` : 'no runs yet'}
              icon={RefreshCw}
            />
            <StatCard
              label="Best score"
              value={s?.bestScore !== undefined ? s.bestScore.toFixed(3) : '—'}
              hint="across all iterations"
              icon={TrendingUp}
            />
            <StatCard
              label="Status"
              value={s?.status ?? 'idle'}
              icon={Activity}
              hint={s?.nextRunAt ? `next ${new Date(s.nextRunAt).toLocaleString()}` : undefined}
            />
            <StatCard
              label="Cooldown"
              value={s?.cooldownSeconds !== undefined ? `${s.cooldownSeconds}s` : '—'}
              hint="min seconds between cycles"
            />
          </div>

          <Surface>
            <SurfaceHeader title="Current state" />
            <div className="flex flex-wrap items-center gap-3">
              <StatusPill kind={(s?.status as never) ?? 'neutral'} />
              <Badge>{s?.status === 'cooling-down' ? 'awaiting cooldown' : s?.status ?? 'idle'}</Badge>
              {error && <span className="text-xs text-destructive">{error}</span>}
            </div>
          </Surface>

          {s?.history && s.history.length > 0 && (
            <Surface padded={false}>
              <SurfaceHeader className="px-5 pt-5" title="Iteration history" description={`${s.history.length} iteration(s)`} />
              <ol className="divide-y divide-border-subtle">
                {s.history.map((h, i) => {
                  const max = Math.max(...s.history!.map((x) => x.score));
                  const pct = max > 0 ? (h.score / max) * 100 : 0;
                  return (
                    <li key={i} className="grid grid-cols-12 items-center gap-3 px-5 py-3 text-sm">
                      <span className="col-span-1 font-mono text-xs text-text-subtle">#{h.iteration}</span>
                      <span className="col-span-2 font-mono text-xs text-text-muted">
                        {new Date(h.at).toLocaleString()}
                      </span>
                      <div className="col-span-7">
                        <div className="h-2 overflow-hidden rounded-full bg-surface-2">
                          <div
                            className="h-full bg-brand"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      </div>
                      <span className="col-span-2 text-right font-mono text-xs text-text-default">
                        {h.score.toFixed(3)}
                      </span>
                    </li>
                  );
                })}
              </ol>
            </Surface>
          )}

          <Surface>
            <div className="text-xs text-text-subtle">
              Self-evolution is gated by <code className="rounded bg-surface-2 px-1">PROMPTSHEON_SELF_EVOLVE_ENABLED</code>
              {' '}and rate-limited via the configured cooldown. Cycles run in the background — refresh this page or wait for the next auto-refresh to see new scores.
            </div>
          </Surface>
        </>
      )}
    </div>
  );
}