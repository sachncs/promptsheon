'use client';

import { useEffect } from 'react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Surface } from '@/components/brand/surface';

/** Recoverable boundary for unexpected App Router render failures. */
export default function RootError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('Unhandled application error', error);
  }, [error]);

  return (
    <main className="flex min-h-screen items-center justify-center bg-surface-0 p-6">
      <Surface className="w-full max-w-lg border-destructive/30 bg-destructive/5">
        <div className="flex items-start gap-3">
          <AlertTriangle className="mt-0.5 size-5 shrink-0 text-destructive" aria-hidden="true" />
          <div>
            <h1 className="text-lg font-semibold text-text-strong">Something went wrong</h1>
            <p className="mt-2 text-sm text-text-muted">
              The application hit an unexpected error. Retry the view, or reload the page if the problem persists.
            </p>
            <Button className="mt-5" onClick={reset}>Try again</Button>
          </div>
        </div>
      </Surface>
    </main>
  );
}
