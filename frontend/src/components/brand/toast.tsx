'use client';

import * as React from 'react';
import * as ToastPrimitive from '@radix-ui/react-toast';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

type ToastVariant = 'default' | 'success' | 'warning' | 'destructive';

interface ToastItem {
  id: string;
  title: string;
  description?: string;
  variant?: ToastVariant;
  durationMs?: number;
}

interface ToastContextValue {
  toast: (input: Omit<ToastItem, 'id'>) => void;
}

const ToastContext = React.createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = React.useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<ToastItem[]>([]);

  const toast = React.useCallback((input: Omit<ToastItem, 'id'>) => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
    setItems((prev) => [...prev, { ...input, id }]);
  }, []);

  const dismiss = React.useCallback((id: string) => {
    setItems((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const ctx = React.useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={ctx}>
      <ToastPrimitive.Provider swipeDirection="right" duration={5000}>
        {children}
        {items.map((item) => (
          <ToastPrimitive.Root
            key={item.id}
            duration={item.durationMs ?? 5000}
            onOpenChange={(open) => {
              if (!open) dismiss(item.id);
            }}
            className={cn(
              'group pointer-events-auto relative flex w-full items-start gap-3 overflow-hidden rounded-xl border bg-surface-1 p-4 shadow-2 transition-all',
              'data-[state=open]:animate-in data-[state=closed]:animate-out data-[swipe=end]:animate-out',
              'data-[state=closed]:fade-out-80 data-[state=closed]:slide-out-to-right-full',
              'data-[state=open]:slide-in-from-top-full data-[state=open]:sm:slide-in-from-bottom-full',
              item.variant === 'success' && 'border-success/40 bg-success/10',
              item.variant === 'warning' && 'border-warning/40 bg-warning/10',
              item.variant === 'destructive' && 'border-destructive/40 bg-destructive/10',
            )}
          >
            <div className="flex-1 min-w-0">
              <ToastPrimitive.Title className="text-sm font-semibold text-text-strong">
                {item.title}
              </ToastPrimitive.Title>
              {item.description && (
                <ToastPrimitive.Description className="mt-1 text-xs text-text-muted">
                  {item.description}
                </ToastPrimitive.Description>
              )}
            </div>
            <ToastPrimitive.Close
              aria-label="Dismiss"
              className="rounded-md p-1 text-text-muted hover:bg-surface-2 hover:text-text-default"
            >
              <X className="size-4" />
            </ToastPrimitive.Close>
          </ToastPrimitive.Root>
        ))}
        <ToastPrimitive.Viewport className="fixed bottom-4 right-4 z-50 flex w-full max-w-sm flex-col gap-2 outline-none" />
      </ToastPrimitive.Provider>
    </ToastContext.Provider>
  );
}