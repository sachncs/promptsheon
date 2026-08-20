'use client';

import Link from 'next/link';
import * as React from 'react';
import { Github, Menu } from 'lucide-react';
import { Logo } from '@/brand/logo';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export interface TopNavLink {
  label: string;
  href: string;
}

export function TopNav({ links, className }: { links: TopNavLink[]; className?: string | undefined }) {
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
          <Link href="/onboarding">
            <Button variant="ghost" size="sm" className="text-text-default">
              Sign in
            </Button>
          </Link>
          <Link href="/onboarding">
            <Button size="sm">Open dashboard</Button>
          </Link>
          <button
            type="button"
            className="md:hidden grid h-9 w-9 place-items-center rounded-md text-text-muted hover:bg-surface-2"
            aria-label="Menu"
          >
            <Menu className="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>
  );
}
