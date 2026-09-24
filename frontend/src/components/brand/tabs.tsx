'use client';

import * as React from 'react';
import { cn } from '@/lib/utils';

export type TabsVariant = 'pill' | 'underline';

interface TabsContextValue {
  value: string;
  setValue: (v: string) => void;
  variant: TabsVariant;
  id: string;
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
  const id = React.useId();
  const current = value ?? internal;

  const setValue = React.useCallback(
    (next: string) => {
      if (value === undefined) setInternal(next);
      onValueChange?.(next);
    },
    [value, onValueChange],
  );

  return (
    <TabsContext.Provider value={{ value: current, setValue, variant, id }}>
      <div className={className}>{children}</div>
    </TabsContext.Provider>
  );
}

export function TabsList({
  className,
  ariaLabel = 'Sections',
  children,
}: {
  className?: string;
  ariaLabel?: string;
  children: React.ReactNode;
}) {
  const { variant, id } = useTabsContext('TabsList');
  return (
    <div
      id={`${id}-list`}
      role="tablist"
      aria-label={ariaLabel}
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
  const tabKey = value.replace(/[^a-zA-Z0-9_-]/g, '-');
  const tabId = `${ctx.id}-tab-${tabKey}`;
  const panelId = `${ctx.id}-panel-${tabKey}`;

  const moveFocus = (direction: 'next' | 'previous' | 'first' | 'last') => {
    const list = document.getElementById(`${ctx.id}-list`);
    if (!list) return;
    const tabs = Array.from(list.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
    const currentIndex = tabs.indexOf(document.activeElement as HTMLButtonElement);
    if (currentIndex < 0 || tabs.length === 0) return;
    const nextIndex = direction === 'first'
      ? 0
      : direction === 'last'
        ? tabs.length - 1
        : (currentIndex + (direction === 'next' ? 1 : -1) + tabs.length) % tabs.length;
    tabs[nextIndex]?.focus();
    const nextValue = tabs[nextIndex]?.dataset['tabValue'];
    if (nextValue) ctx.setValue(nextValue);
  };

  return (
    <button
      type="button"
      id={tabId}
      role="tab"
      aria-selected={active}
      aria-controls={panelId}
      tabIndex={active ? 0 : -1}
      data-tab-value={value}
      onClick={() => ctx.setValue(value)}
      onKeyDown={(event) => {
        if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
          event.preventDefault();
          moveFocus('next');
        } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
          event.preventDefault();
          moveFocus('previous');
        } else if (event.key === 'Home') {
          event.preventDefault();
          moveFocus('first');
        } else if (event.key === 'End') {
          event.preventDefault();
          moveFocus('last');
        }
      }}
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
  const tabKey = value.replace(/[^a-zA-Z0-9_-]/g, '-');
  return (
    <div
      id={`${ctx.id}-panel-${tabKey}`}
      role="tabpanel"
      aria-labelledby={`${ctx.id}-tab-${tabKey}`}
      tabIndex={0}
      className={cn('mt-4 focus-visible:outline-none', className)}
    >
      {children}
    </div>
  );
}
