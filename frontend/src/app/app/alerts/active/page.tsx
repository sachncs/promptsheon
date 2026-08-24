'use client';

import { useQuery } from '@tanstack/react-query';
import { alertApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { Bell } from 'lucide-react';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';

export default function AlertsActivePage() {
  const session = useRequireSession();
  const alerts = useQuery({
    queryKey: ['alerts', 'active'],
    queryFn: () => alertApi.listAlerts().then((r) => r.data).catch(() => []),
    enabled: Boolean(session),
  });
  const rows = Array.isArray(alerts.data) ? alerts.data : [];

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Release" title="Active alerts" subtitle="Currently firing. Acknowledge to silence; root-cause from the linked run or audit row." />
      {rows.length === 0 ? (
        <EmptyState icon={Bell} title="All clear" description="No active alerts. New alerts from eval regressions, latency spikes, or approval windows appear here." />
      ) : (
        <Surface padded={false}>
          <SurfaceHeader className="px-5 pt-5" title={`${rows.length} active`} />
          <ul className="divide-y divide-border-subtle">
            {rows.map((a: Record<string, unknown>) => (
              <li key={String(a['id'])} className="flex items-center gap-3 px-5 py-4">
                <StatusPill kind={(a['severity'] as never) ?? 'warning'} label={String(a['severity'] ?? 'alert')} />
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-text-strong">{String(a['name'] ?? a['id'])}</div>
                  <div className="text-xs text-text-muted">{String(a['rule'] ?? a['kind'] ?? '')}</div>
                </div>
                <time className="text-xs text-text-subtle">{new Date(String(a['firedAt'] ?? Date.now())).toLocaleString()}</time>
              </li>
            ))}
          </ul>
        </Surface>
      )}
    </div>
  );
}
