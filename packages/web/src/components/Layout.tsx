import { Outlet, Link, useLocation } from 'react-router-dom';
import { useState } from 'react';
import { cn } from '@/lib/utils';
import {
  LayoutDashboard, FolderOpen, Layers, Rocket, Activity, Settings, Bell, Database,
  Zap, Users, KeyRound, Flag, Webhook, Calendar, FileText, GitBranch, Shield,
  ChevronDown, ChevronRight, ArrowLeftRight, BookOpen, TestTube, History,
} from 'lucide-react';

interface NavItem {
  to: string;
  label: string;
  icon: typeof LayoutDashboard;
}

interface NavGroup {
  label: string;
  items: NavItem[];
}

const groups: NavGroup[] = [
  {
    label: 'Overview',
    items: [
      { to: '/', label: 'Dashboard', icon: LayoutDashboard },
    ],
  },
  {
    label: 'Development',
    items: [
      { to: '/workspaces', label: 'Workspaces', icon: FolderOpen },
      { to: '/capabilities', label: 'Capabilities', icon: Layers },
      { to: '/compiler', label: 'Compiler', icon: BookOpen },
      { to: '/feature-flags', label: 'Feature Flags', icon: Flag },
    ],
  },
  {
    label: 'Operations',
    items: [
      { to: '/operations', label: 'Operations Hub', icon: Activity },
      { to: '/alerts/rules', label: 'Alert Rules', icon: Bell },
      { to: '/alerts/active', label: 'Active Alerts', icon: Shield },
      { to: '/schedules', label: 'Schedules', icon: Calendar },
      { to: '/webhooks', label: 'Webhooks', icon: Webhook },
    ],
  },
  {
    label: 'Testing',
    items: [
      { to: '/capabilities', label: 'Eval Runs', icon: TestTube },
    ],
  },
  {
    label: 'Admin',
    items: [
      { to: '/users', label: 'Users', icon: Users },
      { to: '/api-keys', label: 'API Keys', icon: KeyRound },
      { to: '/audit', label: 'Audit Log', icon: History },
      { to: '/settings', label: 'Settings', icon: Settings },
    ],
  },
];

function NavGroupSection({ group }: { group: NavGroup }) {
  const location = useLocation();
  const hasActive = group.items.some((item) => location.pathname === item.to ||
    location.pathname.startsWith(item.to + '/'));
  const [open, setOpen] = useState(hasActive);

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
          {group.items.map((item) => (
            <Link
              key={item.to}
              to={item.to}
              className={cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors hover:bg-accent',
                location.pathname === item.to ? 'bg-accent text-accent-foreground' : 'text-muted-foreground',
              )}
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </Link>
          ))}
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
    </aside>
  );
}

export function Layout() {
  return (
    <div className="flex h-screen">
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-hidden">
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
