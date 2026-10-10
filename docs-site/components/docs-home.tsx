import Link from 'next/link';
import Image from 'next/image';
import { Video } from '@/components/video';
import { basePath } from '@/lib/shared';

const actions = [
  {
    number: '01',
    title: 'Install SYSC',
    detail: 'Check the supported route and follow installation.',
    href: '/docs/start',
  },
  {
    number: '02',
    title: 'Configure SYSC',
    detail: 'Find the component that owns a setting.',
    href: '/docs/guides',
  },
  {
    number: '03',
    title: 'Troubleshoot a problem',
    detail: 'Collect read-only checks before recovery.',
    href: '/docs/troubleshooting',
  },
  {
    number: '04',
    title: 'Develop with components',
    detail: 'Find source, interfaces, and build rules.',
    href: '/docs/developers',
  },
  {
    number: '05',
    title: 'Author a plugin',
    detail: 'Build a plugin and propose catalog inclusion.',
    href: '/docs/plugins',
  },
] as const;

const projects = [
  'sysc',
  'sysc-greet',
  'sysc-lock',
  'sysc-shell',
  'sysc-tray',
  'sysc-terminal',
  'sysc-clipboard',
  'sysc-wayland',
  'sysc-metrics',
  'sysc-launch',
  'sysc-walls',
  'sysc-notify',
  'gSlapper',
] as const;

export function DocsHome() {
  return (
    <div className="docs-home" data-docs-home>
      <header className="docs-home-heading">
        <h1>
          <Image
            className="docs-home-logo"
            src={`${basePath}/sysc-logo.png`}
            alt="SYSC"
            width={873}
            height={140}
            priority
          />
          <span>Documentation</span>
        </h1>
        <p>Install, configure, troubleshoot, develop, and extend SYSC.</p>
      </header>

      <figure className="docs-home-demo">
        <Video
          src="/media/shell-tour.mp4"
          poster="/media/shell-tour-poster.png"
          label="Settings, Bar settings and the system monitor opening over a static wallpaper, with the bar in view"
        />
        <figcaption>The SYSC desktop: sysc-shell with its bar, settings and system monitor.</figcaption>
      </figure>

      <nav className="docs-home-actions" aria-label="Documentation tasks">
        {actions.map((action) => (
          <Link className="docs-home-action" href={action.href} key={action.number}>
            <span className="docs-home-action-number">{action.number} / TASK</span>
            <span className="docs-home-action-title">{action.title}</span>
            <span className="docs-home-action-detail">{action.detail}</span>
          </Link>
        ))}
      </nav>

      <section className="docs-home-components" aria-labelledby="docs-home-components-title">
        <div className="docs-home-components-heading">
          <div>
            <p className="docs-home-kicker">PROJECT INDEX</p>
            <h2 id="docs-home-components-title">Component directory</h2>
          </div>
          <Link href="/docs/components">Browse components</Link>
        </div>
        <nav aria-label="Component directory">
          <ul>
            {projects.map((project) => (
              <li key={project}>
                <a href={`https://github.com/Nomadcxx/${project}`}>{project}</a>
              </li>
            ))}
          </ul>
        </nav>
        <p className="docs-home-reference-links">
          <Link href="/docs/reference/compatibility">Compatibility</Link>
          <span aria-hidden="true">/</span>
          <Link href="/docs/reference/sources">Source commits and release pins</Link>
          <span aria-hidden="true">/</span>
          <Link href="/docs/plugins">Plugin catalog and authoring</Link>
        </p>
      </section>
    </div>
  );
}
