/**
 * @param {string} pathname
 */
export function normalizeRoutePath(pathname) {
  const path = pathname.replace(/\/+$/, '');
  return path ? `${path}/` : '/';
}

/**
 * @param {string} href
 */
export function pageHref(href) {
  return href.replace(/\/+(?=[?#]|$)/, '') || '/';
}
