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
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { QueryError } from '@/components/brand/query-error';
import { getErrorMessage } from '@/lib/errors';

export default function SelfEvolvePage() {
  const session = useRequireSession();
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const qc = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const state = useQuery({
    queryKey: ['self-evolve', capabilityId],
    queryFn: () => selfEvolveApi.getState(capabilityId).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
    refetchInterval: 5000,
  });

  const runCycle = useMutation({
    mutationFn: () => selfEvolveApi.runCycle(capabilityId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['self-evolve', capabilityId] });
      setError(null);
    },
    onError: (err) => setError(getErrorMessage(err)),
  });

  if (!session) return null;

  const s = state.data;
  const isLoading = state.isLoading;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Self-evolve"
        title="Self-evolution loop"
        subtitle="Monitors live eval scores of the active release; on regression, re-plans and re-releases with cooldown."
        actions={
          <Button
            onClick={() => runCycle.mutate()}
            disabled={runCycle.isPending}
          >
            <Play className="mr-1.5 size-3.5" />
            {runCycle.isPending ? 'Running…' : 'Run cycle'}
          </Button>
        }
      />

      {state.isError ? (
        <QueryError message={state.error} onRetry={() => void state.refetch()} />
      ) : !s && !isLoading ? (
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
              value={String(s?.cycleCount ?? 0)}
              hint="completed evolution cycles"
              icon={RefreshCw}
            />
            <StatCard
              label="Best score"
              value={s?.lastEvalScore !== undefined ? s.lastEvalScore.toFixed(3) : '—'}
              hint="score from the latest evaluation"
              icon={TrendingUp}
            />
            <StatCard
              label="Status"
              value={s?.status ?? 'idle'}
              icon={Activity}
              hint="current evolution state"
            />
            <StatCard
              label="Revision"
              value={s?.lastRevisionHash ? `${s.lastRevisionHash.slice(0, 10)}…` : '—'}
              hint="content-addressed manifest hash"
            />
          </div>

          <Surface>
            <SurfaceHeader title="Current state" />
            <div className="flex flex-wrap items-center gap-3">
              <StatusPill kind={statusKindOf(s?.status)} />
              <Badge>{s?.status ?? 'idle'}</Badge>
              {error && <span className="text-xs text-destructive">{error}</span>}
            </div>
          </Surface>

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
