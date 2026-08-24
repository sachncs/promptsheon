'use client';

import * as React from 'react';
import { cn } from '@/lib/utils';

export type TabsVariant = 'pill' | 'underline';

interface TabsContextValue {
  value: string;
  setValue: (v: string) => void;
  variant: TabsVariant;
}

const TabsContext = React.createContext<TabsContextValue | null>(null);

function useTabsContext(component: string): TabsContextValue {
  const ctx = React.useContext(TabsContext);
  if (!ctx) throw new Error(`${component} must be used inside <Tabs>`);
  return ctx;
}

export function Tabs({
  value,
  defaultValue,
  onValueChange,
  variant = 'pill',
  className,
  children,
}: {
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  variant?: TabsVariant;
  className?: string;
  children: React.ReactNode;
}) {
  const [internal, setInternal] = React.useState(defaultValue ?? '');
  const current = value ?? internal;

  const setValue = React.useCallback(
    (next: string) => {
      if (value === undefined) setInternal(next);
      onValueChange?.(next);
    },
    [value, onValueChange],
  );

  return (
    <TabsContext.Provider value={{ value: current, setValue, variant }}>
      <div className={className}>{children}</div>
    </TabsContext.Provider>
  );
}

export function TabsList({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  const { variant } = useTabsContext('TabsList');
  return (
    <div
      role="tablist"
      className={cn(
        variant === 'pill'
          ? 'inline-flex items-center gap-1 rounded-lg border border-border-subtle bg-surface-1 p-0.5'
          : 'inline-flex items-center gap-4 border-b border-border-subtle',
        className,
      )}
    >
      {children}
    </div>
  );
}

export function TabsTrigger({
  value,
  className,
  children,
}: {
  value: string;
  className?: string;
  children: React.ReactNode;
}) {
  const ctx = useTabsContext('TabsTrigger');
  const active = ctx.value === value;
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={() => ctx.setValue(value)}
      className={cn(
        'inline-flex items-center justify-center whitespace-nowrap text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/40',
        ctx.variant === 'pill'
          ? 'rounded-md px-3 py-1.5'
          : 'border-b-2 -mb-px px-1 pb-2',
        active
          ? ctx.variant === 'pill'
            ? 'bg-brand text-brand-foreground shadow-1'
            : 'border-brand text-text-strong'
          : ctx.variant === 'pill'
            ? 'text-text-muted hover:bg-surface-2 hover:text-text-default'
            : 'border-transparent text-text-muted hover:text-text-default',
        className,
      )}
    >
      {children}
    </button>
  );
}

export function TabsContent({
  value,
  className,
  children,
}: {
  value: string;
  className?: string;
  children: React.ReactNode;
}) {
  const ctx = useTabsContext('TabsContent');
  if (ctx.value !== value) return null;
  return (
    <div role="tabpanel" className={cn('mt-4 focus-visible:outline-none', className)}>
      {children}
    </div>
  );
}