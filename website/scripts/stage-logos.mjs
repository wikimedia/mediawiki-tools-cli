import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, renameSync, unlinkSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const hash = (data) => createHash('sha256').update(data).digest('hex');
const extensions = ['svg', 'png', 'ico', 'jpg', 'gif'];

function imageExtension(data) {
  const text = data.subarray(0, 4096).toString('utf8');
  if (/^\s*(?:<\?xml[\s\S]*?\?>\s*)?(?:<!--[\s\S]*?-->\s*)*<svg\b/.test(text)) return 'svg';
  if (data.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))) return 'png';
  if (data.subarray(0, 4).equals(Buffer.from([0, 0, 1, 0]))) return 'ico';
  if (data[0] === 255 && data[1] === 216 && data[2] === 255) return 'jpg';
  if (/^GIF8[79]a/.test(text)) return 'gif';
  throw new Error('Response is not a supported logo image');
}

function writeChanged(path, data) {
  if (existsSync(path) && readFileSync(path).equals(Buffer.from(data))) return;
  const temporary = `${path}.${process.pid}.tmp`;
  try { writeFileSync(temporary, data); renameSync(temporary, path); }
  finally { if (existsSync(temporary)) unlinkSync(temporary); }
}

// Fetch once per source URL. Builds serve local files, reuse the cache offline,
// and use a neutral icon when an uncached source is unavailable.
export async function stageLogos(website, { fetchImage = fetch, warn = console.warn, refresh = false } = {}) {
  const metadataPath = join(website, 'src/content/docs/manual');
  const metadata = JSON.parse(readFileSync(join(metadataPath, '_meta.json'), 'utf8'));
  const output = join(website, 'public/logos');
  mkdirSync(output, { recursive: true });
  const all = [];
  function collect(services) {
    for (const service of services) { all.push(service); collect(service.children ?? []); }
  }
  collect(metadata.services);
  const urls = [...new Set(all.map((service) => service.logo).filter(Boolean))];
  const staged = new Map();
  async function stage(source) {
    const key = hash(source).slice(0, 20);
    const cached = extensions.map((extension) => `${key}.${extension}`).find((name) => existsSync(join(output, name)));
    try {
      if (new URL(source).protocol !== 'https:') throw new Error('Logo source must use HTTPS');
      let name = cached;
      if (!cached || refresh) {
        const response = await fetchImage(source, { headers: { 'User-Agent': 'a-wikimedia-cli-docs/1.0 (https://www.mediawiki.org/wiki/Cli)' }, signal: AbortSignal.timeout(10000) });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const data = Buffer.from(await response.arrayBuffer());
        if (data.length > 2 * 1024 * 1024) throw new Error('Logo exceeds 2 MiB');
        name = `${key}.${imageExtension(data)}`;
        writeChanged(join(output, name), data);
      }
      staged.set(source, `/logos/${name}?v=${hash(readFileSync(join(output, name))).slice(0, 12)}`);
    } catch (error) {
      warn(`Logo unavailable (${source}): ${error.message}${cached ? '; using cached image' : '; using neutral icon'}`);
      if (cached) staged.set(source, `/logos/${cached}?v=${hash(readFileSync(join(output, cached))).slice(0, 12)}`);
    }
  }
  for (let i = 0; i < urls.length; i += 4) await Promise.all(urls.slice(i, i + 4).map(stage));
  for (const service of all) {
    if (service.logo) {
      service.logoSource = service.logo;
      service.logo = staged.get(service.logo);
    }
  }
  for (const [source, target] of [['logo-credits.txt', 'NOTICE'], ['simple-icons-LICENSE', 'simple-icons-LICENSE'], ['opensearch-LICENSE', 'opensearch-LICENSE']]) {
    writeChanged(join(output, target), readFileSync(join(website, 'assets', source)));
  }
  writeChanged(join(metadataPath, '_services.json'), JSON.stringify({ services: metadata.services }, null, 2) + '\n');
  console.log(`Staged ${staged.size} cached service logos`);
}
