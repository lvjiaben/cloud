import type { PublicContentKind } from '../../app/components/public/content-client';

interface SitemapItem {
  contentNo: string;
  publicPath: string;
  updatedAt: number;
}

interface SitemapEnvelope {
  code?: number | string;
  data?: { items?: SitemapItem[] };
}

const kinds: PublicContentKind[] = [
  'news',
  'announcements',
  'knowledge',
  'downloads',
];

const kindToType: Record<PublicContentKind, string> = {
  announcements: 'announcement',
  downloads: 'download',
  knowledge: 'knowledge',
  news: 'news',
};

function xmlEscape(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;');
}

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event);
  const siteURL = String(config.public.siteUrl).replace(/\/$/u, '');
  const apiOrigin = String(config.apiOrigin).replace(/\/$/u, '');
  const staticPaths = [
    '/',
    '/products',
    '/pages/news',
    '/pages/announcements',
    '/pages/knowledge',
    '/pages/downloads',
    '/pages/help',
  ];

  const dynamic = await Promise.all(
    kinds.map(async (kind) => {
      try {
        const response = await $fetch<SitemapEnvelope>(
          `${apiOrigin}/x/idc-core/api/v1/public/content/items?type=${kindToType[kind]}&limit=100`,
        );
        return (response.data?.items || []).map((item) => ({
          lastmod: new Date(item.updatedAt).toISOString(),
          path: item.publicPath,
        }));
      } catch {
        return [];
      }
    }),
  );

  const entries = [
    ...staticPaths.map((path) => ({ lastmod: '', path })),
    ...dynamic.flat(),
  ];
  const urls = entries
    .map(({ lastmod, path }) => {
      const zh = `${siteURL}${path === '/' ? '' : path}`;
      const en = `${siteURL}/en-US${path === '/' ? '' : path}`;
      return [
        '<url>',
        `<loc>${xmlEscape(zh)}</loc>`,
        lastmod ? `<lastmod>${lastmod}</lastmod>` : '',
        `<xhtml:link rel="alternate" hreflang="zh-CN" href="${xmlEscape(zh)}" />`,
        `<xhtml:link rel="alternate" hreflang="en-US" href="${xmlEscape(en)}" />`,
        `<xhtml:link rel="alternate" hreflang="x-default" href="${xmlEscape(zh)}" />`,
        '</url>',
      ].join('');
    })
    .join('');

  setHeader(event, 'content-type', 'application/xml; charset=utf-8');
  return `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">${urls}</urlset>`;
});
