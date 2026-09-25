'use client';

import { useEffect } from 'react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Surface } from '@/components/brand/surface';

/** Recoverable boundary for failures inside the authenticated product shell. */
export default function AppError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error('Authenticated application error', error);
  }, [error]);

  return (
    <div className="mx-auto flex min-h-[60vh] max-w-2xl items-center px-6 py-12">
      <Surface className="w-full border-destructive/30 bg-destructive/5">
        <div className="flex items-start gap-3">
          <AlertTriangle className="mt-0.5 size-5 shrink-0 text-destructive" aria-hidden="true" />
          <div>
            <h1 className="text-lg font-semibold text-text-strong">This view could not be rendered</h1>
            <p className="mt-2 text-sm text-text-muted">Your data is safe. Retry the view or use the sidebar to continue.</p>
            <Button className="mt-5" onClick={reset}>Retry view</Button>
          </div>
        </div>
      </Surface>
    </div>
  );
}
