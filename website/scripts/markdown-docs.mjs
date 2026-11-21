import { readFileSync, readdirSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { fromMarkdown } from 'mdast-util-from-markdown';
import { toMarkdown } from 'mdast-util-to-markdown';
import { decodeHTML } from 'entities';
import { parse } from 'yaml';
import { rewriteGuideLink } from './guide-links.mjs';

export const markdownPath = (pathname) => `${pathname.replace(/\/$/, '') || '/index'}.md`;

// Read the same generated sources as Starlight, rather than scraping navigation,
// rendered syntax highlighting or other browser-only controls out of the HTML.
export function readDocuments(website = new URL('../', import.meta.url)) {
  const root = new URL('src/content/docs/', typeof website === 'string' ? pathToFileURL(`${website}/`) : website);
  const guides = JSON.parse(readFileSync(new URL('guides/_meta.json', root), 'utf8'));
  const manual = JSON.parse(readFileSync(new URL('manual/_meta.json', root), 'utf8'));
  const files = Object.fromEntries(readdirSync(root, { recursive: true }).filter((name) => name.endsWith('.md')).map((name) => [name, readFileSync(new URL(name, root), 'utf8')]));
  return createDocuments(files, guides, manual);
}

export function createDocuments(files, guides, manual) {
  const documents = Object.entries(files).sort(([a], [b]) => a.localeCompare(b)).map(([name, raw]) => {
    const match = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/.exec(raw);
    if (!match) throw new Error(`Missing frontmatter: ${name}`);
    const data = parse(match[1]);
    const slug = name.replace(/(?:^|\/)index\.md$/, '').replace(/\.md$/, '');
    return { pathname: `/${slug}${slug ? '/' : ''}`, title: data.title, description: data.description,
      version: data.manualVersion || manual.version.label, source: data.guideSource, body: raw.slice(match[0].length), markdown: '' };
  });
  const paths = new Set(documents.map((page) => page.pathname));
  const sources = new Map(guides.pages.map((page) => [page.source, page.url]));
  const wikiPages = new Map(guides.pages.filter((page) => page.wikiURL).map((page) => [decodeURI(page.wikiURL).replaceAll('_', ' '), page.url]));
  const context = { siteURL: guides.siteURL, sources, wikiPages, paths };
  for (const page of documents) {
    if (page.pathname === '/') {
      page.body += '\n\n## Services\n\n' + manual.services.map((service) => `- [${service.name}](${service.link})`).join('\n');
    }
    page.markdown = renderMarkdown(page, context);
  }
  return { documents, siteURL: guides.siteURL };
}

function inlineCode(value) {
  const delimiter = '`'.repeat(Math.max(0, ...Array.from(value.matchAll(/`+/g), (match) => match[0].length)) + 1);
  return `${delimiter} ${value} ${delimiter}`;
}

export function renderMarkdown(page, context) {
  const tree = fromMarkdown(page.body);
  function visit(node) {
    if (node.type === 'html') {
      // The web manual uses definition lists for flags. Export ordinary Markdown.
      if (node.value.startsWith('<dl class="command-options">')) {
        const options = Array.from(node.value.matchAll(/<dt><code>([\s\S]*?)<\/code><\/dt>\s*<dd>([\s\S]*?)<\/dd>/g), ([, label, description]) => {
          const text = description.replace(/<code>([\s\S]*?)<\/code>/g, (_, value) => inlineCode(decodeHTML(value))).replace(/<\/?span[^>]*>/g, '');
          return `- ${inlineCode(decodeHTML(label))}: ${decodeHTML(text).replace(/\n/g, '\n  ')}`;
        });
        return fromMarkdown(options.join('\n')).children;
      }
      if (/^<!--/.test(node.value)) return [];
    }
    if (['link', 'image', 'definition'].includes(node.type)) {
      const rewritten = page.source ? rewriteGuideLink(node.url, page.source, context.sources, context.wikiPages, context.siteURL) : node.url;
      const url = new URL(rewritten, new URL(page.pathname, context.siteURL));
      if (url.origin === new URL(context.siteURL).origin && node.type !== 'image' && context.paths.has(url.pathname)) url.pathname = markdownPath(url.pathname);
      node.url = url.href;
    }
    if (node.children) node.children = node.children.flatMap(visit);
    return [node];
  }
  visit(tree);
  const description = page.description && !page.body.includes(page.description) ? `${page.description}\n\n` : '';
  const canonicalPath = page.pathname === '/' ? '/' : page.pathname.replace(/\/$/, '');
  return `# ${page.title}\n\n${description}Documentation version: ${page.version}\n\nCanonical page: ${new URL(canonicalPath, context.siteURL).href}\n\n${toMarkdown(tree, { fences: true, bullet: '-' })}`;
}

export function llmsIndex({ documents, siteURL }, scope = '/') {
  const link = (page) => `- [${page.title}](${new URL(markdownPath(page.pathname), siteURL).href})${page.description ? `: ${page.description.replace(/\s+/g, ' ')}` : ''}`;
  let body = '# a Wikimedia CLI\n\n> Command reference and workflow guides for working with Wikimedia from your terminal.\n\n';
  body += 'Pages describe the documentation version printed in each document. For a different installed version, use `wm --help` or append `--help` to a command.\n\n';
  body += 'Each HTML page links to its Markdown representation. Request `Accept: text/markdown` on the published site, or follow the explicit `.md` links below.\n\n';
  if (scope === '/') {
    const start = ['/', '/guides/installation/', '/guides/', '/manual/'].map((path) => documents.find((page) => page.pathname === path)).filter(Boolean);
    body += '## Start here\n\n' + start.map(link).join('\n');
    body += `\n\n## Documentation indexes\n\n- [Workflow guides](${new URL('/guides/llms.txt', siteURL).href})\n- [Complete command index](${new URL('/manual/llms.txt', siteURL).href})\n`;
  } else {
    body += `## ${scope === '/guides/' ? 'Workflow guides' : 'Commands'}\n\n` + documents.filter((page) => page.pathname.startsWith(scope)).map(link).join('\n') + '\n';
  }
  return body;
}
