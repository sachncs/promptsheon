import { cn } from '@/lib/utils';
import { StatusPill, type StatusKind } from './status-pill';

export interface Step {
  id: string;
  label: string;
  description?: string | undefined;
  status: StatusKind;
}

export function StepRail({ steps, current, className }: { steps: Step[]; current: string; className?: string }) {
  const idx = Math.max(
    0,
    steps.findIndex((s) => s.id === current),
  );

  return (
    <ol className={cn('flex flex-col gap-0', className)}>
      {steps.map((step, i) => {
        const isComplete = i < idx;
        const isActive = i === idx;
        return (
          <li key={step.id} className="flex gap-4 pb-6 last:pb-0">
            <div className="flex flex-col items-center">
              <span
                className={cn(
                  'grid h-8 w-8 place-items-center rounded-full border text-xs font-semibold',
                  isComplete && 'border-success bg-success/10 text-success',
                  isActive && 'border-brand bg-brand/10 text-brand-highlight',
                  !isComplete && !isActive && 'border-border-subtle bg-surface-1 text-text-subtle',
                )}
              >
                {isComplete ? '✓' : i + 1}
              </span>
              {i < steps.length - 1 && (
                <span
                  className={cn(
                    'mt-1 h-full min-h-[24px] w-px',
                    i < idx ? 'bg-success/50' : 'bg-border-subtle',
                  )}
                  aria-hidden="true"
                />
              )}
            </div>
            <div className="flex-1 pt-1">
              <div className="flex items-center gap-2">
                <div className="text-sm font-medium text-text-strong">{step.label}</div>
                <StatusPill kind={step.status} />
              </div>
              {step.description && (
                <div className="mt-1 text-sm text-text-muted">{step.description}</div>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
