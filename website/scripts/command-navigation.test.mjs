import assert from 'node:assert/strict';
import test from 'node:test';
import { commandSections } from '../src/command-navigation.mjs';
const command = (name, link, parent) => ({ name, title: `mw ${link.slice(8).replaceAll('/', ' ').trim()}`, link, parent });
const commands = [
  command('mw', '/manual/'),
  command('gerrit', '/manual/gerrit/', '/manual/'),
  command('access', '/manual/gerrit/access/', '/manual/gerrit/'),
  command('accounts', '/manual/gerrit/accounts/', '/manual/gerrit/'),
  command('get', '/manual/gerrit/accounts/get/', '/manual/gerrit/accounts/'),
  command('list', '/manual/gerrit/accounts/list/', '/manual/gerrit/accounts/'),
  command('dev', '/manual/dev/', '/manual/'),
  command('mediawiki', '/manual/dev/mediawiki/', '/manual/dev/'),
  command('create', '/manual/dev/mediawiki/create/', '/manual/dev/mediawiki/'),
  command('version', '/manual/version/', '/manual/'),
];

test('command family and active branch remain available on nested command pages', () => {
  const sections = commandSections(commands, '/manual/gerrit/accounts/get/');
  assert.deepEqual(sections.map((section) => section.name), ['gerrit', 'accounts']);
  assert.deepEqual(sections[0].children.map((child) => child.name), ['access', 'accounts']);
  assert.deepEqual(sections[1].children.map((child) => child.name), ['get', 'list']);
});

test('development navigation starts at the service area already selected on the left', () => {
  assert.deepEqual(commandSections(commands, '/manual/dev/'), []);
  assert.deepEqual(commandSections(commands, '/manual/dev/mediawiki/').map((section) => section.name), ['mediawiki']);
  assert.deepEqual(commandSections(commands, '/manual/dev/mediawiki/create/').map((section) => section.name), ['mediawiki']);
});

test('guides and commands with no subcommands do not get reference navigation', () => {
  assert.deepEqual(commandSections(commands, '/guides/gitlab/'), []);
  assert.deepEqual(commandSections(commands, '/manual/version/'), []);
});
