import type { LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface StepIndicatorItem {
  id: string;
  label: string;
  icon: LucideIcon;
}

export function StepIndicator({
  steps,
  currentIndex,
  className,
}: {
  steps: StepIndicatorItem[];
  currentIndex: number;
  className?: string | undefined;
}) {
  return (
    <ol className={cn('flex w-full', className)}>
      {steps.map((step, i) => {
        const Icon = step.icon;
        const active = i === currentIndex;
        const done = i < currentIndex;
        return (
          <li
            key={step.id}
            className={cn(
              'flex flex-1 items-center gap-3 border-t-2 pt-3 transition-colors',
              active ? 'border-brand text-text-strong' : done ? 'border-success/60 text-text-default' : 'border-border-subtle text-text-subtle',
            )}
          >
            <span
              className={cn(
                'grid h-7 w-7 shrink-0 place-items-center rounded-full text-[11px] font-semibold ring-1',
                active && 'bg-brand/15 text-brand-highlight ring-brand',
                done && 'bg-success/15 text-success ring-success',
                !active && !done && 'bg-surface-1 text-text-subtle ring-border-subtle',
              )}
            >
              {done ? '✓' : i + 1}
            </span>
            <div className="hidden sm:block">
              <div className="text-xs font-medium leading-none">{step.label}</div>
            </div>
            {i < steps.length - 1 && (
              <span aria-hidden="true" className="flex-1 border-t border-border-subtle" />
            )}
          </li>
        );
      })}
    </ol>
  );
}
