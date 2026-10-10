import defaultMdxComponents from 'fumadocs-ui/mdx';
import type { MDXComponents } from 'mdx/types';
import { basePath } from '@/lib/shared';

export function getMDXComponents(components?: MDXComponents) {
  return {
    ...defaultMdxComponents,
    // Root-relative images live in public/ and need the GitHub Pages base path.
    img: ({ src, alt, ...props }) => {
      // The MDX image plugin passes local files as { src, width, height }, with the base path applied.
      const imported = typeof src === 'object' && src !== null;
      const url = imported ? (src as unknown as { src: string }).src : src;
      return (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          {...props}
          src={!imported && typeof url === 'string' && url.startsWith('/') ? `${basePath}${url}` : url}
          alt={alt ?? ''}
          loading="lazy"
          decoding="async"
          style={{ maxWidth: '100%', height: 'auto' }}
        />
      );
    },
    ...components,
  } satisfies MDXComponents;
}

export const useMDXComponents = getMDXComponents;

declare global {
  type MDXProvidedComponents = ReturnType<typeof getMDXComponents>;
}
