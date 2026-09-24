'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Command, LogOut, Menu, Plus, Search } from 'lucide-react';
import { Logo } from '@/brand/logo';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ThemeToggle } from '@/components/theme/theme-toggle';
import { clearSession } from '@/lib/session';
import { useSession } from '@/hooks/use-session';

export function AppHeader({ onMenu }: { onMenu?: (() => void) | undefined }) {
  const session = useSession();
  const router = useRouter();
  const [query, setQuery] = useState('');
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const submitSearch = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = query.trim();
    if (value.length >= 2) router.push(`/app/search?q=${encodeURIComponent(value)}`);
  };

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-border-subtle bg-surface-0/85 px-4 backdrop-blur">
      <button
        type="button"
        onClick={onMenu}
        className="grid size-9 place-items-center rounded-md text-text-muted hover:bg-surface-2 hover:text-text-default md:hidden"
        aria-label="Open navigation"
      >
        <Menu className="size-4" />
      </button>
      <Link href="/app" className="md:hidden">
        <Logo size="xs" />
      </Link>
      <form className="relative flex-1 max-w-xl" onSubmit={submitSearch} role="search">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-subtle" />
        <Input
          ref={searchRef}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search capabilities, releases, hashes…"
          aria-label="Search capabilities, releases, and hashes"
          className="pl-9 pr-12 h-9 bg-surface-1 border-border-subtle focus-visible:ring-brand"
        />
        <kbd className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 rounded border border-border-subtle bg-surface-2 px-1.5 py-0.5 text-[10px] font-medium text-text-subtle">
          <Command className="inline h-2.5 w-2.5 mr-0.5" />K
        </kbd>
      </form>
      <div className="flex items-center gap-2 ml-auto">
        <ThemeToggle className="text-text-muted hover:text-text-default" />
        <Link href="/app/capabilities" className="sm:hidden" aria-label="New capability">
          <Button size="icon" variant="default">
            <Plus className="h-4 w-4" />
            <span className="sr-only">New capability</span>
          </Button>
        </Link>
        <Link href="/app/capabilities" className="hidden sm:inline-flex">
          <Button size="sm" variant="default">
            <Plus className="h-3.5 w-3.5 mr-1.5" />New capability
          </Button>
        </Link>
        {session && (
          <button
            type="button"
            onClick={() => { clearSession(); router.push('/'); }}
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
