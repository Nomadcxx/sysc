'use client';

import { Brand } from '@/components/brand';
import { SidebarTrigger, useSidebar } from 'fumadocs-ui/layouts/docs/slots/sidebar';
import { FullSearchTrigger, SearchTrigger } from 'fumadocs-ui/layouts/shared/slots/search-trigger';
import { PanelLeft } from 'lucide-react';
import Link from 'next/link';
import { useEffect, useRef } from 'react';
import type { ComponentProps } from 'react';

export function SiteHeader({ className, ...props }: ComponentProps<'header'>) {
  const { mode, open, setOpen } = useSidebar();
  const navigationTrigger = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (mode !== 'drawer' || !open) return;

    function closeOnEscape(event: KeyboardEvent) {
      if (event.key !== 'Escape' || event.target instanceof Element && event.target.closest('[role="dialog"]')) return;
      setOpen(false);
      navigationTrigger.current?.focus();
    }

    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [mode, open, setOpen]);

  return (
    <header {...props} className={`site-header ${className ?? ''}`}>
      <Link href="/docs" className="site-brand" aria-label="SYSC documentation home">
        <Brand />
      </Link>
      <div className="site-header-search">
        <div className="site-search-full">
          <FullSearchTrigger hideIfDisabled />
        </div>
        <SearchTrigger hideIfDisabled aria-label="Open search" className="site-search-icon" />
      </div>
      <div className="site-header-meta">
        <a href="https://github.com/Nomadcxx/sysc">SYSC source</a>
        <SidebarTrigger
          ref={navigationTrigger}
          className="site-sidebar-trigger"
          aria-label={open ? 'Close navigation' : 'Open navigation'}
          aria-expanded={mode === 'drawer' ? open : undefined}
          aria-controls={mode === 'drawer' ? 'nd-sidebar-mobile' : undefined}
        >
          <PanelLeft aria-hidden="true" />
        </SidebarTrigger>
      </div>
    </header>
  );
}
