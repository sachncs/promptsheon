'use client';

import { useEffect } from 'react';

/**
 * Last-resort boundary for failures that prevent the root layout from rendering.
 * This file must remain self-contained because the normal application providers
 * and layout may be unavailable when it is rendered.
 */
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('Fatal application error', error);
  }, [error]);

  return (
    <html lang="en">
      <body>
        <main
          role="alert"
          style={{
            alignItems: 'center',
            background: '#0b0d12',
            color: '#f4f5f7',
            display: 'flex',
            fontFamily: 'system-ui, sans-serif',
            justifyContent: 'center',
            minHeight: '100vh',
            padding: '24px',
          }}
        >
          <section style={{ maxWidth: '520px', width: '100%' }}>
            <p style={{ color: '#f87171', fontSize: '13px', fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase' }}>
              Promptsheon
            </p>
            <h1 style={{ fontSize: '28px', margin: '12px 0 8px' }}>Something went wrong</h1>
            <p style={{ color: '#a7acb8', lineHeight: 1.6, margin: 0 }}>
              The application could not start this view. Try again, or reload the page if the problem continues.
            </p>
            <button
              type="button"
              onClick={reset}
              style={{
                background: '#f4f5f7',
                border: 0,
                borderRadius: '8px',
                color: '#111318',
                cursor: 'pointer',
                fontSize: '14px',
                fontWeight: 600,
                marginTop: '20px',
                padding: '10px 16px',
              }}
            >
              Try again
            </button>
          </section>
        </main>
      </body>
    </html>
  );
}
