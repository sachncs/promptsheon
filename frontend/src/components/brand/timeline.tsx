import { cn } from '@/lib/utils';
import type { LucideIcon } from 'lucide-react';

export interface TimelineEntry {
  id: string;
  title: string;
  description?: string | undefined;
  actor?: string | undefined;
  timestamp: string;
  icon?: LucideIcon | undefined;
  tone?: 'neutral' | 'info' | 'success' | 'warning' | 'danger' | undefined;
}

const toneClass: Record<NonNullable<TimelineEntry['tone']>, string> = {
  neutral: 'bg-surface-2 text-text-muted ring-border-subtle',
  info: 'bg-info/15 text-info ring-info/30',
  success: 'bg-success/15 text-success ring-success/30',
  warning: 'bg-warning/15 text-warning ring-warning/30',
  danger: 'bg-destructive/15 text-destructive ring-destructive/30',
};

export function Timeline({
  entries,
  className,
}: {
  entries: TimelineEntry[];
  className?: string;
}) {
  return (
    <ol className={cn('relative space-y-4 border-l border-border-subtle pl-6', className)}>
      {entries.map((entry) => {
        const Icon = entry.icon;
        const tone = entry.tone ?? 'neutral';
        return (
          <li key={entry.id} className="relative">
            <span
              className={cn(
                'absolute -left-[29px] grid h-6 w-6 place-items-center rounded-full ring-1',
                toneClass[tone],
              )}
              aria-hidden="true"
            >
              {Icon ? <Icon className="h-3 w-3" /> : <span className="h-1.5 w-1.5 rounded-full bg-current" />}
            </span>
            <div className="text-sm font-medium text-text-strong">{entry.title}</div>
            {entry.description && (
              <div className="mt-0.5 text-sm text-text-muted">{entry.description}</div>
            )}
            <div className="mt-1 flex items-center gap-2 text-xs text-text-subtle">
              {entry.actor && <span>{entry.actor}</span>}
              <span aria-hidden="true">·</span>
              <time>{entry.timestamp}</time>
            </div>
          </li>
        );
      })}
    </ol>
  );
}
