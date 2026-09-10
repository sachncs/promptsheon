'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bell, Plus, Trash2 } from 'lucide-react';
import { alertApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';

interface AlertRule {
  id: string;
  name?: string;
  type?: string;
  severity?: string;
  threshold?: number;
  window?: number;
  enabled?: boolean;
  createdAt?: string;
}

const TYPE_OPTIONS = [
  { value: 'eval-regression', label: 'Eval regression' },
  { value: 'latency', label: 'Latency' },
  { value: 'error-rate', label: 'Error rate' },
  { value: 'approval-window', label: 'Approval window' },
  { value: 'canary-drift', label: 'Canary drift' },
] as const;

const SEVERITY_OPTIONS = ['info', 'warning', 'critical'] as const;

export default function AlertRulesPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const rules = useQuery({
    queryKey: ['alert-rules'],
    queryFn: () => alertApi.listRules().then((r) => r.data).catch(() => [] as AlertRule[]),
  });
  const rows = (rules.data ?? []) as AlertRule[];

  const [name, setName] = useState('');
  const [type, setType] = useState<typeof TYPE_OPTIONS[number]['value']>('eval-regression');
  const [severity, setSeverity] = useState<typeof SEVERITY_OPTIONS[number]>('warning');
  const [threshold, setThreshold] = useState('0.85');
  const [window, setWindow] = useState('120');

  const create = useMutation({
    mutationFn: () => {
      const payload: { name: string; type: string; severity: string; threshold?: number; window?: number } = {
        name,
        type,
        severity,
      };
      const t = Number(threshold);
      if (!Number.isNaN(t)) payload.threshold = t;
      const w = Number(window);
      if (!Number.isNaN(w)) payload.window = w;
      return alertApi.createRule(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['alert-rules'] });
      setName('');
      setThreshold('0.85');
      setWindow('120');
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => alertApi.deleteRule(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alert-rules'] }),
  });

  if (!session) return null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Release"
        title="Alert rules"
        subtitle="Define when alerts fire: eval regression thresholds, latency, error rate, approval-window breaches."
      />

      <Surface>
        <SurfaceHeader title="New rule" description="Fires once per window when condition holds." />
        <div className="grid gap-3 sm:grid-cols-5">
          <div className="sm:col-span-2">
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="eval-score-drop"
              className="mt-2 font-mono"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Type</label>
            <div className="mt-2">
              <ThemedSelect
                value={type}
                onValueChange={(v) => setType(v as typeof TYPE_OPTIONS[number]['value'])}
                options={TYPE_OPTIONS.map((t) => ({ value: t.value, label: t.label }))}
                ariaLabel="Alert type"
                triggerClassName="w-full"
              />
            </div>
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Severity</label>
            <div className="mt-2">
              <ThemedSelect
                value={severity}
                onValueChange={(v) => setSeverity(v as typeof SEVERITY_OPTIONS[number])}
                options={SEVERITY_OPTIONS.map((s) => ({ value: s, label: s }))}
                ariaLabel="Severity"
                triggerClassName="w-full"
              />
            </div>
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Threshold</label>
            <Input
              value={threshold}
              onChange={(e) => setThreshold(e.target.value)}
              className="mt-2 font-mono"
            />
          </div>
        </div>
        <div className="mt-3 grid gap-3 sm:grid-cols-5">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Window (s)</label>
            <Input
              value={window}
              onChange={(e) => setWindow(e.target.value)}
              className="mt-2 font-mono"
            />
          </div>
          <div className="col-span-4 flex items-end justify-end">
            <Button onClick={() => create.mutate()} disabled={!name || create.isPending}>
              <Plus className="mr-1.5 size-3.5" />
              {create.isPending ? 'Creating…' : 'Create rule'}
            </Button>
          </div>
        </div>
        {create.isError && (
          <div className="mt-3 text-xs text-destructive">{(create.error as Error).message}</div>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Configured rules" description={`${rows.length} rule(s)`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Bell}
            title="No alert rules yet"
            description="Define a rule — for example, fail release activation when eval score drops below 0.85 for two consecutive runs."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              { key: 'name', header: 'Name', render: (r) => <code className="font-mono text-xs">{String(r['name'] ?? '—')}</code> },
              { key: 'type', header: 'Type', render: (r) => <Badge>{String(r['type'] ?? '—')}</Badge> },
              { key: 'severity', header: 'Severity', render: (r) => <Badge>{String(r['severity'] ?? '—')}</Badge> },
              {
                key: 'threshold',
                header: 'Threshold',
                render: (r) => r['threshold'] !== undefined ? <span className="font-mono text-xs">{String(r['threshold'])}</span> : '—',
              },
              {
                key: 'window',
                header: 'Window',
                render: (r) => r['window'] !== undefined ? `${String(r['window'])}s` : '—',
              },
              {
                key: 'created',
                header: 'Created',
                render: (r) => r['createdAt'] ? new Date(String(r['createdAt'])).toLocaleDateString() : '—',
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