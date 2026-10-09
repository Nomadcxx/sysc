import type { ComponentProps } from 'react';

export function Brand({ className, ...props }: ComponentProps<'span'>) {
  return (
    <span className={`sysc-brand ${className ?? ''}`} {...props}>
      <span>SYSC</span>
      <span className="sysc-brand-mark" aria-hidden="true">///</span>
      <span className="sysc-brand-label">DOCUMENTATION</span>
    </span>
  );
}
