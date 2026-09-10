'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, ShieldAlert } from 'lucide-react';
import { preconditionApi, versionApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { useToast } from '@/components/brand/toast';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

interface PreconditionRow {
  id: string;
  name: string;
  command: string;
  enabled: boolean;
}

interface VersionRow {
  id: string;
  version?: number;
  createdAt?: string;
}

export default function PreconditionsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const session = useRequireSession();
  const qc = useQueryClient();
  const { toast } = useToast();

  const data = useQuery({
    queryKey: ['preconditions', capabilityId],
    queryFn: () => preconditionApi.list(capabilityId!).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });

  const versions = useQuery({
    queryKey: ['versions', capabilityId],
    queryFn: () => versionApi.list(capabilityId!).then((r) => r.data).catch(() => [] as VersionRow[]),
    enabled: Boolean(capabilityId) && Boolean(session),
  });
  const latestVersionId = useMemo(() => {
    const list = (Array.isArray(versions.data) ? versions.data : []) as VersionRow[];
    return list[0]?.id;
  }, [versions.data]);

  const rows = (Array.isArray(data.data) ? data.data : []) as PreconditionRow[];

  const [name, setName] = useState('');
  const [command, setCommand] = useState('');

  const create = useMutation({
    mutationFn: () => {
      if (!latestVersionId) throw new Error('Compile at least one capability version first.');
      return preconditionApi.create({
        capabilityVersionId: latestVersionId,
        name: name.trim(),
        command: command.trim(),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['preconditions', capabilityId] });
      setName('');
      setCommand('');
      toast({ title: 'Precondition created', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Create failed', variant: 'destructive', description: (err as Error).message }),
  });

  const toggle = useMutation({
    mutationFn: (row: PreconditionRow) =>
      preconditionApi.update(row.id, { enabled: !row.enabled }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['preconditions', capabilityId] });
      toast({ title: 'Precondition updated', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Update failed', variant: 'destructive', description: (err as Error).message }),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capability"
        title="Preconditions"
        subtitle="Shell guards that must pass before this capability version is invocable. Toggle on/off per row."
      />

      <Surface>
        <SurfaceHeader
          title="New precondition"
          description="Runs before capability invocation. Failure blocks the call."
        />
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="PG reachable"
              className="mt-2"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Command</label>
            <Input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              placeholder="pg_isready -h $DB_HOST"
              mono
              className="mt-2"
            />
          </div>
        </div>
        <div className="mt-4 flex items-center justify-between">
          <div className="text-xs text-text-subtle">
            {latestVersionId
              ? `Preconditions attach to capabilityVersionId ${latestVersionId.slice(0, 8)}… (latest).`
              : 'Compile a capability version first; preconditions attach to a version.'}
          </div>
          <Button
            onClick={() => create.mutate()}
            disabled={!name.trim() || !command.trim() || !latestVersionId || create.isPending}
          >
            <Plus className="mr-1.5 size-3.5" />
            {create.isPending ? 'Creating…' : 'Create precondition'}
          </Button>
        </div>
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Configured" description={`${rows.length} precondition(s)`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={ShieldAlert}
            title="No preconditions yet"
            description="Create one above to gate invocations on a shell check."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              {
                key: 'name',
                header: 'Name',
                render: (r) => <span className="font-medium text-text-strong">{String(r['name'])}</span>,
              },
              {
                key: 'command',
                header: 'Command',
                render: (r) => <code className="font-mono text-xs text-text-muted">{String(r['command'])}</code>,
              },
              {
                key: 'enabled',
                header: 'Enabled',
                render: (r) => (
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={Boolean(r['enabled'])}
                      onCheckedChange={() => toggle.mutate(r as unknown as PreconditionRow)}
                    />
                    <StatusPill
                      kind={r['enabled'] ? 'approved' : 'neutral'}
                      label={r['enabled'] ? 'On' : 'Off'}
                    />
                  </div>
                ),
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}