'use client';

interface BarPoint {
  label: string;
  value: number;
}

export function BarChart({
  data,
  format,
  height = 80,
}: {
  data: BarPoint[];
  format?: (n: number) => string;
  height?: number;
}) {
  if (data.length === 0) {
    return <div className="text-text-muted text-sm">No data.</div>;
  }
  const max = Math.max(...data.map((d) => d.value), 1);
  const fmt = format ?? ((n) => `${n}`);
  return (
    <div className="space-y-1.5">
      {data.map((d) => {
        const pct = Math.max(2, Math.round((d.value / max) * 100));
        return (
          <div key={d.label} className="flex items-center gap-3">
            <div className="w-32 truncate text-xs text-text-muted">{d.label}</div>
            <div className="relative h-2 flex-1 rounded-full bg-surface-2" style={{ height }}>
              <div
                className="absolute inset-y-0 left-0 rounded-full bg-brand"
                style={{ width: `${pct}%` }}
              />
            </div>
            <div className="w-24 text-right font-mono text-xs text-text-default">{fmt(d.value)}</div>
          </div>
        );
      })}
    </div>
  );
}
