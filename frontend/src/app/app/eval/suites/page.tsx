'use client';

import { useQuery, useMutation } from '@tanstack/react-query';
import { FlaskConical, Plus, Beaker } from 'lucide-react';
import Link from 'next/link';
import { useState } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { evalSuiteApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Button } from '@/components/ui/button';

export default function EvalSuitesPage() {
  const session = useRequireSession();
  const suites = useQuery({
    queryKey: ['eval-suites'],
    queryFn: () => evalSuiteApi.list(),
  });
  const [capabilityId, setCapabilityId] = useState('');
  const [name, setName] = useState('');
  const [passThreshold, setPassThreshold] = useState(0.92);
  const [borderlineBand, setBorderlineBand] = useState(0.05);
  const [graderPattern, setGraderPattern] = useState('hello');
  const [graderField, setGraderField] = useState<'output' | 'transcript' | 'metadata'>('output');

  const createSuite = useMutation({
    mutationFn: () => evalSuiteApi.create({
      capabilityId,
      name,
      passThreshold,
      borderlineBand,
      initialGraders: [
        {
          name: 'match',
          kind: 'regex_match',
          weight: 1,
          config: { pattern: graderPattern, field: graderField, kind: 'regex_match' },
        },
      ],
    }),
    onSuccess: () => {
      suites.refetch();
      setName('');
    },
  });

  if (!session) return null;
  const rows = Array.isArray(suites.data) ? (suites.data as Array<Record<string, unknown>>) : [];

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Quality"
        title="Eval suites"
        subtitle="Versioned, threshold-gated collections of eval cases. Borderline cases auto-route to human review."
      />

      <Surface>
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Capability id</label>
            <input
              value={capabilityId}
              onChange={(e) => setCapabilityId(e.target.value)}
              className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 text-sm text-text-default focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/30"
              placeholder="cap-uuid"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Suite name</label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 text-sm text-text-default focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/30"
              placeholder="smoke"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Pass threshold</label>
            <input
              type="number"
              step="0.01"
              min={0}
              max={1}
              value={passThreshold}
              onChange={(e) => setPassThreshold(Number(e.target.value))}
              className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 text-sm text-text-default"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Borderline band</label>
            <input
              type="number"
              step="0.01"
              min={0}
              max={1}
              value={borderlineBand}
              onChange={(e) => setBorderlineBand(Number(e.target.value))}
              className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 text-sm text-text-default"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Regex pattern</label>
            <input
              value={graderPattern}
              onChange={(e) => setGraderPattern(e.target.value)}
              className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 text-sm font-mono"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-wider text-text-subtle">Match field</label>
            <div className="mt-2">
              <ThemedSelect
                value={graderField}
                onValueChange={(v) => setGraderField(v as 'output' | 'transcript' | 'metadata')}
                options={[
                  { value: 'output', label: 'output' },
                  { value: 'transcript', label: 'transcript' },
                  { value: 'metadata', label: 'metadata' },
                ]}
                ariaLabel="Grader match field"
                triggerClassName="w-full"
              />
            </div>
          </div>
        </div>
        <div className="mt-4 flex items-center gap-2">
          <Button
            onClick={() => createSuite.mutate()}
            disabled={!capabilityId || !name || createSuite.isPending}
          >
            <Plus className="mr-1.5 h-3.5 w-3.5" />Create suite (v1)
          </Button>
        </div>
      </Surface>

      <Surface padded={false}>
        {rows.length === 0 ? (
          <EmptyState
            icon={FlaskConical}
            title="No suites yet"
            description="Create a suite with a starter regex grader to land gate decisions on a deterministic test."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows}
            rowKey={(r) => String(r['id'])}
            onRowClick={(r) => { window.location.href = `/app/eval/suites/${String(r['id'])}`; }}
            columns={[
              { key: 'name', header: 'Name', render: (r) => <span className="font-medium text-text-strong">{String(r['name'])}</span> },
              { key: 'capability', header: 'Capability', render: (r) => <span className="font-mono text-xs">{String(r['capabilityId'])}</span> },
              { key: 'threshold', header: 'Threshold', render: (r) => `${(Number(r['passThreshold']) * 100).toFixed(0)}%` },
              { key: 'borderline', header: 'Borderline', render: (r) => `±${(Number(r['borderlineBand']) * 100).toFixed(0)}%` },
              { key: 'version', header: 'Version', render: (r) => `v${String(r['currentVersion'])}` },
              {
                key: 'created',
                header: 'Created',
                render: (r) => new Date(String(r['createdAt'])).toLocaleDateString(),
              },
            ]}
          />
        )}
      </Surface>
    </div>
  );
}
