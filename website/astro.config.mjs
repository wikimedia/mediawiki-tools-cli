import { readFileSync } from 'node:fs';
import { satteri } from '@astrojs/markdown-satteri';
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { projectLinks } from './src/project-links.ts';
import guideLinks from './scripts/guide-links.mjs';
import { watchManual } from './scripts/watch-manual.mjs';

const manual = JSON.parse(readFileSync(new URL('./src/content/docs/manual/_meta.json', import.meta.url), 'utf8'));

const guides = JSON.parse(readFileSync(new URL('./src/content/docs/guides/_meta.json', import.meta.url), 'utf8'));
const generalGuides = guides.pages.filter((page) => page.url !== '/guides/' && !page.url.startsWith('/guides/dev/'));
const guideLink = (page) => ({ label: page.title, link: page.url });

export default defineConfig({
  markdown: { processor: satteri({ mdastPlugins: [guideLinks(guides)] }) },
  site: 'https://a-wikimedia-cli.toolforge.org',
  trailingSlash: 'always',
  server: { host: '127.0.0.1', port: 4321 },
  devToolbar: { enabled: false },
  integrations: [watchManual(), starlight({
    title: 'a Wikimedia CLI',
    description: 'Command reference for a Wikimedia CLI.',
    logo: { src: './public/logo.svg' },
    favicon: '/logo.svg',
    social: [...projectLinks],
    pagination: false,
    tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
    customCss: ['./src/styles/custom.css'],
    routeMiddleware: './src/manual-navigation.ts',
    components: {
      PageTitle: './src/components/PageTitle.astro',
      Sidebar: './src/components/Sidebar.astro',
      TableOfContents: './src/components/TableOfContents.astro',
      MobileTableOfContents: './src/components/MobileTableOfContents.astro',
      SocialIcons: './src/components/SocialIcons.astro',
      MarkdownContent: './src/components/MarkdownContent.astro',
    },
    sidebar: [
      { label: 'Home', link: '/' },
      { label: 'Guides', items: [
        { label: 'All guides', link: '/guides/' },
        ...generalGuides.filter((page) => page.source.split('/').length === 2).map(guideLink),
        { label: 'Development guides', link: '/guides/dev/' },
        ...generalGuides.filter((page) => page.source.split('/').length > 2).map(guideLink),
      ] },
      { label: 'Overview', link: '/manual/' },
      ...manual.sidebar,
    ],
  })],
});
