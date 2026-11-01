import assert from 'node:assert/strict';
import test from 'node:test';
import { referenceDestination, referenceTitles, retirementPlan } from './retirement-plan.mjs';

const commands = [
  { name: 'mw', title: 'mw', link: '/manual/' },
  { name: 'dev', aliases: ['docker'], title: 'mw dev', link: '/manual/dev/', parent: '/manual/' },
  { name: 'mysql', title: 'mw dev mysql', link: '/manual/dev/mysql/', parent: '/manual/dev/' },
  { name: 'start', title: 'mw dev mysql start', link: '/manual/dev/mysql/start/', parent: '/manual/dev/mysql/' },
  { name: 'tools', title: 'mw tools', link: '/manual/tools/', parent: '/manual/' },
  { name: 'quip', title: 'mw tools quip', link: '/manual/tools/quip/', parent: '/manual/tools/' },
  { name: 'toolforge', title: 'mw toolforge', link: '/manual/toolforge/', parent: '/manual/' },
  { name: 'job', title: 'mw toolforge job', link: '/manual/toolforge/job/', parent: '/manual/toolforge/' },
  { name: 'cloudvps', title: 'mw cloudvps', link: '/manual/cloudvps/', parent: '/manual/' },
  { name: 'auth', title: 'mw cloudvps auth', link: '/manual/cloudvps/auth/', parent: '/manual/cloudvps/' },
  { name: 'status', title: 'mw cloudvps auth status', link: '/manual/cloudvps/auth/status/', parent: '/manual/cloudvps/auth/' },
];

test('retirement destinations use current paths, aliases, known renames and ancestors', () => {
  for (const [title, match, path] of [
    ['Cli/ref', 'exact', '/manual/'],
    ['Cli/ref/mw_dev_mysql_start', 'exact', '/manual/dev/mysql/start/'],
    ['Cli/ref/mw docker mysql start', 'alias', '/manual/dev/mysql/start/'],
    ['Cli/ref/mw cloudvps auth check', 'renamed', '/manual/cloudvps/auth/status/'],
    ['Cli/ref/mw tools job', 'renamed', '/manual/toolforge/job/'],
    ['Cli/ref/mw tools quip', 'exact', '/manual/tools/quip/'],
    ['Cli/ref/mw dev mysql removed', 'ancestor', '/manual/dev/mysql/'],
    ['Cli/ref/mw dev/fr', 'translation-or-subpage', undefined],
    ['Cli/ref/something unrelated', 'unrecognized-title', undefined],
    ['Cli/guide/Installation', 'out-of-scope', undefined],
  ]) {
    const result = referenceDestination(title, commands);
    assert.equal(result.match, match, title);
    assert.equal(result.path, path, title);
  }
});

test('plans retain review requirements and never invent targets for translations', () => {
  const plan = retirementPlan(['Cli/ref/mw docker mysql', 'Cli/ref/mw docker/fr'], commands, 'https://a-wikimedia-cli.toolforge.org');
  assert.equal(plan[0].target, 'https://a-wikimedia-cli.toolforge.org/manual/dev/mysql/');
  assert.ok(plan[0].deletionReason.includes(plan[0].target));
  assert.ok(plan[0].notice.includes(plan[0].target));
  assert.equal(plan[1].target, undefined);
  assert.ok(plan.every((page) => page.requiresReview));
});

test('public inventory follows continuation using GET and excludes other wiki content', async () => {
  const calls = [];
  const titles = await referenceTitles(async (url, options) => {
    calls.push(url);
    assert.equal(options.method, undefined);
    assert.equal(url.searchParams.get('apnamespace'), '0');
    if (calls.length === 1) return new Response(JSON.stringify({ query: { allpages: [{ title: 'Cli/ref' }] }, continue: { apcontinue: 'next', continue: '-||' } }));
    assert.equal(url.searchParams.get('apcontinue'), 'next');
    return new Response(JSON.stringify({ query: { allpages: [{ title: 'Cli/ref/mw dev' }, { title: 'Cli/reference unrelated' }] } }));
  });
  assert.deepEqual(titles, ['Cli/ref', 'Cli/ref/mw dev']);
  assert.equal(calls.length, 2);
  await assert.rejects(referenceTitles(async () => new Response(JSON.stringify({ error: { code: 'readonly' } }))), /did not return a page list/);
});
