import { cn } from '@/lib/utils';

export type StatusKind =
  | 'draft'
  | 'review'
  | 'approved'
  | 'canary'
  | 'active'
  | 'rolled-back'
  | 'rejected'
  | 'pending'
  | 'error'
  | 'neutral';

const styles: Record<StatusKind, { dot: string; ring: string; text: string; label: string }> = {
  draft: { dot: 'bg-text-subtle', ring: 'bg-text-subtle/15', text: 'text-text-muted', label: 'Draft' },
  review: { dot: 'bg-warning', ring: 'bg-warning/15', text: 'text-warning', label: 'In review' },
  approved: { dot: 'bg-info', ring: 'bg-info/15', text: 'text-info', label: 'Approved' },
  canary: { dot: 'bg-brand', ring: 'bg-brand/15', text: 'text-brand-highlight', label: 'Canary' },
  active: { dot: 'bg-success', ring: 'bg-success/15', text: 'text-success', label: 'Active' },
  'rolled-back': { dot: 'bg-warning', ring: 'bg-warning/15', text: 'text-warning', label: 'Rolled back' },
  rejected: { dot: 'bg-destructive', ring: 'bg-destructive/15', text: 'text-destructive', label: 'Rejected' },
  pending: { dot: 'bg-info', ring: 'bg-info/15', text: 'text-info', label: 'Pending' },
  error: { dot: 'bg-destructive', ring: 'bg-destructive/15', text: 'text-destructive', label: 'Error' },
  neutral: { dot: 'bg-text-muted', ring: 'bg-text-muted/15', text: 'text-text-muted', label: 'Neutral' },
};

export function StatusPill({
  kind,
  label,
  className,
}: {
  kind: StatusKind;
  label?: string | undefined;
  className?: string | undefined;
}) {
  const s = styles[kind];
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset',
        s.ring,
        s.text,
        className,
      )}
    >
      <span className={cn('h-1.5 w-1.5 rounded-full', s.dot)} aria-hidden="true" />
      {label ?? s.label}
    </span>
  );
}
