'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import Link from 'next/link';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, FlaskConical, Play } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { evalSuiteApi, getErrorMessage } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { Button } from '@/components/ui/button';
import { QueryError } from '@/components/brand/query-error';

export default function EvalSuiteDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params.id;
  const queryClient = useQueryClient();
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [trialsJson, setTrialsJson] = useState(JSON.stringify([{ caseId: 'sample', output: 'hello world' }], null, 2));

  const suite = useQuery({
    queryKey: ['eval-suite', id],
    queryFn: () => evalSuiteApi.get(id).then((r) => r.data),
    enabled: Boolean(id),
  });
  const runs = useQuery({
    queryKey: ['eval-suite-runs', id],
    queryFn: () => evalSuiteApi.runs(id),
    enabled: Boolean(id),
  });
  const runDetail = useQuery({
    queryKey: ['eval-suite-run', id, selectedRunId],
    queryFn: () => evalSuiteApi.runDetail(id, selectedRunId ?? ''),
    enabled: Boolean(id && selectedRunId),
  });
  const run = useMutation({
    mutationFn: () => {
      let trials: unknown;
      try {
        trials = JSON.parse(trialsJson);
      } catch {
        throw new Error('Trials must be valid JSON.');
      }
      if (!Array.isArray(trials) || trials.length === 0) {
        throw new Error('Add at least one trial object before running the suite.');
      }
      return evalSuiteApi.run(id, { trials });
    },
    onSuccess: (result) => {
      void queryClient.invalidateQueries({ queryKey: ['eval-suite-runs', id] });
      setSelectedRunId(result.data.id);
    },
  });

  if (!session) return null;
  if (suite.isError) {
    return <QueryError message={suite.error} onRetry={() => void suite.refetch()} />;
  }
  if (suite.isLoading) return <div className="text-text-muted text-sm">Loading…</div>;
  if (!suite.data) return <div className="text-text-muted text-sm">Suite not found.</div>;

  const out = suite.data;

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/eval/suites" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Suites
        </Link>
        <PageHeader
          eyebrow="Eval suite"
          title={out.suite.name}
          subtitle={`Threshold ${(out.suite.passThreshold * 100).toFixed(0)}% · Borderline ±${(out.suite.borderlineBand * 100).toFixed(0)}% · ${out.versions.length} version(s)`}
          actions={<FlaskConical className="h-5 w-5 text-brand-highlight" />}
        />
      </div>

      <div className="grid gap-5 lg:grid-cols-2">
        <Surface>
          <SurfaceHeader title="Versions" />
          {out.versions.length === 0 ? (
            <p className="text-text-muted text-sm">No versions yet.</p>
          ) : (
            <ul className="space-y-2">
              {out.versions.map((v) => (
                <li key={v.id} className="rounded-lg border border-border-subtle bg-surface-2/40 p-3">
                  <div className="text-sm font-medium text-text-default">v{v.version}</div>
                  <div className="text-xs text-text-subtle">
                    k={v.k} n={v.n} · {v.graderConfig.length} grader(s)
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Surface>

        <Surface>
          <SurfaceHeader title="Run" description="Paste trial JSON; runner applies the suite's current version." />
          <textarea
            value={trialsJson}
            onChange={(e) => setTrialsJson(e.target.value)}
            className="h-64 w-full rounded-md border border-border-subtle bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
          <div className="mt-3 flex items-center gap-2">
            <Button onClick={() => run.mutate()} disabled={run.isPending}>
              <Play className="mr-1.5 h-3.5 w-3.5" />Run
            </Button>
          </div>
          {run.isError ? (
            <p role="alert" className="mt-3 text-sm text-destructive">
              {getErrorMessage(run.error, 'The evaluation run failed. Check the trial payload and try again.')}
            </p>
          ) : null}
          {run.data && (
            <pre className="mt-4 max-h-80 overflow-auto rounded-md border border-border-subtle bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
{JSON.stringify(run.data, null, 2)}
            </pre>
          )}
        </Surface>
      </div>

      <Surface>
        <SurfaceHeader title="Run history" description="Durable runs survive refreshes and can be inspected by trial." />
        {runs.isError ? <p className="text-sm text-destructive">Run history could not be loaded.</p> : null}
        {!runs.isError && (runs.data?.length ?? 0) === 0 ? <p className="text-sm text-text-muted">No runs yet. Run the suite above to create the first record.</p> : null}
        <div className="space-y-2">
          {(runs.data ?? []).map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setSelectedRunId(item.id)}
              className={`flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-sm transition-colors ${selectedRunId === item.id ? 'border-brand bg-brand/5' : 'border-border-subtle hover:border-border-strong'}`}
            >
              <span>
                <span className="font-mono text-xs">{item.id.slice(0, 14)}</span>
                <span className="ml-3 text-text-muted">{new Date(item.startedAt).toLocaleString()}</span>
              </span>
              <span className={item.passed ? 'text-success' : 'text-destructive'}>{(item.rawScore * 100).toFixed(0)}% · {item.status}</span>
            </button>
          ))}
        </div>
        {runDetail.data ? (
          <div className="mt-4 border-t border-border-subtle pt-4">
            <div className="mb-2 text-xs uppercase tracking-wider text-text-subtle">Selected trial results</div>
            {runDetail.data.results.length === 0 ? (
              <p className="text-sm text-text-muted">This run did not produce any trial results.</p>
            ) : (
              <ol className="space-y-2">
                {runDetail.data.results.map((result) => (
                  <li key={result.id} className="rounded-lg border border-border-subtle bg-surface-1/50 p-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex items-center gap-2 text-sm text-text-default">
                        <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${result.passed ? 'bg-success/15 text-success' : 'bg-destructive/15 text-destructive'}`}>
                          {result.passed ? 'Passed' : 'Failed'}
                        </span>
                        <span>Case {result.caseId}</span>
                      </div>
                      <span className="font-mono text-xs text-text-muted">
                        Trial {result.seq + 1} · {(result.weightedScore * 100).toFixed(0)}%
                      </span>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-3 text-xs text-text-subtle">
                      <details>
                        <summary className="cursor-pointer hover:text-text-default">Trial input</summary>
                        <pre className="mt-2 max-w-full overflow-auto rounded-md bg-surface-0 p-2 font-mono text-[11px] text-text-default">{JSON.stringify(result.trial, null, 2)}</pre>
                      </details>
                      <details>
                        <summary className="cursor-pointer hover:text-text-default">Grader output</summary>
                        <pre className="mt-2 max-w-full overflow-auto rounded-md bg-surface-0 p-2 font-mono text-[11px] text-text-default">{JSON.stringify(result.graderResult, null, 2)}</pre>
                      </details>
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </div>
        ) : null}
      </Surface>
    </div>
  );
}
