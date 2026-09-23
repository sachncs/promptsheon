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
        <main className="flex-1 overflow-y-auto">
          <div className="mx-auto w-full max-w-7xl px-6 py-8">{children}</div>
        </main>
      </div>
    </div>
  );
}
