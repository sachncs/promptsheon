import { LogoMark } from '@/brand/logo-mark';

export default function OnboardingLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-surface-0 text-foreground">
      <div className="ps-vignette min-h-screen">
        <header className="flex h-16 items-center border-b border-border-subtle px-6">
          <div className="flex items-center gap-3">
            <LogoMark size={28} />
            <span className="font-semibold tracking-tight">Promptsheon</span>
            <span className="text-xs text-text-subtle">— first-run setup</span>
          </div>
        </header>
        <main className="flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12">
          <div className="w-full max-w-3xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
