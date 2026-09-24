'use client';

import Link from 'next/link';
import * as React from 'react';
import { Github, Menu } from 'lucide-react';
import { Logo } from '@/brand/logo';
import { Button } from '@/components/ui/button';
import { ThemeToggle } from '@/components/theme/theme-toggle';
import { cn } from '@/lib/utils';

export interface TopNavLink {
  label: string;
  href: string;
}

export function TopNav({ links, className }: { links: TopNavLink[]; className?: string | undefined }) {
  const [mobileOpen, setMobileOpen] = React.useState(false);

  return (
    <header
      className={cn(
        'sticky top-0 z-40 w-full border-b border-border-subtle bg-surface-0/80 backdrop-blur',
        className,
      )}
    >
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-6">
        <div className="flex items-center gap-8">
          <Link href="/" aria-label="Promptsheon home">
            <Logo size="sm" />
          </Link>
          <nav className="hidden md:flex items-center gap-1">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="rounded-md px-3 py-1.5 text-sm text-text-muted hover:bg-surface-2 hover:text-text-default transition-colors"
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex items-center gap-2">
          <a
            href="https://github.com/sachncs/promptsheon"
            target="_blank"
            rel="noreferrer"
            className="hidden sm:inline-flex h-9 w-9 items-center justify-center rounded-md text-text-muted hover:bg-surface-2 hover:text-text-default"
            aria-label="GitHub"
          >
            <Github className="h-4 w-4" />
          </a>
          <ThemeToggle className="text-text-muted hover:text-text-default" />
          <Link href="/onboarding" className="hidden sm:inline-flex">
            <Button variant="ghost" size="sm" className="text-text-default">
              Sign in
            </Button>
          </Link>
          <Link href="/onboarding" className="hidden sm:inline-flex">
            <Button size="sm">Open dashboard</Button>
          </Link>
          <button
            type="button"
            onClick={() => setMobileOpen((open) => !open)}
            className="md:hidden grid h-9 w-9 place-items-center rounded-md text-text-muted hover:bg-surface-2"
            aria-label="Menu"
            aria-controls="mobile-navigation"
            aria-expanded={mobileOpen}
          >
            <Menu className="h-4 w-4" />
          </button>
        </div>
      </div>
      {mobileOpen && (
        <nav
          id="mobile-navigation"
          aria-label="Mobile navigation"
          className="border-t border-border-subtle bg-surface-0 px-6 py-3 md:hidden"
        >
          <div className="mx-auto flex max-w-6xl flex-col gap-1">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setMobileOpen(false)}
                className="rounded-md px-3 py-2.5 text-sm text-text-muted hover:bg-surface-2 hover:text-text-default"
              >
                {link.label}
              </Link>
            ))}
            <div className="mt-2 flex gap-2 border-t border-border-subtle pt-3">
              <Link href="/onboarding" onClick={() => setMobileOpen(false)} className="flex-1">
                <Button variant="ghost" className="w-full">Sign in</Button>
              </Link>
              <Link href="/onboarding" onClick={() => setMobileOpen(false)} className="flex-1">
                <Button className="w-full">Open dashboard</Button>
              </Link>
            </div>
          </div>
        </nav>
      )}
    </header>
  );
}
