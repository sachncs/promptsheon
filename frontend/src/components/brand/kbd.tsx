import * as React from 'react';
import { cn } from '@/lib/utils';

type KbdVariant = 'default' | 'pressed';

export function Kbd({
  children,
  variant = 'default',
  className,
}: {
  children: React.ReactNode;
  variant?: KbdVariant;
  className?: string;
}) {
  return (
    <kbd
      className={cn(
        'inline-flex select-none items-center gap-0.5 rounded-md border px-1.5 py-0.5 font-mono text-[10.5px] font-medium leading-none',
        variant === 'pressed'
          ? 'border-border-strong bg-surface-3 text-text-strong shadow-[inset_0_1px_2px_oklch(0_0_0_/_0.06)]'
          : 'border-border-subtle bg-surface-2 text-text-subtle',
        className,
      )}
    >
      {children}
    </kbd>
  );
}