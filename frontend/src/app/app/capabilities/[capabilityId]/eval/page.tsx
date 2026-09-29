'use client';

import { useParams, useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import * as React from 'react';
import { evalApi, releaseApi, datasetApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { StatusPill } from '@/components/brand/status-pill';
import { Progress } from '@/components/ui/progress';
import { EmptyState } from '@/components/brand/empty-state';
import { FlaskConical } from 'lucide-react';
import { QueryError } from '@/components/brand/query-error';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useToast } from '@/components/brand/toast';
import { getErrorMessage } from '@/lib/errors';

export default function EvalRunsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const session = useRequireSession();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const releases = useQuery({
    queryKey: ['releases', capabilityId],
    queryFn: () => releaseApi.list(capabilityId).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });
  const datasets = useQuery({
    queryKey: ['datasets', capabilityId],
    queryFn: () => datasetApi.list(capabilityId).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });
  const evaluators = useQuery({
    queryKey: ['eval-evaluators'],
    queryFn: () => evalApi.evaluators().then((r) => r.data),
    enabled: Boolean(session),
  });

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['eval-runs', capabilityId],
    queryFn: async () => {
      const releaseRows = releases.data ?? [];
      const grouped = await Promise.all(releaseRows.map((release) => evalApi.list(release.id).then((r) => r.data)));
      return grouped.flat().sort((a, b) => b.startedAt.localeCompare(a.startedAt));
    },
    enabled: Boolean(capabilityId) && Boolean(session) && releases.isSuccess,
  });

  const [releaseId, setReleaseId] = React.useState('');
  const [datasetId, setDatasetId] = React.useState('');
  const [scorer, setScorer] = React.useState('deterministic');
  const [actualUrl, setActualUrl] = React.useState('');
  const selectedReleaseId = releaseId || releases.data?.[0]?.id || '';
  const selectedDatasetId = datasetId || (Array.isArray(datasets.data) ? datasets.data[0]?.id : '') || '';
  const selectedScorer = scorer || evaluators.data?.[0] || 'deterministic';

  const createRun = useMutation({
    mutationFn: async () => {
      const created = await evalApi.create({ releaseId: selectedReleaseId, datasetId: selectedDatasetId, scorer: selectedScorer });
      if (!actualUrl.trim()) return created.data;
      return (await evalApi.run({ evalRunId: created.data.id, getActualUrl: actualUrl.trim() })).data;
    },
    onSuccess: (run) => {
      queryClient.invalidateQueries({ queryKey: ['eval-runs', capabilityId] });
      toast({ title: run.status === 'running' ? 'Eval run created' : 'Eval run complete', variant: 'success' });
      router.push(`/app/eval/${run.id}`);
    },
    onError: (err) => toast({ title: 'Eval run failed', description: getErrorMessage(err), variant: 'destructive' }),
  });

  const rows = (Array.isArray(data) ? data : []) as Array<{
    id: string;
    scorer: string;
    score: number;
    status: string;
    startedAt: string;
  }>;

  if (!session) return null;

  if (isError) {
    return <QueryError message={error} onRetry={() => void refetch()} />;
  }

  const releaseRows = releases.data ?? [];
  const datasetRows = Array.isArray(datasets.data) ? datasets.data as Array<{ id: string; name: string }> : [];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capability"
        title="Eval runs"
        subtitle="Run-on-save evals against this capability. Score is the per-case mean across the suite."
      />

      <Surface>
        <SurfaceHeader title="Run an evaluation" description="Use the deterministic scorer locally, or connect a reachable agent endpoint for a live run." />
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="Release">
            <Select value={selectedReleaseId} onValueChange={setReleaseId}>
              <SelectTrigger><SelectValue placeholder="Select a release" /></SelectTrigger>
              <SelectContent>{releaseRows.map((release) => <SelectItem key={release.id} value={release.id}>v{release.capabilityVersion} · {release.environment} · {release.status}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Dataset">
            <Select value={selectedDatasetId} onValueChange={setDatasetId}>
              <SelectTrigger><SelectValue placeholder="Select a dataset" /></SelectTrigger>
              <SelectContent>{datasetRows.map((dataset) => <SelectItem key={dataset.id} value={dataset.id}>{dataset.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Scorer">
            <Select value={selectedScorer} onValueChange={setScorer}>
              <SelectTrigger><SelectValue placeholder="Select a scorer" /></SelectTrigger>
              <SelectContent>{(evaluators.data ?? ['deterministic']).map((name) => <SelectItem key={name} value={name}>{name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Agent endpoint">
            <Input value={actualUrl} onChange={(event) => setActualUrl(event.target.value)} placeholder="https://your-agent.example/evaluate" />
          </Field>
        </div>
        <div className="mt-4 flex items-center justify-between gap-3">
          <p className="text-xs text-text-muted">The endpoint receives each case input and must return the agent output. A run needs at least one dataset case.</p>
          <Button onClick={() => createRun.mutate()} disabled={!selectedReleaseId || !selectedDatasetId || !selectedScorer || !actualUrl.trim() || createRun.isPending}>
            {createRun.isPending ? 'Running…' : 'Create and run'}
          </Button>
        </div>
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Recent runs" description={`${rows.length} recorded`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={FlaskConical}
            title="No eval runs yet"
            description={isLoading ? 'Loading…' : 'Activate a release to start collecting eval runs.'}
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => r.id}
            onRowClick={(r) => router.push(`/app/eval/${r.id}`)}
            columns={[
              { key: 'id', header: 'Run', render: (r) => <span className="font-mono text-xs">{r.id.slice(0, 12)}…</span> },
              { key: 'scorer', header: 'Scorer', render: (r) => r.scorer || '—' },
              {
                key: 'score',
                header: 'Score',
                render: (r) => {
                  const s = r.score;
                  return (
                    <div className="flex items-center gap-2 w-40">
                      <Progress value={s * 100} />
                      <span className="text-xs text-text-muted">{(s * 100).toFixed(0)}%</span>
                    </div>
                  );
                },
              },
              {
                key: 'status',
                header: 'Status',
                render: (r) => (
                  <StatusPill
                    kind={r.status === 'passed' ? 'active' : r.status === 'failed' ? 'rejected' : 'review'}
                    label={r.status}
                  />
                ),
              },
              {
                key: 'started',
                header: 'Started',
                render: (r) => r.startedAt ? new Date(r.startedAt).toLocaleString() : '—',
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="space-y-2 text-xs uppercase tracking-wider text-text-subtle"><span>{label}</span>{children}</label>;
}
