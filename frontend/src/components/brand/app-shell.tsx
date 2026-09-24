'use client';

import * as React from 'react';
import { AppSidebar } from './app-sidebar';
import { AppHeader } from './app-header';

export function AppShell({ children }: { children: React.ReactNode }) {
  const [mobileNavOpen, setMobileNavOpen] = React.useState(false);

  return (
    <div className="flex h-screen w-full bg-surface-0 text-foreground">
      <AppSidebar mobileOpen={mobileNavOpen} onMobileClose={() => setMobileNavOpen(false)} />
      <div className="flex min-w-0 flex-1 flex-col">
        <AppHeader onMenu={() => setMobileNavOpen(true)} />
        <a
          href="#main-content"
          className="sr-only z-50 rounded-md bg-surface-1 px-3 py-2 text-sm text-text-strong focus:not-sr-only focus:absolute focus:left-4 focus:top-16 focus:ring-2 focus:ring-brand"
        >
          Skip to main content
        </a>
        <main id="main-content" tabIndex={-1} className="flex-1 overflow-y-auto outline-none">
          <div className="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 sm:py-8">{children}</div>
        </main>
      </div>
    </div>
  );
}
