import Link from 'next/link';
import { ChevronRight } from 'lucide-react';
import * as React from 'react';
import { cn } from '@/lib/utils';

export interface BreadcrumbItem {
  label: string;
  href?: string;
}

export function Breadcrumb({
  items,
  separator,
  className,
}: {
  items: BreadcrumbItem[];
  separator?: React.ReactNode;
  className?: string;
}) {
  const sep = separator ?? <ChevronRight className="size-3.5 text-text-subtle" />;
  return (
    <nav aria-label="Breadcrumb" className={cn('flex flex-wrap items-center gap-1 text-xs text-text-muted', className)}>
      {items.map((item, i) => {
        const isLast = i === items.length - 1;
        return (
          <React.Fragment key={`${item.label}-${i}`}>
            {item.href && !isLast ? (
              <Link href={item.href} className="rounded-md px-1.5 py-0.5 transition-colors hover:bg-surface-2 hover:text-text-default">
                {item.label}
              </Link>
            ) : (
              <span className={cn('px-1.5 py-0.5', isLast && 'font-medium text-text-strong')}>{item.label}</span>
            )}
            {!isLast && <span aria-hidden className="text-text-subtle">{sep}</span>}
          </React.Fragment>
        );
      })}
    </nav>
  );
}