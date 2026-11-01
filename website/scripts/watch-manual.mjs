import { readFileSync } from 'node:fs';
import { join, resolve, sep } from 'node:path';
import { generate, repository, website } from './generate.mjs';

// Reuse Astro/Vite's watcher and server lifecycle, including its HMR support.
export function watchManual() {
  return {
    name: 'wikimedia-cli-command-help',
    hooks: {
      'astro:server:setup': ({ server, logger }) => {
        const directories = ['cmd', 'commands', 'internal', 'docs', 'mount', 'tools/docs-gen', 'website/assets']
          .map((directory) => `${join(repository, directory)}${sep}`);
        const modules = ['go.mod', 'go.sum'].map((name) => join(repository, name));
        const metadata = ['manual', 'guides'].map((directory) => join(website, `src/content/docs/${directory}/_meta.json`));
        const snapshot = () => metadata.map((filename) => readFileSync(filename, 'utf8')).join('\n');
        const recordings = `${join(repository, '_recordings')}${sep}`;
        server.watcher.add([...directories, ...modules, recordings]);
        let timer;
        let current;
        let pending = false;
        let stopped = false;

        async function refresh() {
          if (stopped) return;
          if (current) { pending = true; return; }
          do {
            pending = false;
            const previous = snapshot();
            current = generate();
            try {
              await current.done;
              logger.info('Command reference and guides regenerated.');
              if (snapshot() !== previous) {
                current = undefined;
                await server.restart();
                return;
              }
            } catch (error) {
              logger.error(error.message);
            } finally { current = undefined; }
          } while (pending && !stopped);
        }

        const changed = (filename) => {
          filename = resolve(filename);
          const isRecording = filename.startsWith(recordings) && !filename.slice(recordings.length).includes(sep) && /\.(webm|mp4|webp)$/.test(filename);
          if (!isRecording && !modules.includes(filename) && (!directories.some((directory) => filename.startsWith(directory)) || !/(\.(go|md|example|json|txt|ya?ml)|LICENSE)$/.test(filename))) return;
          clearTimeout(timer);
          timer = setTimeout(() => { refresh().catch((error) => logger.error(error.message)); }, 350);
        };
        for (const event of ['add', 'change', 'unlink']) server.watcher.on(event, changed);
        server.httpServer?.once('close', () => {
          stopped = true;
          clearTimeout(timer);
          current?.child.kill();
          for (const event of ['add', 'change', 'unlink']) server.watcher.off(event, changed);
        });
      },
    },
  };
}
