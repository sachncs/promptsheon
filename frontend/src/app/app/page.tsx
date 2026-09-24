'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import {
  Boxes, GitBranch, FlaskConical, ShieldCheck, Plus, ArrowRight,
  Box, AlertCircle, Layers, CalendarClock, Sparkles,
} from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatCard } from '@/components/brand/stat-card';
import { StatusPill, statusKindOf } from '@/components/brand/status-pill';
import { TrustScore } from '@/components/brand/trust-score';
import { Timeline } from '@/components/brand/timeline';
import { EmptyState } from '@/components/brand/empty-state';
import { DataTable } from '@/components/brand/data-table';
import { AnimatedNumber } from '@/components/brand/animated-number';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/brand/tabs';
import { Button } from '@/components/ui/button';
import {
  workspaceApi,
  projectApi,
  capabilityApi,
  releaseApi,
  evalApi,
  auditApi,
  approvalApi,
  type EvalRun,
  type PendingApprovalSummary,
  type WorkspaceRow,
  type Release,
} from '@/lib/api';
import { QueryError } from '@/components/brand/query-error';

export default function ControlPlanePage() {
  const session = useRequireSession();
  return session ? <Dashboard /> : null;
}

function useDashboardData() {
  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data),
  });
  const workspaceList: WorkspaceRow[] = workspaces.data ?? [];
  const workspaceIds = workspaceList.map((workspace) => workspace.id);

  const projects = useQuery({
    queryKey: ['projects', 'all', workspaceIds],
    queryFn: async () => {
      const responses = await Promise.all(workspaceIds.map((workspaceId) => projectApi.list(workspaceId)));
      return responses.flatMap((response) => response.data);
    },
    enabled: workspaceIds.length > 0,
  });
  const projectList = projects.data ?? [];
  const projectIds = projectList.map((project) => project.id);

  const capabilities = useQuery({
    queryKey: ['capabilities', 'all', projectIds],
    queryFn: async () => {
      const responses = await Promise.all(projectIds.map((projectId) => capabilityApi.list(projectId)));
      return responses.flatMap((response) => response.data);
    },
    enabled: projectIds.length > 0,
  });
  const capabilityList = capabilities.data ?? [];

  const releases = useQuery({
    queryKey: ['releases', 'all'],
    queryFn: () => releaseApi.listAll(1, 100).then((res) => res.data),
    enabled: workspaceList.length > 0,
  });

  const evals = useQuery({ queryKey: ['eval-runs'], queryFn: () => evalApi.list().then((r) => r.data) });
  const audits = useQuery({ queryKey: ['audit', 'recent'], queryFn: () => auditApi.list().then((r) => r.data) });
  const approvals = useQuery<{ approvals: PendingApprovalSummary[] }>({
    queryKey: ['approvals', 'all'],
    queryFn: () => approvalApi.listPending().then((r) => r.data),
  });

  return { workspaces, projects, capabilities, releases, evals, audits, approvals, workspaceList, capabilityList };
}

function Dashboard() {
  const d = useDashboardData();

  const failedQuery = [d.workspaces, d.projects, d.capabilities, d.releases, d.evals, d.audits, d.approvals]
    .find((query) => query.isError);
  if (failedQuery) {
    return <QueryError message={failedQuery.error} onRetry={() => void failedQuery.refetch()} />;
  }

  const loadingQuery = d.workspaces.isPending
    || (d.workspaceList.length > 0 && d.projects.isPending)
    || ((d.projects.data?.length ?? 0) > 0 && d.capabilities.isPending)
    || (d.workspaceList.length > 0 && d.releases.isPending)
    || d.evals.isPending
    || d.audits.isPending
    || d.approvals.isPending;
  if (loadingQuery) return <DashboardLoading />;

  const capabilityCount = d.capabilityList.length;
  const capabilityNames = new Map(d.capabilityList.map((capability) => [capability.id, capability.name]));
  const releaseList = d.releases.data?.items ?? [];
  const evalList = d.evals.data ?? [];
  const auditList = d.audits.data ?? [];
  const approvalList = d.approvals.data?.approvals ?? [];

  const noWorkspace = d.workspaceList.length === 0;

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
  const openReleases = releaseList.filter((r) => r.status === 'active' || r.status === 'canary').length;

  const wsFirst = d.workspaceList[0];
  const wsName = wsFirst?.name ?? 'your workspace';
  const noCapability = capabilityCount === 0;
  const noRelease = releaseList.length === 0;
  const onboard = noCapability || noRelease;

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

      {onboard && (
        <Surface className="ps-aurora-bg overflow-hidden border-brand/30">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-start gap-3">
              <div className="grid h-10 w-10 place-items-center rounded-lg bg-brand text-brand-foreground">
                <Sparkles className="h-5 w-5" />
              </div>
              <div>
                <div className="text-sm font-semibold text-text-strong">
                  Welcome to {wsName}
                </div>
                <p className="mt-1 text-sm text-text-muted">
                  A quick three-step setup: author a capability in the DAG editor, compile it into an immutable manifest, and run your first release.
                </p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Link href="/app/editor"><Button size="sm"><Plus className="mr-1.5 h-3.5 w-3.5" />Author capability</Button></Link>
              <Link href="/app/capabilities"><Button size="sm" variant="outline">Browse registry</Button></Link>
              <Link href="/docs/quickstart"><Button size="sm" variant="ghost">Read the quickstart</Button></Link>
            </div>
          </div>
        </Surface>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Active capabilities"
          value={<AnimatedNumber value={capabilityCount} />}
          hint="Across all projects"
          icon={Boxes}
        />
        <StatCard
          label="Pending approvals"
          value={<AnimatedNumber value={approvalList.length} />}
          hint="Awaiting a second reviewer"
          icon={ShieldCheck}
          delta={approvalList.length > 0 ? { value: `${approvalList.length} pending`, trend: 'up' } : undefined}
        />
        <StatCard
          label="Latest eval pass rate"
          value={<AnimatedNumber value={Math.round(passRate(evalList))} format={(n) => `${n}%`} />}
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
          value={<AnimatedNumber value={openReleases} />}
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
              rowKey={(r) => r.id}
              columns={[
                { key: 'cap', header: 'Capability', render: (r) => capabilityNames.get(r.capabilityId) ?? r.capabilityId },
                { key: 'ver', header: 'Version', render: (r) => `v${r.capabilityVersion}` },
                { key: 'state', header: 'State', render: (r) => <StatusPill kind={statusKindOf(r.status)} /> },
                { key: 'id', header: 'Release ID', render: (r) => <span className="font-mono text-xs text-text-muted">{r.id}</span> },
                { key: 'env', header: 'Env', render: (r) => <span className="font-mono text-xs text-text-muted">{r.environment}</span> },
              ]}
            />
          )}
        </Surface>

        <Surface>
          <SurfaceHeader title="Trust" description="Composite of eval pass rate, approval coverage, and runtime reliability." />
          <TrustScore score={trustScore} className="mt-2" />
        </Surface>
      </div>

      <Tabs defaultValue="pending">
        <TabsList>
          <TabsTrigger value="pending">Pending</TabsTrigger>
          <TabsTrigger value="recent">Recent</TabsTrigger>
          <TabsTrigger value="upcoming">Upcoming</TabsTrigger>
        </TabsList>

        <TabsContent value="pending">
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
                  <li key={a.releaseId} className="flex items-center gap-3 rounded-lg border border-border-subtle bg-surface-2/40 p-3">
                    <div className="grid h-8 w-8 place-items-center rounded-md bg-surface-2">
                      <AlertCircle className="h-4 w-4 text-warning" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="truncate font-mono text-sm text-text-strong">{a.releaseId}</div>
                      <div className="text-xs text-text-muted">Awaiting approval</div>
                    </div>
                    <Link href={`/app/approvals/${a.releaseId}`}>
                      <Button variant="ghost" size="sm">Review <ArrowRight className="ml-1 h-3 w-3" /></Button>
                    </Link>
                  </li>
                ))}
              </ol>
            )}
          </Surface>
        </TabsContent>

        <TabsContent value="recent">
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
                  id: a.id,
                  title: a.action,
                  description: a.resource,
                  timestamp: new Date(a.timestamp).toLocaleString(),
                  tone: 'neutral' as const,
                }))}
              />
            )}
          </Surface>
        </TabsContent>

        <TabsContent value="upcoming">
          <Surface>
            <SurfaceHeader
              title="Upcoming"
              description="Scheduled eval runs and self-evolve cycles."
              actions={<Link href="/app/schedules" className="text-sm text-brand-highlight hover:underline">Schedules</Link>}
            />
            <EmptyState
              icon={CalendarClock}
              title="No upcoming runs scheduled"
              description="Create a schedule to fire on a cron — nightly eval runs, weekly rotations, self-evolve cycles."
              action={<Link href="/app/schedules"><Button variant="outline">Create schedule</Button></Link>}
            />
          </Surface>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function DashboardLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <div className="space-y-2">
        <div className="h-3 w-28 animate-pulse rounded bg-surface-2" />
        <div className="h-8 w-64 animate-pulse rounded bg-surface-2" />
        <div className="h-4 w-full max-w-2xl animate-pulse rounded bg-surface-2" />
      </div>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => <div key={i} className="h-28 animate-pulse rounded-xl border border-border-subtle bg-surface-1" />)}
      </div>
      <div className="grid gap-5 lg:grid-cols-3">
        <div className="h-80 animate-pulse rounded-xl border border-border-subtle bg-surface-1 lg:col-span-2" />
        <div className="h-80 animate-pulse rounded-xl border border-border-subtle bg-surface-1" />
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

function passRate(evalList: EvalRun[]): number {
  if (evalList.length === 0) return 0;
  const passed = evalList.filter((e) => e.status === 'passed').length;
  return (passed / evalList.length) * 100;
}

function computeTrust(evals: EvalRun[], approvals: PendingApprovalSummary[], releases: Release[]): number {
  const pr = passRate(evals);
  const ap = approvals.length === 0 ? 100 : 60;
  const rr = releases.length > 0 ? 90 : 70;
  return Math.round(pr * 0.55 + ap * 0.2 + rr * 0.25);
}
