'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, ArrowLeft, Coins, Cpu, Users } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { analyticsApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { EmptyState } from '@/components/brand/empty-state';
import { ThemedSelect } from '@/components/brand/themed-select';
import { StatCard } from '@/components/brand/stat-card';
import type { LucideIcon } from 'lucide-react';

export default function AnalyticsPage() {
  const session = useRequireSession();
  const [days, setDays] = useState(30);

  const totals = useQuery({
    queryKey: ['analytics', 'org-totals', days],
    queryFn: () => analyticsApi.orgTotals(days),
    enabled: Boolean(session),
  });
  const leaderboard = useQuery({
    queryKey: ['analytics', 'leaderboard', days],
    queryFn: () => analyticsApi.leaderboard(days, 25),
    enabled: Boolean(session),
  });

  if (!session) return null;

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/app/admin/cost"
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
        >
          <ArrowLeft className="h-3 w-3" /> Cost & analytics
        </Link>
      </div>

      <PageHeader
        eyebrow="Observability"
        title="Per-user analytics"
        subtitle="See who's running what, when, and at what cost. Useful for finding runaway consumers and right-sizing per-user rate limits."
        actions={
          <ThemedSelect
            value={String(days)}
            onValueChange={(v) => setDays(Number(v))}
            options={[
              { value: '1', label: 'Last 24h' },
              { value: '7', label: 'Last 7d' },
              { value: '30', label: 'Last 30d' },
              { value: '90', label: 'Last 90d' },
            ]}
          />
        }
      />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <Tile
          label="Total runs"
          value={String(totals.data?.totals.runs ?? 0)}
          hint={`${totals.data?.totals.activeDays ?? 0} active days`}
          Icon={Activity}
        />
        <Tile
          label="Total tokens"
          value={formatNumber(totals.data?.totals.tokens ?? 0)}
          hint="all runs"
          Icon={Cpu}
        />
        <Tile
          label="Total cost"
          value={`$${(totals.data?.totals.cost ?? 0).toFixed(4)}`}
          hint="per run, raw-string cost"
          Icon={Coins}
        />
        <Tile
          label="Active users"
          value={String(leaderboard.data?.items.length ?? 0)}
          hint={`top ${leaderboard.data?.limit ?? 25}`}
          Icon={Users}
        />
      </div>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Top consumers"
          description={`${leaderboard.data?.items.length ?? 0} actors in the last ${days} days, ranked by tokens consumed.`}
        />
        {leaderboard.data && leaderboard.data.items.length > 0 ? (
          <ul className="divide-y divide-border-subtle">
            {leaderboard.data.items.map((u) => (
              <li key={u.actorId} className="flex items-baseline gap-4 px-5 py-3 text-sm">
                <span className="font-mono text-xs text-text-muted">{u.actorId.slice(0, 12)}…</span>
                <span className="font-mono text-text-default">{u.tokens.toLocaleString()} tokens</span>
                <span className="text-text-muted">·</span>
                <span className="font-mono text-text-default">${u.cost.toFixed(4)}</span>
                <span className="text-text-muted">·</span>
                <span className="text-xs text-text-muted">{u.runs} runs over {u.days} days</span>
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState
            className="m-5 border-0 bg-transparent shadow-none p-12"
            icon={Users}
            title="No per-user activity yet"
            description="Once runs start landing under user accounts, this leaderboard fills in."
          />
        )}
      </Surface>
    </div>
  );
}

function Tile({
  label,
  value,
  hint,
  Icon,
}: {
  label: string;
  value: string;
  hint: string;
  Icon: LucideIcon;
}) {
  return (
    <StatCard label={label} value={value} hint={hint} icon={Icon} />
  );
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}
