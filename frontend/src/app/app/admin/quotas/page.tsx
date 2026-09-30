'use client';

import { useState } from 'react';
import { Gauge, ArrowLeft, Users } from 'lucide-react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRequireSession } from '@/hooks/use-session';
import { userQuotaApi, type UserQuota } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { EmptyState } from '@/components/brand/empty-state';
import { Field } from '@/components/brand/field';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { StatusPill } from '@/components/brand/status-pill';
import { QueryError } from '@/components/brand/query-error';

const nullableLimit = (value: string): number | null => value.trim() === '' ? null : Number(value);

export default function UserQuotasPage() {
  const session = useRequireSession();
  const queryClient = useQueryClient();
  const [userId, setUserId] = useState('');
  const [label, setLabel] = useState('');
  const [dailyRuns, setDailyRuns] = useState('');
  const [dailyTokens, setDailyTokens] = useState('');
  const [dailyCost, setDailyCost] = useState('');
  const quotas = useQuery({
    queryKey: ['user-quotas', session?.orgId],
    queryFn: () => userQuotaApi.list(session!.orgId),
    enabled: Boolean(session),
  });
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['user-quotas', session?.orgId] });
  const create = useMutation({
    mutationFn: () => userQuotaApi.create({
      organizationId: session!.orgId,
      userId: userId.trim(),
      label: label.trim(),
      dailyRuns: nullableLimit(dailyRuns),
      dailyTokens: nullableLimit(dailyTokens),
      dailyCostMicros: nullableLimit(dailyCost),
      enabled: true,
    }),
    onSuccess: async () => { setUserId(''); setLabel(''); setDailyRuns(''); setDailyTokens(''); setDailyCost(''); await refresh(); },
  });
  const update = useMutation({ mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => userQuotaApi.update(id, { enabled }), onSuccess: refresh });
  const remove = useMutation({ mutationFn: userQuotaApi.remove, onSuccess: refresh });

  if (!session) return null;
  if (quotas.isError) return <QueryError message={quotas.error} onRetry={() => void quotas.refetch()} />;
  const items = quotas.data?.items ?? [];
  return <div className="space-y-6">
    <Link href="/app/admin/analytics" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"><ArrowLeft className="h-3 w-3" /> Per-user analytics</Link>
    <PageHeader eyebrow="Governance" title="User quotas" subtitle="Persist daily execution limits per user. Blank limits remain unlimited; blocked submissions return a clear quota error." />
    <Surface><SurfaceHeader title="Add a quota" description="Runs are enforced before durable execution enqueue. Token and spend usage remain visible from attributed traces." /><div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <Field label="User ID" htmlFor="quota-user" required><Input id="quota-user" value={userId} onChange={(event) => setUserId(event.target.value)} placeholder="user id" /></Field>
      <Field label="Label" htmlFor="quota-label" required><Input id="quota-label" value={label} onChange={(event) => setLabel(event.target.value)} placeholder="Researcher daily limit" /></Field>
      <Field label="Daily runs" htmlFor="quota-runs"><Input id="quota-runs" type="number" min="1" value={dailyRuns} onChange={(event) => setDailyRuns(event.target.value)} placeholder="Unlimited" /></Field>
      <Field label="Daily tokens" htmlFor="quota-tokens"><Input id="quota-tokens" type="number" min="1" value={dailyTokens} onChange={(event) => setDailyTokens(event.target.value)} placeholder="Unlimited" /></Field>
      <Field label="Daily cost (microdollars)" htmlFor="quota-cost"><Input id="quota-cost" type="number" min="1" value={dailyCost} onChange={(event) => setDailyCost(event.target.value)} placeholder="Unlimited" /></Field>
      <div className="flex items-end"><Button onClick={() => create.mutate()} disabled={create.isPending || !userId.trim() || !label.trim()}>{create.isPending ? 'Adding…' : 'Add quota'}</Button></div>
    </div>{create.isError && <p role="alert" className="mt-3 text-sm text-destructive">Unable to add this quota. Check that the user does not already have a policy.</p>}</Surface>
    <Surface padded={false}><SurfaceHeader className="px-5 pt-5" title="Active policies" description={`${items.length} user quota${items.length === 1 ? '' : 's'} in this organization.`} />{items.length === 0 ? <EmptyState className="m-5 border-0 bg-transparent shadow-none p-12" icon={Gauge} title="No user quotas" description="Add a policy when a user needs a durable daily execution boundary." /> : <ul className="divide-y divide-border-subtle">{items.map((quota) => <QuotaRow key={quota.id} quota={quota} onToggle={() => update.mutate({ id: quota.id, enabled: !quota.enabled })} onRemove={() => { if (window.confirm(`Delete quota “${quota.label}”?`)) remove.mutate(quota.id); }} />)}</ul>}</Surface>
  </div>;
}

function QuotaRow({ quota, onToggle, onRemove }: { quota: UserQuota; onToggle: () => void; onRemove: () => void }) {
  return <li className="flex flex-wrap items-center justify-between gap-4 px-5 py-4"><div className="flex min-w-0 items-start gap-3"><Users className="mt-1 h-4 w-4 shrink-0 text-text-muted" /><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><span className="font-medium text-text-strong">{quota.label}</span><StatusPill kind={quota.enabled ? 'active' : 'neutral'} label={quota.enabled ? 'enabled' : 'disabled'} /></div><p className="mt-1 truncate text-xs text-text-muted">{quota.userId} · {quota.usage.runs}/{quota.dailyRuns ?? '∞'} runs · {quota.usage.tokens.toLocaleString()}/{quota.dailyTokens ?? '∞'} tokens · {quota.dailyCostMicros ? `$${(quota.usage.costMicros / 1_000_000).toFixed(2)}/$${(quota.dailyCostMicros / 1_000_000).toFixed(2)}` : '∞'} / day</p></div></div><div className="flex gap-2"><Button size="sm" variant="outline" onClick={onToggle}>{quota.enabled ? 'Disable' : 'Enable'}</Button><Button size="sm" variant="ghost" onClick={onRemove}>Delete</Button></div></li>;
}
