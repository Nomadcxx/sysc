import { basePath } from '@/lib/shared';

export default function HomePage() {
  const docsUrl = `${basePath}/docs/`;

  return (
    <main id="main-content" tabIndex={-1} className="root-redirect">
      <meta httpEquiv="refresh" content={`0;url=${docsUrl}`} />
      <p>
        Opening <a href={docsUrl}>SYSC documentation</a>.
      </p>
    </main>
  );
}
