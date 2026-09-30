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
import { QueryError } from '@/components/brand/query-error';

export default function AnalyticsPage() {
  const session = useRequireSession();
  const [days, setDays] = useState(30);
  const [selectedActor, setSelectedActor] = useState<string | null>(null);

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
  const userUsage = useQuery({
    queryKey: ['analytics', 'user', selectedActor, days],
    queryFn: () => analyticsApi.userPerDay(selectedActor ?? '', days),
    enabled: Boolean(session && selectedActor),
  });

  if (!session) return null;
  if (totals.isError) return <QueryError message={totals.error} onRetry={() => void totals.refetch()} />;
  if (leaderboard.isError) return <QueryError message={leaderboard.error} onRetry={() => void leaderboard.refetch()} />;
  if (userUsage.isError) return <QueryError message={userUsage.error} onRetry={() => void userUsage.refetch()} />;

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
              <li key={u.actorId}>
                <button
                  type="button"
                  onClick={() => setSelectedActor(u.actorId)}
                  className={`flex w-full items-baseline gap-4 px-5 py-3 text-left text-sm transition-colors hover:bg-surface-0 ${selectedActor === u.actorId ? 'bg-surface-0' : ''}`}
                  aria-pressed={selectedActor === u.actorId}
                >
                  <span className="font-mono text-xs text-text-muted">{u.actorId.slice(0, 12)}…</span>
                  <span className="font-mono text-text-default">{u.tokens.toLocaleString()} tokens</span>
                  <span className="text-text-muted">·</span>
                  <span className="font-mono text-text-default">${u.cost.toFixed(4)}</span>
                  <span className="text-text-muted">·</span>
                  <span className="text-xs text-text-muted">{u.runs} runs over {u.days} days</span>
                </button>
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

      {selectedActor ? (
        <Surface padded={false}>
          <SurfaceHeader
            className="px-5 pt-5"
            title="Selected user usage"
            description={`Daily activity for ${selectedActor} over the last ${days} days.`}
            actions={
              <button
                type="button"
                onClick={() => setSelectedActor(null)}
                className="text-xs text-text-muted underline-offset-2 hover:text-text-default hover:underline"
              >
                Clear selection
              </button>
            }
          />
          {userUsage.data && userUsage.data.perDay.length > 0 ? (
            <div className="overflow-x-auto px-5 pb-5">
              <table className="w-full min-w-[32rem] text-left text-sm">
                <thead className="text-xs uppercase tracking-wider text-text-subtle">
                  <tr>
                    <th className="py-2 font-medium">Day</th>
                    <th className="py-2 font-medium">Runs</th>
                    <th className="py-2 font-medium">Tokens</th>
                    <th className="py-2 font-medium">Cost</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-subtle">
                  {userUsage.data.perDay.map((day) => (
                    <tr key={day.day}>
                      <td className="py-2 font-mono text-xs text-text-muted">{day.day}</td>
                      <td className="py-2 text-text-default">{day.runs}</td>
                      <td className="py-2 font-mono text-text-default">{day.tokens.toLocaleString()}</td>
                      <td className="py-2 font-mono text-text-default">${day.cost.toFixed(4)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="px-5 pb-5 text-sm text-text-muted">No daily activity found for this user in the selected period.</p>
          )}
        </Surface>
      ) : null}
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
