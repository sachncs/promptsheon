import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Surface } from './surface';

interface QueryErrorProps {
  message?: string;
  onRetry: () => void;
}

/** Consistent recoverable state for failed authenticated data queries. */
export function QueryError({ message = 'We could not load this data right now.', onRetry }: QueryErrorProps) {
  return (
    <Surface className="border-destructive/30 bg-destructive/5">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 size-5 shrink-0 text-destructive" aria-hidden="true" />
        <div>
          <h2 className="text-sm font-semibold text-text-strong">Unable to load this view</h2>
          <p className="mt-1 text-sm text-text-muted">{message}</p>
          <Button className="mt-4" size="sm" variant="outline" onClick={onRetry}>Try again</Button>
        </div>
      </div>
    </Surface>
  );
}
