'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound, RefreshCw, Shield } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { useToast } from '@/components/brand/toast';
import { vaultApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export default function VaultPage() {
  const session = useRequireSession();
  const qc = useQueryClient();
  const { toast } = useToast();
  const keys = useQuery({ queryKey: ['vault', 'keys'], queryFn: () => vaultApi.listKeys() });
  const rotate = useMutation({
    mutationFn: () => vaultApi.rotateKey(`key-${Date.now()}`, true),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['vault', 'keys'] });
      toast({ title: 'Key rotated', variant: 'success', description: 'Ciphertext re-encrypted with the new key version.' });
    },
    onError: (err) => toast({ title: 'Rotate failed', variant: 'destructive', description: (err as Error).message }),
  });

  const [name, setName] = useState('OPENAI_API_KEY');
  const [value, setValue] = useState('');

  const write = useMutation({
    mutationFn: () => vaultApi.writeSecret(session!.orgId, name.trim(), value),
    onSuccess: () => {
      setValue('');
      qc.invalidateQueries({ queryKey: ['vault', 'keys'] });
      toast({ title: 'Secret stored', variant: 'success', description: `${name} written to the vault.` });
    },
    onError: (err) => toast({ title: 'Write failed', variant: 'destructive', description: (err as Error).message }),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Vault"
        subtitle="AES-256-GCM at rest with a swappable KMS. Keys rotate via a one-call endpoint; ciphertext re-encrypts by default."
      />

      <Surface padded={false}>
        <div className="flex items-center justify-between border-b border-border-subtle px-5 py-4">
          <SurfaceHeader title="Keyring" description="Each entry is one key version. Exactly one is active." className="mb-0 border-0" />
          <Button onClick={() => rotate.mutate()} disabled={rotate.isPending}>
            <RefreshCw className="mr-1.5 size-3.5" />
            {rotate.isPending ? 'Rotating…' : 'Rotate key'}
          </Button>
        </div>
        {Array.isArray(keys.data) && keys.data.length > 0 ? (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={(keys.data as Array<Record<string, unknown>>)}
            rowKey={(r) => String(r['id'])}
            columns={[
              { key: 'label', header: 'Label', render: (r) => String(r['label']) },
              { key: 'fingerprint', header: 'Fingerprint', render: (r) => <span className="font-mono text-xs">{String(r['fingerprint']).slice(0, 24)}…</span> },
              {
                key: 'active',
                header: 'Active',
                render: (r) => (r['active']
                  ? <Badge className="bg-success/15 text-success">active</Badge>
                  : <span className="text-text-subtle text-xs">—</span>),
              },
              {
                key: 'created',
                header: 'Created',
                render: (r) => new Date(String(r['createdAt'])).toLocaleString(),
              },
              {
                key: 'rotated',
                header: 'Rotated',
                render: (r) => r['rotatedAt'] ? new Date(String(r['rotatedAt'])).toLocaleString() : '—',
              },
            ]}
            empty={
              <EmptyState
                icon={KeyRound}
                title="No keys yet"
                description="Rotate to create the first key version."
                className="m-5 border-0 bg-transparent shadow-none p-12"
              />
            }
          />
        ) : (
          <div className="p-5 text-sm text-text-muted">
            {keys.isLoading ? 'Loading keyring…' : 'No keys yet. Rotate to create the first key version.'}
          </div>
        )}
      </Surface>

      <Surface>
        <SurfaceHeader title="Write a secret" description="Stored encrypted; the plaintext never returns through the API." />
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} className="mt-2" />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Value</label>
            <Input
              value={value}
              onChange={(e) => setValue(e.target.value)}
              type="password"
              className="mt-2 font-mono"
              autoComplete="off"
            />
          </div>
        </div>
        <div className="mt-4 flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-text-subtle">
            <Shield className="size-3.5" />
            <span>Wrapped in AES-256-GCM with the active key. Reads fall back to older key versions.</span>
          </div>
          <Button
            onClick={() => write.mutate()}
            disabled={!name.trim() || !value || write.isPending}
          >
            <KeyRound className="mr-1.5 size-3.5" />
            {write.isPending ? 'Writing…' : 'Write secret'}
          </Button>
        </div>
      </Surface>
    </div>
  );
}