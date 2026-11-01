import { commandSections } from './command-navigation.mjs';
import { defineRouteMiddleware, type StarlightRouteData } from '@astrojs/starlight/route-data';
import guides from './content/docs/guides/_meta.json';
import manual from './content/docs/manual/_meta.json';

type SidebarEntry = StarlightRouteData['sidebar'][number];
interface ManualItem {
  label: string;
  link?: string;
  collapsed?: boolean;
  items?: ManualItem[];
}

export const onRequest = defineRouteMiddleware((context) => {
  const pathname = context.url.pathname;
  const route = context.locals.starlightRoute;
  if (pathname.startsWith('/manual/')) {
    route.toc = commandSections(manual.commands, pathname).length
      ? { minHeadingLevel: 2, maxHeadingLevel: 3, items: [] }
      : undefined;
  } else if (route.toc && !route.headings.some((heading) => heading.depth >= route.toc!.minHeadingLevel && heading.depth <= route.toc!.maxHeadingLevel)) {
    route.toc = undefined;
  }
  function highlight(item: SidebarEntry): SidebarEntry {
    return item.type === 'link'
      ? { ...item, attrs: { ...item.attrs, ...(item.href.startsWith('/manual/') && item.href !== '/manual/' && pathname.startsWith(item.href) ? { 'data-active-branch': true } : {}) } }
      : { ...item, entries: item.entries.map(highlight) };
  }
  route.sidebar = route.sidebar.map(highlight);
  if (!pathname.startsWith('/manual/dev/') && !pathname.startsWith('/guides/dev/')) return;

  function entry(item: ManualItem): SidebarEntry {
    if (item.link) {
      return {
        type: 'link', label: item.label, href: item.link,
        isCurrent: pathname === item.link, attrs: item.link.startsWith('/manual/') && item.link !== '/manual/' && pathname.startsWith(item.link) ? { 'data-active-branch': true } : {}, badge: undefined,
      };
    }
    return {
      type: 'group', label: item.label, collapsed: item.collapsed ?? false,
      entries: (item.items ?? []).map(entry), badge: undefined,
    };
  }

  const categories: ManualItem[] = manual.developmentSidebar;
  const mediawiki = categories.flatMap((group) => group.items ?? []).find((item) => item.label === 'mediawiki');
  const supporting = categories.map((group) => ({
    ...group, items: group.items?.filter((item) => item !== mediawiki),
  })).filter((group) => group.items?.length);

  const developmentGuides = entry({ label: 'Guides', items: [
    { label: 'All development guides', link: '/guides/dev/' },
    ...guides.pages.filter((page) => page.url.startsWith('/guides/dev/')).map((page) => ({ label: page.title, link: page.url })),
  ] });
  route.sidebar = [
    entry({ label: '← Overview', link: '/manual/' }),
    entry({ label: 'Development overview', link: '/manual/dev/' }),
    ...(pathname.startsWith('/guides/dev/')
      ? [developmentGuides]
      : [
          entry({ label: 'Guides', link: '/guides/dev/' }),
          ...(mediawiki ? [entry({ ...mediawiki, label: 'MediaWiki' })] : []),
          ...supporting.map(entry),
        ]),
  ];
});
