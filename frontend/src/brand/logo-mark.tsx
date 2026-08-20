import { cn } from '@/lib/utils';

export function LogoMark({ size = 22, className }: { size?: number; className?: string }) {
  return (
    <svg
      viewBox="0 0 64 64"
      width={size}
      height={size}
      role="img"
      aria-label="Promptsheon"
      className={cn('inline-block', className)}
    >
      <defs>
        <linearGradient id="psMark" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="oklch(0.78 0.04 260)" />
          <stop offset="60%" stopColor="oklch(0.42 0.02 280)" />
          <stop offset="100%" stopColor="oklch(0.18 0.01 270)" />
        </linearGradient>
      </defs>
      <rect width="64" height="64" rx="14" fill="oklch(0.07 0.005 270)" />
      <path
        d="M19 14 H38 a13 13 0 0 1 0 26 H28 V50 H19 Z M28 22 V32 H38 a5 5 0 0 0 0 -10 Z"
        fill="url(#psMark)"
      />
    </svg>
  );
}
