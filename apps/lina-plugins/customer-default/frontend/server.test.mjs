// Verify proxy transport failures cannot terminate the shared customer entry point.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { gzipSync } from 'node:zlib';
import { once } from 'node:events';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import { createCustomerGateway } from './server.mjs';
test('gateway forwards auth, cookies, compressed assets and deep links', async () => {
 let active = true, template = "demo";
 const upstream = http.createServer(async (req, res) => {
  if (req.url.includes('/public/customer-center/configuration')) { res.setHeader('content-type','application/json'); res.end(JSON.stringify({data:{active,entry:{url:'/x-assets/'+template+'/v1/index.html',assetBase:'/x-assets/'+template+'/v1/',spa:true}}})); return; }
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
  template='alternate';assert.match(await(await fetch(base+'/account/orders')).text(),/\/x-assets\/alternate\/v1\//);
  active=false; assert.equal((await fetch(base+'/account/orders')).status,503);
 } finally { gateway.closeAllConnections();upstream.closeAllConnections();await Promise.all([new Promise(r=>gateway.close(r)),new Promise(r=>upstream.close(r))]); }
});
test('purchase redirects use the configured website and preserve local hostname', async () => {
 const previous=process.env.WEBSITE_PUBLIC_ORIGIN;
 const gateway=createCustomerGateway('http://127.0.0.1:9');
 gateway.listen(0,'127.0.0.1');await once(gateway,'listening');
 const base='http://127.0.0.1:'+gateway.address().port;
 try{
  process.env.WEBSITE_PUBLIC_ORIGIN='https://shop.example.test';
  assert.deepEqual(await (await fetch(base+'/__website/configuration')).json(),{origin:'https://shop.example.test',origins:['https://shop.example.test']});
  process.env.WEBSITE_PUBLIC_ORIGIN='http://localhost:16680';
  assert.deepEqual(await (await fetch(base+'/__website/configuration')).json(),{origin:'http://127.0.0.1:16680',origins:['http://127.0.0.1:16680','http://localhost:16680']});
 }finally{
  if(previous===undefined)delete process.env.WEBSITE_PUBLIC_ORIGIN;else process.env.WEBSITE_PUBLIC_ORIGIN=previous;
  gateway.closeAllConnections();await new Promise(resolve=>gateway.close(resolve));
 }
});

async function transportFixture(t, handler) {
 const upstream=http.createServer(handler);
 upstream.listen(0,'127.0.0.1');await once(upstream,'listening');
 const gateway=createCustomerGateway('http://127.0.0.1:'+upstream.address().port);
 gateway.listen(0,'127.0.0.1');await once(gateway,'listening');
 t.after(async()=>{
  gateway.closeAllConnections();upstream.closeAllConnections();
  await Promise.all([new Promise(resolve=>gateway.close(resolve)),new Promise(resolve=>upstream.close(resolve))]);
 });
 return 'http://127.0.0.1:'+gateway.address().port;
}

test('upstream stream failure closes only the affected response', {timeout:3000}, async t=>{
 let interrupt;
 const base=await transportFixture(t,(req,res)=>{
  if(req.url==='/x/broken'){res.writeHead(200);res.write('partial');interrupt=()=>res.destroy();}
  else res.end('healthy');
 });
 const response=await fetch(base+'/x/broken');
 assert.equal(response.status,200);interrupt();
 await assert.rejects(response.text());
 assert.equal(await (await fetch(base+'/x/healthy')).text(),'healthy');
});

test('upstream body timeout leaves the gateway available', {timeout:3000}, async t=>{
 const timeout=AbortSignal.timeout;
 t.mock.method(AbortSignal,'timeout',milliseconds=>timeout(Math.min(milliseconds,100)));
 const base=await transportFixture(t,(req,res)=>{res.writeHead(200);res.write('partial');});
 const response=await fetch(base+'/x/timeout');
 assert.equal(response.status,200);
 await assert.rejects(response.text());
 assert.equal((await fetch(base+'/__website/configuration')).status,200);
});

test('client disconnect releases the upstream body and allows the next request', {timeout:3000}, async t=>{
 let closed;
 const base=await transportFixture(t,(req,res)=>{
  if(req.url==='/x/stream'){
   closed=once(res,'close');res.writeHead(200);res.write('partial');
  }else res.end('healthy');
 });
 const controller=new AbortController();
 const response=await fetch(base+'/x/stream',{signal:controller.signal});
 controller.abort();await assert.rejects(response.text());await closed;
 assert.equal(await (await fetch(base+'/x/healthy')).text(),'healthy');
});

test('upstream connection failure returns 503 and the next request succeeds', {timeout:3000}, async t=>{
 const base=await transportFixture(t,(req,res)=>{
  if(req.url==='/x/unavailable')req.socket.destroy();else res.end('healthy');
 });
 const response=await fetch(base+'/x/unavailable');
 assert.equal(response.status,503);await response.text();
 assert.equal(await (await fetch(base+'/x/healthy')).text(),'healthy');
});


// Reproduce Node 24's delayed TimeoutError after a client closes before upstream headers.
test('gateway process survives disconnect before headers and delayed upstream body timeout',{timeout:10000},async()=>{
 let arrived,upstreamClosed;
 const seen=new Promise(resolve=>{arrived=resolve;});
 const upstream=http.createServer((req,res)=>{
  if(req.url.endsWith('/public/brand')){res.setHeader('content-type','application/json');return res.end(JSON.stringify({data:{brand:{templateEntry:{url:'/x-assets/fixture/v1/index.html',assetBase:'/x-assets/fixture/v1/',spa:true}}}}));}
  res.setHeader('content-type','application/octet-stream');
  if(req.url.endsWith('/early.bin')){
   upstreamClosed=once(res,'close');arrived();
   const timer=setTimeout(()=>{res.writeHead(200);res.write('partial');},40);timer.unref();
  }else if(req.url.endsWith('/timeout.bin')||req.url.endsWith('/cancel.bin')){
   upstreamClosed=once(res,'close');res.writeHead(200);res.write('partial');
  }else res.end('healthy');
 });
 const listen=async server=>{server.listen(0,'127.0.0.1');await once(server,'listening');return 'http://127.0.0.1:'+server.address().port;};
 const backend=await listen(upstream),probe=http.createServer(),origin=await listen(probe);await new Promise(resolve=>probe.close(resolve));
 const preload='const timeout=AbortSignal.timeout;AbortSignal.timeout=()=>timeout(250);';
 const child=spawn(process.execPath,['--import','data:text/javascript,'+encodeURIComponent(preload),fileURLToPath(new URL('./server.mjs',import.meta.url))],{env:{...process.env,CUSTOMER_PORT:new URL(origin).port,LINAPRO_BACKEND_PROXY_TARGET:backend},stdio:['ignore','ignore','pipe']});
 let stderr='';child.stderr.on('data',chunk=>{stderr+=chunk;});
 try{
  let ready=false;for(let n=0;n<80;n++){try{if((await fetch(origin+'/__website/configuration')).ok){ready=true;break;}}catch{}await new Promise(resolve=>setTimeout(resolve,25));}
  assert.ok(ready,'gateway did not start: '+stderr);
  const early=http.get(origin+'/x/early.bin');early.on('error',()=>{});await seen;early.destroy();
  await Promise.race([upstreamClosed,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('upstream request was not released before its timeout')),150);timer.unref();})]);
  // Await the original timeout even after downstream close: this is where the old orphaned Readable crashed.
  await new Promise(resolve=>setTimeout(resolve,400));
  assert.equal(child.exitCode,null,'disconnect before headers killed gateway: '+stderr);
  await upstreamClosed;
  assert.equal((await fetch(origin+'/__website/configuration')).status,200);
  const response=await fetch(origin+'/x/timeout.bin');assert.equal(response.status,200);await assert.rejects(response.text());
  assert.equal(child.exitCode,null,'mid-body timeout killed gateway: '+stderr);
  const controller=new AbortController();const cancel=await fetch(origin+'/x/cancel.bin',{signal:controller.signal});
  controller.abort();await assert.rejects(cancel.text());
  await Promise.race([upstreamClosed,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('upstream body leaked after disconnect')),1000);timer.unref();})]);
  await new Promise(resolve=>setTimeout(resolve,300));
  assert.equal(child.exitCode,null,'late abort after cancellation killed gateway: '+stderr);
  assert.equal((await fetch(origin+'/__website/configuration')).status,200);
  assert.doesNotMatch(stderr,/Unhandled 'error' event/);
 }finally{
  if(child.exitCode===null&&child.signalCode===null){const ended=once(child,'exit');child.kill();await ended;}
  upstream.closeAllConnections();await new Promise(resolve=>upstream.close(resolve));
 }
});
