'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import * as React from 'react';
import {
  LayoutDashboard, FolderOpen, Boxes, Workflow, Compass, Flag,
  GitBranch, Target, Activity, CalendarClock, FlaskConical,
  ShieldCheck, ScrollText, Users, KeyRound, Webhook, Cog, Search, GitMerge, ListChecks,
  ChevronDown, ChevronRight,
} from 'lucide-react';
import { Logo } from '@/brand/logo';
import { Separator } from '@/components/ui/separator';
import { getSession } from '@/lib/session';
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
    label: 'Capabilities',
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
    label: 'Quality',
    items: [
      { href: '/app/eval', label: 'Eval runs', icon: FlaskConical },
      { href: '/app/eval/suites', label: 'Suites', icon: ListChecks },
      { href: '/app/approvals', label: 'Approvals', icon: ShieldCheck },
      { href: '/app/audit', label: 'Audit log', icon: ScrollText },
    ],
  },
  {
    label: 'Admin',
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

function NavGroupSection({ group }: { group: NavGroup }) {
  const pathname = usePathname();
  const hasActive = group.items.some(
    (item) => pathname === item.href || pathname.startsWith(item.href + '/'),
  );
  const [open, setOpen] = React.useState(hasActive);

  return (
    <div className="py-2">
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
                    ? 'bg-surface-2 text-text-strong ring-1 ring-border-subtle'
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
  return (
    <aside className="hidden md:flex w-60 shrink-0 flex-col border-r border-border-subtle bg-surface-1">
      <div className="flex h-14 items-center border-b border-border-subtle px-4">
        <Link href="/app" aria-label="Promptsheon home">
          <Logo size="sm" />
        </Link>
      </div>
      <div className="flex-1 overflow-y-auto py-2">
        {groups.map((group) => (
          <NavGroupSection key={group.label} group={group} />
        ))}
      </div>
      <Separator />
      <div className="px-4 py-3">
        {session ? (
          <div>
            <div className="text-[13px] font-medium text-text-strong">{session.userName}</div>
            <div className="text-xs text-text-subtle">{session.orgName}</div>
          </div>
        ) : (
          <Link href="/onboarding" className="text-xs text-brand-highlight hover:underline">
            Set up workspace
          </Link>
        )}
      </div>
    </aside>
  );
}
