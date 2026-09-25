'use client';

import { useQuery } from '@tanstack/react-query';
import { alertApi, type Alert } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { Bell } from 'lucide-react';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { QueryError } from '@/components/brand/query-error';

export default function AlertsActivePage() {
  const session = useRequireSession();
  const alerts = useQuery({
    queryKey: ['alerts', 'active'],
    queryFn: () => alertApi.listAlerts().then((r) => r.data),
    enabled: Boolean(session),
  });
  const rows: Alert[] = alerts.data ?? [];

  if (alerts.isError) return <QueryError message={alerts.error} onRetry={() => void alerts.refetch()} />;

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Release" title="Active alerts" subtitle="Currently firing. Acknowledge to silence; root-cause from the linked run or audit row." />
      {rows.length === 0 ? (
        <EmptyState icon={Bell} title="All clear" description="No active alerts. New alerts from eval regressions, latency spikes, or approval windows appear here." />
      ) : (
        <Surface padded={false}>
          <SurfaceHeader className="px-5 pt-5" title={`${rows.length} active`} />
          <ul className="divide-y divide-border-subtle">
            {rows.map((a) => (
              <li key={a.id} className="flex items-center gap-3 px-5 py-4">
                <StatusPill kind={statusKindOf(a.severity, 'warning')} label={a.severity} />
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-text-strong">{a.ruleName}</div>
                  <div className="text-xs text-text-muted">{a.message}</div>
                </div>
                <time className="text-xs text-text-subtle">{new Date(a.triggeredAt).toLocaleString()}</time>
              </li>
            ))}
          </ul>
        </Surface>
      )}
    </div>
  );
}
