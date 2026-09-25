import { cn } from '@/lib/utils';

export function PageHeader({
  eyebrow,
  title,
  subtitle,
  actions,
  className,
}: {
  eyebrow?: string | undefined;
  title: string;
  subtitle?: string | undefined;
  actions?: React.ReactNode;
  className?: string | undefined;
}) {
  return (
    <div className={cn('flex flex-col items-stretch gap-3 sm:flex-row sm:items-start sm:justify-between sm:gap-6', className)}>
      <div className="min-w-0">
        {eyebrow && (
          <div className="text-micro font-semibold uppercase tracking-[0.16em] text-text-subtle">{eyebrow}</div>
        )}
        <h1 className="mt-2 font-semibold text-h2 text-text-strong">{title}</h1>
        {subtitle && <p className="mt-2 max-w-3xl text-sm leading-relaxed text-text-muted">{subtitle}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2 sm:shrink-0">{actions}</div>}
    </div>
  );
}
