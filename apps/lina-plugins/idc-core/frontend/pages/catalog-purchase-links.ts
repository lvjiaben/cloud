// Purchase links use the active customer deployment's public website settings.
export interface CatalogWebsiteSettings { websiteurl?:string; websiteconfigurepath?:string; websitecatalogpath?:string }
export function catalogPurchaseLinks(settings:CatalogWebsiteSettings, slug:string, categoryNo:string) {
  const origin = new URL(settings.websiteurl || '');
  if (!['http:', 'https:'].includes(origin.protocol) || origin.username || origin.password || origin.pathname !== '/' || origin.search || origin.hash) throw new Error('website_configuration_invalid');
  function target(path:string | undefined) {
    if (!path?.startsWith('/') || path.startsWith('//') || path.includes('\\')) throw new Error('website_path_invalid');
    const url = new URL(path, origin);
    if (url.origin !== origin.origin) throw new Error('website_path_invalid');
    return url;
  }
  if (!slug || !settings.websiteconfigurepath?.includes('{product}') || !categoryNo) throw new Error('website_path_invalid');
  const product = target(settings.websiteconfigurepath.replaceAll('{product}', encodeURIComponent(slug)));
  const group = target(settings.websitecatalogpath);
  group.searchParams.set('category', categoryNo);
  return {product:product.href, group:group.href};
}
