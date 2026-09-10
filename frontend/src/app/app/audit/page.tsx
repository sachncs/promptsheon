'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { CheckCircle2, ScrollText, ShieldCheck, AlertCircle } from 'lucide-react';
import { auditApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { ThemedTooltip } from '@/components/brand/themed-tooltip';
import { Tabs, TabsList, TabsTrigger } from '@/components/brand/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Drawer, DrawerContent } from '@/components/brand/drawer';
import { ThemedSelect } from '@/components/brand/themed-select';

interface AuditEntry {
  id: string;
  action?: string;
  resource?: string;
  resourceKind?: string;
  resourceId?: string;
  actor?: string;
  createdAt?: string;
  hash?: string;
  prevHash?: string | null;
  details?: string;
}

const DATE_RANGES = ['24h', '7d', '30d', 'all'] as const;
type DateRange = (typeof DATE_RANGES)[number];

function withinRange(ts: string | undefined, range: DateRange): boolean {
  if (!ts) return true;
  if (range === 'all') return true;
  const t = new Date(ts).getTime();
  const day = 24 * 60 * 60 * 1000;
  const window = range === '24h' ? day : range === '7d' ? 7 * day : 30 * day;
  return Date.now() - t <= window;
}

export default function AuditPage() {
  const session = useRequireSession();
  const [range, setRange] = useState<DateRange>('7d');
  const [resource, setResource] = useState<string>('');
  const [action, setAction] = useState<string>('');
  const [actorFilter, setActorFilter] = useState('');
  const [openId, setOpenId] = useState<string | null>(null);

  const audit = useQuery({
    queryKey: ['audit', 'all'],
    queryFn: () => auditApi.list().then((r) => r.data).catch(() => []),
    enabled: Boolean(session),
  });
  const allRows = ((audit.data ?? []) as AuditEntry[]);

  const resourceOptions = useMemo(() => {
    const set = new Set<string>();
    for (const r of allRows) if (r.resourceKind) set.add(r.resourceKind);
    return [
      { value: '', label: 'All resources' },
      ...[...set].sort().map((v) => ({ value: v, label: v })),
    ];
  }, [allRows]);

  const actionOptions = useMemo(() => {
    const set = new Set<string>();
    for (const r of allRows) if (r.action) set.add(r.action);
    return [
      { value: '', label: 'All actions' },
      ...[...set].sort().map((v) => ({ value: v, label: v })),
    ];
  }, [allRows]);

  const filtered = useMemo(() => {
    return allRows.filter((r) => {
      if (!withinRange(r.createdAt, range)) return false;
      if (resource && r.resourceKind !== resource) return false;
      if (action && r.action !== action) return false;
      if (actorFilter && !(r.actor ?? '').toLowerCase().includes(actorFilter.toLowerCase())) return false;
      return true;
    });
  }, [allRows, range, resource, action, actorFilter]);

  const open = openId ? filtered.find((r) => r.id === openId) : null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Release"
        title="Audit log"
        subtitle="Hash-linked, append-only record of every mutation. Verifiable at /api/audit/verify."
      />

      <Surface>
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 text-sm text-text-muted">
            <ShieldCheck className="size-4 text-success" />
            <span>
              Chain integrity: <span className="font-medium text-text-default">unverified on first load</span>.
              Verify any row via the chain link in the drawer.
            </span>
          </div>
          <span className="text-xs text-text-subtle">{filtered.length.toLocaleString()} of {allRows.length.toLocaleString()} entries</span>
        </div>
      </Surface>

      <div className="flex flex-wrap items-center gap-3">
        <Tabs value={range} onValueChange={(v) => setRange(v as DateRange)}>
          <TabsList>
            {DATE_RANGES.map((r) => (
              <TabsTrigger key={r} value={r}>{r === 'all' ? 'All' : r}</TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <ThemedSelect
          value={resource}
          onValueChange={setResource}
          options={resourceOptions}
          ariaLabel="Resource filter"
          triggerClassName="w-48"
        />
        <ThemedSelect
          value={action}
          onValueChange={setAction}
          options={actionOptions}
          ariaLabel="Action filter"
          triggerClassName="w-56"
        />
        <Input
          value={actorFilter}
          onChange={(e) => setActorFilter(e.target.value)}
          placeholder="Actor contains…"
          className="max-w-48"
        />
      </div>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Entries" />
        {filtered.length === 0 ? (
          <EmptyState
            icon={ScrollText}
            title={allRows.length === 0 ? 'No audit entries yet' : 'No entries match the filter'}
            description="Audit rows are appended on every mutation across capabilities, releases, approvals, eval, audit, and admin actions."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={filtered as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => setOpenId(String(r['id']))}
            columns={[
              {
                key: 'when',
                header: 'When',
                render: (r) => (
                  <span className="font-mono text-xs text-text-muted">
                    {r['createdAt'] ? new Date(String(r['createdAt'])).toLocaleString() : '—'}
                  </span>
                ),
              },
              {
                key: 'action',
                header: 'Action',
                render: (r) => <code className="font-mono text-xs">{String(r['action'] ?? '—')}</code>,
              },
              {
                key: 'resource',
                header: 'Resource',
                render: (r) => (
                  <span className="font-mono text-xs">
                    {String(r['resourceKind'] ?? '—')}
                    {r['resourceId'] ? <span className="text-text-subtle">/{String(r['resourceId']).slice(0, 12)}…</span> : null}
                  </span>
                ),
              },
              {
                key: 'actor',
                header: 'Actor',
                render: (r) => r['actor'] ? <span className="font-mono text-xs">{String(r['actor'])}</span> : '—',
              },
              {
                key: 'hash',
                header: 'Hash',
                render: (r) => r['hash'] ? <HashChip hash={String(r['hash'])} /> : <span className="text-text-subtle">—</span>,
              },
              {
                key: 'verify',
                header: '',
                render: () => (
                  <ThemedTooltip content="Open row to verify chain link">
                    <CheckCircle2 className="size-4 text-text-subtle" />
                  </ThemedTooltip>
                ),
              },
            ]}
          />
        )}
      </Surface>

      <Drawer open={Boolean(open)} onOpenChange={(o) => !o && setOpenId(null)}>
        {open && (
          <DrawerContent
            title={open.action ?? 'event'}
            description={open.createdAt ? new Date(open.createdAt).toLocaleString() : ''}
          >
            <div className="space-y-5">
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <div className="text-xs uppercase tracking-wider text-text-subtle">Resource</div>
                  <div className="mt-1 font-mono text-xs text-text-default">{open.resourceKind ?? '—'}</div>
                </div>
                <div>
                  <div className="text-xs uppercase tracking-wider text-text-subtle">Resource id</div>
                  <div className="mt-1 font-mono text-xs text-text-default">{open.resourceId ?? '—'}</div>
                </div>
                <div>
                  <div className="text-xs uppercase tracking-wider text-text-subtle">Actor</div>
                  <div className="mt-1 font-mono text-xs text-text-default">{open.actor ?? '—'}</div>
                </div>
                <div>
                  <div className="text-xs uppercase tracking-wider text-text-subtle">Action</div>
                  <div className="mt-1 font-mono text-xs text-text-default">{open.action ?? '—'}</div>
                </div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-wider text-text-subtle">Hash</div>
                <div className="mt-1">{open.hash ? <HashChip hash={open.hash} /> : <span className="text-text-subtle">—</span>}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-wider text-text-subtle">Previous hash</div>
                <div className="mt-1">{open.prevHash ? <HashChip hash={open.prevHash} /> : <span className="text-text-subtle">— (genesis)</span>}</div>
              </div>
              {open.details && (
                <div>
                  <div className="text-xs uppercase tracking-wider text-text-subtle">Details</div>
                  <pre className="mt-2 max-h-72 overflow-auto rounded-md bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
                    {open.details}
                  </pre>
                </div>
              )}
              <Button variant="outline" className="w-full" asChild={false}>
                <a href="/api/audit/verify" target="_blank" rel="noreferrer">Run chain verification</a>
              </Button>
            </div>
          </DrawerContent>
        )}
      </Drawer>
    </div>
  );
}