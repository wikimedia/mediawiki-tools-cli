import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, readFileSync, existsSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const script = fileURLToPath(new URL('./retire-wiki-reference.sh', import.meta.url));
function fixture(t) {
  const directory = mkdtempSync(join(tmpdir(), 'reference-retirement-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const plan = join(directory, 'plan.json');
  const log = join(directory, 'calls');
  const binary = join(directory, 'mw');
  const pages = ['first', 'second'].map((name) => ({ title: `Cli/ref/mw ${name}`, match: 'exact', target: `https://a-wikimedia-cli.toolforge.org/manual/${name}`, deletionReason: `Reference moved: https://a-wikimedia-cli.toolforge.org/manual/${name}` }));
  writeFileSync(plan, JSON.stringify({ pages: [...pages, { title: 'Cli/ref/mw/fr', match: 'translation-or-subpage' }] }));
  writeFileSync(binary, '#!/bin/bash\nprintf "%s\\n" "$@" >> "$RETIRE_TEST_LOG"\nif [[ ${RETIRE_TEST_FAIL:-} == yes ]]; then exit 7; fi\n', { mode: 0o700 });
  const run = (args, extra = {}) => spawnSync('bash', [script, '--plan', plan, ...args], { encoding: 'utf8', env: { ...process.env, MW_BIN: binary, RETIRE_TEST_LOG: log, ...extra } });
  return { run, log, plan, pages };
}

test('retirement previews never call the CLI and one-page selection is exact', (t) => {
  const f = fixture(t);
  const result = f.run(['--title', f.pages[0].title]);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /Preview: 1 pages/);
  assert.equal(existsSync(f.log), false);
});

test('one real invocation receives the exact title, profile and planned reason', (t) => {
  const f = fixture(t);
  const result = f.run(['--title', f.pages[1].title, '--profile', 'wikimedia/test', '--apply']);
  assert.equal(result.status, 0, result.stderr);
  const args = readFileSync(f.log, 'utf8').trimEnd().split('\n');
  assert.deepEqual(args, ['--no-interaction', 'wiki', 'page', 'delete', '--wiki', 'https://www.mediawiki.org/w/api.php', '--title', f.pages[1].title, '--reason', f.pages[1].deletionReason, '--profile', 'wikimedia/test']);
});

test('a failed deletion stops the batch; unmapped pages are excluded', (t) => {
  const f = fixture(t);
  const result = f.run(['--all', '--profile', 'wikimedia/test', '--apply'], { RETIRE_TEST_FAIL: 'yes' });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Stopped at: Cli\/ref\/mw first/);
  assert.equal(readFileSync(f.log, 'utf8').includes(f.pages[1].title), false);
  const preview = f.run(['--all']);
  assert.match(preview.stdout, /Preview: 2 pages/);
});

test('missing selectors, unknown titles, duplicate plan entries and missing profiles do not delete', (t) => {
  const f = fixture(t);
  for (const args of [[], ['--title', 'Cli/ref/unknown'], ['--all', '--apply'], ['--all', '--title', f.pages[0].title]]) {
    assert.notEqual(f.run(args).status, 0);
  }
  writeFileSync(f.plan, JSON.stringify({ pages: [f.pages[0], f.pages[0]] }));
  assert.notEqual(f.run(['--all', '--apply', '--profile', 'test']).status, 0);
  assert.equal(existsSync(f.log), false);
});

test('slash-free manual root can be selected and applied', (t) => {
  const f = fixture(t);
  const target = 'https://a-wikimedia-cli.toolforge.org/manual';
  writeFileSync(f.plan, JSON.stringify({ pages: [{ title: 'Cli/ref', match: 'exact', target, deletionReason: `Reference moved: ${target}` }] }));
  const result = f.run(['--title', 'Cli/ref', '--profile', 'test', '--apply']);
  assert.equal(result.status, 0, result.stderr);
  assert.ok(readFileSync(f.log, 'utf8').includes(`Reference moved: ${target}\n`));
});

test('stale trailing-slash targets or reasons stop the whole batch before deletion', (t) => {
  const f = fixture(t);
  for (const stale of [
    { ...f.pages[1], target: `${f.pages[1].target}/`, deletionReason: `${f.pages[1].deletionReason}/` },
    { ...f.pages[1], deletionReason: `${f.pages[1].deletionReason}/` },
    { ...f.pages[1], deletionReason: 'Missing target' },
  ]) {
    writeFileSync(f.plan, JSON.stringify({ pages: [f.pages[0], stale] }));
    const result = f.run(['--all', '--profile', 'test', '--apply']);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /Noncanonical target or deletion reason/);
    assert.equal(existsSync(f.log), false);
  }
});
