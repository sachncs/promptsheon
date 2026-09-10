'use client';

import * as React from 'react';
import { useParams } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import { datasetApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { useToast } from '@/components/brand/toast';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

interface DatasetSummary {
  id: string;
  name: string;
  description?: string;
  createdAt?: string;
  caseCount?: number;
}

export default function DatasetsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const session = useRequireSession();
  const qc = useQueryClient();
  const { toast } = useToast();

  const datasets = useQuery({
    queryKey: ['datasets', capabilityId],
    queryFn: () => datasetApi.list(capabilityId!).then((r) => r.data),
    enabled: Boolean(capabilityId) && Boolean(session),
  });

  const [name, setName] = React.useState('');
  const [inputs, setInputs] = React.useState('{}');
  const [expected, setExpected] = React.useState('{}');
  const [activeDatasetId, setActiveDatasetId] = React.useState<string | null>(null);
  const [caseDescription, setCaseDescription] = React.useState('');

  const createDataset = useMutation({
    mutationFn: () => datasetApi.create({ capabilityId: capabilityId!, name }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['datasets', capabilityId] });
      setName('');
      toast({ title: 'Dataset created', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Create failed', variant: 'destructive', description: (err as Error).message }),
  });

  const deleteDataset = useMutation({
    mutationFn: (id: string) => datasetApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['datasets', capabilityId] });
      toast({ title: 'Dataset deleted', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Delete failed', variant: 'destructive', description: (err as Error).message }),
  });

  const addCase = useMutation({
    mutationFn: () => {
      let parsedInputs: unknown;
      let parsedExpected: unknown;
      try { parsedInputs = JSON.parse(inputs); } catch { parsedInputs = inputs; }
      try { parsedExpected = JSON.parse(expected); } catch { parsedExpected = expected; }
      const payload: { inputs: string; expected: string; description?: string } = {
        inputs: typeof parsedInputs === 'string' ? parsedInputs : JSON.stringify(parsedInputs),
        expected: typeof parsedExpected === 'string' ? parsedExpected : JSON.stringify(parsedExpected),
      };
      if (caseDescription.trim()) payload.description = caseDescription.trim();
      return datasetApi.addCase(activeDatasetId!, payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['datasets', capabilityId] });
      setInputs('{}');
      setExpected('{}');
      setCaseDescription('');
      toast({ title: 'Case added', variant: 'success' });
    },
    onError: (err) => toast({ title: 'Add case failed', variant: 'destructive', description: (err as Error).message }),
  });

  const rows = (Array.isArray(datasets.data) ? datasets.data : []) as DatasetSummary[];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Capability"
        title="Datasets"
        subtitle="Eval datasets scoped to this capability. Add JSON inputs and expected outputs to gate releases."
      />

      <Surface>
        <SurfaceHeader title="Create dataset" description="Datasets scope eval cases to this capability." />
        <div className="flex items-end gap-3">
          <div className="flex-1">
            <label className="text-xs uppercase tracking-wider text-text-subtle">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="refund-suite" className="mt-2" />
          </div>
          <Button onClick={() => createDataset.mutate()} disabled={!name.trim() || createDataset.isPending}>
            <Plus className="mr-1.5 size-3.5" />
            {createDataset.isPending ? 'Creating…' : 'Create dataset'}
          </Button>
        </div>
      </Surface>

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Datasets" description={`${rows.length} configured`} />
        {rows.length === 0 ? (
          <EmptyState
            icon={Plus}
            title="No datasets yet"
            description="Create one above, then click a row to add cases."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => setActiveDatasetId(String(r['id']))}
            columns={[
              {
                key: 'name',
                header: 'Name',
                render: (r) => (
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); setActiveDatasetId(String(r['id'])); }}
                    className="font-medium text-text-strong hover:underline"
                  >
                    {String(r['name'])}
                  </button>
                ),
              },
              {
                key: 'id',
                header: 'Identifier',
                render: (r) => <HashChip hash={String(r['id'])} />,
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
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={(e) => { e.stopPropagation(); deleteDataset.mutate(String(r['id'])); }}
                  >
                    <Trash2 className="mr-1 size-3" />
                    Delete
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Surface>

      {activeDatasetId && (
        <Surface>
          <SurfaceHeader
            title="Add case to selected dataset"
            description="Inputs and expected are JSON. Strings are passed through as-is if parsing fails."
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label className="text-xs uppercase tracking-wider text-text-subtle">Inputs (JSON)</label>
              <Textarea
                value={inputs}
                onChange={(e) => setInputs(e.target.value)}
                className="mt-2 min-h-32 font-mono text-xs"
                rows={6}
              />
            </div>
            <div>
              <label className="text-xs uppercase tracking-wider text-text-subtle">Expected (JSON)</label>
              <Textarea
                value={expected}
                onChange={(e) => setExpected(e.target.value)}
                className="mt-2 min-h-32 font-mono text-xs"
                rows={6}
              />
            </div>
          </div>
          <div className="mt-3">
            <label className="text-xs uppercase tracking-wider text-text-subtle">Description (optional)</label>
            <Input
              value={caseDescription}
              onChange={(e) => setCaseDescription(e.target.value)}
              placeholder="What does this case test?"
              className="mt-2"
            />
          </div>
          <div className="mt-4 flex items-center justify-end">
            <Button
              onClick={() => addCase.mutate()}
              disabled={!inputs.trim() || !expected.trim() || addCase.isPending}
            >
              <Plus className="mr-1.5 size-3.5" />
              {addCase.isPending ? 'Adding…' : 'Add case'}
            </Button>
          </div>
        </Surface>
      )}
    </div>
  );
}