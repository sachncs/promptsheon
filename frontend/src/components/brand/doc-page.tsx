import Link from 'next/link';
import { HashChip } from '@/components/brand/hash-chip';

export function DocPage({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <article className="space-y-8">
      <header>
        <h1 className="text-3xl font-semibold tracking-tight text-text-strong">{title}</h1>
        {subtitle && <p className="mt-2 max-w-2xl text-text-muted">{subtitle}</p>}
      </header>
      <div className="space-y-8 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:tracking-tight [&_h2]:text-text-strong [&_h2]:mt-8 [&_h3]:text-base [&_h3]:font-semibold [&_h3]:text-text-default [&_h3]:mt-6 [&_p]:text-sm [&_p]:text-text-muted [&_p]:leading-relaxed [&_ul]:text-sm [&_ul]:text-text-muted [&_ul]:space-y-1 [&_ul]:list-disc [&_ul]:list-inside [&_code]:font-mono [&_code]:text-text-default [&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-surface-0 [&_pre]:p-4 [&_pre]:font-mono [&_pre]:text-xs [&_pre]:leading-relaxed [&_pre]:text-text-default">
        {children}
      </div>
    </article>
  );
}

export function DocCurl({ cmd }: { cmd: string }) {
  return (
    <pre>
      <code>{cmd}</code>
    </pre>
  );
}

export function DocNext({ href, label }: { href: string; label: string }) {
  return (
    <p>
      Next: <Link href={href} className="text-text-strong underline-offset-4 hover:underline">{label}</Link>
    </p>
  );
}

export function DocHashSample() {
  return (
    <div className="flex items-center gap-2 text-xs text-text-subtle">
      <HashChip hash="sha256:9c4f…a02b" length={20} />
      <span>deterministic, content-addressed</span>
    </div>
  );
}
