import { spawn } from 'node:child_process';
import { stageRecordings } from './stage-recordings.mjs';
import { stageLogos } from './stage-logos.mjs';
import { generateGuides } from './generate-guides.mjs';
import { fileURLToPath } from 'node:url';

export const repository = fileURLToPath(new URL('../../', import.meta.url));
export const website = fileURLToPath(new URL('../', import.meta.url));

export function generate() {
  const child = spawn('go', ['run', '-mod=mod', './tools/docs-gen', '-mode', 'manual'], {
    cwd: repository,
    stdio: 'inherit',
  });
  return { child, done: new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', async (code, signal) => {
      if (code !== 0) { reject(new Error(`Manual generation failed (${signal || code})`)); return; }
      try { stageRecordings(repository, website); generateGuides(repository, website); await stageLogos(website); resolve(); }
      catch (error) { reject(error); }
    });
  }) };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try { await generate().done; }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
