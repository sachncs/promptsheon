import { cn } from '@/lib/utils';

export function Surface({
  children,
  className,
  padded = true,
}: {
  children: React.ReactNode;
  className?: string | undefined;
  padded?: boolean;
}) {
  return (
    <section
      className={cn(
        'rounded-xl border border-border-subtle bg-surface-1 shadow-1',
        padded && 'p-5',
        className,
      )}
    >
      {children}
    </section>
  );
}

export function SurfaceHeader({
  title,
  description,
  actions,
  className,
}: {
  title: string;
  description?: string | React.ReactNode | undefined;
  actions?: React.ReactNode;
  className?: string | undefined;
}) {
  return (
    <header className={cn('flex items-start justify-between gap-4 mb-5', className)}>
      <div className="min-w-0">
        <h2 className="text-base font-semibold tracking-tight text-text-strong">{title}</h2>
        {description && <p className="mt-1 text-sm text-text-muted">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </header>
  );
}
