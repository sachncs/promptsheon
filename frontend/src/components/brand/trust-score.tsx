import { cn } from '@/lib/utils';

export function TrustScore({ score, className }: { score: number; className?: string | undefined }) {
  const safe = Math.max(0, Math.min(100, Math.round(score)));
  const tone = safe >= 90 ? 'text-success' : safe >= 70 ? 'text-info' : safe >= 50 ? 'text-warning' : 'text-destructive';

  return (
    <div className={cn('flex items-center gap-5', className)}>
      <div className="relative grid h-24 w-24 place-items-center">
        <svg viewBox="0 0 100 100" className="absolute inset-0 -rotate-90">
          <circle cx="50" cy="50" r="44" fill="none" strokeWidth="8" className="stroke-surface-2" />
          <circle
            cx="50"
            cy="50"
            r="44"
            fill="none"
            strokeWidth="8"
            strokeDasharray={`${(safe / 100) * 276} 276`}
            strokeLinecap="round"
            className={cn('transition-all duration-slow', tone.replace('text-', 'stroke-'))}
          />
        </svg>
        <div className="text-2xl font-semibold tracking-tight text-text-strong">{safe}</div>
      </div>
      <div>
        <div className="text-sm font-semibold text-text-strong">Trust score</div>
        <div className="text-sm text-text-muted">Composite of eval pass rate, approval coverage, and runtime reliability.</div>
      </div>
    </div>
  );
}
