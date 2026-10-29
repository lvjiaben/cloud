// Verify page ownership and credential forwarding without a real user or provider.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServer,get as clientGet} from 'node:http';
import {once} from 'node:events';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {gzipSync} from 'node:zlib';
const listen=async server=>{server.listen(0,'127.0.0.1');await once(server,'listening');return 'http://127.0.0.1:'+server.address().port;};
test('website owns configuration and checkout; only authentication and invoices use customer pages',{timeout:10000},async()=>{
 let interrupt,upstreamClosed,template="website-default";
 const backend=createServer(async(req,res)=>{
  if(req.url==='/x/idc-core/api/v1/account/stream-failure'){
   res.writeHead(200);res.write('partial');interrupt=()=>res.destroy();return;
  }
  if(req.url==='/x/idc-core/api/v1/account/stream-cancel'){
   upstreamClosed=once(res,'close');res.writeHead(200);res.write('partial');return;
  }
  if(req.url==='/x/idc-core/api/v1/account/connection-failure'){req.socket.destroy();return;}
  if(req.url==='/x-assets/website-alternate/v1/index.html'){res.setHeader('content-type','text/html; charset=utf-8');res.end('<html><head></head><body>alternate website</body></html>');return;}
  res.setHeader('content-type','application/json');
  if(req.url.endsWith('/public/brand')){res.end(JSON.stringify({data:{brand:{templateEntry:{url:'/x-assets/'+template+'/v1/index.html',assetBase:'/x-assets/'+template+'/v1/',spa:true}}}}));return;}
  if(req.headers.authorization==='Bearer expired-plain'){res.writeHead(401,{'content-type':'text/plain'});res.end('Unauthorized');return;}
  if(req.headers.authorization==='Bearer expired-json'){res.end(JSON.stringify({code:401}));return;}
  if(req.headers.authorization==='Bearer forbidden'){res.writeHead(403,{'content-type':'text/plain'});res.end('Forbidden');return;}
  if(req.headers.authorization==='Bearer unavailable'){res.writeHead(503,{'content-type':'text/plain'});res.end('Unavailable');return;}
  let body='';for await(const chunk of req)body+=chunk;
  res.end(JSON.stringify({code:0,method:req.method,authenticated:Boolean(req.headers.authorization),path:req.url,body,signature:req.headers['wechatpay-signature']}));
 });
 const customer=createServer((req,res)=>res.end('customer '+req.url));
 const renderer=createServer((req,res)=>res.end('website '+req.url));
 const backendOrigin=await listen(backend),customerOrigin=await listen(customer),rendererOrigin=await listen(renderer);
 const probe=createServer();const origin=await listen(probe);const port=new URL(origin).port;await new Promise(resolve=>probe.close(resolve));
 const child=spawn(process.execPath,[fileURLToPath(new URL('./server.mjs',import.meta.url))],{env:{...process.env,HOST:'127.0.0.1',PORT:port,WEBSITE_API_ORIGIN:backendOrigin,WEBSITE_CUSTOMER_URL:customerOrigin,WEBSITE_SSR_ORIGINS:JSON.stringify({'website-default':rendererOrigin})},stdio:'ignore'});
 try{
  let ready=false;for(let n=0;n<80;n++){try{if((await fetch(origin+'/cart')).ok){ready=true;break;}}catch{}await new Promise(resolve=>setTimeout(resolve,25));}
  assert.ok(ready,'gateway did not start');
  // Previously an interrupted response emitted an uncaught stream error and killed this process.
  const interrupted=await fetch(origin+'/x/idc-core/api/v1/account/stream-failure',{signal:AbortSignal.timeout(1500)});
  assert.equal(interrupted.status,200);interrupt();await assert.rejects(interrupted.text());
  assert.equal(await(await fetch(origin+'/cart')).text(),'website /cart');
  assert.equal(child.exitCode,null);
  const controller=new AbortController();
  const cancelled=await fetch(origin+'/x/idc-core/api/v1/account/stream-cancel',{signal:controller.signal});
  controller.abort();await assert.rejects(cancelled.text());
  await Promise.race([upstreamClosed,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('upstream body leaked')),1000);timer.unref();})]);
  assert.equal((await fetch(origin+'/x/idc-core/api/v1/account/connection-failure')).status,502);
  assert.equal(await(await fetch(origin+'/checkout')).text(),'website /checkout');
  const configuration=await(await fetch(origin+'/__website/configuration',{headers:{host:'localhost:'+port}})).json();
  assert.equal(configuration.customerOrigin,customerOrigin,'website aliases must retain the configured customer origin');
  for(const path of ['/products/11','/cart','/checkout'])assert.equal(await (await fetch(origin+path)).text(),'website '+path);
  for(const path of ['/auth/login','/account/website-purchase','/account/billing/invoices/INV_test?pay=1'])assert.equal(await (await fetch(origin+path)).text(),'customer '+path);
  const privatePath='/x/idc-core/api/v1/account/commerce/cart';
  assert.equal((await (await fetch(origin+privatePath,{headers:{authorization:'Bearer fixture'}})).json()).authenticated,true);
  const publicPath='/x/idc-core/api/v1/public/catalog/products/11';
  assert.equal((await (await fetch(origin+publicPath,{headers:{authorization:'Bearer fixture'}})).json()).authenticated,false);
  const removed=await fetch(origin+'/x/idc-core/api/v1/public/commerce/cart-items/item-test?expectedCartVersion=1&expectedItemVersion=1',{method:'DELETE'});
  assert.equal((await removed.json()).method,'DELETE');
  assert.equal((await fetch(origin+'/x/idc-core/api/v1/admin/catalog/products')).status,404);
  for(const kind of ['payments','refunds']){
   const callbackPath='/x/idc-payment/api/v1/callbacks/'+kind+'/channel_test';
   const raw='{"amount":100,"literal":"a+b%20c"}';
   const response=await fetch(origin+callbackPath,{method:'POST',headers:{authorization:'Bearer must-not-forward','content-type':'application/json','wechatpay-signature':'signed-fixture'},body:raw});
   const result=await response.json();
   assert.equal(result.path,callbackPath);assert.equal(result.body,raw);assert.equal(result.signature,'signed-fixture');assert.equal(result.authenticated,false);
   if(kind==='payments'){
    const query='?name=a%2Bb+space&sign=signed-fixture&out_trade_no=PAT_test';
    const notification=await(await fetch(origin+callbackPath+query,{headers:{authorization:'Bearer must-not-forward'}})).json();
    assert.equal(notification.method,'GET');assert.equal(notification.path,callbackPath+query);assert.equal(notification.authenticated,false);
   }else assert.equal((await fetch(origin+callbackPath)).status,405);
   assert.equal((await fetch(origin+callbackPath,{method:'PUT'})).status,405);
  }
  assert.equal((await fetch(origin+'/x/idc-payment/api/v1/admin/channels')).status,404);
  const denied=await fetch(origin+'/__website/session',{method:'POST',headers:{origin:'https://other.invalid','content-type':'application/x-www-form-urlencoded'},body:new URLSearchParams({accessToken:'fixture',returnTo:'/checkout'}),redirect:'manual'});
  assert.equal(denied.status,403);
  const handoff=await fetch(origin+'/__website/session',{method:'POST',headers:{origin:customerOrigin,'content-type':'application/x-www-form-urlencoded'},body:new URLSearchParams({accessToken:'fixture',returnTo:'/checkout'}),redirect:'manual'});
  assert.equal(handoff.status,303);assert.equal(handoff.headers.get('location'),'/checkout');
  const cookie=handoff.headers.get('set-cookie');assert.match(cookie,/HttpOnly/);assert.match(cookie,/Max-Age=86400/);
  const session=await fetch(origin+'/__website/session',{headers:{cookie:cookie.split(';')[0],'sec-fetch-site':'same-origin'}});
  assert.deepEqual(await session.json(),{checked:true,accessToken:'fixture'});assert.equal(session.headers.get('set-cookie'),null);const repeated=await fetch(origin+'/__website/session',{headers:{cookie:cookie.split(';')[0],'sec-fetch-site':'same-origin'}});assert.deepEqual(await repeated.json(),{checked:true,accessToken:'fixture'});
  const savedSession=await fetch(origin+'/__website/session',{method:'PUT',headers:{origin,authorization:'Bearer fixture','sec-fetch-site':'same-origin'}});assert.equal(savedSession.status,200);assert.match(savedSession.headers.get('set-cookie'),/HttpOnly/);assert.match(savedSession.headers.get('set-cookie'),/Max-Age=86400/);
  assert.equal((await fetch(origin+'/__website/session',{method:'PUT',headers:{origin:'https://foreign.invalid',authorization:'Bearer fixture'}})).status,403);
  assert.equal((await fetch(origin+'/__website/session',{method:'PUT',headers:{origin,authorization:'Bearer expired-plain'}})).status,401);
  const crossRead=await fetch(origin+'/__website/session',{headers:{cookie:cookie.split(';')[0],'sec-fetch-site':'cross-site'}});assert.equal(crossRead.status,403);
  for(const token of ['expired-plain','expired-json']){
   const expired=await fetch(origin+'/__website/session',{method:'POST',headers:{origin:customerOrigin},body:new URLSearchParams({accessToken:token,returnTo:'/checkout'}),redirect:'manual'});
   assert.equal(expired.status,303);assert.equal(expired.headers.get('location'),'/checkout');assert.equal(expired.headers.get('cache-control'),'no-store');
   const guest=await fetch(origin+'/__website/session',{headers:{cookie:expired.headers.get('set-cookie').split(';')[0]}});
   assert.deepEqual(await guest.json(),{checked:true,accessToken:''});
   assert.equal(await(await fetch(origin+'/checkout')).text(),'website /checkout');
  }
  for(const [token,status] of [['forbidden',403],['unavailable',503]]){
   const blocked=await fetch(origin+'/__website/session',{method:'POST',headers:{origin:customerOrigin},body:new URLSearchParams({accessToken:token,returnTo:'/checkout'}),redirect:'manual'});
   assert.equal(blocked.status,status);assert.equal(blocked.headers.get('set-cookie'),null);assert.doesNotMatch(await blocked.text(),/模板/);
  }
  // The independently packaged HTML website uses the same trusted handoff.
  for(const returnTo of ['/cart.html?lang=en-US','/configure.html?product=real-slug&currency=CNY','/products.html','/services.html?category=public-category']){
   const htmlReturn=await fetch(origin+'/__website/session',{method:'POST',headers:{origin:customerOrigin},body:new URLSearchParams({accessToken:'fixture',returnTo}),redirect:'manual'});
   assert.equal(htmlReturn.status,303);assert.equal(htmlReturn.headers.get('location'),returnTo);
  }
  for(const returnTo of ['/admin.html','//other.invalid/cart.html','/cart.html/../../admin']){
   const blockedReturn=await fetch(origin+'/__website/session',{method:'POST',headers:{origin:customerOrigin},body:new URLSearchParams({accessToken:'fixture',returnTo}),redirect:'manual'});
   assert.equal(blockedReturn.status,400);
  }
  template='website-alternate';assert.match(await(await fetch(origin+'/cart')).text(),/alternate website/);
  template='website-default';assert.equal(await(await fetch(origin+'/cart')).text(),'website /cart');
  const external=await fetch(origin+'/__website/session',{method:'POST',headers:{origin:customerOrigin},body:new URLSearchParams({accessToken:'fixture',returnTo:'https://other.invalid'}),redirect:'manual'});assert.equal(external.status,400);

 }finally{
  if(child.exitCode===null&&child.signalCode===null){const exited=once(child,'exit');child.kill();await exited;}
  for(const server of [backend,customer,renderer]){server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
 }
});

// Both independently delivered gateways expose only the announcement owner's public GET resources.
for(const template of ['website-default','website-minimal']){
 test(template+' proxies public announcement reads without widening private routes',{timeout:10000},async()=>{
  const calls=[];
  const backend=createServer((req,res)=>{
   calls.push({method:req.method,path:req.url,authorization:req.headers.authorization,forwardedHost:req.headers['x-forwarded-host']});
   res.setHeader('content-type','application/json');res.end(JSON.stringify({code:0,path:req.url,method:req.method}));
  });
  const backendOrigin=await listen(backend);
  const probe=createServer();const origin=await listen(probe);await new Promise(resolve=>probe.close(resolve));
  const script=template==='website-default'?new URL('./server.mjs',import.meta.url):new URL('../../website-minimal/gateway/server.mjs',import.meta.url);
  const child=spawn(process.execPath,[fileURLToPath(script)],{env:{...process.env,HOST:'127.0.0.1',PORT:new URL(origin).port,WEBSITE_API_ORIGIN:backendOrigin,WEBSITE_CUSTOMER_URL:backendOrigin,WEBSITE_SSR_ORIGINS:'{}'},stdio:'ignore'});
  try{
   const base='/x/content-notice/api/v1/public/announcements';
   let ready=false;for(let n=0;n<80;n++){try{if((await fetch(origin+base)).ok){ready=true;break;}}catch{}await new Promise(resolve=>setTimeout(resolve,25));}
   assert.ok(ready,'announcement gateway did not start');calls.length=0;
   for(const path of [base+'?pageNum=2&pageSize=10',base+'/123']){
    const response=await fetch(origin+path,{headers:{authorization:'Bearer must-not-forward','x-forwarded-host':'untrusted.invalid'}});
    assert.equal(response.status,200);assert.deepEqual(await response.json(),{code:0,path,method:'GET'});
   }
   assert.deepEqual(calls,[
    {method:'GET',path:base+'?pageNum=2&pageSize=10',authorization:undefined,forwardedHost:undefined},
    {method:'GET',path:base+'/123',authorization:undefined,forwardedHost:undefined},
   ]);
   for(const method of ['POST','PUT','PATCH','DELETE','HEAD','OPTIONS']){
    const response=await fetch(origin+base,{method});assert.equal(response.status,405);assert.equal(response.headers.get('allow'),'GET');
   }
   for(const path of [
    '/x/content-notice/api/v1/notice',
    '/x/content-notice/api/v1/notice/announcements',
    base+'/123/read',base+'/123/publish',base+'/other',base+'/0',
    '/x/other/api/v1/public/announcements',
   ])assert.equal((await fetch(origin+path)).status,404,path);
   assert.equal((await fetch(origin+base+'/123/read',{method:'PUT'})).status,404);
   assert.equal(calls.length,2,'a rejected route or method reached the backend');
  }finally{
   if(child.exitCode===null&&child.signalCode===null){const exited=once(child,'exit');child.kill();await exited;}
   backend.closeAllConnections();await new Promise(resolve=>backend.close(resolve));
  }
 });
}

test('selected HTML website preserves root image/script bytes while HTML injection and SSR remain distinct',{timeout:10000},async()=>{
 const calls=[];
 const image=Buffer.from([137,80,78,71,13,10,26,10,0,255,128,42]);
 const script=Buffer.from('export const label="主官网资源";\nexport const body="<head>must stay script bytes</head>";');
 const backend=createServer((req,res)=>{
  calls.push({method:req.method,path:req.url});
  if(req.url==='/x/idc-portal/api/v1/public/brand'){
   res.setHeader('content-type','application/json');return res.end(JSON.stringify({data:{brand:{templateEntry:{url:'/x-assets/website-main/current/index.html',assetBase:'/x-assets/website-main/current/',spa:false}}}}));
  }
  if(req.url==='/x-assets/website-main/current/static/compressed.png'){const bytes=gzipSync(image);res.writeHead(200,{'content-type':'image/png','content-encoding':'gzip','content-length':bytes.length});return res.end(bytes);}
  if(req.url==='/x-assets/website-main/current/static/image.png'){
   res.writeHead(200,{'content-type':'image/png','cache-control':'public, max-age=600'});return res.end(req.method==='HEAD'?undefined:image);
  }
  if(req.url==='/x-assets/website-main/current/lina/module.js'){
   res.writeHead(200,{'content-type':'application/javascript; charset=utf-8'});return res.end(req.method==='HEAD'?undefined:script);
  }
  if(req.url==='/x-assets/website-main/current/index.html'||req.url==='/x-assets/website-main/current/cart.html'){
   res.writeHead(200,{'content-type':'text/html; charset=utf-8'});return res.end(req.method==='HEAD'?undefined:'<html><head><title>主官网</title></head><body>官网正文</body></html>');
  }
  if(req.url==='/x-assets/website-main/current/has-base.html'){
   res.setHeader('content-type','text/html');return res.end('<html><head><base href="/custom/"></head></html>');
  }
  res.writeHead(404,{'content-type':'application/octet-stream'});res.end(Buffer.from([255,0,127]));
 });
 const renderer=createServer((req,res)=>{res.writeHead(200,{'content-type':'text/html'});res.end('<html><head></head><body>original SSR '+req.url+'</body></html>');});
 const backendOrigin=await listen(backend),rendererOrigin=await listen(renderer);
 const children=[];
 async function start(ssr){
  const probe=createServer(),origin=await listen(probe);await new Promise(resolve=>probe.close(resolve));
  const child=spawn(process.execPath,[fileURLToPath(new URL('./server.mjs',import.meta.url))],{env:{...process.env,HOST:'127.0.0.1',PORT:new URL(origin).port,WEBSITE_API_ORIGIN:backendOrigin,WEBSITE_CUSTOMER_URL:backendOrigin,WEBSITE_SSR_ORIGINS:JSON.stringify(ssr)},stdio:'ignore'});children.push(child);
  for(let n=0;n<80;n++){try{if((await fetch(origin+'/')).ok)return origin;}catch{}await new Promise(resolve=>setTimeout(resolve,25));}
  throw new Error('gateway did not start');
 }
 try{
  const origin=await start({});
  const png=await fetch(origin+'/static/image.png');assert.equal(png.headers.get('content-type'),'image/png');assert.deepEqual(Buffer.from(await png.arrayBuffer()),image);assert.equal(png.headers.get('cache-control'),'public, max-age=600');
  const compressed=await fetch(origin+'/static/compressed.png');assert.equal(compressed.headers.get('content-encoding'),null);assert.deepEqual(Buffer.from(await compressed.arrayBuffer()),image);
  const js=await fetch(origin+'/lina/module.js');assert.match(js.headers.get('content-type'),/^application\/javascript/);assert.deepEqual(Buffer.from(await js.arrayBuffer()),script);
  for(const path of ['/','/cart.html']){
   const page=await fetch(origin+path);assert.match(page.headers.get('content-type'),/^text\/html/);const html=await page.text();assert.match(html,/__LINA_TEMPLATE__/);assert.match(html,/<base href="\/x-assets\/website-main\/current\/">/);assert.match(html,/官网正文/);
  }
  const based=await(await fetch(origin+'/has-base.html')).text();assert.equal((based.match(/<base /g)||[]).length,1);assert.match(based,/<base href="\/custom\/">/);assert.match(based,/__LINA_TEMPLATE__/);
  const head=await fetch(origin+'/static/image.png',{method:'HEAD'});assert.equal(head.headers.get('content-type'),'image/png');assert.equal((await head.arrayBuffer()).byteLength,0);assert.ok(calls.some(c=>c.method==='HEAD'&&c.path.endsWith('/static/image.png')));
  const missing=await fetch(origin+'/static/missing.bin');assert.equal(missing.status,404);assert.deepEqual(Buffer.from(await missing.arrayBuffer()),Buffer.from([255,0,127]));
  const ssrOrigin=await start({'website-main':rendererOrigin});const rendered=await(await fetch(ssrOrigin+'/cart.html?lang=en-US')).text();assert.equal(rendered,'<html><head></head><body>original SSR /cart.html?lang=en-US</body></html>');assert.doesNotMatch(rendered,/__LINA_TEMPLATE__|<base/);
 }finally{
  for(const child of children)if(child.exitCode===null&&child.signalCode===null){const exited=once(child,'exit');child.kill();await exited;}
  for(const server of [backend,renderer]){server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
 }
});


// Reproduce Node 24's delayed TimeoutError after a client closes before upstream headers.
test('gateway process survives disconnect before headers and delayed upstream body timeout',{timeout:10000},async()=>{
 let arrived,upstreamClosed;
 const seen=new Promise(resolve=>{arrived=resolve;});
 const upstream=createServer((req,res)=>{
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
 const backend=await listen(upstream),probe=createServer(),origin=await listen(probe);await new Promise(resolve=>probe.close(resolve));
 const preload='const timeout=AbortSignal.timeout;AbortSignal.timeout=()=>timeout(250);';
 const child=spawn(process.execPath,['--import','data:text/javascript,'+encodeURIComponent(preload),fileURLToPath(new URL('./server.mjs',import.meta.url))],{env:{...process.env,HOST:'127.0.0.1',PORT:new URL(origin).port,WEBSITE_API_ORIGIN:backend,WEBSITE_CUSTOMER_URL:backend,WEBSITE_SSR_ORIGINS:'{}'},stdio:['ignore','ignore','pipe']});
 let stderr='';child.stderr.on('data',chunk=>{stderr+=chunk;});
 try{
  let ready=false;for(let n=0;n<80;n++){try{if((await fetch(origin+'/__website/configuration')).ok){ready=true;break;}}catch{}await new Promise(resolve=>setTimeout(resolve,25));}
  assert.ok(ready,'gateway did not start: '+stderr);
  const early=clientGet(origin+'/static/early.bin');early.on('error',()=>{});await seen;early.destroy();
  await Promise.race([upstreamClosed,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('upstream request was not released before its timeout')),150);timer.unref();})]);
  // Await the original timeout even after downstream close: this is where the old orphaned Readable crashed.
  await new Promise(resolve=>setTimeout(resolve,400));
  assert.equal(child.exitCode,null,'disconnect before headers killed gateway: '+stderr);
  await upstreamClosed;
  assert.equal((await fetch(origin+'/__website/configuration')).status,200);
  const response=await fetch(origin+'/static/timeout.bin');assert.equal(response.status,200);await assert.rejects(response.text());
  assert.equal(child.exitCode,null,'mid-body timeout killed gateway: '+stderr);
  const controller=new AbortController();const cancel=await fetch(origin+'/static/cancel.bin',{signal:controller.signal});
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
