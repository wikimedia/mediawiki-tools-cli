import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { taskContent, taskURL, taskCommand } from '../src/docs-feedback.mjs';

test('both reporting routes preserve page, section, quote and issue details', () => {
  const task = taskContent({
    page: 'https://a-wikimedia-cli.toolforge.org/manual/dev/start/#examples',
    title: 'mw dev start', version: 'v2026.11.1.0', section: 'Examples',
    source: 'https://gitlab.wikimedia.org/example', quote: 'First line\nSecond line',
    issue: 'The instructions are unclear.', summary: '',
  });
  const url = new URL(taskURL(task));
  assert.equal(task.title, 'Docs: mw dev start');
  assert.equal(url.searchParams.get('projects'), 'a_wikimedia_cli');
  assert.equal(url.searchParams.get('title'), task.title);
  assert.equal(url.searchParams.get('description'), task.description);
  for (const expected of ['#examples', 'Section: Examples', 'v2026.11.1.0', 'Source: https://', '> First line\n> Second line', 'Issue:\nThe instructions are unclear.']) {
    assert.ok(task.description.includes(expected), expected);
  }
  assert.ok(!task.description.includes('undefined'));
});

test('copied POSIX command preserves literal user text without running shell syntax', { skip: process.platform === 'win32' }, () => {
  const task = { title: 'A "title" with apostrophe\'s', description: '$(echo INJECTED) `echo BACKTICK`\n\'; echo ESCAPED; #\nUnicode: é' };
  const script = 'mw() { printf "%s\\0" "$@"; }; ' + taskCommand(task);
  const result = spawnSync('/bin/sh', ['-c', script], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(result.stdout.split('\0'), ['phab', 'create', '--title', task.title, '--project', 'a_wikimedia_cli', '--description', task.description, '']);
});

test('PowerShell uses temporary UTF-8 JSON and leaves stdin free for confirmation', () => {
  const command = taskCommand({ title: "What's wrong?", description: 'It’s “quoted” $env:SECRET `example`' }, 'powershell');
  assert.ok(command.includes('UTF8Encoding'));
  assert.ok(command.includes('"title":"What\'\'s wrong?"'));
  assert.ok(command.includes('"description":"It’’s “quoted” $env:SECRET `example`"'));
  assert.ok(command.includes('mw phab create --input $mwDocsTaskFile'));
  assert.ok(command.includes('finally { Remove-Item -LiteralPath $mwDocsTaskFile }'));
  assert.ok(!command.includes('--yes'));
});
