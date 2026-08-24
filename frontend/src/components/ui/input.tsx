import * as React from 'react';
import { cn } from '@/lib/utils';

interface InputProps extends React.ComponentProps<'input'> {
  mono?: boolean;
}

function Input({ className, type, mono, ...props }: InputProps) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'file:text-foreground placeholder:text-text-muted selection:bg-brand selection:text-brand-foreground border-border-subtle bg-surface-1 flex h-10 w-full min-w-0 rounded-lg border px-3 py-2 text-sm transition-[color,box-shadow] outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
        'focus-visible:border-brand focus-visible:ring-brand/30 focus-visible:ring-[3px]',
        'aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive',
        mono && 'font-mono tracking-tight',
        className,
      )}
      {...props}
    />
  );
}

export { Input };
