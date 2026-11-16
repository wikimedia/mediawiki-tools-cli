import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { extname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const output = fileURLToPath(new URL('../dist/', import.meta.url));
const origin = 'https://a-wikimedia-cli.toolforge.org';
const pages = readdirSync(output, { recursive: true }).filter((name) => name.endsWith('.html'));
const broken = new Set();
for (const page of pages) {
  const html = readFileSync(join(output, page), 'utf8');
  const pageURL = new URL(page.replace(/index\.html$/, ''), `${origin}/`);
  for (const [, href] of html.matchAll(/\bhref="([^"]+)"/g)) {
    if (href.startsWith('#')) continue;
    const url = new URL(href.replaceAll('&amp;', '&'), pageURL);
    if (url.origin !== origin) continue;
    const path = decodeURIComponent(url.pathname);
    const routePath = path.replace(/\/$/, '') || '/';
    if (path !== '/' && path.endsWith('/') && (routePath === '/manual' || routePath.startsWith('/manual/') || routePath === '/guides' || routePath.startsWith('/guides/'))) {
      broken.add(`${page}: noncanonical page link ${href}`);
      continue;
    }
    const targets = [
      join(output, routePath),
      ...(extname(routePath) ? [] : [join(output, `${routePath}.html`), join(output, routePath, 'index.html')]),
    ];
    if (!targets.some((target) => existsSync(target) && statSync(target).isFile())) broken.add(`${page}: ${href}`);
  }
}
if (broken.size) {
  console.error([...broken].slice(0, 30).join('\n'));
  throw new Error(`${broken.size} broken local links in the built manual`);
}
console.log(`Checked local links in ${pages.length} HTML pages.`);
