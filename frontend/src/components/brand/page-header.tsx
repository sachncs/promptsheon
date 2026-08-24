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
          <div className="text-micro font-semibold uppercase tracking-[0.16em] text-text-subtle">{eyebrow}</div>
        )}
        <h1 className="mt-2 font-semibold text-h2 text-text-strong">{title}</h1>
        {subtitle && <p className="mt-2 max-w-3xl text-sm leading-relaxed text-text-muted">{subtitle}</p>}
      </div>
      {actions}
    </div>
  );
}
