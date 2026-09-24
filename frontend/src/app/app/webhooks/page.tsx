'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Webhook, Power, Trash2 } from 'lucide-react';
import { webhookApi } from '@/lib/api';
import { unwrapList } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { QueryError } from '@/components/brand/query-error';
import { getErrorMessage } from '@/lib/errors';
import { useToast } from '@/components/brand/toast';

interface WebhookItem {
  id: string;
  url?: string;
  events?: string[];
  active?: boolean;
  createdAt?: string;
  lastDeliveredAt?: string | null;
  deliveryCount?: number;
  failureCount?: number;
}

const EVENT_PRESETS = [
  'release.created',
  'release.activated',
  'release.rolled_back',
  'approval.requested',
  'approval.granted',
  'approval.rejected',
  'eval.completed',
  'audit.appended',
];

export default function WebhooksPage() {
  const session = useRequireSession();
  const qc = useQueryClient();
  const { toast } = useToast();

  const hooks = useQuery({
    queryKey: ['webhooks'],
    queryFn: () => webhookApi.list().then((r) => unwrapList<WebhookItem>(r.data, 'webhooks')),
    enabled: Boolean(session),
  });
  const rows = (hooks.data ?? []) as WebhookItem[];

  const [url, setUrl] = useState('');
  const [events, setEvents] = useState<string[]>(['release.activated', 'approval.requested']);

  const create = useMutation({
    mutationFn: () => {
      if (!session) throw new Error('A session is required to add a webhook');
      return webhookApi.create({ organizationId: session.orgId, label: url, url, events });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['webhooks'] });
      setUrl('');
      toast({ title: 'Webhook added', variant: 'success' });
    },
    onError: (error) => toast({ title: 'Could not add webhook', description: getErrorMessage(error), variant: 'destructive' }),
  });

  const toggle = useMutation({
    mutationFn: (item: WebhookItem) => webhookApi.update(item.id, { active: !(item.active ?? false) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['webhooks'] });
      toast({ title: 'Webhook updated', variant: 'success' });
    },
    onError: (error) => toast({ title: 'Could not update webhook', description: getErrorMessage(error), variant: 'destructive' }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => webhookApi.delete(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['webhooks'] });
      toast({ title: 'Webhook deleted', variant: 'success' });
    },
    onError: (error) => toast({ title: 'Could not delete webhook', description: getErrorMessage(error), variant: 'destructive' }),
  });

  if (!session) return null;
  if (hooks.isError) return <QueryError message={hooks.error} onRetry={() => void hooks.refetch()} />;

  const toggleEvent = (ev: string) => {
    setEvents((prev) => prev.includes(ev) ? prev.filter((e) => e !== ev) : [...prev, ev]);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Webhooks"
        subtitle="Outbound webhooks for capability lifecycle events. Each delivery is signed with HMAC and recorded for replay."
      />

      <Surface>
        <SurfaceHeader title="Add a webhook" description="Receives signed POSTs at the URL you specify." />
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label htmlFor="webhook-url" className="text-xs uppercase tracking-wider text-text-subtle">URL</label>
            <Input
              id="webhook-url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://example.com/webhooks/promptsheon"
              className="mt-2"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Events</label>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {EVENT_PRESETS.map((ev) => {
                const active = events.includes(ev);
                return (
                  <button
                    key={ev}
                  type="button"
                  onClick={() => toggleEvent(ev)}
                  aria-pressed={active}
                  aria-label={`${active ? 'Remove' : 'Add'} ${ev} event`}
                    className={`rounded-full border px-2.5 py-0.5 text-xs transition-colors ${
                      active
                        ? 'border-brand bg-brand text-brand-foreground'
                        : 'border-border-subtle bg-surface-1 text-text-muted hover:border-border-strong'
                    }`}
                  >
                    {ev}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
        <div className="mt-4 flex items-center justify-end">
          <Button onClick={() => create.mutate()} disabled={!url || events.length === 0 || create.isPending}>
            <Plus className="mr-1.5 size-3.5" />
            {create.isPending ? 'Adding…' : 'Add webhook'}
          </Button>
        </div>
        {create.isError && (
          <div className="mt-3 text-xs text-destructive">{getErrorMessage(create.error)}</div>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Registered webhooks" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Webhook}
            title="No webhooks configured"
            description="Add a webhook to receive capability lifecycle events with HMAC verification."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              {
                key: 'url',
                header: 'URL',
                render: (r) => <code className="font-mono text-xs">{String(r['url'] ?? '—')}</code>,
              },
              {
                key: 'events',
                header: 'Events',
                render: (r) => {
                  const evs = (r['events'] as string[] | undefined) ?? [];
                  return (
                    <div className="flex flex-wrap gap-1">
                      {evs.slice(0, 3).map((ev) => <Badge key={ev}>{ev}</Badge>)}
                      {evs.length > 3 && <Badge className="bg-surface-3">+{evs.length - 3}</Badge>}
                    </div>
                  );
                },
              },
              {
                key: 'delivery',
                header: 'Delivery',
                render: (r) => {
                  const total = Number(r['deliveryCount'] ?? 0);
                  const fail = Number(r['failureCount'] ?? 0);
                  const last = r['lastDeliveredAt'] ? new Date(String(r['lastDeliveredAt'])).toLocaleString() : 'never';
                  return (
                    <div className="text-xs">
                      <div className="text-text-default">{total} sent · {fail} failed</div>
                      <div className="text-text-subtle">last {last}</div>
                    </div>
                  );
                },
              },
              {
                key: 'active',
                header: 'Active',
                render: (r) => (
                  <Switch
                    checked={Boolean(r['active'])}
                    onCheckedChange={() => toggle.mutate(r as unknown as WebhookItem)}
                  />
                ),
              },
              {
                key: 'actions',
                header: '',
                render: (r) => (
                  <Button size="sm" variant="outline" onClick={() => remove.mutate(String(r['id']))}>
                    <Trash2 className="mr-1 size-3" />
                    Delete
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
