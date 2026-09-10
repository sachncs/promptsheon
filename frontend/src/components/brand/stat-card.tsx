import * as React from 'react';
import { cn } from '@/lib/utils';
import type { LucideIcon } from 'lucide-react';

export interface StatCardProps {
  label: string;
  value: React.ReactNode;
  hint?: string | undefined;
  delta?: { value: string; trend: 'up' | 'down' | 'flat' } | undefined;
  icon?: LucideIcon | undefined;
  className?: string | undefined;
}

export function StatCard({ label, value, hint, delta, icon: Icon, className }: StatCardProps) {
  return (
    <div
      className={cn(
        'group rounded-xl border border-border-subtle bg-surface-1 p-5 shadow-1 transition-colors hover:border-border-strong',
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs font-medium uppercase tracking-wider text-text-muted">{label}</div>
          <div className="mt-3 text-3xl font-semibold tracking-tight text-text-strong">{value}</div>
          {hint && <div className="mt-1.5 text-sm text-text-muted">{hint}</div>}
        </div>
        {Icon && (
          <div className="grid h-9 w-9 place-items-center rounded-lg border border-border-subtle bg-surface-2 text-text-muted group-hover:border-brand">
            <Icon className="h-4 w-4" aria-hidden="true" />
          </div>
        )}
      </div>
      {delta && (
        <div className="mt-4 flex items-center gap-1.5 text-xs">
          <span
            className={cn(
              'inline-flex items-center rounded-full px-2 py-0.5 font-medium',
              delta.trend === 'up' && 'bg-success/10 text-success',
              delta.trend === 'down' && 'bg-destructive/10 text-destructive',
              delta.trend === 'flat' && 'bg-surface-2 text-text-muted',
            )}
          >
            {delta.value}
          </span>
          {hint && <span className="text-text-subtle">vs. baseline</span>}
        </div>
      )}
    </div>
  );
}
