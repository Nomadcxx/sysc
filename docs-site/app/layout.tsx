import { Provider } from '@/components/provider';
import { basePath } from '@/lib/shared';
import type { Metadata } from 'next';
import './global.css';

export const metadata: Metadata = {
  metadataBase: new URL('https://nomadcxx.github.io'),
  title: {
    default: 'SYSC documentation',
    template: '%s | SYSC',
  },
  description: 'Install, configure, troubleshoot, develop, and extend the SYSC desktop.',
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="en" className="dark" style={{ colorScheme: 'dark' }}>
      <body className="flex flex-col min-h-screen">
        <a className="skip-link" href="#main-content">Skip to main content</a>
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
