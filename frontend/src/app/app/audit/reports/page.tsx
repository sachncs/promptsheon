'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, CheckCircle2, Download, FileText, ShieldCheck, XCircle } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { auditApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { Field, FieldGroup } from '@/components/brand/field';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { ThemedSelect } from '@/components/brand/themed-select';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill } from '@/components/brand/status-pill';
import { EmptyState } from '@/components/brand/empty-state';

export default function AuditReportsPage() {
  const session = useRequireSession();
  const [actor, setActor] = useState('');
  const [resource, setResource] = useState('');
  const [action, setAction] = useState('');
  const [fromTime, setFromTime] = useState('');
  const [toTime, setToTime] = useState('');
  const [submitted, setSubmitted] = useState<null | { actor: string; resource: string; action: string; fromTime: string; toTime: string }>(null);

  const report = useQuery({
    queryKey: ['audit-report', submitted],
    queryFn: () => {
      if (!submitted) throw new Error('no submission');
      const opts: Parameters<typeof auditApi.report>[0] = {};
      if (submitted.actor) opts.actor = submitted.actor;
      if (submitted.resource) opts.resource = submitted.resource;
      if (submitted.action) opts.action = submitted.action;
      if (submitted.fromTime) opts.fromTime = submitted.fromTime;
      if (submitted.toTime) opts.toTime = submitted.toTime;
      return auditApi.report(opts);
    },
    enabled: Boolean(session && submitted !== null),
  });

  const downloadJson = () => {
    if (!report.data) return;
    const blob = new Blob([JSON.stringify(report.data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `audit-report-${report.data.organizationId}-${report.data.generatedAt.slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const printToPdf = () => {
    window.print();
  };

  if (!session) return null;

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/app/audit"
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
        >
          <ArrowLeft className="h-3 w-3" /> Audit log
        </Link>
      </div>

      <PageHeader
        eyebrow="Compliance"
        title="Audit reports"
        subtitle="Signed JSON document containing the audit-chain head + filtered entries. Suitable for SOC 2 evidence packs and external auditor review."
        actions={
          report.data ? (
            <div className="flex items-center gap-2">
              <Button size="sm" variant="ghost" onClick={printToPdf}>
                <FileText className="mr-1.5 h-3.5 w-3.5" />
                Print to PDF
              </Button>
              <Button size="sm" onClick={downloadJson}>
                <Download className="mr-1.5 h-3.5 w-3.5" />
                Download signed JSON
              </Button>
            </div>
          ) : undefined
        }
      />

      <Surface padded={false}>
        <SurfaceHeader className="px-5 pt-5" title="Generate a report" />
        <div className="px-5 pb-5">
          <FieldGroup>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
              <Field label="Actor" htmlFor="rpt-actor">
                <Input id="rpt-actor" value={actor} onChange={(e) => setActor(e.target.value)} placeholder="u-alice" />
              </Field>
              <Field label="Resource" htmlFor="rpt-resource">
                <Input id="rpt-resource" value={resource} onChange={(e) => setResource(e.target.value)} placeholder="manifest" />
              </Field>
              <Field label="Action" htmlFor="rpt-action">
                <ThemedSelect
                  value={action || 'all'}
                  onValueChange={(v) => setAction(v === 'all' ? '' : v)}
                  options={[
                    { value: 'all', label: 'All actions' },
                    { value: 'workspace.create', label: 'workspace.create' },
                    { value: 'release.activate', label: 'release.activate' },
                    { value: 'manifest.save', label: 'manifest.save' },
                  ]}
                />
              </Field>
              <Field label="From" htmlFor="rpt-from">
                <Input id="rpt-from" type="datetime-local" value={fromTime} onChange={(e) => setFromTime(e.target.value)} />
              </Field>
              <Field label="To" htmlFor="rpt-to">
                <Input id="rpt-to" type="datetime-local" value={toTime} onChange={(e) => setToTime(e.target.value)} />
              </Field>
            </div>
          </FieldGroup>
          <div className="mt-3">
            <Button onClick={() => setSubmitted({ actor, resource, action, fromTime, toTime })}>
              Generate report
            </Button>
            {report.error && (
              <span className="ml-3 text-xs text-destructive">{String((report.error as Error).message)}</span>
            )}
          </div>
        </div>
      </Surface>

      {report.data ? (
        <Surface padded={false}>
          <SurfaceHeader
            className="px-5 pt-5"
            title={`${report.data.entryCount} entries · ${report.data.organizationId}`}
            description={`Generated ${new Date(report.data.generatedAt).toLocaleString()}${report.data.generatedBy ? ` by ${report.data.generatedBy}` : ''}`}
            actions={
              <div className="flex items-center gap-2">
                {report.data.chainValid ? (
                  <StatusPill kind="active" label="chain valid" />
                ) : (
                  <StatusPill kind="error" label="chain BROKEN" />
                )}
              </div>
            }
          />
          <div className="space-y-3 px-5 pb-5 text-sm">
            <div className="grid grid-cols-2 gap-3 rounded-md bg-surface-0 p-3 md:grid-cols-4">
              <Stat label="Chain head" value={report.data.chainHead.slice(0, 12) + '…'} mono />
              <Stat label="Algorithm" value={report.data.signature.algorithm} />
              <Stat label="Signature" value={report.data.signature.value.slice(0, 12) + '…'} mono />
              <Stat label="Generated" value={new Date(report.data.generatedAt).toLocaleString()} />
            </div>
            <div className="rounded-md border border-border-subtle bg-surface-0">
              <div className="max-h-96 overflow-auto p-3 font-mono text-[11px] leading-relaxed">
                {report.data.entries.length === 0 ? (
                  <span className="text-text-muted">No entries match the filter.</span>
                ) : (
                  <ul className="space-y-1">
                    {report.data.entries.map((e) => (
                      <li key={e.id} className="flex items-baseline gap-2 border-b border-border-subtle/40 pb-1 last:border-0">
                        <span className="text-text-muted">{new Date(e.timestamp).toLocaleString()}</span>
                        <span className="text-text-default">{e.actor}</span>
                        <span className="text-brand-highlight">{e.action}</span>
                        <span className="text-text-default">{e.resource}</span>
                        <HashChip hash={e.id} length={10} />
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
            {report.data.chainValid ? (
              <div className="flex items-start gap-2 rounded-md bg-success/5 p-3 text-xs text-text-default">
                <CheckCircle2 className="h-4 w-4 mt-0.5 shrink-0 text-success" aria-hidden="true" />
                <div>
                  Chain integrity verified. SHA-256 signature binds the report to these exact bytes.
                  Auditor verifies by computing <code>sha256(canonical_json)</code> and comparing to{' '}
                  <code>signature.value</code>.
                </div>
              </div>
            ) : (
              <div className="flex items-start gap-2 rounded-md bg-destructive/5 p-3 text-xs text-text-default">
                <XCircle className="h-4 w-4 mt-0.5 shrink-0 text-destructive" aria-hidden="true" />
                <div>Chain integrity FAILED. Do not use this report until the tamper is investigated.</div>
              </div>
            )}
          </div>
        </Surface>
      ) : submitted ? (
        <Surface>
          <div className="px-5 py-4 text-sm text-text-muted">Generating…</div>
        </Surface>
      ) : (
        <EmptyState
          className="border-0 bg-transparent shadow-none p-12"
          icon={ShieldCheck}
          title="Generate an audit report"
          description="Pick the actor / resource / action filters and a date range, then click 'Generate report'."
        />
      )}
    </div>
  );
}

function Stat({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wider text-text-subtle">{label}</div>
      <div className={`mt-0.5 truncate ${mono ? 'font-mono text-[11px]' : ''} text-text-strong`}>
        {value}
      </div>
    </div>
  );
}
