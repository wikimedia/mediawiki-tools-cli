import assert from 'node:assert/strict';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { stageLogos } from './stage-logos.mjs';

test('logos are deduplicated, cached offline, refreshed and omitted when unavailable', async () => {
  const website = mkdtempSync(join(tmpdir(), 'mwcli-logos-'));
  const directory = join(website, 'src/content/docs/manual');
  try {
    mkdirSync(directory, { recursive: true });
    mkdirSync(join(website, 'assets'));
    for (const name of ['logo-credits.txt', 'simple-icons-LICENSE', 'opensearch-LICENSE']) writeFileSync(join(website, 'assets', name), 'credits');
    const source = 'https://example.org/logo.svg';
    const favicon = 'https://example.org/favicon.ico';
    writeFileSync(join(directory, '_meta.json'), JSON.stringify({ services: [
      { name: 'Parent', logo: source, children: [{ name: 'Shared logo', logo: source }, { name: 'Tool', logo: favicon }] },
      { name: 'Not an image', logo: 'https://example.org/html' },
      { name: 'Missing', logo: 'https://example.org/missing' },
      { name: 'Insecure', logo: 'http://example.org/logo.svg' },
    ] }));
    const calls = [];
    const warnings = [];
    const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==', 'base64');
    const fetchImage = async (url) => {
      calls.push(url);
      if (url === source) return new Response('<svg xmlns="http://www.w3.org/2000/svg"/>');
      if (url === favicon) return new Response(png);
      if (url.endsWith('/html')) return new Response('<html>Not a favicon</html>');
      throw new Error('offline');
    };
    const metadata = () => JSON.parse(readFileSync(join(directory, '_services.json'), 'utf8')).services;
    await stageLogos(website, { fetchImage, warn: (value) => warnings.push(value) });
    const first = metadata()[0];
    assert.equal(calls.filter((url) => url === source).length, 1);
    assert.equal(first.logo, first.children[0].logo);
    assert.equal(first.logoSource, source);
    assert.match(first.logo, /^\/logos\/[a-f0-9]+\.svg\?v=/);
    assert.match(first.children[1].logo, /\.png\?v=/);
    for (const service of metadata().slice(1)) assert.equal(service.logo, undefined);
    assert.equal(warnings.length, 3);
    assert.equal(calls.includes('http://example.org/logo.svg'), false);
    assert.ok(existsSync(join(website, 'public', first.logo.split('?')[0])));
    const offline = async () => { throw new Error('offline'); };
    await stageLogos(website, { fetchImage: offline, warn: () => {} });
    assert.equal(metadata()[0].logo, first.logo);
    await stageLogos(website, { fetchImage: async () => new Response('<svg xmlns="http://www.w3.org/2000/svg"><title>Updated</title></svg>'), refresh: true, warn: () => {} });
    const refreshed = metadata()[0].logo;
    assert.notEqual(refreshed, first.logo);
    await stageLogos(website, { fetchImage: offline, refresh: true, warn: () => {} });
    assert.equal(metadata()[0].logo, refreshed);
  } finally { rmSync(website, { recursive: true, force: true }); }
});
