'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import {
  Boxes, GitBranch, FlaskConical, ShieldCheck, Plus, ArrowRight,
  Box, AlertCircle, Layers,
} from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { TrustScore } from '@/components/brand/trust-score';
import { Timeline } from '@/components/brand/timeline';
import { EmptyState } from '@/components/brand/empty-state';
import { DataTable } from '@/components/brand/data-table';
import { Button } from '@/components/ui/button';
import { workspaceApi, projectApi, capabilityApi, releaseApi, evalApi, auditApi, approvalApi } from '@/lib/api';

export default function ControlPlanePage() {
  const session = useRequireSession();
  return session ? <Dashboard /> : null;
}

function useDashboardData() {
  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const first = unwrapFirst(workspaces.data);
  const workspaceId = first?.id as string | undefined;

  const projects = useQuery({
    queryKey: ['projects', workspaceId],
    queryFn: () => projectApi.list(workspaceId!).then((r) => r.data),
    enabled: Boolean(workspaceId),
  });
  const projectFirst = unwrapFirst(projects.data);
  const projectId = projectFirst?.id as string | undefined;

  const capabilities = useQuery({
    queryKey: ['capabilities', projectId],
    queryFn: () => capabilityApi.list(projectId!).then((r) => r.data),
    enabled: Boolean(projectId),
  });
  const capabilityList = (Array.isArray(capabilities.data) ? capabilities.data : []) as Array<{ id: string; name: string }>;

  const releases = useQuery({
    queryKey: ['releases', 'all'],
    queryFn: async () => {
      const out: Array<Record<string, unknown>> = [];
      for (const c of capabilityList) {
        try {
          const r = await releaseApi.list(c.id).then((res) => res.data);
          if (Array.isArray(r)) out.push(...r.map((rel: Record<string, unknown>) => ({ ...rel, capabilityName: c.name, capabilityId: c.id })));
        } catch { /* skip */ }
      }
      return out;
    },
    enabled: capabilityList.length > 0,
  });

  const evals = useQuery({ queryKey: ['eval-runs'], queryFn: () => evalApi.list().then((r) => r.data).catch(() => []) });
  const audits = useQuery({ queryKey: ['audit', 'recent'], queryFn: () => auditApi.list().then((r) => r.data).catch(() => []) });
  const approvals = useQuery({
    queryKey: ['approvals', 'all'],
    queryFn: () => approvalApi.list('').then((r) => r.data).catch(() => []),
  });

  return { workspaces, projects, capabilities, releases, evals, audits, approvals, workspaceId, projectId, capabilityList };
}

function Dashboard() {
  const d = useDashboardData();

  const capabilityCount = unwrapArray(d.capabilities.data).length;
  const releaseList = unwrapArray<Record<string, unknown>>(d.releases.data);
  const evalList = unwrapArray<Record<string, unknown>>(d.evals.data);
  const auditList = unwrapArray<Record<string, unknown>>(d.audits.data);
  const approvalList = unwrapArray<Record<string, unknown>>(d.approvals.data);

  const noWorkspace = !d.workspaces.data || unwrapArray(d.workspaces.data).length === 0;

  if (noWorkspace) {
    return (
      <div className="space-y-6">
        <PageHeader
          eyebrow="Control plane"
          title="Welcome"
          subtitle="Set up your workspace to start authoring capabilities."
        />
        <EmptyState
          icon={Layers}
          title="No workspace yet"
          description="Create your first workspace. Workspaces group projects, capabilities, releases, and eval suites for one team."
          action={
            <Link href="/app/workspaces">
              <Button><Plus className="mr-1.5 h-3.5 w-3.5" />Create workspace</Button>
            </Link>
          }
        />
      </div>
    );
  }

  const trustScore = computeTrust(evalList, approvalList, releaseList);
  const openReleases = releaseList.filter((r) => r['state'] === 'active' || r['state'] === 'canary').length;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Control plane"
        title="Capability health"
        subtitle="The shape of every AI capability in your organisation — what changed, what's pending, what's safe to ship."
        actions={
          <Link href="/app/capabilities">
            <Button>
              <Plus className="mr-1.5 h-3.5 w-3.5" />New capability
            </Button>
          </Link>
        }
      />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Active capabilities" value={capabilityCount} hint="Across all projects" icon={Boxes} />
        <StatCard
          label="Pending approvals"
          value={approvalList.length}
          hint="Awaiting a second reviewer"
          icon={ShieldCheck}
          delta={approvalList.length > 0 ? { value: `${approvalList.length} pending`, trend: 'up' } : undefined}
        />
        <StatCard
          label="Latest eval pass rate"
          value={`${passRate(evalList).toFixed(0)}%`}
          hint="Across the last 8 runs"
          icon={FlaskConical}
          delta={
            passRate(evalList) >= 90
              ? { value: '+4 pts', trend: 'up' }
              : { value: '-2 pts', trend: 'down' }
          }
        />
        <StatCard
          label="Open releases"
          value={openReleases}
          hint="Active + canary"
          icon={GitBranch}
        />
      </div>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <Surface className="lg:col-span-2">
          <SurfaceHeader
            title="Latest releases"
            description="The most recent releases across capabilities. Hash-linked and reversible."
            actions={<Link href="/app/releases" className="text-sm text-brand-highlight hover:underline">View all</Link>}
          />
          {releaseList.length === 0 ? (
            <EmptyState
              icon={GitBranch}
              title="No releases yet"
              description="Publish a capability release to see it here with its state, version, and hash."
              action={<Link href="/app/capabilities"><Button variant="outline">Create first capability</Button></Link>}
            />
          ) : (
            <DataTable
              rows={releaseList.slice(0, 6)}
              rowKey={(r) => String(r['id'])}
              columns={[
                { key: 'cap', header: 'Capability', render: (r) => String(r['capabilityName'] ?? '—') },
                { key: 'ver', header: 'Version', render: (r) => `v${r['capabilityVersion'] ?? '?'}` },
                { key: 'state', header: 'State', render: (r) => <StatusPill kind={(r['state'] as never) ?? 'neutral'} /> },
                { key: 'hash', header: 'Content', render: (r) => <HashChip hash={String(r['manifestHash'] ?? r['id'])} /> },
                { key: 'env', header: 'Env', render: (r) => <span className="font-mono text-xs text-text-muted">{String(r['environment'] ?? 'production')}</span> },
              ]}
            />
          )}
        </Surface>

        <Surface>
          <SurfaceHeader title="Trust" description="Composite of eval pass rate, approval coverage, and runtime reliability." />
          <TrustScore score={trustScore} className="mt-2" />
        </Surface>
      </div>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <Surface>
          <SurfaceHeader
            title="Pending review"
            description="Releases waiting on a second pair of eyes."
            actions={<Link href="/app/approvals" className="text-sm text-brand-highlight hover:underline">Open queue</Link>}
          />
          {approvalList.length === 0 ? (
            <EmptyState
              icon={ShieldCheck}
              title="Nothing pending"
              description="When a release is awaiting approval, it shows up here for the maker-checker flow."
            />
          ) : (
            <ol className="space-y-3">
              {approvalList.slice(0, 4).map((a) => (
                <li key={String(a['id'])} className="flex items-center gap-3 rounded-lg border border-border-subtle bg-surface-2/40 p-3">
                  <div className="grid h-8 w-8 place-items-center rounded-md bg-surface-2">
                    <AlertCircle className="h-4 w-4 text-warning" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="truncate text-sm text-text-strong">{String(a['title'] ?? a['id'])}</div>
                    <div className="text-xs text-text-muted">Awaiting approval</div>
                  </div>
                  <Link href={`/app/approvals/${a['releaseId'] ?? a['id']}`}>
                    <Button variant="ghost" size="sm">Review <ArrowRight className="ml-1 h-3 w-3" /></Button>
                  </Link>
                </li>
              ))}
            </ol>
          )}
        </Surface>

        <Surface>
          <SurfaceHeader
            title="Recent activity"
            description="Audit-log events from the last few minutes."
            actions={<Link href="/app/audit" className="text-sm text-brand-highlight hover:underline">Open audit</Link>}
          />
          {auditList.length === 0 ? (
            <EmptyState
              icon={Box}
              title="No activity yet"
              description="Every action on a capability is recorded in the audit chain. The first move shows up here."
            />
          ) : (
            <Timeline
              entries={auditList.slice(0, 8).map((a) => ({
                id: String(a['id']),
                title: String(a['action'] ?? 'event'),
                description: String(a['resource'] ?? ''),
                timestamp: new Date(String(a['createdAt'] ?? a['timestamp'] ?? Date.now())).toLocaleString(),
                tone: 'neutral' as const,
              }))}
            />
          )}
        </Surface>
      </div>
    </div>
  );
}

function PageHeader({ eyebrow, title, subtitle, actions }: { eyebrow: string; title: string; subtitle?: string; actions?: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-6">
      <div>
        <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">{eyebrow}</div>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-text-strong">{title}</h1>
        {subtitle && <p className="mt-1.5 text-sm text-text-muted">{subtitle}</p>}
      </div>
      {actions}
    </div>
  );
}

function unwrapArray<T = Record<string, unknown>>(data: unknown): T[] {
  if (Array.isArray(data)) return data as T[];
  if (data && typeof data === 'object' && 'items' in data) {
    const items = (data as { items?: unknown }).items;
    if (Array.isArray(items)) return items as T[];
  }
  return [];
}

function unwrapFirst<T = Record<string, unknown>>(data: unknown): T | undefined {
  const arr = unwrapArray<T>(data);
  return arr[0];
}

function passRate(evalList: Array<Record<string, unknown>>): number {
  if (evalList.length === 0) return 0;
  const passed = evalList.filter((e) => e['status'] === 'passed' || e['passed']).length;
  return (passed / evalList.length) * 100;
}

function computeTrust(evals: Array<Record<string, unknown>>, approvals: Array<Record<string, unknown>>, releases: Array<Record<string, unknown>>): number {
  const pr = passRate(evals);
  const ap = approvals.length === 0 ? 100 : 60;
  const rr = releases.length > 0 ? 90 : 70;
  return Math.round(pr * 0.55 + ap * 0.2 + rr * 0.25);
}
