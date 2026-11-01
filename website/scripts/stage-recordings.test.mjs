import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, unlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { stageRecordings } from './stage-recordings.mjs';

test('optional recordings can be absent, staged, updated, or reused without the original', () => {
  const repository = mkdtempSync(join(tmpdir(), 'mwcli-recordings-'));
  const website = join(repository, 'website');
  try {
    mkdirSync(join(repository, 'docs'));
    mkdirSync(join(repository, '_recordings'));
    mkdirSync(join(website, 'src/content/docs/manual'), { recursive: true });
    writeFileSync(join(repository, 'docs/recordings.json'), JSON.stringify({ recordings: [
      { id: 'demo', title: 'Demo', file: 'demo.webm', guide: 'dev/setup', commands: [] },
    ] }));
    const metadata = () => JSON.parse(readFileSync(join(website, 'src/content/docs/manual/_recordings.json'))).recordings;
    stageRecordings(repository, website);
    assert.deepEqual(metadata(), []);
    const source = join(repository, '_recordings/demo.webm');
    writeFileSync(source, 'first recording');
    stageRecordings(repository, website);
    const originalURL = metadata()[0].video;
    assert.match(originalURL, /^\/media\/demo.webm\?v=/);
    assert.equal(readFileSync(join(website, 'public/media/demo.webm'), 'utf8'), 'first recording');
    writeFileSync(source, 'updated recording');
    stageRecordings(repository, website);
    const updatedURL = metadata()[0].video;
    assert.notEqual(updatedURL, originalURL);
    unlinkSync(source);
    stageRecordings(repository, website);
    assert.equal(metadata()[0].video, updatedURL);
    unlinkSync(join(website, 'public/media/demo.webm'));
    stageRecordings(repository, website);
    assert.deepEqual(metadata(), []);
  } finally { rmSync(repository, { recursive: true, force: true }); }
});
