'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import Link from 'next/link';
import { useQuery, useMutation } from '@tanstack/react-query';
import { ArrowLeft, FlaskConical, Play, Sparkles } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { evalSuiteApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { Button } from '@/components/ui/button';

export default function EvalSuiteDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params.id;
  const [trialsJson, setTrialsJson] = useState(JSON.stringify([{ caseId: 'sample', output: 'hello world' }], null, 2));

  const suite = useQuery({
    queryKey: ['eval-suite', id],
    queryFn: () => evalSuiteApi.get(id),
    enabled: Boolean(id),
  });
  const run = useMutation({
    mutationFn: () => evalSuiteApi.run(id, { trials: JSON.parse(trialsJson) }),
  });

  if (!session) return null;
  if (suite.isLoading) return <div className="text-text-muted text-sm">Loading…</div>;
  if (!suite.data) return <div className="text-text-muted text-sm">Suite not found.</div>;

  const out = suite.data as { suite: Record<string, unknown>; versions: Array<Record<string, unknown>> };

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/eval/suites" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Suites
        </Link>
        <PageHeader
          eyebrow="Eval suite"
          title={String(out.suite.name)}
          subtitle={`Threshold ${(Number(out.suite.passThreshold) * 100).toFixed(0)}% · Borderline ±${(Number(out.suite.borderlineBand) * 100).toFixed(0)}% · ${out.versions.length} version(s)`}
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
                <li key={String(v.id)} className="rounded-lg border border-border-subtle bg-surface-2/40 p-3">
                  <div className="text-sm font-medium text-text-default">v{String(v.version)}</div>
                  <div className="text-xs text-text-subtle">
                    k={String(v.k)} n={String(v.n)} · {(v.graderConfig as unknown[]).length} grader(s)
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
          {run.data && (
            <pre className="mt-4 max-h-80 overflow-auto rounded-md border border-border-subtle bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
{JSON.stringify(run.data, null, 2)}
            </pre>
          )}
        </Surface>
      </div>
    </div>
  );
}
