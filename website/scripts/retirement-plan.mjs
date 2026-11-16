import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const referenceRoot = 'Cli/ref';
const repository = fileURLToPath(new URL('../../', import.meta.url));

// These names were replaced rather than retained as runtime aliases.
const renamed = new Map([
  ['cloudvps auth add', 'cloudvps auth login'],
  ['cloudvps auth check', 'cloudvps auth status'],
  ['cloudvps auth list', 'cloudvps auth profiles'],
  ['wiki auth accounts', 'wiki auth profiles'],
]);

export function referenceDestination(title, commands) {
  if (title !== referenceRoot && !title.startsWith(`${referenceRoot}/`)) return { match: 'out-of-scope' };
  const root = commands.find((command) => !command.parent);
  const raw = title.slice(referenceRoot.length).replace(/^\//, '').replaceAll('_', ' ').trim();
  if (raw.includes('/')) return { match: 'translation-or-subpage' };
  const words = raw ? raw.split(/\s+/) : [root.name];
  if (![root.name, 'wm', ...(root.aliases ?? [])].includes(words.shift())) return { match: 'unrecognized-title' };
  let name = words.join(' ');
  let changed = false;
  for (const [before, after] of renamed) {
    if (name === before || name.startsWith(`${before} `)) {
      name = after + name.slice(before.length);
      changed = true;
      break;
    }
  }
  // "tools" used to contain the Toolforge CLI; it now contains Quip/Toolviews.
  if (name.startsWith('tools ') && !commands.some((command) => command.parent === '/manual/tools/' && command.name === name.split(' ')[1])) {
    name = `toolforge ${name.slice('tools '.length)}`;
    changed = true;
  }
  let current = root;
  const remaining = name ? name.split(' ') : [];
  while (remaining.length) {
    const child = commands.find((command) => command.parent === current.link && [command.name, ...(command.aliases ?? [])].includes(remaining[0]));
    if (!child) break;
    current = child;
    remaining.shift();
  }
  return {
    match: remaining.length ? 'ancestor' : changed ? 'renamed' : raw && raw !== current.title ? 'alias' : 'exact',
    canonicalCommand: current.title, path: current.link,
    ...(remaining.length ? { unmatched: remaining.join(' ') } : {}),
  };
}

export function retirementPlan(titles, commands, siteURL) {
  return titles.map((title) => {
    const destination = referenceDestination(title, commands);
    const target = destination.path ? new URL(destination.path, siteURL).href : undefined;
    return {
      title, ...destination, requiresReview: true,
      ...(target ? {
        target,
        notice: `The generated command reference has moved. See [${target} ${destination.canonicalCommand}] on the a Wikimedia CLI documentation site.\n`,
        deletionReason: `Generated command reference retired; maintained documentation: ${target}`,
      } : {}),
    };
  });
}

export async function referenceTitles(fetchPage = fetch) {
  const titles = [];
  const seen = new Set();
  let continuation = {};
  while (true) {
    const url = new URL('https://www.mediawiki.org/w/api.php');
    url.search = new URLSearchParams({ action: 'query', format: 'json', formatversion: '2', list: 'allpages', apnamespace: '0', apprefix: referenceRoot, aplimit: '500', ...continuation }).toString();
    const response = await fetchPage(url, { headers: { 'User-Agent': 'a-wikimedia-cli-docs/1.0 (https://a-wikimedia-cli.toolforge.org/)' }, signal: AbortSignal.timeout(15000) });
    if (!response.ok) throw new Error(`Wiki inventory returned HTTP ${response.status}`);
    const data = await response.json();
    if (data.error || !Array.isArray(data.query?.allpages)) throw new Error('Wiki inventory did not return a page list');
    titles.push(...data.query.allpages.map((page) => page.title).filter((title) => title === referenceRoot || title.startsWith(`${referenceRoot}/`)));
    if (!data.continue?.apcontinue) return titles;
    if (seen.has(data.continue.apcontinue)) throw new Error('Wiki inventory repeated a continuation token');
    seen.add(data.continue.apcontinue);
    continuation = data.continue;
  }
}

async function main() {
  const { values } = parseArgs({ options: { inventory: { type: 'string' }, output: { type: 'string' } } });
  const metadata = JSON.parse(readFileSync(resolve(repository, 'website/src/content/docs/manual/_meta.json'), 'utf8'));
  const { site_url: siteURL } = JSON.parse(readFileSync(resolve(repository, 'docs/guides.json'), 'utf8'));
  const inventory = values.inventory ? JSON.parse(readFileSync(values.inventory, 'utf8')) : { titles: await referenceTitles() };
  const titles = Array.isArray(inventory) ? inventory : inventory.titles;
  if (!Array.isArray(titles) || titles.some((title) => typeof title !== 'string')) throw new Error('Inventory must contain an array of page titles');
  const pages = retirementPlan(titles, metadata.commands, siteURL);
  const output = values.output ?? resolve(repository, '_docs/reference-retirement.json');
  mkdirSync(dirname(output), { recursive: true });
  writeFileSync(output, JSON.stringify({ readOnly: true, version: metadata.version, pages }, null, 2) + '\n');
  const counts = pages.reduce((result, page) => ({ ...result, [page.match]: (result[page.match] ?? 0) + 1 }), {});
  console.log(`Prepared ${pages.length} reference-page entries in ${output}: ${JSON.stringify(counts)}`);
  console.log('No pages edited or deleted. Review page contents and verify replacement targets before applying any changes.');
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { await main(); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
