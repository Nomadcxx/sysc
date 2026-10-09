import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { basename, dirname, extname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const siteRoot = fileURLToPath(new URL('../', import.meta.url));
const docsRoot = join(siteRoot, 'content/docs');
const publicRoot = join(siteRoot, 'public');
const repositoryRoot = fileURLToPath(new URL('../../', import.meta.url));
const docsWorkflowFile = join(repositoryRoot, '.github/workflows/docs.yml');
const sourceFile = join(docsRoot, 'reference/sources.mdx');
const compatibilityFile = join(docsRoot, 'reference/compatibility.mdx');
const homeFile = join(docsRoot, 'index.mdx');
const homeComponent = join(siteRoot, 'components/docs-home.tsx');
const rootMeta = join(docsRoot, 'meta.json');
const baseConfig = readFileSync(join(siteRoot, 'next.config.mjs'), 'utf8');
const theme = readFileSync(join(siteRoot, 'app/global.css'), 'utf8');
const requiredDocs = [
  'start/requirements', 'start/install', 'start/wizard', 'start/verify',
  'guides/conflicts', 'guides/upgrade', 'guides/uninstall', 'guides/niri-session',
  'guides/service-integration', 'guides/themes', 'guides/wallpapers',
  'troubleshooting/installer', 'troubleshooting/services', 'troubleshooting/greeter', 'troubleshooting/lock',
  'reference/suite', 'reference/paths', 'reference/privileges',
  'developers/architecture', 'developers/build', 'developers/interfaces',
  'developers/releases', 'developers/documentation',
  'components/sysc', 'components/sysc-greet', 'components/sysc-lock',
  'components/sysc-shell', 'components/sysc-tray', 'components/sysc-terminal',
  'components/sysc-clipboard', 'components/sysc-wayland', 'components/sysc-metrics',
  'components/sysc-launch', 'components/sysc-go', 'components/sysc-walls', 'components/sysc-notify', 'components/gslapper',
  'plugins/index',
  'plugins/catalog', 'plugins/write', 'plugins/manifest', 'plugins/protocol',
  'plugins/ui', 'plugins/test', 'plugins/publish', 'plugins/submit',
];
const pluginCatalogFile = join(siteRoot, 'data/plugin-catalog.json');
const pluginCatalogPage = join(docsRoot, 'plugins/catalog.mdx');

for (const [path, label] of [
  [sourceFile, 'source record'],
  [compatibilityFile, 'compatibility reference'],
  [homeFile, 'documentation home'],
  [homeComponent, 'home component'],
  [rootMeta, 'navigation metadata'],
]) {
  assert.ok(existsSync(path), `missing ${label}: ${relative(siteRoot, path)}`);
}

const sources = readFileSync(sourceFile, 'utf8');
const compatibility = readFileSync(compatibilityFile, 'utf8');
const home = readFileSync(homeFile, 'utf8');
const homeCode = readFileSync(homeComponent, 'utf8');
const meta = JSON.parse(readFileSync(rootMeta, 'utf8'));
const readme = readFileSync(join(siteRoot, 'README.md'), 'utf8');
const sourceRow = (project) => sources.split('\n').find((line) => {
  const cell = line.startsWith('|') ? line.split('|')[1]?.trim() : '';
  const label = cell.match(/^\[([^\]]+)\]/)?.[1] ?? cell;
  return label === project;
});

assert.ok(existsSync(docsWorkflowFile), 'missing documentation CI workflow');
const docsWorkflow = readFileSync(docsWorkflowFile, 'utf8');
assert.match(docsWorkflow, /^\s*docs-check:/m, 'docs CI must expose the stable docs-check job');
assert.match(docsWorkflow, /pull_request:/, 'docs CI must run on pull requests');
assert.match(docsWorkflow, /push:[\s\S]*branches:[\s\S]*-\s*main/, 'docs CI must run on main pushes');
assert.match(docsWorkflow, /contents:\s*read/, 'docs CI must use read-only repository access');
assert.match(docsWorkflow, /persist-credentials:\s*false/, 'checkout must not persist credentials');
assert.match(docsWorkflow, /timeout-minutes:\s*\d+/, 'docs CI needs a bounded timeout');
assert.match(docsWorkflow, /cancel-in-progress:\s*\$\{\{\s*github\.event_name\s*==\s*'pull_request'\s*\}\}/, 'only superseded PR checks should cancel');
assert.match(docsWorkflow, /node-version:\s*['"]?22['"]?/, 'docs CI must use Node 22');
assert.match(docsWorkflow, /cache:\s*npm/, 'docs CI must cache npm downloads');
assert.match(docsWorkflow, /cache-dependency-path:\s*docs-site\/package-lock\.json/, 'docs CI cache must follow the docs lockfile');
assert.match(docsWorkflow, /npm ci/, 'docs CI must install from the lockfile');
assert.match(docsWorkflow, /npm run check/, 'docs CI must run the site checks');
assert.match(docsWorkflow, /DOCS_BASE_PATH:\s*\/sysc/, 'docs CI must build the Pages base path');
assert.match(docsWorkflow, /retention-days:\s*7/, 'PR preview artifacts must expire after seven days');
assert.match(docsWorkflow, /github\.event_name\s*==\s*'pull_request'/, 'preview artifacts should be PR-only');
assert.match(docsWorkflow, /uses:\s*actions\/checkout@/, 'docs CI must check out the source');
assert.match(docsWorkflow, /uses:\s*actions\/setup-node@/, 'docs CI must set up Node');
assert.match(docsWorkflow, /uses:\s*actions\/upload-artifact@/, 'docs CI must upload the PR preview');
assert.match(docsWorkflow, /uses:\s*actions\/upload-pages-artifact@/, 'main builds must upload a Pages artifact');
assert.match(docsWorkflow, /uses:\s*actions\/deploy-pages@/, 'main builds must deploy GitHub Pages');
assert.match(docsWorkflow, /docs-site\/out/, 'PR preview artifact must contain the static export');
assert.match(docsWorkflow, /GITHUB_STEP_SUMMARY/, 'docs CI must record build provenance in the job summary');
assert.match(docsWorkflow, /github\.sha/, 'docs CI summary must identify the source SHA');
assert.match(docsWorkflow, /Base path:.*\/sysc/, 'docs CI summary must record the export base path');
assert.match(docsWorkflow, /Command:.*npm run check/, 'docs CI summary must record the build command');
assert.doesNotMatch(docsWorkflow, /pull_request_target/, 'docs CI must not use pull_request_target');
const docsCheckJob = docsWorkflow.match(/  docs-check:[\s\S]*?(?=\n  deploy:)/)?.[0] ?? '';
const deployJob = docsWorkflow.match(/  deploy:[\s\S]*$/)?.[0] ?? '';
assert.ok(docsCheckJob, 'docs build job is missing');
assert.doesNotMatch(docsCheckJob, /^\s+(?:pages|id-token):\s*write\s*$/m, 'build and PR checks must not receive Pages write permissions');
assert.match(deployJob, /if:\s*github\.event_name\s*==\s*'push'\s*&&\s*github\.ref\s*==\s*'refs\/heads\/main'/, 'only main pushes may deploy');
assert.match(deployJob, /pages:\s*write/, 'deploy job must have Pages permission');
assert.match(deployJob, /id-token:\s*write/, 'deploy job must have OIDC permission');
assert.match(deployJob, /name:\s*github-pages/, 'deploy job must use the Pages environment');
assert.match(deployJob, /docs\/components\/sysc-go\//, 'deployment smoke test must check a component route');
for (const [, action, revision] of docsWorkflow.matchAll(/uses:\s*([^\s@]+)@([^\s#]+)/g)) {
  assert.match(revision, /^[a-f0-9]{40}$/, `${action} must be pinned to a full commit SHA`);
}

for (const route of requiredDocs) {
  const page = [
    join(docsRoot, route + '.mdx'),
    join(docsRoot, route, 'index.mdx'),
  ].find(existsSync);
  assert.ok(page, 'missing implementation page: /docs/' + route);
  const [section, ...slug] = route.split('/');
  const sectionMeta = JSON.parse(readFileSync(join(docsRoot, section, 'meta.json'), 'utf8'));
  assert.ok(sectionMeta.pages.includes(slug.join('/')), `navigation is missing /docs/${route}`);
  const content = readFileSync(page, 'utf8');
  assert.match(content, /^title:\s*.+$/m, 'page has no title: /docs/' + route);
  assert.doesNotMatch(content, /^# .+$/m, 'page body must use lower-level headings because DocsTitle supplies the page heading: /docs/' + route);
}

for (const metaFile of contentFiles(docsRoot).filter((path) => basename(path) === 'meta.json')) {
  const section = relative(docsRoot, dirname(metaFile));
  const { pages } = JSON.parse(readFileSync(metaFile, 'utf8'));
  for (const entry of pages) {
    const isNavigationDirective = entry.startsWith('!')
      || entry === 'z...a'
      || entry.startsWith('...')
      || /^---(?:\[[^\]]+])?.+---$|^---$/.test(entry)
      || /^(?:external:)?(?:\[[^\]]+])?\[[^\]]+\]\([^)]+\)$/.test(entry);
    if (isNavigationDirective) continue;

    const route = join(section, entry);
    const pageExists = [
      join(docsRoot, `${route}.mdx`),
      join(docsRoot, route, 'index.mdx'),
    ].some(existsSync);
    assert.ok(pageExists, `navigation refers to missing content page: /docs/${route}`);
  }
}

for (const project of [
  'sysc', 'sysc-greet', 'sysc-lock', 'sysc-shell', 'sysc-tray', 'sysc-terminal',
  'sysc-clipboard', 'sysc-wayland', 'sysc-metrics', 'sysc-launch', 'sysc-plugins',
  'sysc-walls', 'sysc-notify', 'gSlapper', 'sysc-Go',
]) {
  const row = sourceRow(project);
  assert.ok(row, `source table is missing ${project}`);
  assert.match(row, /[a-f0-9]{40}/, `${project} source row must link a full commit`);
  assert.match(row, /\]\(https:\/\/github\.com\/Nomadcxx\/[^/]+\/commit\/[a-f0-9]{40}\)/, `${project} source row must link its reviewed commit`);
}

const installerPin = JSON.parse(readFileSync(join(repositoryRoot, 'internal/pin/pin.json'), 'utf8'));
assert.ok(sourceRow('sysc')?.includes(`installer ${installerPin.release}`), 'source table suite release differs from the installer pin');
assert.ok(Array.isArray(installerPin.components), 'installer pin must list components');
for (const component of installerPin.components) {
  const row = sourceRow(component.id);
  assert.ok(row, `source table is missing installer component ${component.id}`);
  if (component.disabled) {
    assert.ok(row.includes('disabled in the installer manifest'), `${component.id} source row must reflect its disabled installer status`);
  } else {
    assert.ok(component.tag, `${component.id} installer component has no release tag`);
    assert.ok(row.includes(`${component.id} ${component.tag}`), `${component.id} source row pin differs from the installer manifest`);
  }
}
assert.ok(sourceRow('gSlapper')?.includes(`gSlapper v${installerPin.gslapper.version}`), 'gSlapper source row differs from the installer pin');

assert.ok(sources.includes('4c98abc5bdddbf6c84d2a55d71b9120cbfccad0f'), 'missing reviewed greet shell SHA');
assert.ok(readme.includes('84e207224241e0c9cad3992ffcba24c098f6aa743ff9f70ddf50afddc012cb80'), 'missing copied lockfile SHA');
assert.ok(readme.includes('d927105097d79259a24493d20944dd3ae5fdd4b3'), 'missing last deployed greet SHA');
assert.match(compatibility, /Arch family/i, 'compatibility must describe the installer distro gate');
assert.match(compatibility, /Niri/i, 'compatibility must describe the compositor route');
assert.match(compatibility, /amd64/i, 'compatibility must distinguish available component architectures');
assert.match(compatibility, /not established|unknown/i, 'unqualified support must remain explicitly unknown');
assert.match(compatibility, /normal user/i, 'compatibility must state the installer privilege boundary');
assert.match(baseConfig, /DOCS_BASE_PATH/, 'Next config must use the explicit docs base path');
assert.match(theme, /--color-fd-background:\s*#000000/i, 'site background must use the approved black canvas');

for (const section of ['Start', 'Guides', 'Components', 'Developers', 'Plugins']) {
  assert.ok(meta.pages.includes(section.toLowerCase()), `root navigation is missing ${section}`);
  assert.ok(existsSync(join(docsRoot, section.toLowerCase(), 'meta.json')), `missing ${section} navigation metadata`);
}

for (const action of [
  'Install SYSC', 'Configure SYSC', 'Troubleshoot a problem',
  'Develop with components', 'Author a plugin',
]) {
  assert.ok(homeCode.includes(action), `home action hub is missing: ${action}`);
}
assert.match(home, /<DocsHome\s*\/>/, 'home MDX must render the action hub');
assert.match(homeCode, /component directory/i, 'home must label the component directory');
assert.match(readme, /DOCS_BASE_PATH/, 'site README must document local and Pages base paths');
assert.match(readme, /docs-preview/i, 'site README must explain the PR preview artifact');
assert.match(readme, /seven days/i, 'site README must document the preview artifact retention');
assert.match(readme, /python3 -m http\.server 8000/, 'site README must document a local artifact preview command');
assert.match(readme, /sysc-greet.*4c98abc5/i, 'site README must attribute the reused shell source');

const pluginCatalog = JSON.parse(readFileSync(pluginCatalogFile, 'utf8'));
const pluginCatalogContent = readFileSync(pluginCatalogPage, 'utf8');
assert.equal(pluginCatalog.schema, 1, 'plugin catalog snapshot must use schema 1');
assert.ok(Array.isArray(pluginCatalog.plugins) && pluginCatalog.plugins.length > 0, 'plugin catalog snapshot is empty');
for (const plugin of pluginCatalog.plugins) {
  assert.ok(pluginCatalogContent.includes(plugin.id), `plugin catalog listing is missing ${plugin.id}`);
  assert.ok(pluginCatalogContent.includes(plugin.version), `plugin catalog listing is missing ${plugin.id} version`);
  for (const asset of Object.values(plugin.assets ?? {})) {
    assert.ok(pluginCatalogContent.includes(asset.url), `plugin catalog listing is missing asset for ${plugin.id}`);
  }
}

const pluginProtocol = readFileSync(join(docsRoot, 'plugins/protocol.mdx'), 'utf8');
const pluginUI = readFileSync(join(docsRoot, 'plugins/ui.mdx'), 'utf8');
const pluginPublish = readFileSync(join(docsRoot, 'plugins/publish.mdx'), 'utf8');
const pluginSubmit = readFileSync(join(docsRoot, 'plugins/submit.mdx'), 'utf8');
assert.match(pluginProtocol, /host\.hello/);
assert.match(pluginProtocol, /plugin\.hello/);
assert.match(pluginProtocol, /stderr/i, 'protocol guidance must keep diagnostics off stdout');
assert.match(pluginProtocol, /capabilit/i, 'protocol guidance must explain host grants');
assert.match(pluginUI, /lint\.Tree/);
assert.match(pluginUI, /BarWidth/);
assert.match(pluginUI, /TooltipWidth/);
assert.match(pluginUI, /accessible name/i);
assert.match(pluginPublish, /-community/);
assert.match(pluginPublish, /64 MiB/);
assert.match(pluginPublish, /320 px/);
assert.match(pluginPublish, /4 MiB/);
assert.match(pluginPublish, /does not.*dimension|does not check.*dimension/i);
assert.match(pluginSubmit, /pull request/i);
assert.match(pluginSubmit, /sysc-plugins/);
assert.match(pluginSubmit, /license.*optional|optional.*license/i);
assert.match(pluginSubmit, /review time|review-time|SLA/i);
assert.match(pluginSubmit, /same-user/i);

function contentFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? contentFiles(path) : [path];
  });
}

for (const path of contentFiles(docsRoot).filter((file) => ['.md', '.mdx'].includes(extname(file)))) {
  const content = readFileSync(path, 'utf8');
  const rel = relative(docsRoot, path);
  assert.doesNotMatch(content, /^!!!/m, `legacy admonition syntax in ${rel}`);
  for (const [, src] of content.matchAll(/!\[[^\]]*\]\(([^)\s]+)[^)]*\)/g)) {
    assert.ok(!/^https?:\/\//.test(src), `remote image ${src} in ${rel} — use a reviewed local asset`);
    if (src.startsWith('/')) {
      assert.ok(existsSync(join(publicRoot, src.slice(1))), `${rel} references missing local asset ${src}`);
    }
  }
}

console.log('content check passed');
