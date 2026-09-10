import type { Metadata } from 'next';
import { TooltipProvider } from '@/components/ui/tooltip';
import { ThemeScript } from '@/components/theme/theme-script';
import { Providers } from './providers';
import { fontSans, fontMono } from '@/lib/fonts';
import './globals.css';

export const metadata: Metadata = {
  title: 'Promptsheon',
  description: 'The control plane for AI capabilities. Git-native, content-addressed, governed.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning className={`${fontSans.variable} ${fontMono.variable}`}>
      <head>
        <ThemeScript />
      </head>
      <body className="antialiased">
        <Providers>
          <TooltipProvider>{children}</TooltipProvider>
        </Providers>
      </body>
    </html>
  );
}
