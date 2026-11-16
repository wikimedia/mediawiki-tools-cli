import assert from 'node:assert/strict';
import test from 'node:test';
import { fromMarkdown } from 'mdast-util-from-markdown';
import { llmsIndex, markdownPath, readDocuments, renderMarkdown } from './markdown-docs.mjs';

const context = {
  siteURL: 'https://docs.example/', paths: new Set(['/guides/one/', '/guides/two/', '/manual/']),
  sources: new Map([['guides/one.md', '/guides/one/'], ['guides/two.md', '/guides/two/']]), wikiPages: new Map(),
};

test('Markdown paths cover the homepage and nested pages', () => {
  assert.equal(markdownPath('/'), '/index.md');
  assert.equal(markdownPath('/manual/wiki/'), '/manual/wiki.md');
});

test('exports metadata, local links and flag lists while preserving examples and attribution', () => {
  const body = '<!-- Generated from docs/ -->\n\n[Next](two.md#usage) and [reference](/manual/)\n\n' +
    '```sh\necho "[Next](two.md)"\n```\n\n' +
    '<dl class="command-options">\n<dt><code>--name &lt;string&gt;</code></dt>\n' +
    '<dd>A person&#39;s name. <span class="option-default">Default: <code>World</code>.</span></dd>\n</dl>\n\n' +
    'Adapted from [MediaWiki.org](https://www.mediawiki.org/w/index.php?oldid=123) under CC BY-SA 4.0.\n';
  const result = renderMarkdown({ pathname: '/guides/one/', title: 'One', version: 'v1.2', source: 'guides/one.md', body }, context);
  assert.match(result, /^# One\n/);
  assert.match(result, /Documentation version: v1.2/);
  assert.match(result, /Canonical page: https:\/\/docs.example\/guides\/one\//);
  assert.match(result, /https:\/\/docs.example\/guides\/two.md#usage/);
  assert.match(result, /https:\/\/docs.example\/manual.md/);
  assert.match(result, /echo "\[Next\]\(two.md\)"/);
  assert.doesNotMatch(result, /<dl|<dt|<dd|<span|Generated from/);
  assert.match(result, /person's name/);
  assert.match(result, /oldid=123/);
  assert.match(result, /CC BY-SA 4.0/);
});

test('root discovery stays concise while scoped indexes include every page', () => {
  const documents = [
    { pathname: '/', title: 'Home' }, { pathname: '/manual/', title: 'Commands' },
    { pathname: '/manual/wiki/page/get/', title: 'mw wiki page get' },
    { pathname: '/guides/one/', title: 'One' },
  ];
  const docs = { documents, siteURL: context.siteURL };
  assert.doesNotMatch(llmsIndex(docs), /mw wiki page get/);
  assert.match(llmsIndex(docs), /\/manual\/llms.txt/);
  assert.match(llmsIndex(docs, '/manual/'), /mw wiki page get/);
  assert.doesNotMatch(llmsIndex(docs, '/manual/'), /\[One\]/);
});

test('all exported documentation links resolve to generated Markdown pages', () => {
  const { documents, siteURL } = readDocuments();
  const paths = new Set(documents.map((page) => markdownPath(page.pathname)));
  function check(node, page) {
    if (['link', 'definition'].includes(node.type)) {
      const url = new URL(node.url);
      if (url.origin === new URL(siteURL).origin && url.pathname.endsWith('.md')) {
        assert.ok(paths.has(url.pathname), `${page.pathname}: missing ${url.pathname}`);
      }
    }
    node.children?.forEach((child) => check(child, page));
  }
  assert.ok(documents.length > 100, 'generated command corpus should be present');
  for (const page of documents) {
    assert.doesNotMatch(page.markdown, /<dl class="command-options">/, page.pathname);
    check(fromMarkdown(page.markdown), page);
  }
});
