'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import {
  ArrowLeft, GitBranch, ShieldCheck, AlertCircle, Play, RotateCcw, FastForward,
} from 'lucide-react';
import { useMemo } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { releaseApi, approvalApi, auditApi } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill, type StatusKind } from '@/components/brand/status-pill';
import { StepRail, type Step } from '@/components/brand/step-rail';
import { HashChip } from '@/components/brand/hash-chip';
import { Timeline } from '@/components/brand/timeline';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';

const RAIL_STEPS: Step[] = [
  { id: 'draft', label: 'Draft', description: 'Initial capability manifest.', status: 'draft' },
  { id: 'review', label: 'Review', description: 'Awaiting approvals from a second pair of eyes.', status: 'review' },
  { id: 'approved', label: 'Approved', description: 'Maker-checker satisfied. Ready to route.', status: 'approved' },
  { id: 'canary', label: 'Canary', description: 'Weighted rollout. Eval scores are monitored.', status: 'canary' },
  { id: 'active', label: 'Active', description: 'Receiving 100% of production traffic.', status: 'active' },
  { id: 'rolled-back', label: 'Rolled back', description: 'Reverted to a prior stable release.', status: 'rolled-back' },
];

export default function ReleaseDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params.id;

  const release = useQuery({
    queryKey: ['release', id],
    queryFn: () => releaseApi.get(id).then((r) => r.data).catch(() => null),
    enabled: Boolean(id),
  });

  const approvals = useQuery({
    queryKey: ['approvals', id],
    queryFn: () => approvalApi.list(id).then((r) => r.data).catch(() => []),
    enabled: Boolean(id),
  });

  const audit = useQuery({
    queryKey: ['audit', 'release', id],
    queryFn: () => auditApi.list({ resource: id }).then((r) => r.data).catch(() => []),
    enabled: Boolean(id),
  });

  const currentStep = useMemo(() => {
    const r = release.data as { state?: string } | null | undefined;
    const s = r?.state ?? 'draft';
    if (RAIL_STEPS.some((step) => step.id === s)) return String(s);
    return 'draft';
  }, [release.data]);

  if (!session) return null;
  if (release.isLoading) return <div className="text-text-muted text-sm">Loading release…</div>;
  if (!release.data) {
    return (
      <EmptyState
        icon={GitBranch}
        title="Release not found"
        description="The release may have been removed or the link is stale."
        action={
          <Link href="/app/releases">
            <Button variant="outline"><ArrowLeft className="mr-1.5 h-3.5 w-3.5" />Back to releases</Button>
          </Link>
        }
      />
    );
  }

  const r = release.data as {
    id: string; capabilityName?: string; capabilityVersion?: number; state?: string;
    manifestHash?: string; environment?: string; canaryPercent?: number;
    createdAt?: string; updatedAt?: string;
  };

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/releases" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Releases
        </Link>
        <PageHeader
          eyebrow={`Release · ${r.environment ?? 'production'}`}
          title={`${r.capabilityName ?? 'Capability'} v${r.capabilityVersion ?? '?'}`}
          subtitle="Governed progression through draft → review → approved → canary → active. Rollback is one click."
          actions={
            <div className="flex items-center gap-2">
              {r.manifestHash && <HashChip hash={r.manifestHash} />}
              <StatusPill kind={(r.state as StatusKind) ?? 'draft'} />
            </div>
          }
        />
      </div>

      <div className="grid gap-5 lg:grid-cols-3">
        <Surface>
          <SurfaceHeader title="Release path" description="Where this release is in the state machine." />
          <StepRail steps={RAIL_STEPS} current={currentStep} className="mt-3" />
          <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-border-subtle pt-4">
            <Button size="sm"><Play className="mr-1.5 h-3.5 w-3.5" />Activate</Button>
            <Button size="sm" variant="outline"><FastForward className="mr-1.5 h-3.5 w-3.5" />Canary 10%</Button>
            <Button size="sm" variant="outline"><RotateCcw className="mr-1.5 h-3.5 w-3.5" />Roll back</Button>
          </div>
        </Surface>

        <Surface>
          <SurfaceHeader title="Approvals" description="Maker-checker coverage on this release." />
          {(approvals.data as unknown[] | undefined)?.length ? (
            <ul className="space-y-3">
              {((approvals.data as Array<{ id: string; actor?: string; decision?: string; createdAt?: string }>) ?? []).map((a) => (
                <li key={a.id} className="flex items-center gap-3">
                  <ShieldCheck className={`h-4 w-4 ${a.decision === 'approve' ? 'text-success' : a.decision === 'reject' ? 'text-destructive' : 'text-info'}`} />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-text-strong">{a.actor ?? a.id}</div>
                    <div className="text-xs text-text-muted">{a.decision === 'approve' ? 'approved' : a.decision === 'reject' ? 'rejected' : 'pending'} · {new Date(a.createdAt ?? Date.now()).toLocaleString()}</div>
                  </div>
                </li>
              ))}
            </ul>
          ) : (
            <EmptyState
              icon={AlertCircle}
              title="No approvals yet"
              description="When a second reviewer signs off, the approval will be shown here."
            />
          )}
        </Surface>

        <Surface>
          <SurfaceHeader title="Lifecycle" description="Append-only audit events for this release." />
          {(audit.data as unknown[] | undefined)?.length ? (
            <Timeline
              entries={((audit.data as Array<{ id: string; action?: string; actor?: string; createdAt?: string }>) ?? []).map((a) => ({
                id: a.id,
                title: String(a.action ?? 'event'),
                actor: a.actor,
                timestamp: new Date(a.createdAt ?? Date.now()).toLocaleString(),
                tone: 'info' as const,
              }))}
            />
          ) : (
            <EmptyState icon={AlertCircle} title="No lifecycle events yet" description="State transitions are appended to the audit chain." />
          )}
        </Surface>
      </div>
    </div>
  );
}
