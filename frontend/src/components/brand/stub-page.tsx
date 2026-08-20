import type { LucideIcon } from 'lucide-react';
import { PageHeader } from './page-header';
import { Surface } from './surface';
import { EmptyState } from './empty-state';
import { Button } from '@/components/ui/button';
import Link from 'next/link';

interface Cta { label: string; href: string; icon?: LucideIcon; primary?: boolean }

export function StubPage({
  eyebrow,
  title,
  subtitle,
  description,
  icon,
  primary,
  ctas,
}: {
  eyebrow: string;
  title: string;
  subtitle?: string | undefined;
  description: string;
  icon: LucideIcon;
  primary?: { title: string; description: string; action: { label: string; href: string } } | undefined;
  ctas?: Cta[] | undefined;
}) {
  return (
    <div className="space-y-6">
      <PageHeader eyebrow={eyebrow} title={title} subtitle={subtitle} />
      {primary && (
        <EmptyState
          icon={icon}
          title={primary.title}
          description={primary.description}
          action={
            <Link href={primary.action.href}>
              <Button><span>{primary.action.label}</span></Button>
            </Link>
          }
        />
      )}
      {!primary && (
        <Surface padded={false}>
          <EmptyState
            icon={icon}
            title={title}
            description={description}
            className="m-5 border-0 bg-transparent shadow-none p-12"
            action={ctas?.[0] ? (
              <Link href={ctas[0].href}>
                <Button variant={ctas[0].primary ? 'default' : 'outline'}>{ctas[0].label}</Button>
              </Link>
            ) : undefined}
          />
        </Surface>
      )}
      {ctas && ctas.length > 1 && (
        <Surface>
          <ul className="grid gap-2 sm:grid-cols-2">
            {ctas.slice(1).map((c) => (
              <li key={c.label}>
                <Link href={c.href} className="flex items-center justify-between rounded-lg border border-border-subtle bg-surface-2/40 p-3 text-sm hover:border-border-strong">
                  <span>{c.label}</span>
                  <span className="text-xs text-text-muted">→</span>
                </Link>
              </li>
            ))}
          </ul>
        </Surface>
      )}
    </div>
  );
}
