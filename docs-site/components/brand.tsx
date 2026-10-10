import type { ComponentProps } from 'react';
import Image from 'next/image';
import { basePath } from '@/lib/shared';

export function Brand({ className, ...props }: ComponentProps<'span'>) {
  return (
    <span className={`sysc-brand ${className ?? ''}`} {...props}>
      <Image
        className="sysc-brand-logo"
        src={`${basePath}/sysc-logo.svg`}
        alt=""
        width={1024}
        height={1024}
        priority
      />
      <span className="sysc-brand-label">Documentation</span>
    </span>
  );
}
