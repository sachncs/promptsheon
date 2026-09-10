'use client';

import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import * as React from 'react';
import { cn } from '@/lib/utils';

export const Drawer = DialogPrimitive.Root;
export const DrawerTrigger = DialogPrimitive.Trigger;

export function DrawerContent({
  side = 'right',
  title,
  description,
  children,
  className,
}: {
  side?: 'right' | 'left';
  title: string;
  description?: string | undefined;
  children: React.ReactNode;
  className?: string | undefined;
}) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-background/70 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=open]:fade-in-0 data-[state=closed]:fade-out-0" />
      <DialogPrimitive.Content
        className={cn(
          'fixed z-50 top-0 bottom-0 w-full max-w-md border-l border-border-subtle bg-surface-1 shadow-2',
          'data-[state=open]:animate-in data-[state=closed]:animate-out',
          side === 'right'
            ? 'right-0 data-[state=open]:slide-in-from-right data-[state=closed]:slide-out-to-right'
            : 'left-0 border-l-0 border-r data-[state=open]:slide-in-from-left data-[state=closed]:slide-out-to-left',
          'duration-base',
          className,
        )}
      >
        <div className="flex h-full flex-col">
          <div className="flex items-start justify-between border-b border-border-subtle px-6 py-4">
            <div>
              <DialogPrimitive.Title className="text-base font-semibold tracking-tight text-text-strong">
                {title}
              </DialogPrimitive.Title>
              {description && (
                <DialogPrimitive.Description className="mt-1 text-sm text-text-muted">
                  {description}
                </DialogPrimitive.Description>
              )}
            </div>
            <DialogPrimitive.Close
              className="grid h-8 w-8 place-items-center rounded-md text-text-muted hover:bg-surface-2 hover:text-text-strong focus:outline-none focus-visible:ring-2 focus-visible:ring-brand"
              aria-label="Close"
            >
              <X className="h-4 w-4" />
            </DialogPrimitive.Close>
          </div>
          <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>
        </div>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}
