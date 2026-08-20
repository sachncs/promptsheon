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
    <div className={cn('flex items-start justify-between gap-6', className)}>
      <div>
        {eyebrow && (
          <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">{eyebrow}</div>
        )}
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-text-strong">{title}</h1>
        {subtitle && <p className="mt-1.5 max-w-3xl text-sm text-text-muted">{subtitle}</p>}
      </div>
      {actions}
    </div>
  );
}
