'use client';

import { useQuery } from '@tanstack/react-query';
import { Target, Sparkles } from 'lucide-react';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { EmptyState } from '@/components/brand/empty-state';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { Button } from '@/components/ui/button';

interface GoalSummary {
  manifestHash: string;
  bestScore: number;
  iterations: number;
  lastUpdated: string;
}

export default function GoalsPage() {
  const goals = useQuery<{ goals: GoalSummary[] }>({
    queryKey: ['goals'],
    queryFn: () => fetch('/api/goals').then((r) => r.json()),
    refetchInterval: 5000,
  });
  const list = goals.data?.goals ?? [];

  return (
    <div className="space-y-6">
      <div>
        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">Release</div>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-text-strong">Goals</h1>
        <p className="mt-1.5 text-sm text-text-muted">Goal-based evolution runs. Promptsheon re-plans and re-releases when the live eval score regresses.</p>
      </div>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Active evolution runs" description="Refreshes every 5s" />
        {list.length === 0 ? (
          <EmptyState
            className="m-5 border-0 bg-transparent shadow-none p-12"
            icon={Target}
            title="No goals running"
            description="Start a goal-based evolution cycle from any capability release. It will appear here while it iterates."
            action={
              <a href="/api/manifests" className="text-sm text-brand-highlight hover:underline">
                Browse manifests →
              </a>
            }
          />
        ) : (
          <ul className="divide-y divide-border-subtle">
            {list.map((g) => (
              <li key={g.manifestHash} className="flex items-center gap-5 px-5 py-4">
                <div className="grid h-10 w-10 place-items-center rounded-lg bg-surface-2 text-brand-highlight">
                  <Sparkles className="h-4 w-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-text-strong">Evolution cycle</div>
                  <div className="mt-0.5 flex items-center gap-2">
                    <HashChip hash={g.manifestHash} />
                    <StatusPill kind={g.bestScore >= 0.7 ? 'active' : 'review'} label={`iter ${g.iterations}`} />
                  </div>
                </div>
                <div className="text-right">
                  <div className="font-mono text-sm text-text-strong">{(g.bestScore * 100).toFixed(1)}%</div>
                  <div className="text-xs text-text-muted">{new Date(g.lastUpdated).toLocaleTimeString()}</div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Surface>
      <p className="text-xs text-text-subtle">
        Trigger a cycle via <code className="font-mono">POST /api/manifests/:hash/evolve</code>.
      </p>
    </div>
  );
}
