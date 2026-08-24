'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Plus, Shield } from 'lucide-react';
import { apiKeyApi, userApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

interface ApiKey {
  id: string;
  userId: string;
  name: string;
  keyPrefix: string;
  role: string;
  expiresAt: string | null;
  lastUsed: string | null;
  createdAt: string;
  revoked: boolean;
}

export default function ApiKeysPage() {
  const session = useRequireSession();
  const qc = useQueryClient();
  const keys = useQuery({
    queryKey: ['api-keys'],
    queryFn: async () => {
      const r = await apiKeyApi.list();
      return (r.data as unknown as { keys: ApiKey[] }) ?? { keys: [] };
    },
  });
  const me = useQuery({
    queryKey: ['me'],
    queryFn: async () => {
      const r = await userApi.me();
      return r.data as unknown as { user: { id: string; email: string; name: string; role: string } };
    },
  });
  const [name, setName] = useState('');
  const [role, setRole] = useState<string>('reader');
  const [issued, setIssued] = useState<{ key: string; id: string; name: string } | null>(null);

  const create = useMutation({
    mutationFn: () => apiKeyApi.create({ name: name || 'untitled', role }),
    onSuccess: async (resp) => {
      const o = resp.data as unknown as { key: string; id: string; name: string };
      setIssued(o);
      setName('');
      void qc.invalidateQueries({ queryKey: ['api-keys'] });
    },
  });

  const revoke = useMutation({
    mutationFn: (id: string) => apiKeyApi.revoke(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  });

  if (!session) return null;
  const rows = keys.data?.keys ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="API keys"
        subtitle="Org-scoped bearer tokens. Issued in plaintext exactly once; only the SHA-256 fingerprint is stored. The new key immediately takes effect for the SDK and CLI."
      />

      <Surface>
        <SurfaceHeader
          title="Issue a key"
          description={me.data ? `as ${me.data.user.email}` : 'as the current actor'}
        />
        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-2"
              placeholder="ci-server"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Role</label>
            <div className="mt-2">
              <ThemedSelect
                value={role}
                onValueChange={setRole}
                options={[
                  { value: 'reader', label: 'reader' },
                  { value: 'editor', label: 'editor' },
                  { value: 'approver', label: 'approver' },
                  { value: 'admin', label: 'admin' },
                  { value: 'system', label: 'system' },
                ]}
                ariaLabel="API key role"
                triggerClassName="w-full"
              />
            </div>
          </div>
          <div className="flex items-end">
            <Button onClick={() => create.mutate()} disabled={create.isPending} className="w-full">
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              Issue
            </Button>
          </div>
        </div>
        {issued && (
          <div className="mt-4 rounded-lg border border-warning/30 bg-warning/10 p-3">
            <div className="text-sm font-semibold text-warning">Save this key — it will not be shown again.</div>
            <pre className="mt-2 overflow-x-auto rounded-md bg-surface-0 p-3 font-mono text-xs text-text-default">
{issued.key}
            </pre>
            <div className="mt-2 text-xs text-text-muted">
              Use as <code className="rounded bg-surface-2 px-1 py-0.5">Authorization: Bearer {issued.key}</code>
            </div>
          </div>
        )}
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Active keys" description="Revoked keys are kept for historical verification." />
        {rows.length === 0 ? (
          <div className="px-5 pb-5 text-text-muted text-sm">No keys yet.</div>
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              { key: 'name', header: 'Name', render: (r) => String(r['name']) },
              {
                key: 'prefix',
                header: 'Prefix',
                render: (r) => <span className="font-mono text-xs">{String(r['keyPrefix'])}…</span>,
              },
              { key: 'role', header: 'Role', render: (r) => <Badge>{String(r['role'])}</Badge> },
              {
                key: 'last',
                header: 'Last used',
                render: (r) => r['lastUsed'] ? new Date(String(r['lastUsed'])).toLocaleString() : '—',
              },
              {
                key: 'created',
                header: 'Created',
                render: (r) => new Date(String(r['createdAt'])).toLocaleString(),
              },
              {
                key: 'state',
                header: 'State',
                render: (r) => r['revoked']
                  ? <Badge className="bg-surface-3 text-text-muted">revoked</Badge>
                  : <Badge className="bg-success/15 text-success">active</Badge>,
              },
              {
                key: 'actions',
                header: '',
                render: (r) => r['revoked'] ? null : (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => revoke.mutate(String(r['id']))}
                  >
                    <KeyRound className="mr-1 h-3 w-3" />
                    Revoke
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Surface>

      <Surface>
        <div className="flex items-center gap-3 text-xs text-text-subtle">
          <Shield className="h-3.5 w-3.5" />
          <span>Tokens are sha256-hashed; only the prefix is stored. Use a fresh token per CI runner.</span>
        </div>
      </Surface>
    </div>
  );
}
