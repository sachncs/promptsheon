import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { LogoMark } from '@/brand/logo-mark';

export default function NotFound() {
  return (
    <div className="min-h-[60vh] flex flex-col items-center justify-center text-center">
      <LogoMark size={48} />
      <h1 className="mt-6 text-4xl font-semibold tracking-tight text-text-strong">404</h1>
      <p className="mt-2 max-w-md text-text-muted">
        That capability, release, or artefact doesn't exist. The URL may be stale, or the item may have been removed.
      </p>
      <Link href="/app" className="mt-6">
        <Button>Back to the control plane</Button>
      </Link>
    </div>
  );
}
