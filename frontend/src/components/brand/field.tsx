import * as React from 'react';
import { cn } from '@/lib/utils';

export function Field({
  label,
  hint,
  error,
  required,
  htmlFor,
  className,
  children,
}: {
  label?: React.ReactNode;
  hint?: React.ReactNode;
  error?: React.ReactNode;
  required?: boolean;
  htmlFor?: string;
  className?: string;
  children: React.ReactNode;
}) {
  const generatedId = React.useId();
  const errorId = `${htmlFor ?? generatedId}-error`;
  const describedBy = error ? errorId : undefined;
  const enhancedChildren = React.Children.map(children, (child) => {
    if (!React.isValidElement(child)) return child;
    const childProps = child.props as {
      'aria-describedby'?: string;
      'aria-invalid'?: boolean | 'true' | 'false';
      'aria-required'?: boolean | 'true' | 'false';
    };
    const existingDescribedBy = childProps['aria-describedby'];
    return React.cloneElement(child, {
      'aria-describedby': [existingDescribedBy, describedBy].filter(Boolean).join(' ') || undefined,
      'aria-invalid': error ? true : childProps['aria-invalid'],
      'aria-required': required ? true : childProps['aria-required'],
    } as React.HTMLAttributes<HTMLElement>);
  });

  return (
    <div className={cn('space-y-1.5', className)}>
      {label && (
        <label htmlFor={htmlFor} className="flex items-baseline justify-between text-xs font-medium uppercase tracking-wider text-text-subtle">
          <span>
            {label}
            {required && <span className="ml-1 text-destructive">*</span>}
          </span>
          {hint && <span className="font-normal normal-case tracking-normal text-text-muted">{hint}</span>}
        </label>
      )}
      {enhancedChildren}
      {error && <p id={errorId} role="alert" className="text-xs text-destructive">{error}</p>}
    </div>
  );
}

export function FieldGroup({
  cols = 2,
  className,
  children,
}: {
  cols?: 2 | 3 | 4;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        'grid gap-3',
        cols === 2 && 'sm:grid-cols-2',
        cols === 3 && 'sm:grid-cols-3',
        cols === 4 && 'sm:grid-cols-4',
        className,
      )}
    >
      {children}
    </div>
  );
}
