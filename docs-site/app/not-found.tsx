import { basePath } from '@/lib/shared';

export default function NotFound() {
  return (
    <main id="main-content" tabIndex={-1} className="root-not-found">
      <p className="docs-home-kicker">404</p>
      <h1>Page not found</h1>
      <p>The address does not match a published page.</p>
      <a href={`${basePath}/docs/`}>Return to SYSC documentation</a>
    </main>
  );
}
