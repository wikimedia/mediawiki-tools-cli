import assert from 'node:assert/strict';
import test from 'node:test';
import guideLinks, { rewriteGuideLink } from './guide-links.mjs';

const pages = [
  { source: 'guides/dev/setup.md', url: '/guides/dev/setup/', wikiURL: 'https://www.mediawiki.org/wiki/Cli/guide/First_Setup' },
  { source: 'guides/gitlab/index.md', url: '/guides/gitlab/', wikiURL: 'https://www.mediawiki.org/wiki/Cli/guide/Gitlab' },
];
const siteURL = 'https://a-wikimedia-cli.toolforge.org';
const sources = new Map(pages.map((page) => [page.source, page.url]));
const wikiPages = new Map(pages.map((page) => [page.wikiURL.replaceAll('_', ' '), page.url]));

test('guide links resolve from their original source, including index pages and fragments', () => {
  for (const [raw, expected] of [
    ['../gitlab/index.md#merge-requests', '/guides/gitlab/#merge-requests'],
    ['https://www.mediawiki.org/wiki/Cli/guide/First_Setup', '/guides/dev/setup/'],
    ['https://www.mediawiki.org/wiki/Cli/ref/mw_wiki_page_put#options', '/manual/wiki/page/put/#options'],
    ['https://www.mediawiki.org/wiki/Cli/ref', '/manual/'],
    ['https://a-wikimedia-cli.toolforge.org/manual/gitlab/#options', '/manual/gitlab/#options'],
    ['https://a-wikimedia-cli.toolforge.org/guides/installation/', '/guides/installation/'],
    ['setup.tape', 'https://gitlab.wikimedia.org/repos/releng/cli/-/blob/main/docs/guides/dev/setup.tape'],
    ['#create-mediawiki', '#create-mediawiki'],
    ['https://example.org/setup.md', 'https://example.org/setup.md'],
    ['https://www.mediawiki.org/wiki/Cli/guide/First_Setup?oldid=123', 'https://www.mediawiki.org/wiki/Cli/guide/First_Setup?oldid=123'],
  ]) assert.equal(rewriteGuideLink(raw, 'guides/dev/setup.md', sources, wikiPages, siteURL), expected);
});

test('missing relative guides fail the build instead of becoming broken links', () => {
  assert.throws(() => rewriteGuideLink('missing.md', 'guides/dev/setup.md', sources, wikiPages), /Unknown guide link/);
});

test('guide visitors target generated guide pages, leaving code and reference pages alone', () => {
  const plugin = guideLinks({ pages, siteURL });
  const visitors = plugin({ fileURL: new URL('file:///project/website/src/content/docs/guides/gitlab/index.md') });
  const link = { type: 'link', url: '../dev/setup.md', children: [] };
  const definition = { type: 'definition', url: '../dev/setup.md#install' };
  assert.equal(visitors.link(link).url, '/guides/dev/setup/');
  assert.equal(visitors.definition(definition).url, '/guides/dev/setup/#install');
  assert.equal(link.url, '../dev/setup.md');
  assert.equal(visitors.code, undefined);
  assert.equal(visitors.inlineCode, undefined);
  assert.equal(visitors.html, undefined);
  assert.equal(plugin({ fileURL: new URL('file:///project/website/src/content/docs/manual/wiki/index.md') }), undefined);
});
