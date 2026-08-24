'use client';

import { useQuery } from '@tanstack/react-query';
import { auditApi } from '@/lib/api';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { ScrollText } from 'lucide-react';

export default function AuditPage() {
  const audit = useQuery({
    queryKey: ['audit', 'all'],
    queryFn: () => auditApi.list().then((r) => r.data).catch(() => []),
  });
  const rows = (Array.isArray(audit.data) ? audit.data : []) as Array<Record<string, unknown>>;

  return (
    <div className="space-y-6">
      <div>
        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">Release</div>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-text-strong">Audit log</h1>
        <p className="mt-1.5 text-sm text-text-muted">
          Hash-linked, append-only record of every mutation. Verifiable at <code className="font-mono text-text-default">/api/audit/verify</code>.
        </p>
      </div>

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="All events"
          description={`${rows.length} entries`}
        />
        <DataTable
          className="rounded-none border-0 border-t border-border-subtle"
          rows={rows}
          rowKey={(r) => String(r['id'])}
          columns={[
            { key: 'action', header: 'Action', render: (r) => <span className="font-mono text-xs">{String(r['action'])}</span> },
            { key: 'resource', header: 'Resource', render: (r) => <span className="font-mono text-xs text-text-muted">{String(r['resource'])}</span> },
            { key: 'actor', header: 'Actor', render: (r) => (r['actor'] as string | undefined) ?? 'system' },
            { key: 'ts', header: 'Timestamp', render: (r) => new Date(String(r['timestamp'] ?? r['createdAt'] ?? Date.now())).toLocaleString() },
            { key: 'hash', header: 'Hash', render: (r) => <HashChip hash={(r['hash'] as string | undefined) ?? '—'} /> },
          ]}
          empty={
            <EmptyState
              icon={ScrollText}
              title="No audit entries yet"
              description="Every action on a capability will be recorded here. The first one is a setup event from the platform itself."
              className="m-5 border-0 bg-transparent shadow-none p-12"
            />
          }
        />
      </Surface>
    </div>
  );
}
