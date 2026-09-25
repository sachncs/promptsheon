export default function Loading() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-surface-0 p-6" aria-busy="true">
      <div className="flex items-center gap-3 text-sm text-text-muted">
        <span className="size-4 animate-spin rounded-full border-2 border-border-subtle border-t-brand" aria-hidden="true" />
        Loading Promptsheon…
      </div>
    </main>
  );
}
