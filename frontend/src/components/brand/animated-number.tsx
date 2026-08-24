'use client';

import * as React from 'react';

interface AnimatedNumberProps {
  value: number;
  durationMs?: number;
  format?: (n: number) => string;
  className?: string;
}

export function AnimatedNumber({ value, durationMs = 600, format, className }: AnimatedNumberProps) {
  const [display, setDisplay] = React.useState(value);
  const fromRef = React.useRef(value);
  const startRef = React.useRef<number | null>(null);
  const rafRef = React.useRef<number | null>(null);

  React.useEffect(() => {
    const from = fromRef.current;
    const to = value;
    if (from === to) return;
    const start = performance.now();
    startRef.current = start;

    const tick = (now: number) => {
      const elapsed = now - start;
      const t = Math.min(1, elapsed / durationMs);
      const eased = 1 - Math.pow(1 - t, 3);
      const current = from + (to - from) * eased;
      setDisplay(current);
      if (t < 1) {
        rafRef.current = requestAnimationFrame(tick);
      } else {
        fromRef.current = to;
      }
    };

    rafRef.current = requestAnimationFrame(tick);
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    };
  }, [value, durationMs]);

  const text = format ? format(display) : Math.round(display).toLocaleString();
  return <span className={className}>{text}</span>;
}