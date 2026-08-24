'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Cog, Save } from 'lucide-react';
import { settingsApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';

interface SettingItem {
  key: string;
  value?: unknown;
  description?: string;
  updatedAt?: string;
  updatedBy?: string;
}

const KNOWN_KEYS: Array<{ key: string; label: string; description: string; placeholder: string }> = [
  { key: 'llm.provider', label: 'LLM provider', description: 'openai / anthropic / bedrock', placeholder: 'openai' },
  { key: 'llm.model', label: 'LLM model', description: 'Model identifier passed to the provider.', placeholder: 'claude-3-5-sonnet' },
  { key: 'audit.retentionDays', label: 'Audit retention (days)', description: 'How long to keep audit rows. The chain itself is never swept.', placeholder: '365' },
  { key: 'release.canaryDefaultPercent', label: 'Canary default (%)', description: 'Initial traffic split for new canary releases.', placeholder: '5' },
  { key: 'release.canaryMaxPercent', label: 'Canary cap (%)', description: 'Maximum canary percent before activation is required.', placeholder: '50' },
  { key: 'eval.defaultThreshold', label: 'Eval gate threshold', description: 'Score below which a release is blocked from promotion.', placeholder: '0.85' },
  { key: 'observability.otelEndpoint', label: 'OTel endpoint', description: 'OTLP collector URL. Leave blank to disable.', placeholder: 'http://otel:4317' },
];

export default function SettingsPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const settings = useQuery({
    queryKey: ['settings'],
    queryFn: () => settingsApi.list().then((r) => r.data).catch(() => ({ settings: [] as SettingItem[] })),
  });

  const list = ((settings.data as { settings?: SettingItem[] } | undefined)?.settings ?? []) as SettingItem[];
  const known = KNOWN_KEYS.map((k) => {
    const found = list.find((s) => s.key === k.key);
    return { ...k, current: found?.value, updatedAt: found?.updatedAt };
  });
  const extras = list.filter((s) => !KNOWN_KEYS.some((k) => k.key === s.key));

  const [draft, setDraft] = useState<Record<string, string>>({});

  useEffect(() => {
    const next: Record<string, string> = {};
    for (const k of known) {
      if (k.current !== undefined && k.current !== null) next[k.key] = String(k.current);
      else next[k.key] = '';
    }
    setDraft(next);
  }, [list.map((s) => `${s.key}:${String(s.value ?? '')}`).join('|')]);

  const save = useMutation({
    mutationFn: ({ key, value }: { key: string; value: unknown }) => settingsApi.set(key, value),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['settings'] }),
  });

  if (!session) return null;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Settings"
        subtitle="Platform-wide configuration. Defaults come from environment variables; override them here at runtime."
      />

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Configuration"
          description="Each setting flows through the same Zod-validated API; changes take effect on the next request."
        />
        <div className="divide-y divide-border-subtle">
          {known.map((k) => {
            const dirty = (draft[k.key] ?? '') !== (k.current !== undefined && k.current !== null ? String(k.current) : '');
            return (
              <div key={k.key} className="grid grid-cols-12 items-center gap-4 px-5 py-4">
                <div className="col-span-4">
                  <div className="text-sm font-medium text-text-strong">{k.label}</div>
                  <code className="font-mono text-xs text-text-subtle">{k.key}</code>
                </div>
                <div className="col-span-5">
                  <Input
                    value={draft[k.key] ?? ''}
                    onChange={(e) => setDraft((prev) => ({ ...prev, [k.key]: e.target.value }))}
                    placeholder={k.placeholder}
                    className="font-mono text-sm"
                  />
                  <p className="mt-1 text-xs text-text-muted">{k.description}</p>
                </div>
                <div className="col-span-2 text-xs text-text-subtle">
                  {k.updatedAt ? new Date(k.updatedAt).toLocaleDateString() : '—'}
                </div>
                <div className="col-span-1 flex justify-end">
                  <Button
                    size="sm"
                    variant={dirty ? 'default' : 'outline'}
                    disabled={!dirty || save.isPending}
                    onClick={() => {
                      const raw = draft[k.key] ?? '';
                      let parsed: unknown = raw;
                      const num = Number(raw);
                      if (!Number.isNaN(num) && raw.trim() !== '' && /^-?\d+(\.\d+)?$/.test(raw.trim())) {
                        parsed = num;
                      }
                      save.mutate({ key: k.key, value: parsed });
                    }}
                  >
                    <Save className="size-3" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      </Surface>

      {extras.length > 0 && (
        <Surface padded={false}>
          <SurfaceHeader className="px-5 pt-5" title="Other settings" description={`${extras.length} not surfaced in this UI.`} />
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={extras as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['key'])}
            columns={[
              { key: 'key', header: 'Key', render: (r) => <code className="font-mono text-xs">{String(r['key'])}</code> },
              { key: 'value', header: 'Value', render: (r) => <code className="font-mono text-xs">{String(JSON.stringify(r['value']))}</code> },
              { key: 'when', header: 'Updated', render: (r) => r['updatedAt'] ? new Date(String(r['updatedAt'])).toLocaleString() : '—' },
            ]}
          />
        </Surface>
      )}

      <Surface>
        <div className="flex items-start gap-3 text-xs text-text-subtle">
          <Cog className="mt-0.5 size-3.5 shrink-0" />
          <div>
            Settings are stored per-organisation and validated by Zod. Bad values are rejected by the API
            with a typed error message; the UI never persists malformed config.
            <Badge className="ml-2">Zod-validated</Badge>
          </div>
        </div>
      </Surface>
    </div>
  );
}