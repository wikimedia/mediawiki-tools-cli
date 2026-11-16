import guides from './content/docs/guides/_meta.json';
import manual from './content/docs/manual/_meta.json';
import { createDocuments } from '../scripts/markdown-docs.mjs';

// Bundle sources into the prerenderer: import.meta.url there points into dist,
// rather than the source checkout. Vite also watches these imports in dev mode.
const sources = import.meta.glob<string>('./content/docs/**/*.md', {
  query: '?raw', import: 'default', eager: true,
});

export function markdownDocuments() {
  const files = Object.fromEntries(Object.entries(sources).map(([name, body]) => [
    name.replace('./content/docs/', ''), body,
  ]));
  return createDocuments(files, guides, manual);
}
