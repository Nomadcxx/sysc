import type { ComponentProps } from 'react';
import Image from 'next/image';
import { basePath } from '@/lib/shared';

export function Brand({ className, ...props }: ComponentProps<'span'>) {
  return (
    <span className={`sysc-brand ${className ?? ''}`} {...props}>
      <Image
        className="sysc-brand-logo"
        src={`${basePath}/sysc-logo.png`}
        alt=""
        width={640}
        height={620}
        priority
      />
      <span className="sysc-brand-label">Documentation</span>
    </span>
  );
}
