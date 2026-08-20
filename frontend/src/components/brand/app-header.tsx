'use client';

import * as React from 'react';
import Link from 'next/link';
import { Command, LogOut, Plus, Search } from 'lucide-react';
import { Logo } from '@/brand/logo';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { getSession, clearSession } from '@/lib/session';

export function AppHeader() {
  const [session, setSessionState] = React.useState(() => getSession());

  React.useEffect(() => {
    function onStorage() { setSessionState(getSession()); }
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-border-subtle bg-surface-0/85 px-4 backdrop-blur">
      <Link href="/app" className="md:hidden">
        <Logo size="xs" />
      </Link>
      <div className="relative flex-1 max-w-xl">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-subtle" />
        <Input
          placeholder="Search capabilities, releases, hashes…"
          className="pl-9 pr-12 h-9 bg-surface-1 border-border-subtle focus-visible:ring-brand"
        />
        <kbd className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 rounded border border-border-subtle bg-surface-2 px-1.5 py-0.5 text-[10px] font-medium text-text-subtle">
          <Command className="inline h-2.5 w-2.5 mr-0.5" />K
        </kbd>
      </div>
      <div className="flex items-center gap-2 ml-auto">
        <Link href="/app/capabilities">
          <Button size="sm" variant="default">
            <Plus className="h-3.5 w-3.5 mr-1.5" />New capability
          </Button>
        </Link>
        {session && (
          <button
            type="button"
            onClick={() => { clearSession(); window.location.href = '/'; }}
            className="grid h-9 w-9 place-items-center rounded-md text-text-muted hover:bg-surface-2 hover:text-text-default"
            aria-label="Sign out"
            title="Sign out"
          >
            <LogOut className="h-4 w-4" />
          </button>
        )}
      </div>
    </header>
  );
}
