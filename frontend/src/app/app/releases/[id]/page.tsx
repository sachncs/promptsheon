'use client';

import { useQuery, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import {
  ArrowLeft, GitBranch, ShieldCheck, AlertCircle, Play, RotateCcw, FastForward, KeyRound,
} from 'lucide-react';
import { useMemo, useState } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { releaseApi, approvalApi, auditApi, evalApi, type ApprovalEntry, type AuditEntry, type Release } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { StepRail, type Step } from '@/components/brand/step-rail';
import { Timeline } from '@/components/brand/timeline';
import { EmptyState } from '@/components/brand/empty-state';
import { QueryError } from '@/components/brand/query-error';
import { getErrorMessage } from '@/lib/errors';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/brand/tabs';
import { useToast } from '@/components/brand/toast';
import { Button } from '@/components/ui/button';
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';

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
  const qc = useQueryClient();
  const { toast } = useToast();
  const [canaryOpen, setCanaryOpen] = useState(false);
  const [canaryPercent, setCanaryPercent] = useState('10');
  const [rollbackOpen, setRollbackOpen] = useState(false);
  const [signOpen, setSignOpen] = useState(false);
  const [signingKeyId, setSigningKeyId] = useState('');
  const [signature, setSignature] = useState('');
  const [signedAt, setSignedAt] = useState(() => new Date().toISOString());

  const release = useQuery({
    queryKey: ['release', id],
    queryFn: () => releaseApi.get(id).then((r) => r.data),
    enabled: Boolean(id),
  });

  const approvals = useQuery({
    queryKey: ['approvals', id],
    queryFn: (): Promise<ApprovalEntry[]> => approvalApi.list(id).then((r) => r.data.approvals),
    enabled: Boolean(id),
  });

  const audit = useQuery({
    queryKey: ['audit', 'release', id],
    queryFn: () => auditApi.list({ resource: id }).then((r) => r.data),
    enabled: Boolean(id),
  });
  const evaluations = useQuery({
    queryKey: ['eval-runs', 'release', id],
    queryFn: () => evalApi.list(id).then((r) => r.data),
    enabled: Boolean(id),
  });

  const currentStep = useMemo(() => {
    const s = release.data?.status ?? 'draft';
    const stepId = s === 'rolled_back' ? 'rolled-back' : s;
    if (RAIL_STEPS.some((step) => step.id === stepId)) return stepId;
    return 'draft';
  }, [release.data]);

  const refreshRelease = () => qc.invalidateQueries({ queryKey: ['release', id] });

  const handleActivate = async () => {
    try {
      await releaseApi.transition(id, 'active');
      refreshRelease();
      toast({ title: 'Release activated', variant: 'success', description: 'Now receiving 100% of production traffic.' });
    } catch (err) {
      toast({ title: 'Activate failed', variant: 'destructive', description: getErrorMessage(err) });
    }
  };

  const handleTransition = async (to: 'review' | 'canary', label: string) => {
    try {
      await releaseApi.transition(id, to);
      refreshRelease();
      toast({ title: `${label} started`, variant: 'success', description: `Release moved to ${to}.` });
    } catch (err) {
      toast({ title: `${label} failed`, variant: 'destructive', description: getErrorMessage(err) });
    }
  };

  const handleCanary = async () => {
    const pct = Number(canaryPercent);
    if (!Number.isFinite(pct) || pct < 0 || pct > 100) {
      toast({ title: 'Invalid percent', variant: 'warning', description: 'Canary percent must be 0-100.' });
      return;
    }
    try {
      await releaseApi.canary(id, Math.round(pct));
      setCanaryOpen(false);
      refreshRelease();
      toast({ title: `Canary at ${pct}%`, variant: 'success', description: 'Weighted rollout updated.' });
    } catch (err) {
      toast({ title: 'Canary failed', variant: 'destructive', description: getErrorMessage(err) });
    }
  };

  const handleRollback = async () => {
    try {
      await releaseApi.rollback(id);
      setRollbackOpen(false);
      refreshRelease();
      toast({ title: 'Rolled back', variant: 'success', description: 'Atomic rollback completed.' });
    } catch (err) {
      toast({ title: 'Rollback failed', variant: 'destructive', description: getErrorMessage(err) });
    }
  };

  const handleAutoRollback = async () => {
    try {
      const assessment = await releaseApi.autoRollback(id);
      refreshRelease();
      toast({
        title: assessment.action === 'rolled_back' ? 'Regression detected — rolled back' : 'Canary is healthy',
        variant: assessment.action === 'rolled_back' ? 'warning' : 'success',
        description: assessment.reason,
      });
    } catch (err) {
      toast({ title: 'Canary assessment failed', variant: 'destructive', description: getErrorMessage(err) });
    }
  };

  const handleSign = async () => {
    if (!signingKeyId.trim() || !signature.trim()) {
      toast({ title: 'Signature details required', variant: 'warning', description: 'Provide the registered key ID and base64 signature.' });
      return;
    }
    try {
      await releaseApi.sign(id, { keyId: signingKeyId.trim(), signature: signature.trim(), signedAt });
      setSignOpen(false);
      setSignature('');
      await qc.invalidateQueries({ queryKey: ['release', id] });
      toast({ title: 'Release signed', variant: 'success', description: 'The operator signature was verified and persisted.' });
    } catch (err) {
      toast({ title: 'Signing failed', variant: 'destructive', description: getErrorMessage(err) });
    }
  };

  if (!session) return null;
  if (release.isLoading) return <div className="text-text-muted text-sm">Loading release…</div>;
  if (release.isError) return <QueryError message={release.error} onRetry={() => void release.refetch()} />;
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

  const r: Release = release.data;

  const releaseStatus = r.status;
  const isTerminal = releaseStatus === 'rolled_back';
  const canaryOrActive = releaseStatus === 'canary' || releaseStatus === 'active';
  const approvalCount = approvals.data?.length ?? 0;
  const hasPassingEvaluation = (evaluations.data ?? []).some((run) => run.status === 'passed');

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/releases" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Releases
        </Link>
        <PageHeader
          eyebrow={`Release · ${r.environment ?? 'production'}`}
          title={`${r.capabilityId} v${r.capabilityVersion}`}
          subtitle="Governed progression through draft → review → approved → canary → active. Rollback is one click."
          actions={
            <div className="flex items-center gap-2">
              <StatusPill kind={statusKindOf(releaseStatus, 'draft')} label={releaseStatus} />
            </div>
          }
        />
      </div>

      <div className="grid gap-5 lg:grid-cols-3">
        <Surface>
          <SurfaceHeader title="Release path" description="Where this release is in the state machine." />
          <StepRail steps={RAIL_STEPS} current={currentStep} className="mt-3" />
          <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-border-subtle pt-4">
            {releaseStatus === 'draft' && (
              <Button size="sm" onClick={() => void handleTransition('review', 'Review')}>
                <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />Send to review
              </Button>
            )}
            {releaseStatus === 'review' && (
              <Link href={`/app/approvals/${id}`}>
                <Button size="sm"><ShieldCheck className="mr-1.5 h-3.5 w-3.5" />Open approvals</Button>
              </Link>
            )}
            {releaseStatus === 'approved' && (
              <Button size="sm" onClick={() => void handleTransition('canary', 'Canary')}>
                <FastForward className="mr-1.5 h-3.5 w-3.5" />Start canary
              </Button>
            )}
            {releaseStatus === 'canary' && (
              <Button size="sm" onClick={handleActivate}>
                <Play className="mr-1.5 h-3.5 w-3.5" />Activate
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={() => setCanaryOpen(true)} disabled={!canaryOrActive || isTerminal}>
              <FastForward className="mr-1.5 h-3.5 w-3.5" />Canary {r.canaryPercent ?? 10}%
            </Button>
            <Button size="sm" variant="outline" onClick={() => setRollbackOpen(true)} disabled={!canaryOrActive || isTerminal}>
              <RotateCcw className="mr-1.5 h-3.5 w-3.5" />Roll back
            </Button>
          </div>
          <div className="mt-5 border-t border-border-subtle pt-4">
            <div className="text-xs font-semibold uppercase tracking-wider text-text-subtle">Promotion checks</div>
            <div className="mt-3 grid gap-2 text-xs sm:grid-cols-3">
              <CheckRow label="Distinct approvals" value={`${approvalCount}/2`} ready={approvalCount >= 2} />
              <CheckRow
                label="Passing evaluation"
                value={evaluations.isPending ? 'checking…' : hasPassingEvaluation ? 'passed' : 'required'}
                ready={hasPassingEvaluation}
              />
              <CheckRow label="Operator signature" value={r.signature && r.signedKeyId ? 'valid' : 'required'} ready={Boolean(r.signature && r.signedKeyId)} />
            </div>
            <div className="mt-3 flex justify-end">
              <Button size="sm" variant="outline" onClick={() => setSignOpen(true)} disabled={isTerminal}>
                <KeyRound className="mr-1.5 h-3.5 w-3.5" />{r.signature ? 'Replace signature' : 'Sign release'}
              </Button>
            </div>
          </div>
        </Surface>
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="approvals">Approvals</TabsTrigger>
          <TabsTrigger value="canary">Canary</TabsTrigger>
          <TabsTrigger value="audit">Audit</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <Surface>
            <SurfaceHeader title="Identity" />
            <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
          <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Capability</dt>
                <dd className="mt-1 font-mono text-xs text-text-default">{r.capabilityId}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Version</dt>
                <dd className="mt-1 font-mono text-xs text-text-default">v{r.capabilityVersion}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Environment</dt>
                <dd className="mt-1 text-text-default">{r.environment}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">State</dt>
                <dd className="mt-1"><StatusPill kind={statusKindOf(releaseStatus, 'draft')} label={releaseStatus} /></dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Release signature</dt>
                <dd className="mt-1">
                  <StatusPill
                    kind={r.signature && r.signedKeyId ? 'approved' : 'warning'}
                    label={r.signature && r.signedKeyId ? 'signed' : 'unsigned — signing required'}
                  />
                </dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Updated</dt>
                <dd className="mt-1 text-text-default">—</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-text-subtle">Created</dt>
                <dd className="mt-1 text-text-default">{r.createdAt ? new Date(r.createdAt).toLocaleString() : '—'}</dd>
              </div>
            </dl>
          </Surface>
        </TabsContent>

        <TabsContent value="approvals">
          <Surface>
            <SurfaceHeader title="Approvals" description="Maker-checker coverage on this release." />
            {approvals.isError ? (
              <QueryError message={approvals.error} onRetry={() => void approvals.refetch()} />
            ) : approvals.data?.length ? (
              <ul className="space-y-3">
                {approvals.data.map((a) => (
                  <li key={`${a.userId}-${a.createdAt}`} className="flex items-center gap-3">
                    <ShieldCheck className={`h-4 w-4 ${a.vote === 'approve' ? 'text-success' : 'text-destructive'}`} />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm text-text-strong">{a.userId}</div>
                      <div className="text-xs text-text-muted">
                        {a.vote === 'approve' ? 'approved' : 'rejected'} · {new Date(a.createdAt).toLocaleString()}
                      </div>
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
        </TabsContent>

        <TabsContent value="canary">
          <Surface>
            <SurfaceHeader title="Canary" description="Weighted traffic split for this release." />
            {(() => {
              const pct = Number(r.canaryPercent ?? 0);
              return (
                <div className="space-y-4">
                  <div className="flex items-baseline justify-between">
                    <div className="font-mono text-3xl font-semibold text-text-strong">{pct}%</div>
                    <div className="text-xs text-text-muted">of production traffic</div>
                  </div>
                  <div className="h-3 overflow-hidden rounded-full bg-surface-2">
                    <div className="h-full bg-brand" style={{ width: `${pct}%` }} />
                  </div>
                  <p className="text-sm text-text-muted leading-relaxed">
                    Canary releases route a configurable percentage of traffic to the new manifest while
                    live eval scores monitor for drift. Increase the canary percent over time, or activate to
                    send 100% of traffic to this release.
                  </p>
                  <Button size="sm" variant="outline" onClick={handleAutoRollback} disabled={isTerminal || (releaseStatus !== 'canary' && releaseStatus !== 'active')}>
                    <AlertCircle className="mr-1.5 h-3.5 w-3.5" />Assess health &amp; auto-rollback
                  </Button>
                </div>
              );
            })()}
          </Surface>
        </TabsContent>

        <TabsContent value="audit">
          <Surface>
            <SurfaceHeader title="Lifecycle" description="Append-only audit events for this release." />
            {audit.isError ? (
              <QueryError message={audit.error} onRetry={() => void audit.refetch()} />
            ) : audit.data?.length ? (
              <Timeline
                entries={audit.data.map((a: AuditEntry) => ({
                  id: a.id,
                  title: a.action,
                  actor: a.userId,
                  timestamp: new Date(a.timestamp).toLocaleString(),
                  tone: 'info' as const,
                }))}
              />
            ) : (
              <EmptyState icon={AlertCircle} title="No lifecycle events yet" description="State transitions are appended to the audit chain." />
            )}
          </Surface>
        </TabsContent>
      </Tabs>

      <Dialog open={canaryOpen} onOpenChange={setCanaryOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Set canary percent</DialogTitle>
            <DialogDescription>Weighted traffic split for this release. Live eval scores are watched while canary is non-zero.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <label className="text-xs uppercase tracking-wider text-text-subtle">Percent</label>
            <Input
              type="number"
              min={0}
              max={100}
              value={canaryPercent}
              onChange={(e) => setCanaryPercent(e.target.value)}
              className="font-mono"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCanaryOpen(false)}>Cancel</Button>
            <Button onClick={handleCanary}>Apply</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={rollbackOpen} onOpenChange={setRollbackOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Roll back this release</DialogTitle>
            <DialogDescription>Atomically revert to the most recent active release in the same environment. This is recorded in the audit chain.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRollbackOpen(false)}>Cancel</Button>
            <Button variant="destructive" onClick={handleRollback}>
              <RotateCcw className="mr-1.5 h-3.5 w-3.5" />Roll back
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={signOpen} onOpenChange={setSignOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Sign release</DialogTitle>
            <DialogDescription>
              Sign the release message with your external Ed25519 operator key, then paste the base64 signature here. Private keys never enter Promptsheon.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-xs uppercase tracking-wider text-text-subtle" htmlFor="release-signing-key">Registered key ID</label>
              <Input id="release-signing-key" value={signingKeyId} onChange={(e) => setSigningKeyId(e.target.value)} placeholder="key UUID or fingerprint" />
            </div>
            <div className="space-y-2">
              <label className="text-xs uppercase tracking-wider text-text-subtle" htmlFor="release-signed-at">Signed at</label>
              <Input id="release-signed-at" type="datetime-local" value={signedAt.slice(0, 16)} onChange={(e) => setSignedAt(new Date(e.target.value).toISOString())} />
            </div>
            <div className="space-y-2">
              <label className="text-xs uppercase tracking-wider text-text-subtle" htmlFor="release-signature">Base64 signature</label>
              <textarea id="release-signature" value={signature} onChange={(e) => setSignature(e.target.value)} placeholder="Paste the Ed25519 signature" className="min-h-28 w-full rounded-md border border-border-subtle bg-surface-1 px-3 py-2 font-mono text-xs text-text-default outline-none focus:border-brand" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSignOpen(false)}>Cancel</Button>
            <Button onClick={() => void handleSign()}>Verify and sign</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function CheckRow({ label, value, ready }: { label: string; value: string; ready: boolean }) {
  return (
    <div className="flex items-center justify-between gap-2 rounded-lg border border-border-subtle bg-surface-2/40 px-3 py-2">
      <span className="text-text-muted">{label}</span>
      <span className={ready ? 'font-medium text-success' : 'font-medium text-warning'}>{value}</span>
    </div>
  );
}
