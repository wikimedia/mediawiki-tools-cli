import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, renameSync, unlinkSync, writeFileSync } from 'node:fs';
import { basename, extname, join } from 'node:path';

// Media stays outside Git. Reuse previously staged files if a recording is not
// available locally, and omit references that have never been staged.
export function stageRecordings(repository, website) {
  const { recordings } = JSON.parse(readFileSync(join(repository, 'docs/recordings.json'), 'utf8'));
  const output = join(website, 'public/media');
  const available = [];
  for (const recording of recordings) {
    if (basename(recording.file) !== recording.file || !/\.(webm|mp4)$/.test(recording.file)) throw new Error(`Invalid recording filename: ${recording.file}`);
    function stage(name) {
      const source = join(repository, '_recordings', name);
      const target = join(output, name);
      if (!existsSync(source) && !existsSync(target)) return undefined;
      const data = readFileSync(existsSync(source) ? source : target);
      const hash = createHash('sha256').update(data).digest('hex').slice(0, 12);
      if (!existsSync(target) || !readFileSync(target).equals(data)) {
        mkdirSync(output, { recursive: true });
        const temporary = `${target}.${process.pid}.tmp`;
        try { writeFileSync(temporary, data); renameSync(temporary, target); }
        finally { if (existsSync(temporary)) unlinkSync(temporary); }
      }
      return `/media/${name}?v=${hash}`;
    }
    const video = stage(recording.file);
    if (!video) continue;
    const poster = stage(recording.file.slice(0, -extname(recording.file).length) + '.webp');
    available.push({ ...recording, video, ...(poster ? { poster } : {}) });
  }
  const metadata = join(website, 'src/content/docs/manual/_recordings.json');
  const data = JSON.stringify({ recordings: available }, null, 2) + '\n';
  if (!existsSync(metadata) || readFileSync(metadata, 'utf8') !== data) writeFileSync(metadata, data);
  console.log(`Staged ${available.length} optional recordings for the site`);
}
