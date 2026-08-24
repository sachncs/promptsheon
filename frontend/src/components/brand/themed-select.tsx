'use client';

import * as React from 'react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';

export interface ThemedSelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export function ThemedSelect({
  value,
  onValueChange,
  options,
  placeholder,
  disabled = false,
  ariaLabel,
  className,
  triggerClassName,
}: {
  value?: string;
  onValueChange?: (value: string) => void;
  options: ThemedSelectOption[];
  placeholder?: string;
  disabled?: boolean;
  ariaLabel?: string;
  className?: string;
  triggerClassName?: string;
}) {
  const selectProps: { value?: string; onValueChange?: (v: string) => void; disabled: boolean } = { disabled };
  if (value !== undefined) selectProps.value = value;
  if (onValueChange !== undefined) selectProps.onValueChange = onValueChange;

  return (
    <Select {...selectProps}>
      <SelectTrigger
        aria-label={ariaLabel}
        className={cn('h-10 rounded-lg border-border-subtle bg-surface-1', triggerClassName)}
      >
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent className={cn('rounded-lg border-border-subtle bg-surface-1 shadow-2', className)}>
        {options.map((opt) => {
          const itemProps: { value: string; children: string; disabled?: boolean } = {
            value: opt.value,
            children: opt.label,
          };
          if (opt.disabled !== undefined) itemProps.disabled = opt.disabled;
          return <SelectItem key={opt.value} {...itemProps} />;
        })}
      </SelectContent>
    </Select>
  );
}