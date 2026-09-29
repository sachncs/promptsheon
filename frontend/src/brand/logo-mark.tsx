import Image from 'next/image';
import { cn } from '@/lib/utils';

export function LogoMark({ size = 22, className }: { size?: number; className?: string }) {
  return (
    <Image
      src="/promptsheon-mark-hd.png"
      width={size}
      height={size}
      role="img"
      aria-label="Promptsheon"
      alt="Promptsheon"
      className={cn('inline-block', className)}
    />
  );
}
