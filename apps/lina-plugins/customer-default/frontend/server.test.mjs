import { test } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { gzipSync } from 'node:zlib';
import { once } from 'node:events';
import { createCustomerGateway } from './server.mjs';
test('gateway forwards auth, cookies, compressed assets and deep links', async () => {
 let active = true;
 const upstream = http.createServer(async (req, res) => {
  if (req.url.includes('/public/customer-center/configuration')) { res.setHeader('content-type','application/json'); res.end(JSON.stringify({data:{active,entry:{url:'/x-assets/demo/v1/index.html',assetBase:'/x-assets/demo/v1/',spa:true}}})); return; }
  if (req.url === '/api.json') { res.setHeader('content-type','application/json');res.end('{"openapi":"3.0.0","paths":{}}');return; }
  if (req.url.endsWith('index.html')) { res.end('<html><head></head><body>customer</body></html>'); return; }
  if (req.url.endsWith('app.js')) { res.setHeader('content-encoding','gzip'); res.end(gzipSync('window.loaded=true')); return; }
  if (req.url === '/api/login') { let body=''; for await (const chunk of req) body+=chunk; assert.equal(body,'{"username":"test"}'); res.setHeader('set-cookie',['session=abc; HttpOnly','refresh=def; HttpOnly']); res.end('{"accessToken":"test-token"}'); return; }
  if (req.url === '/x/account') { res.end(JSON.stringify({authorization:req.headers.authorization,cookie:req.headers.cookie})); return; }
  res.writeHead(404); res.end();
 });
 upstream.listen(0,'127.0.0.1'); await once(upstream,'listening');
 const gateway=createCustomerGateway('http://127.0.0.1:'+upstream.address().port); gateway.listen(0,'127.0.0.1'); await once(gateway,'listening');
 const base='http://127.0.0.1:'+gateway.address().port;
 try {
  const page=await fetch(base+'/account/orders'); assert.equal(page.status,200); assert.ok((await page.text()).includes('/x-assets/demo/v1/'));
  assert.equal((await (await fetch(base+'/api.json')).json()).openapi,'3.0.0');
  const head=await fetch(base+'/account/orders',{method:'HEAD'}); assert.equal(await head.text(),'');
  const asset=await fetch(base+'/x-assets/demo/v1/app.js'); assert.equal(asset.headers.get('content-encoding'),null); assert.equal(await asset.text(),'window.loaded=true');
  const login=await fetch(base+'/api/login',{method:'POST',headers:{'content-type':'application/json'},body:'{"username":"test"}'}); assert.equal(login.headers.getSetCookie().length,2); assert.equal((await login.json()).accessToken,'test-token');
  const account=await fetch(base+'/x/account',{headers:{authorization:'Bearer test-token',cookie:'session=abc'}}); assert.deepEqual(await account.json(),{authorization:'Bearer test-token',cookie:'session=abc'});
  assert.equal((await fetch(base+'/account/orders',{method:'POST'})).status,405);
  active=false; assert.equal((await fetch(base+'/account/orders')).status,503);
 } finally { gateway.closeAllConnections();upstream.closeAllConnections();await Promise.all([new Promise(r=>gateway.close(r)),new Promise(r=>upstream.close(r))]); }
});
