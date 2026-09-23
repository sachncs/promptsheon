export default function AppLoading() {
  return (
    <div className="flex min-h-[60vh] items-center justify-center text-sm text-text-muted" aria-busy="true">
      <span className="mr-3 size-4 animate-spin rounded-full border-2 border-border-subtle border-t-brand" aria-hidden="true" />
      Loading workspace…
    </div>
  );
}
