import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const out = fileURLToPath(new URL('../out/', import.meta.url));
const basePath = process.env.DOCS_BASE_PATH ?? '';
const page = (path) => join(out, path, 'index.html');
const implementedDocs = [
  'start/requirements', 'start/install', 'start/wizard', 'start/verify',
  'guides/conflicts', 'guides/upgrade', 'guides/uninstall', 'guides/niri-session',
  'guides/service-integration', 'guides/themes', 'guides/wallpapers',
  'troubleshooting/services', 'troubleshooting/greeter', 'troubleshooting/lock',
  'reference/suite', 'reference/paths', 'reference/privileges',
  'developers/architecture', 'developers/build', 'developers/interfaces',
  'developers/releases', 'developers/documentation',
  'components/sysc', 'components/sysc-greet', 'components/sysc-lock',
  'components/sysc-shell', 'components/sysc-tray', 'components/sysc-terminal',
  'components/sysc-clipboard', 'components/sysc-wayland', 'components/sysc-metrics',
  'components/sysc-launch', 'components/sysc-walls', 'components/sysc-notify', 'components/gslapper',
  'plugins/catalog', 'plugins/write', 'plugins/manifest', 'plugins/protocol',
  'plugins/ui', 'plugins/test', 'plugins/publish', 'plugins/submit',
];

for (const route of [
  'index.html',
  'docs/index.html',
  'docs/start/index.html',
  'docs/guides/index.html',
  'docs/troubleshooting/index.html',
  'docs/components/index.html',
  'docs/developers/index.html',
  'docs/plugins/index.html',
  'docs/reference/compatibility/index.html',
  'docs/reference/sources/index.html',
]) {
  assert.ok(existsSync(join(out, route)), `static export is missing ${route}`);
}

for (const route of implementedDocs) {
  assert.ok(existsSync(page('docs/' + route)), 'static export is missing /docs/' + route);
}

const rootHtml = readFileSync(join(out, 'index.html'), 'utf8');
const homeHtml = readFileSync(page('docs'), 'utf8');
const sourceHtml = readFileSync(page('docs/reference/sources'), 'utf8');
const cssRoot = join(out, '_next/static/css');
const jsRoot = join(out, '_next/static/chunks');
const files = (directory) => readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
  const path = join(directory, entry.name);
  return entry.isDirectory() ? files(path) : [path];
});
const css = files(cssRoot).map((path) => readFileSync(path, 'utf8')).join('\n');
const js = files(jsRoot).map((path) => readFileSync(path, 'utf8')).join('\n');

for (const [label, route] of [
  ['Install SYSC', '/docs/start/'],
  ['Configure SYSC', '/docs/guides/'],
  ['Troubleshoot a problem', '/docs/troubleshooting/'],
  ['Develop with components', '/docs/developers/'],
  ['Author a plugin', '/docs/plugins/'],
]) {
  assert.ok(homeHtml.includes(label), `home export is missing ${label}`);
  assert.ok(homeHtml.includes(`href="${basePath}${route}"`), `home export has no ${basePath}${route} action`);
}

for (const name of ['sysc', 'sysc-greet', 'sysc-lock', 'sysc-shell', 'sysc-tray', 'sysc-terminal', 'sysc-clipboard', 'sysc-wayland', 'sysc-metrics', 'sysc-launch', 'sysc-walls', 'sysc-notify', 'gSlapper']) {
  assert.ok(homeHtml.includes(name), `home export is missing directory entry ${name}`);
}

assert.match(css, /--color-fd-background:\s*#(?:000|000000)(?:;|\})/i, 'exported theme is missing the black canvas');
assert.match(css, /IBM Plex Sans Variable/i, 'exported theme is missing IBM Plex Sans');
assert.match(css, /Fira Code/i, 'exported theme is missing Fira Code');
assert.match(homeHtml, /aria-label="Documentation tasks"/, 'home task links need a navigation label');
assert.match(homeHtml, /aria-label="Component directory"/, 'component directory needs a navigation label');
assert.ok(homeHtml.includes(`href="${basePath}/docs/reference/compatibility/"`), 'home is missing its compatibility link');
const siteOrigin = 'https://sysc-docs.invalid';
const htmlFiles = files(out).filter((file) => file.endsWith('.html'));
const pageAnchors = new Map();
const browserPath = (file) => {
  const outputPath = relative(out, file).replaceAll('\\', '/');
  const route = outputPath === 'index.html' ? '' : outputPath.replace(/index\.html$/, '');
  return `${basePath}/${route}`.replace(/\/{2,}/g, '/');
};
const decodeHtml = (value) => value.replaceAll('&amp;', '&').replaceAll('&quot;', '"').replaceAll('&#x27;', "'");

for (const path of htmlFiles.filter((file) => relative(out, file).startsWith('docs/'))) {
  const html = readFileSync(path, 'utf8');
  const h1Count = [...html.matchAll(/<h1\b/g)].length;
  assert.equal(h1Count, 1, `${relative(out, path)} must have one page heading, found ${h1Count}`);
}

for (const path of htmlFiles) {
  const html = readFileSync(path, 'utf8');
  assert.doesNotMatch(html, /<img\b[^>]*\bsrc="https?:\/\//i, `static page fetches a remote image: ${path}`);
  pageAnchors.set(path, new Set([...html.matchAll(/(?:^|\s)(?:id|name)\s*=\s*(["'])(.*?)\1/gi)].map((match) => decodeHtml(match[2]))));
}

function checkExportUrl(value, source, label) {
  const url = new URL(decodeHtml(value.trim()), new URL(browserPath(source), siteOrigin));
  if (url.origin !== siteOrigin) return;

  let pathname = url.pathname;
  if (basePath) {
    assert.ok(pathname === basePath || pathname.startsWith(`${basePath}/`), `${label} omits export base path ${basePath}: ${value} in ${relative(out, source)}`);
    pathname = pathname.slice(basePath.length) || '/';
  }

  const relativePath = decodeURIComponent(pathname).replace(/^\/+/, '');
  const targetPath = join(out, relativePath);
  const candidates = pathname.endsWith('/')
    ? [join(targetPath, 'index.html')]
    : [targetPath, join(targetPath, 'index.html')];
  const target = candidates.find((candidate) => existsSync(candidate) && statSync(candidate).isFile());
  assert.ok(target, `${label} points to missing export content: ${value} in ${relative(out, source)}`);

  if (url.hash && target.endsWith('.html')) {
    const anchor = decodeURIComponent(url.hash.slice(1));
    assert.ok(pageAnchors.get(target)?.has(anchor), `${label} points to missing anchor #${anchor}: ${value} in ${relative(out, source)}`);
  }
}

for (const path of htmlFiles) {
  const html = readFileSync(path, 'utf8');
  for (const match of html.matchAll(/(?:^|\s)(href|src|poster|srcset)\s*=\s*(["'])(.*?)\2/gi)) {
    const [, attribute, , value] = match;
    if (attribute.toLowerCase() === 'srcset') {
      if (/^\s*data:/i.test(value)) continue;
      for (const candidate of value.split(',')) {
        const url = candidate.trim().split(/\s+/, 1)[0];
        if (url) checkExportUrl(url, path, 'srcset asset');
      }
    } else {
      checkExportUrl(value, path, `${attribute.toLowerCase()} URL`);
    }
  }
}

for (const path of files(out).filter((file) => file.endsWith('.css'))) {
  const css = readFileSync(path, 'utf8');
  for (const [, value] of css.matchAll(/url\(\s*(["']?)(.*?)\1\s*\)/gi)) {
    checkExportUrl(value, path, 'CSS asset');
  }
}
assert.match(sourceHtml, /4c98abc5bdddbf6c84d2a55d71b9120cbfccad0f/, 'source export is missing the reviewed shell revision');
assert.ok(
  homeHtml.includes(`https://nomadcxx.github.io${basePath}/og/docs/image.png`),
  'Open Graph image does not use the export base path exactly once',
);
assert.ok(statSync(join(out, 'api/search')).size > 100, 'static search payload is empty');
assert.ok(js.includes(`${basePath}/api/search`), 'search client does not use the export base path');
assert.ok(rootHtml.includes(`href="${basePath}/docs/"`), 'site root does not link to the docs route');
assert.ok(rootHtml.includes(`url=${basePath}/docs/`), 'site root redirect does not include the base path');
assert.ok(css.includes('focus-visible'), 'exported styles are missing visible keyboard focus');

console.log(`export check passed (${basePath || 'root'} base path)`);
