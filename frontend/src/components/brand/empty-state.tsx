import { cn } from '@/lib/utils';
import type { LucideIcon } from 'lucide-react';

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: {
  icon?: LucideIcon | undefined;
  title: string;
  description: string;
  action?: React.ReactNode;
  className?: string | undefined;
}) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center rounded-2xl border border-dashed border-border-subtle bg-surface-1/60 px-8 py-14 text-center',
        className,
      )}
    >
      {Icon && (
        <div className="mb-4 grid h-10 w-10 place-items-center rounded-xl border border-border-subtle bg-surface-2 text-text-muted">
          <Icon className="h-5 w-5" aria-hidden="true" />
        </div>
      )}
      <h3 className="text-sm font-semibold text-text-strong">{title}</h3>
      <p className="mt-1 max-w-md text-sm text-text-muted">{description}</p>
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
