import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Surface } from './surface';

interface QueryErrorProps {
  message?: unknown;
  onRetry: () => void;
}

/** Convert an arbitrary request failure into a safe user-facing message. */
export function getErrorMessage(error: unknown, fallback = 'We could not load this data right now.'): string {
  if (error instanceof Error && error.message.trim().length > 0) return error.message;
  if (typeof error === 'object' && error !== null && 'message' in error) {
    const message = error.message;
    if (typeof message === 'string' && message.trim().length > 0) return message;
  }
  if (typeof error === 'string' && error.trim().length > 0) return error;
  return fallback;
}

/** Consistent recoverable state for failed authenticated data queries. */
export function QueryError({ message, onRetry }: QueryErrorProps) {
  return (
    <Surface className="border-destructive/30 bg-destructive/5">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 size-5 shrink-0 text-destructive" aria-hidden="true" />
        <div>
          <h2 className="text-sm font-semibold text-text-strong">Unable to load this view</h2>
          <p className="mt-1 text-sm text-text-muted">{getErrorMessage(message)}</p>
          <Button className="mt-4" size="sm" variant="outline" onClick={onRetry}>Try again</Button>
        </div>
      </div>
    </Surface>
  );
}
