import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getErrorMessage } from '@/lib/errors';
import { Surface } from './surface';

interface QueryErrorProps {
  message?: unknown;
  onRetry: () => void;
}

function statusOf(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null || !('status' in error)) return undefined;
  const status = error.status;
  return typeof status === 'number' ? status : undefined;
}

/** Consistent recoverable state for failed authenticated data queries. */
export function QueryError({ message, onRetry }: QueryErrorProps) {
  const status = statusOf(message);
  const backendUnavailable = status === undefined || status >= 500;
  return (
    <Surface className="border-destructive/30 bg-destructive/5">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 size-5 shrink-0 text-destructive" aria-hidden="true" />
        <div>
          <h2 className="text-sm font-semibold text-text-strong">Unable to load this view</h2>
          <p className="mt-1 text-sm text-text-muted">
            {backendUnavailable
              ? 'The Promptsheon API is unavailable or encountered an internal error.'
              : getErrorMessage(message, 'We could not load this data right now.')}
          </p>
          {backendUnavailable && (
            <p className="mt-2 text-xs text-text-subtle">
              For local development, run <code className="rounded bg-surface-2 px-1 py-0.5 font-mono">pnpm dev</code> to start the API and console together, then try again.
            </p>
          )}
          <Button className="mt-4" size="sm" variant="outline" onClick={onRetry}>Try again</Button>
        </div>
      </div>
    </Surface>
  );
}
