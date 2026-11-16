import { fileURLToPath } from 'node:url';
import { posix } from 'node:path';
import { pageHref } from '../src/route-path.mjs';

// Operate on Markdown link nodes, leaving code blocks and inline examples intact.
export default function guideLinks({ pages, siteURL }) {
  const sources = new Map(pages.map((page) => [page.source, page.url]));
  const wikiPages = new Map(pages.filter((page) => page.wikiURL).map((page) => [normalize(page.wikiURL), page.url]));
  const generated = new Map(pages.map((page) => [`${page.url}index.md`, page.source]));
  return ({ fileURL }) => {
    if (!fileURL) return;
    const filename = fileURLToPath(fileURL).replaceAll('\\', '/');
    const prefix = '/src/content/docs';
    const index = filename.lastIndexOf(prefix);
    const source = index >= 0 ? generated.get(filename.slice(index + prefix.length)) : undefined;
    function rewrite(node) {
      const url = source ? rewriteGuideLink(node.url, source, sources, wikiPages, siteURL) : node.url;
      const local = new URL(url, siteURL);
      const isPage = local.origin === new URL(siteURL).origin &&
        (local.pathname === '/manual/' || local.pathname.startsWith('/manual/') ||
         local.pathname === '/guides/' || local.pathname.startsWith('/guides/'));
      return { ...node, url: isPage ? pageHref(url) : url };
    }
    return { name: 'local-guide-links', link: rewrite, definition: rewrite, ...(source ? { image: rewrite } : {}) };
  };
}

function normalize(url) { return decodeURI(url).replaceAll('_', ' '); }

export function rewriteGuideLink(raw, source, sources, wikiPages, siteURL) {
  const url = new URL(raw, 'https://guide.invalid/');
  if (siteURL && url.origin === new URL(siteURL).origin && (url.pathname.startsWith('/manual/') || url.pathname.startsWith('/guides/'))) {
    return url.pathname + url.search + url.hash;
  }
  if (url.origin === 'https://guide.invalid' && !raw.startsWith('/')) {
    if (!url.pathname.endsWith('.md') && !posix.extname(url.pathname)) return raw;
    const target = posix.normalize(posix.join(posix.dirname(source), decodeURIComponent(raw.split(/[?#]/)[0])));
    if (sources.has(target)) return sources.get(target) + url.search + url.hash;
    if (target.endsWith('.md')) throw new Error(`Unknown guide link in ${source}: ${raw}`);
    if (target.startsWith('guides/') && !target.split('/').includes('..')) {
      return `https://gitlab.wikimedia.org/repos/releng/cli/-/blob/main/docs/${target}${url.hash}`;
    }
    return raw;
  }
  const wiki = wikiPages.get(normalize(url.origin + url.pathname));
  if (wiki && !url.search) return wiki + url.hash;
  if (url.origin === 'https://www.mediawiki.org' && !url.search && url.pathname.startsWith('/wiki/Cli/ref')) {
    const command = url.pathname.slice('/wiki/Cli/ref'.length);
    if (!command) return '/manual/' + url.hash;
    if (command.startsWith('/mw_')) return `/manual/${command.slice('/mw_'.length).replaceAll('_', '/')}/${url.hash}`;
  }
  return raw;
}
