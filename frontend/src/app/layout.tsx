import type { Metadata } from 'next';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Providers } from './providers';
import './globals.css';

export const metadata: Metadata = {
  title: 'Promptsheon',
  description: 'Prompt Management Platform',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="antialiased">
        <Providers>
          <TooltipProvider>{children}</TooltipProvider>
        </Providers>
      </body>
    </html>
  );
}