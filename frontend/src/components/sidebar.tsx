'use client';

import * as React from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
  LayoutDashboard, FolderOpen, Layers, Activity, Settings, Bell, Flag, Webhook,
  Calendar, Users, KeyRound, History, BookOpen, TestTube, ChevronDown, ChevronRight,
  AlertTriangle, Target, FileCode,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { Separator } from '@/components/ui/separator';

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
    items: [
      { href: '/', label: 'Dashboard', icon: LayoutDashboard },
    ],
  },
  {
    label: 'Development',
    items: [
      { href: '/workspaces', label: 'Workspaces', icon: FolderOpen },
      { href: '/compiler', label: 'Compiler', icon: BookOpen },
      { href: '/editor', label: 'Manifest Editor', icon: FileCode },
      { href: '/feature-flags', label: 'Feature Flags', icon: Flag },
    ],
  },
  {
    label: 'Operations',
    items: [
      { href: '/operations', label: 'Operations Hub', icon: Activity },
      { href: '/goals', label: 'Goals', icon: Target },
      { href: '/alerts/rules', label: 'Alert Rules', icon: Bell },
      { href: '/alerts/active', label: 'Active Alerts', icon: AlertTriangle },
      { href: '/schedules', label: 'Schedules', icon: Calendar },
      { href: '/webhooks', label: 'Webhooks', icon: Webhook },
    ],
  },
  {
    label: 'Testing',
    items: [
      { href: '/eval', label: 'Eval Runs', icon: TestTube },
    ],
  },
  {
    label: 'Admin',
    items: [
      { href: '/users', label: 'Users', icon: Users },
      { href: '/api-keys', label: 'API Keys', icon: KeyRound },
      { href: '/audit', label: 'Audit Log', icon: History },
      { href: '/settings', label: 'Settings', icon: Settings },
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
    <div>
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between px-3 py-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground"
      >
        <span>{group.label}</span>
        {open ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
      </button>
      {open && (
        <nav className="space-y-1">
          {group.items.map((item) => {
            const Icon = item.icon;
            const isActive = pathname === item.href || pathname.startsWith(item.href + '/');
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors hover:bg-accent',
                  isActive ? 'bg-accent text-accent-foreground' : 'text-muted-foreground',
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>
      )}
    </div>
  );
}

export function Sidebar() {
  return (
    <aside className="w-64 border-r bg-muted/30 flex flex-col">
      <div className="p-4 border-b">
        <h1 className="text-lg font-bold tracking-tight">Promptsheon</h1>
        <p className="text-xs text-muted-foreground">Prompt Management</p>
      </div>
      <div className="flex-1 overflow-y-auto p-2 space-y-3">
        {groups.map((group) => (
          <NavGroupSection key={group.label} group={group} />
        ))}
      </div>
      <Separator />
      <div className="p-4 text-xs text-muted-foreground">
        <Layers className="inline h-3 w-3 mr-1" />
        v0.1.0
      </div>
    </aside>
  );
}