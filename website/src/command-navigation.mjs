/**
 * @param {Array<{name: string, title: string, link: string, parent?: string}>} commands
 * @param {string} pathname
 */
export function commandSections(commands, pathname) {
  if (pathname === '/manual/dev/') return [];
  const byLink = new Map(commands.map((command) => [command.link, command]));
  const branch = [];
  let current = byLink.get(pathname);
  while (current) {
    branch.unshift(current);
    current = byLink.get(current.parent);
  }
  // Development service areas already appear in their dedicated left sidebar.
  const skip = branch[1]?.name === 'dev' && branch.length > 2 ? 2 : pathname === '/manual/' ? 0 : 1;
  return branch.slice(skip).map((command) => ({
    ...command,
    label: command.title.replace(/^\S+\s*/, ''),
    children: commands.filter((child) => child.parent === command.link),
  })).filter((section) => section.children.length > 0);
}
