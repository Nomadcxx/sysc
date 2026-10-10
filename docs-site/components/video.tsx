import { basePath } from '@/lib/shared';

export function Video({ src, poster, label }: { src: string; poster?: string; label: string }) {
  return (
    <video
      controls
      muted
      loop
      playsInline
      preload="none"
      aria-label={label}
      poster={poster ? `${basePath}${poster}` : undefined}
      style={{ maxWidth: '100%', height: 'auto', borderRadius: '0.5rem' }}
    >
      <source src={`${basePath}${src}`} type="video/mp4" />
    </video>
  );
}
