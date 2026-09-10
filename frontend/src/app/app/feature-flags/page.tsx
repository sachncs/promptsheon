'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Flag, Plus, Save, Trash2 } from 'lucide-react';
import { featureFlagApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';

interface FlagItem {
  key: string;
  value?: unknown;
  enabled?: boolean;
  description?: string;
  updatedAt?: string;
  updatedBy?: string;
}

export default function FeatureFlagsPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const flags = useQuery({
    queryKey: ['feature-flags'],
    queryFn: () => featureFlagApi.list().then((r) => r.data).catch(() => [] as FlagItem[]),
  });
  const rows = (flags.data ?? []) as FlagItem[];

  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('true');
  const [editing, setEditing] = useState<Record<string, string>>({});

  const upsert = useMutation({
    mutationFn: (input: { key: string; value: unknown; enabled?: boolean }) => {
      const payload: { value: unknown; enabled?: boolean } = { value: input.value };
      if (input.enabled !== undefined) payload.enabled = input.enabled;
      return featureFlagApi.update(input.key, payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['feature-flags'] });
      setNewKey('');
      setNewValue('true');
      setEditing({});
    },
  });

  const toggle = useMutation({
    mutationFn: (key: string) => {
      const current = rows.find((r) => r.key === key);
      const next = !(current?.enabled ?? false);
      return featureFlagApi.update(key, { value: current?.value, enabled: next });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['feature-flags'] }),
  });

  if (!session) return null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capabilities"
        title="Feature flags"
        subtitle="Capability-level feature flags. Toggle which DAG branches run for which users; promote features safely behind an environment rollout."
      />

      <Surface>
        <SurfaceHeader title="New flag" description="Boolean or JSON values. Update PUT /api/feature-flags/:key with the new value to flip rollout state." />
        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Key</label>
            <Input
              value={newKey}
              onChange={(e) => setNewKey(e.target.value)}
              placeholder="enable-refund-fast-path"
              className="mt-2 font-mono"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Value (JSON or bool)</label>
            <Input
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
              placeholder='true | false | {"percent": 25}'
              className="mt-2 font-mono"
            />
          </div>
          <div className="flex items-end">
            <Button
              onClick={() => {
                let parsed: unknown = newValue;
                try {
                  if (newValue === 'true') parsed = true;
                  else if (newValue === 'false') parsed = false;
                  else parsed = JSON.parse(newValue);
                } catch {
                  parsed = newValue;
                }
                upsert.mutate({ key: newKey, value: parsed, enabled: true });
              }}
              disabled={!newKey || upsert.isPending}
              className="w-full"
            >
              <Plus className="mr-1.5 size-3.5" />
              {upsert.isPending ? 'Saving…' : 'Create flag'}
            </Button>
          </div>
        </div>
        {upsert.isError && (
          <div className="mt-3 text-xs text-destructive">{(upsert.error as Error).message}</div>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Flags" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Flag}
            title="No flags yet"
            description="Create a flag to gate risky capability paths behind an environment rollout."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['key'])}
            columns={[
              {
                key: 'key',
                header: 'Key',
                render: (r) => <code className="font-mono text-xs">{String(r['key'])}</code>,
              },
              {
                key: 'value',
                header: 'Value',
                render: (r) => {
                  const k = String(r['key']);
                  const isEditing = k in editing;
                  const value = isEditing ? editing[k] : JSON.stringify(r['value']);
                  return (
                    <div className="flex items-center gap-2">
                      <Input
                        value={value}
                        onChange={(e) => setEditing((prev) => ({ ...prev, [k]: e.target.value }))}
                        className="h-7 max-w-48 font-mono text-xs"
                      />
                      {isEditing && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            const raw = editing[k] ?? '';
                            let parsed: unknown = raw;
                            try { parsed = JSON.parse(raw); } catch { /* keep string */ }
                            upsert.mutate({ key: k, value: parsed });
                          }}
                        >
                          <Save className="size-3" />
                        </Button>
                      )}
                    </div>
                  );
                },
              },
              {
                key: 'enabled',
                header: 'Enabled',
                render: (r) => (
                  <Switch
                    checked={Boolean(r['enabled'])}
                    onCheckedChange={() => toggle.mutate(String(r['key']))}
                  />
                ),
              },
              {
                key: 'updated',
                header: 'Updated',
                render: (r) => {
                  const v = r['updatedAt'];
                  return v ? new Date(String(v)).toLocaleString() : '—';
                },
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}