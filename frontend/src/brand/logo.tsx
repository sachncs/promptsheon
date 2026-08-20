import Image from 'next/image';
import { cn } from '@/lib/utils';

export type LogoSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';

const sizeMap: Record<LogoSize, { mark: number; text: string; height: number }> = {
  xs: { mark: 16, text: 'text-sm', height: 16 },
  sm: { mark: 22, text: 'text-base', height: 22 },
  md: { mark: 32, text: 'text-xl', height: 32 },
  lg: { mark: 44, text: 'text-3xl', height: 44 },
  xl: { mark: 64, text: 'text-5xl', height: 64 },
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
    <div className={cn('inline-flex items-center gap-3 select-none', className)}>
      <Image
        src="/promptsheon-logo.jpeg"
        alt="Promptsheon"
        width={cfg.mark}
        height={cfg.mark}
        priority
        className="rounded-[5px]"
      />
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
