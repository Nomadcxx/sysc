import assert from 'node:assert/strict';
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const sourceFile = fileURLToPath(new URL('../data/plugin-catalog.json', import.meta.url));
const outputFile = fileURLToPath(new URL('../content/docs/plugins/catalog.mdx', import.meta.url));
const sourceRevision = 'b6323d6f131bc8fa2d814942b271faecd045522a';
const sourceURL = `https://raw.githubusercontent.com/Nomadcxx/sysc-plugins/${sourceRevision}/catalog.json`;

// ponytail: format a checked-in catalog snapshot so docs builds never fetch catalog data or plugin assets.
const catalog = JSON.parse(readFileSync(sourceFile, 'utf8'));
assert.equal(catalog.schema, 1, 'unsupported plugin catalog schema');
assert.ok(Array.isArray(catalog.plugins), 'catalog has no plugin list');

function escapeText(value) {
  return String(value)
    .replace(/\s+/g, ' ')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('{', '&#123;')
    .replaceAll('}', '&#125;')
    .replace(/[\\`*_\[\]()#+\-.!|]/g, '\\$&')
    .trim();
}

function httpsURL(value) {
  const url = new URL(value);
  assert.equal(url.protocol, 'https:', `URL must use HTTPS: ${value}`);
  return url.href;
}

function code(value) {
  const text = String(value);
  const fence = '`'.repeat(Math.max(1, ...[...text.matchAll(/`+/g)].map(([run]) => run.length + 1)));
  return `${fence}${text}${fence}`;
}

function assetLink([arch, asset]) {
  assert.ok(Number.isSafeInteger(asset.size) && asset.size > 0, `invalid asset size for ${arch}`);
  assert.match(asset.sha256, /^[a-f0-9]{64}$/, `invalid SHA-256 for ${arch}`);
  return `[${escapeText(arch)} · ${asset.size} bytes](<${httpsURL(asset.url)}>)`;
}

function renderPlugin(plugin) {
  assert.ok(plugin.id && plugin.name && plugin.version, 'catalog entry is missing identity fields');
  const protocol = `${plugin.protocol.major}.${plugin.protocol.minor}`;
  const commands = plugin.requires?.commands ?? [];
  const lines = [
    `## ${escapeText(plugin.name)}`,
    '',
    `${code(plugin.id)} · version ${code(plugin.version)} · protocol ${code(protocol)}`,
    '',
    escapeText(plugin.description),
    '',
    `- **Capabilities:** ${plugin.capabilities?.length ? plugin.capabilities.map(code).join(', ') : 'none listed'}`,
    `- **Commands:** ${commands.length ? commands.map(code).join(', ') : 'none listed'}`,
    `- **License:** ${plugin.license ? code(plugin.license) : 'not listed'}`,
    `- **Assets:** ${Object.entries(plugin.assets ?? {}).map(assetLink).join('; ')}`,
    plugin.release_notes ? `- **Release notes:** [${escapeText(plugin.version)}](<${httpsURL(plugin.release_notes)}>)` : '',
  ];
  return lines.filter(Boolean).join('\n');
}

const content = [
  '---',
  'title: Published plugin catalog',
  'description: Published sysc-plugins releases from the recorded catalog snapshot.',
  '---',
  '',
  `This listing uses [catalog.json from the reviewed sysc-plugins revision](${sourceURL}). The source file contains full SHA-256 values and is authoritative for checksums. This page links published assets; the documentation build does not download or run them.`,
  '',
  ...catalog.plugins.map(renderPlugin),
  '',
].join('\n');

if (process.argv.includes('--check')) {
  assert.equal(readFileSync(outputFile, 'utf8'), content, 'plugin catalog page is out of date; run node scripts/generate-plugin-catalog.mjs');
} else {
  writeFileSync(outputFile, content);
}
