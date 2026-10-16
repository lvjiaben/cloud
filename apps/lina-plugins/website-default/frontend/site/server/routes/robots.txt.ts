export default defineEventHandler((event) => {
  const config = useRuntimeConfig(event);
  const siteURL = String(config.public.siteUrl).replace(/\/$/u, '');
  setHeader(event, 'content-type', 'text/plain; charset=utf-8');
  return `User-agent: *\nAllow: /\nSitemap: ${siteURL}/sitemap.xml\n`;
});
