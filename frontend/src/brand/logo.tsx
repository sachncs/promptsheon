import { cn } from '@/lib/utils';
import { LogoMark } from './logo-mark';

export type LogoSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';

const sizeMap: Record<LogoSize, { mark: number; text: string }> = {
  xs: { mark: 16, text: 'text-sm' },
  sm: { mark: 22, text: 'text-base' },
  md: { mark: 32, text: 'text-xl' },
  lg: { mark: 44, text: 'text-3xl' },
  xl: { mark: 64, text: 'text-5xl' },
};

export function Logo({
  size = 'md',
  className,
  showWordmark = true,
}: {
  size?: LogoSize;
  className?: string;
  showWordmark?: boolean;
}) {
  const cfg = sizeMap[size];
  return (
    <div className={cn('inline-flex items-center gap-2.5 select-none', className)}>
      <LogoMark size={cfg.mark} />
      {showWordmark && (
        <span
          className={cn('font-semibold tracking-tight text-foreground', cfg.text)}
          style={{ letterSpacing: '-0.025em' }}
        >
          Promptsheon
        </span>
      )}
    </div>
  );
}