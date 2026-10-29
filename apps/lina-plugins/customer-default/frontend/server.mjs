// Serve any installed customer template through the public, framework-neutral contract.
import http from 'node:http';
import { Readable, Writable } from 'node:stream';
import { pathToFileURL } from 'node:url';
const hopHeaders = ['connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailer', 'transfer-encoding', 'upgrade'];
function forwardedHeaders(source) {
  const headers = new Headers();
  for (const [name, value] of Object.entries(source)) {
    if (value !== undefined && !hopHeaders.includes(name.toLowerCase()) && !['host', 'content-length', 'accept-encoding'].includes(name.toLowerCase())) headers.set(name, Array.isArray(value) ? value.join(', ') : value);
  }
  headers.set('accept-encoding', 'identity');
  return headers;
}
function responseHeaders(upstream) {
  const headers = {};
  for (const [key, value] of upstream.headers) {
    if (!hopHeaders.includes(key) && !['content-encoding', 'content-length', 'set-cookie'].includes(key)) headers[key] = value;
  }
  const cookies = upstream.headers.getSetCookie();
  if (cookies.length) headers['set-cookie'] = cookies;
  return headers;
}
export function createCustomerGateway(backend) {
  return http.createServer(async (req, res) => {
    const controller = new AbortController();
    const closeRequest = () => { if (!res.writableFinished) controller.abort(); };
    const requestSignal = milliseconds => AbortSignal.any([controller.signal, AbortSignal.timeout(milliseconds)]);
    res.once('close', closeRequest);
    try {
      const url = new URL(req.url || '/', 'http://customer.local');
      if (url.pathname === '/__website/configuration' && req.method === 'GET') {
        const website = new URL(process.env.WEBSITE_PUBLIC_ORIGIN || 'http://localhost:6680');
        const hostname = String(req.headers.host || '').replace(/:\d+$/, '');
        if (['localhost','127.0.0.1'].includes(website.hostname) && ['localhost','127.0.0.1'].includes(hostname)) website.hostname = hostname;
        res.writeHead(200, {'content-type':'application/json; charset=utf-8','cache-control':'no-store'});
        const origins=[website.origin];
        if(['localhost','127.0.0.1'].includes(website.hostname)){const alias=new URL(website);alias.hostname=website.hostname==='localhost'?'127.0.0.1':'localhost';origins.push(alias.origin);}
        res.end(JSON.stringify({origin:website.origin,origins})); return;
      }
      if ((url.pathname === '/api.json' || ['/api/', '/x/', '/x-assets/', '/portal/'].some((prefix) => url.pathname.startsWith(prefix)))) {
        const init = { method: req.method, headers: forwardedHeaders(req.headers), redirect: 'manual', signal: requestSignal(30000) };
        if (!['GET', 'HEAD'].includes(req.method)) { init.body = Readable.toWeb(req); init.duplex = 'half'; }
        const upstream = await fetch(backend + url.pathname + url.search, init);
        res.writeHead(upstream.status, responseHeaders(upstream));
        // Avoid an orphaned fromWeb Readable if the client closed before upstream headers arrived.
        if (upstream.body && req.method !== 'HEAD') await upstream.body.pipeTo(Writable.toWeb(res)); else res.end();
        return;
      }
      if (!['GET', 'HEAD'].includes(req.method)) { res.writeHead(405, { allow: 'GET, HEAD' }); res.end(); return; }
      const response = await fetch(backend + '/x/idc-portal/api/v1/public/customer-center/configuration', { cache: 'no-store', signal: requestSignal(10000) });
      if (!response.ok) throw new Error('configuration unavailable');
      const body = await response.json(), view = body.data ?? body, entry = view.entry;
      if (!view.active || !entry?.url?.startsWith('/x-assets/')) throw new Error('template unavailable');
      if (!entry.spa && url.pathname !== '/') { res.writeHead(404); res.end(); return; }
      const htmlResponse = await fetch(backend + entry.url, { signal: requestSignal(10000) });
      if (!htmlResponse.ok) throw new Error('template unavailable');
      const html = (await htmlResponse.text()).replace(/<head([^>]*)>/i, '<head$1><base href="' + entry.assetBase + '">');
      res.writeHead(200, { 'content-type': 'text/html;charset=utf-8', 'cache-control': 'no-store' });
      res.end(req.method === 'HEAD' ? undefined : html);
    } catch {
      // A streamed response may fail after its headers have already reached the client.
      // Terminate that response without crashing the gateway or writing a second status.
      if (res.headersSent || res.destroyed) { if (!res.destroyed) res.destroy(); return; }
      res.writeHead(503, { 'content-type': 'text/html;charset=utf-8', 'cache-control': 'no-store' });
      res.end(req.method === 'HEAD' ? undefined : '<!doctype html><title>客户中心暂时不可用</title><p>客户中心暂时不可用，请稍后刷新重试。</p>');
    } finally {
      res.off('close', closeRequest);
    }
  });
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const port = Number(process.env.CUSTOMER_PORT || 6032);
  createCustomerGateway(process.env.LINAPRO_BACKEND_PROXY_TARGET || 'http://127.0.0.1:6030').listen(port, '127.0.0.1', () => process.stdout.write('Customer template gateway listening on ' + port + String.fromCharCode(10)));
}
