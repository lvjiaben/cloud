// Serve only the active template's host-published HTML; arbitrary origins are never proxied.
export default defineEventHandler(async event => {
 if (!['GET', 'HEAD'].includes(event.method)) return;
 const pathname = getRequestURL(event).pathname;
 if (['/x/', '/x-assets/', '/_nuxt/', '/__', '/api/'].some(prefix => pathname.startsWith(prefix)) || (pathname.includes('.') && !pathname.endsWith('.html'))) return;
 const config = useRuntimeConfig(event);
 const origin = new URL(String(config.apiOrigin));
 const response = await $fetch<{ data?: { brand?: { templateEntry?: { url: string; assetBase: string; spa: boolean } } } }>(new URL('/x/idc-portal/api/v1/public/brand', origin).href).catch(() => undefined);
 const entry = response?.data?.brand?.templateEntry;
 if (!entry) return;
 // The backend projects this exact URL from an installed, enabled provider's frozen declaration.
 if (!entry.url.startsWith('/x-assets/') || !entry.assetBase.startsWith('/x-assets/') || entry.url.includes('..') || entry.assetBase.includes('..')) throw createError({ statusCode: 503, statusMessage: 'Website template is unavailable' });
 const assetPath = pathname === '/' || entry.spa ? entry.url : entry.assetBase + pathname.replace(/^\//u, '').replace(/\.html$/u, '') + '.html';
 let html = await $fetch<string>(new URL(assetPath, origin).href, { responseType: 'text' });
 const runtime = '<script>window.__LINA_TEMPLATE__=' + JSON.stringify({ customerUrl: config.public.customerUrl }).replaceAll('<', '\u003c') + '</script>';
 const base = /<base[\s>]/iu.test(html) ? runtime : runtime + '<base href="' + entry.assetBase.replaceAll('"', '&quot;') + '">';
 html = /<head[\s>]/iu.test(html) ? html.replace(/(<head[^>]*>)/iu, '$1' + base) : base + html;
 setHeader(event, 'Content-Type', 'text/html; charset=utf-8');
 setHeader(event, 'Cache-Control', 'no-store');
 return event.method === 'HEAD' ? '' : html;
});
