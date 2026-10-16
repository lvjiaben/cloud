// Proxy explicitly published plugin assets through the website origin.
export default defineEventHandler(async event => {
 if (!['GET', 'HEAD'].includes(event.method)) throw createError({ statusCode: 405 });
 const incoming = getRequestURL(event);
 if (incoming.pathname.includes('..') || incoming.pathname.includes('%') || incoming.pathname.includes('\\')) throw createError({ statusCode: 404 });
 const config = useRuntimeConfig(event);
 const origin = new URL(String(config.apiOrigin));
 return proxyRequest(event, new URL(incoming.pathname + incoming.search, origin).href, { headers: { authorization: '', 'x-forwarded-host': '' } });
});
