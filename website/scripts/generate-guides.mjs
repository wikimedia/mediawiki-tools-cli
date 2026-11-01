import { existsSync, mkdirSync, readFileSync, readdirSync, renameSync, unlinkSync, writeFileSync } from 'node:fs';
import { dirname, join, posix } from 'node:path';

const marker = '<!-- Generated from docs/; edit the original guide. -->';
const categories = { dev: 'Development', gitlab: 'GitLab', wiki: 'Wiki', tools: 'Toolforge', analytics: 'Analytics' };

export function guideURL(source) {
  const topic = source === 'index.md' ? '' : source.replace(/^guides\//, '').replace(/(?:^|\/)index\.md$/, '').replace(/\.md$/, '');
  return `/guides/${topic}${topic ? '/' : ''}`;
}

export function generateGuides(repository, website) {
  const manifest = JSON.parse(readFileSync(join(repository, 'docs/wiki.json'), 'utf8'));
  const version = JSON.parse(readFileSync(join(website, 'src/content/docs/manual/_meta.json'), 'utf8')).version;
  const output = join(website, 'src/content/docs/guides');
  const planned = new Map();
  const pages = manifest.guides.map((guide) => {
    const content = readFileSync(join(repository, 'docs', guide.source), 'utf8');
    const heading = /^# (.+)$/m.exec(content);
    if (!heading) throw new Error(`Guide has no title: ${guide.source}`);
    const url = guideURL(guide.source);
    const topic = url.slice('/guides/'.length).replace(/\/$/, '');
    const command = `mw help guides${topic ? ` ${topic.replaceAll('/', ' ')}` : ''}`;
    const wikiURL = `${manifest.page_url}/${guide.title.replaceAll(' ', '_')}`;
    const imported = /<!-- Imported from (\S+) \(CC BY-SA 4\.0\)/.exec(content);
    const attribution = imported ? `\n\nAdapted from [MediaWiki.org](${imported[1]}) under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/); see the linked page history for contributors.` : '';
    const frontmatter = {
      title: heading[1], guideSource: guide.source, guideCommand: command,
      manualVersion: version.label, manualUnreleased: version.unreleased,
      editUrl: `https://gitlab.wikimedia.org/repos/releng/cli/-/blob/main/docs/${guide.source}`,
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
    };
    planned.set(`${topic ? `${topic}/` : ''}index.md`, document(frontmatter,
      `${content.replace(heading[0], '').trim()}\n\n---\n\nRead this guide offline with \`${command}\`.${attribution}\n`));
    return { source: guide.source, title: heading[1], url, command, wikiURL };
  });

  // Folder commands (for example `help guides dev`) need readable landing pages too.
  const folders = new Set();
  for (const page of pages) {
    let folder = posix.dirname(page.url.slice('/guides/'.length).replace(/\/$/, ''));
    while (folder !== '.') { folders.add(folder); folder = posix.dirname(folder); }
  }
  for (const folder of folders) {
    if (planned.has(`${folder}/index.md`)) continue;
    const title = `${categories[folder] ?? folder} guides`;
    const entries = pages.filter((page) => page.url.startsWith(`/guides/${folder}/`));
    const content = entries.map((page) => `- [${page.title}](${page.url})`).join('\n');
    planned.set(`${folder}/index.md`, document({ title, tableOfContents: false }, content + '\n'));
  }

  function write(filename, data) {
    if (existsSync(filename) && readFileSync(filename, 'utf8') === data) return;
    mkdirSync(dirname(filename), { recursive: true });
    const temporary = `${filename}.${process.pid}.tmp`;
    try { writeFileSync(temporary, data); renameSync(temporary, filename); }
    finally { if (existsSync(temporary)) unlinkSync(temporary); }
  }
  for (const [name, content] of planned) {
    const filename = join(output, name);
    if (existsSync(filename) && !readFileSync(filename, 'utf8').includes(marker)) throw new Error(`Refusing to overwrite hand-written guide: ${filename}`);
    write(filename, content);
  }
  for (const name of readdirSync(output, { recursive: true })) {
    if (name.endsWith('.md') && !planned.has(name.replaceAll('\\', '/')) && readFileSync(join(output, name), 'utf8').includes(marker)) unlinkSync(join(output, name));
  }
  write(join(output, '_meta.json'), JSON.stringify({ pages }, null, 2) + '\n');
  console.log(`Generated ${planned.size} guide pages from docs/`);
}

function document(frontmatter, content) {
  // JSON values are valid YAML scalars, including multi-line titles and URLs.
  return `---\n${Object.entries(frontmatter).map(([key, value]) => `${key}: ${JSON.stringify(value)}`).join('\n')}\n---\n\n${marker}\n\n${content}`;
}
