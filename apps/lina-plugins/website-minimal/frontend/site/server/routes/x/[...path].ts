// Proxy only the anonymous website APIs to the operator-configured backend.
export default defineEventHandler(async (event) => {
  const incoming = getRequestURL(event);
  const path = incoming.pathname;
  const accountCommerce = /^\/x\/idc-core\/api\/v1\/account\/(commerce|catalog)(\/|$)/u.test(path);
  const publicPortal = path === '/x/idc-portal/api/v1/public/brand';
  const publicCore = /^\/x\/idc-core\/api\/v1\/public\/(catalog|content|commerce)(\/|$)/u.test(path);
  const publicAnnouncements = /^\/x\/content-notice\/api\/v1\/public\/announcements(?:\/\d+)?\/?$/u.test(path);
  if ((!publicPortal && !publicCore && !accountCommerce && !publicAnnouncements) || path.includes('%') || path.includes('\\')) {
    throw createError({ statusCode: 404, statusMessage: 'Not found' });
  }
  const method = event.method.toUpperCase();
  const publicAction = /^\/x\/idc-core\/api\/v1\/public\/(commerce\/(carts|cart-items)|catalog\/products\/[^/]+\/(purchase-configuration\/(evaluate|quote)|campaign-exposures)|content\/items\/[^/]+\/download-tickets)$/u.test(path);
  if (!accountCommerce && !['GET', 'HEAD'].includes(method) && !(method === 'POST' && publicAction) && !(method === 'DELETE' && /^\/x\/idc-core\/api\/v1\/public\/commerce\/cart-items\/[^/]+$/u.test(path))) {
    throw createError({ statusCode: 405, statusMessage: 'Method not allowed' });
  }
  const config = useRuntimeConfig(event);
  const origin = new URL(String(config.apiOrigin));
  if (!['http:', 'https:'].includes(origin.protocol) || origin.username || origin.password) {
    throw createError({ statusCode: 503, statusMessage: 'Website API unavailable' });
  }
  const target = new URL(path + incoming.search, origin.origin);
  return proxyRequest(event, target.toString(), {
    headers: { authorization: accountCommerce ? getHeader(event, 'authorization') || '' : '', 'x-forwarded-host': '' },
  });
});
