'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import * as React from 'react';
import {
  LayoutDashboard, FolderOpen, Boxes, Workflow, Compass, Flag,
  GitBranch, Target, Activity, CalendarClock, FlaskConical,
  ShieldCheck, ScrollText, Users, KeyRound, Webhook, Cog, Search, GitMerge, ListChecks,
  ChevronDown, ChevronRight, ChevronUp, LogOut, Monitor, Moon, Sun,
} from 'lucide-react';
import { Logo } from '@/brand/logo';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Avatar } from '@/components/ui/avatar';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { workspaceApi } from '@/lib/api';
import { clearSession, getSession } from '@/lib/session';
import { useTheme } from '@/components/theme/theme-provider';
import { cn } from '@/lib/utils';

interface NavItem {
  href: string;
  label: string;
  icon: React.ElementType;
}

interface NavGroup {
  label: string;
  items: NavItem[];
}

const groups: NavGroup[] = [
  {
    label: 'Overview',
    items: [{ href: '/app', label: 'Control plane', icon: LayoutDashboard }],
  },
  {
    label: 'Repositories',
    items: [
      { href: '/app/repos', label: 'Repositories', icon: GitBranch },
      { href: '/app/merge-requests', label: 'Merge requests', icon: GitMerge },
      { href: '/app/search', label: 'Search', icon: Search },
    ],
  },
  {
    label: 'Build',
    items: [
      { href: '/app/capabilities', label: 'Registry', icon: Boxes },
      { href: '/app/editor', label: 'DAG editor', icon: Workflow },
      { href: '/app/compiler', label: 'Compiler', icon: Compass },
      { href: '/app/feature-flags', label: 'Feature flags', icon: Flag },
    ],
  },
  {
    label: 'Release',
    items: [
      { href: '/app/releases', label: 'Releases', icon: GitBranch },
      { href: '/app/goals', label: 'Goals', icon: Target },
      { href: '/app/operations', label: 'Operations', icon: Activity },
      { href: '/app/schedules', label: 'Schedules', icon: CalendarClock },
    ],
  },
  {
    label: 'Observability',
    items: [
      { href: '/app/traces', label: 'Traces', icon: Activity },
      { href: '/app/audit', label: 'Audit log', icon: ScrollText },
    ],
  },
  {
    label: 'Quality',
    items: [
      { href: '/app/eval', label: 'Eval runs', icon: FlaskConical },
      { href: '/app/eval/suites', label: 'Suites', icon: ListChecks },
      { href: '/app/approvals', label: 'Approvals', icon: ShieldCheck },
    ],
  },
  {
    label: 'Settings',
    items: [
      { href: '/app/admin/cost', label: 'Cost & analytics', icon: Activity },
      { href: '/app/vault', label: 'Vault', icon: KeyRound },
      { href: '/app/workspaces', label: 'Workspaces', icon: FolderOpen },
      { href: '/app/users', label: 'Users', icon: Users },
      { href: '/app/api-keys', label: 'API keys', icon: KeyRound },
      { href: '/app/webhooks', label: 'Webhooks', icon: Webhook },
      { href: '/app/settings', label: 'Settings', icon: Cog },
    ],
  },
];

function NavGroupSection({ group, last }: { group: NavGroup; last?: boolean }) {
  const pathname = usePathname();
  const hasActive = group.items.some(
    (item) => pathname === item.href || pathname.startsWith(item.href + '/'),
  );
  const [open, setOpen] = React.useState(hasActive);

  return (
    <div className={cn('py-2', !last && 'border-b border-border-subtle')}>
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-text-subtle hover:text-text-muted"
      >
        <span>{group.label}</span>
        {open ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
      </button>
      {open && (
        <nav className="space-y-0.5 px-2">
          {group.items.map((item) => {
            const Icon = item.icon;
            const isActive = pathname === item.href || pathname.startsWith(item.href + '/');
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium transition-colors',
                  isActive
                    ? 'bg-brand-50 text-brand-700 ring-1 ring-brand-200 dark:bg-surface-2 dark:text-text-strong dark:ring-border-subtle'
                    : 'text-text-muted hover:bg-surface-2/60 hover:text-text-default',
                )}
              >
                <Icon className="h-4 w-4 opacity-70" />
                {item.label}
              </Link>
            );
          })}
        </nav>
      )}
    </div>
  );
}

export function AppSidebar() {
  const session = getSession();
  const router = useRouter();
  const [userOpen, setUserOpen] = React.useState(false);
  const { theme, setTheme } = useTheme();
  const userMenuRef = React.useRef<HTMLDivElement | null>(null);

  const workspaces = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1).then((r) => r.data).catch(() => []),
  });
  const workspaceList = Array.isArray(workspaces.data) ? workspaces.data as Array<{ id: string; name?: string }> : [];
  const currentWsId = session?.orgId ?? workspaceList[0]?.id;
  const currentWs = workspaceList.find((w) => w.id === currentWsId);

  React.useEffect(() => {
    function onClick(e: MouseEvent) {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target as Node)) {
        setUserOpen(false);
      }
    }
    document.addEventListener('mousedown', onClick);
    return () => document.removeEventListener('mousedown', onClick);
  }, []);

  return (
    <aside className="hidden md:flex w-64 shrink-0 flex-col border-r border-border-subtle bg-surface-1">
      <div className="flex h-14 items-center border-b border-border-subtle px-4">
        <Link href="/app" aria-label="Promptsheon home">
          <Logo size="sm" />
        </Link>
      </div>

      {workspaceList.length > 0 && (
        <div className="border-b border-border-subtle p-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.08em] text-text-subtle">Workspace</div>
          <div className="mt-2">
            <ThemedSelect
              value={currentWsId}
              onValueChange={(id) => {
                router.push(`/app/workspaces/${id}`);
              }}
              options={workspaceList.map((w) => ({ value: w.id, label: w.name ?? w.id.slice(0, 8) }))}
              ariaLabel="Active workspace"
              triggerClassName="w-full text-sm"
            />
          </div>
          {currentWs?.name && (
            <div className="mt-2 flex items-center justify-between text-xs text-text-subtle">
              <span className="truncate">{workspaceList.length} workspace(s)</span>
              <Badge>Self-hosted</Badge>
            </div>
          )}
        </div>
      )}

      <div className="flex-1 overflow-y-auto">
        {groups.map((group, i) => (
          <NavGroupSection key={group.label} group={group} last={i === groups.length - 1} />
        ))}
      </div>

      <Separator />
      <div className="relative px-4 py-3" ref={userMenuRef}>
        {session ? (
          <button
            type="button"
            onClick={() => setUserOpen((v) => !v)}
            className="flex w-full items-center gap-3 rounded-md px-1 py-1 text-left hover:bg-surface-2"
          >
            <Avatar className="h-8 w-8 bg-brand text-brand-foreground text-xs">
              {session.userName?.slice(0, 2).toUpperCase() ?? 'U'}
            </Avatar>
            <div className="flex-1 min-w-0">
              <div className="truncate text-[13px] font-medium text-text-strong">{session.userName}</div>
              <div className="truncate text-xs text-text-subtle">{session.orgName}</div>
            </div>
            {userOpen ? <ChevronDown className="h-3 w-3 text-text-subtle" /> : <ChevronUp className="h-3 w-3 text-text-subtle" />}
          </button>
        ) : (
          <Link href="/onboarding" className="text-xs text-brand-highlight hover:underline">
            Set up workspace
          </Link>
        )}
        {session && userOpen && (
          <div className="absolute bottom-full left-3 right-3 mb-2 rounded-xl border border-border-subtle bg-surface-1 p-2 shadow-3">
            <div className="px-2 py-1 text-xs text-text-subtle">Theme</div>
            <div className="mt-1 grid grid-cols-3 gap-1">
              <button
                type="button"
                onClick={() => setTheme('light')}
                className={cn(
                  'flex flex-col items-center gap-1 rounded-md py-2 text-xs transition-colors',
                  theme === 'light' ? 'bg-brand-50 text-brand-700 dark:bg-surface-2 dark:text-text-strong' : 'text-text-muted hover:bg-surface-2',
                )}
              >
                <Sun className="size-3.5" /> Light
              </button>
              <button
                type="button"
                onClick={() => setTheme('dark')}
                className={cn(
                  'flex flex-col items-center gap-1 rounded-md py-2 text-xs transition-colors',
                  theme === 'dark' ? 'bg-brand-50 text-brand-700 dark:bg-surface-2 dark:text-text-strong' : 'text-text-muted hover:bg-surface-2',
                )}
              >
                <Moon className="size-3.5" /> Dark
              </button>
              <button
                type="button"
                onClick={() => {
                  const next = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
                  setTheme(next);
                }}
                className="flex flex-col items-center gap-1 rounded-md py-2 text-xs text-text-muted hover:bg-surface-2"
              >
                <Monitor className="size-3.5" /> System
              </button>
            </div>
            <div className="my-2 border-t border-border-subtle" />
            <button
              type="button"
              onClick={() => { clearSession(); window.location.href = '/'; }}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs text-text-muted hover:bg-surface-2 hover:text-text-strong"
            >
              <LogOut className="size-3.5" /> Sign out
            </button>
          </div>
        )}
      </div>
    </aside>
  );
}