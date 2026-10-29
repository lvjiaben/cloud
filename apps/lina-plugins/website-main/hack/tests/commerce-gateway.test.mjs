import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import {mkdtemp,rm,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {once} from 'node:events';
import {createWebsiteHandler} from '../../gateway/site.mjs';

async function fixture(t){
 const calls=[];
 const upstream=http.createServer(async(req,res)=>{
  const chunks=[];for await(const chunk of req)chunks.push(chunk);
  calls.push({path:req.url,method:req.method,headers:req.headers,body:Buffer.concat(chunks)});
  if(req.headers.authorization==='Bearer rejected'){res.writeHead(401);res.end('{}');return;}
  res.writeHead(200,{'content-type':'application/json','set-cookie':'backend_private=not-for-the-website; HttpOnly'});
  res.end(JSON.stringify({code:0,data:{cart:{version:1},accepted:true}}));
 });
 upstream.listen(0,'127.0.0.1');await once(upstream,'listening');
 t.after(()=>new Promise(resolve=>{upstream.closeAllConnections();upstream.close(resolve);}));
 const root=await mkdtemp(join(tmpdir(),'website-commerce-'));t.after(()=>rm(root,{recursive:true,force:true}));
 await writeFile(join(root,'index.html'),'<html><head></head></html>');
 const backendUrl='http://127.0.0.1:'+upstream.address().port;
 const handle=createWebsiteHandler({publicRoot:root,backendUrl,customerUrl:'https://customer.example/'});
 const request=(path,init={})=>handle(new Request('https://website.local'+path,init));
 return {calls,request,upstream,backendUrl};
}
const bearer={authorization:'Bearer website-customer-token'};
const account='/x/idc-core/api/v1/account';

test('customer handoff validates against a real upstream and preserves an HttpOnly session across page loads',async t=>{
 const {calls,request}=await fixture(t);
 const post=body=>request('/__website/session',{method:'POST',headers:{origin:'https://customer.example','content-type':'application/x-www-form-urlencoded'},body:new URLSearchParams(body)});
 const result=await post({accessToken:'website-customer-token',returnTo:'/configure.html?product=11'});
 assert.equal(result.status,303);assert.equal(result.headers.get('location'),'/configure.html?product=11');
 assert.equal(calls.length,1);assert.equal(calls[0].path,'/api/v1/user/info');assert.equal(calls[0].headers.authorization,bearer.authorization);
 const cookie=result.headers.get('set-cookie');assert.match(cookie,/HttpOnly; SameSite=Lax; Secure/);assert.match(cookie,/Path=\/__website\/session/);
 const headers={cookie:cookie.split(';')[0],'sec-fetch-site':'same-origin',origin:'https://website.local'};
 const first=await request('/__website/session',{headers});assert.equal(first.headers.get('cache-control'),'no-store');
 assert.deepEqual(await first.json(),{checked:true,accessToken:'website-customer-token'});assert.equal(first.headers.get('set-cookie'),null);
 assert.deepEqual(await(await request('/__website/session',{headers})).json(),{checked:true,accessToken:'website-customer-token'});
 assert.equal((await post({accessToken:'rejected',returnTo:'/cart.html'})).status,403);
 assert.equal((await post({accessToken:'website-customer-token',returnTo:'//evil.example/'})).status,400);
 assert.equal((await post({accessToken:'website-customer-token',returnTo:'/cart.html#fragment'})).status,400);
 const count=calls.length;
 assert.equal((await request('/__website/session',{method:'POST',headers:{origin:'https://evil.example'},body:'accessToken=bad'})).status,403);
 assert.equal((await request('/__website/session',{method:'POST',headers:{origin:'https://customer.example'},body:'a'.repeat(16385)})).status,413);
 assert.equal((await request('/__website/session',{headers:{origin:'https://evil.example'}})).status,403);
 assert.equal((await request('/__website/session',{headers:{'sec-fetch-site':'cross-site'}})).status,403);
 assert.equal(calls.length,count,'invalid origins/return paths never reach the upstream');
 const saved=await request('/__website/session',{method:'PUT',headers:{...bearer,origin:'https://website.local','sec-fetch-site':'same-origin'}});assert.equal(saved.status,200);assert.match(saved.headers.get('set-cookie'),/HttpOnly/);assert.match(saved.headers.get('set-cookie'),/Max-Age=86400/);
 assert.equal((await request('/__website/session',{method:'PUT',headers:{...bearer,origin:'https://evil.example'}})).status,403);
 assert.equal((await request('/__website/session',{method:'PUT',headers:{authorization:'Bearer rejected',origin:'https://website.local'}})).status,401);
 const removed=await request('/__website/session',{method:'DELETE',headers:{origin:'https://website.local','sec-fetch-site':'same-origin'}});assert.equal(removed.status,204);assert.match(removed.headers.get('set-cookie'),/Max-Age=0/);

});

test('authenticated commerce proxies only published methods, strips unrelated identity and carries login credentials in the body',async t=>{
 const {calls,request}=await fixture(t);
 const password='test-only-client-password';
 const response=await request(account+'/catalog/products/11/purchase-configuration/server-login',{
  method:'POST',headers:{...bearer,origin:'https://website.local','sec-fetch-site':'same-origin','content-type':'application/json','accept-language':'en-US',cookie:'private_session=never-forward','x-admin-user':'forged'},
  body:JSON.stringify({configurationRevision:'current-revision',hostname:'customer-host',password,previousServerLoginToken:'old-opaque-reference'})});
 assert.equal(response.status,200);assert.deepEqual(await response.json(),{code:0,data:{cart:{version:1},accepted:true}});
 assert.equal(response.headers.get('cache-control'),'no-store');assert.equal(response.headers.get('set-cookie'),null);
 assert.equal(calls[0].headers.authorization,bearer.authorization);assert.equal(calls[0].headers['accept-language'],'en-US');
 assert.equal(calls[0].headers.cookie,undefined);assert.equal(calls[0].headers['x-admin-user'],undefined);
 assert.deepEqual(JSON.parse(calls[0].body),{configurationRevision:'current-revision',hostname:'customer-host',password,previousServerLoginToken:'old-opaque-reference'});
 for(const [method,path]of [
  ['GET','/commerce/cart'],['POST','/commerce/carts'],['POST','/commerce/cart-items'],['PUT','/commerce/cart-items/item-1'],['DELETE','/commerce/cart-items/item-1?expectedCartVersion=1'],
  ['POST','/commerce/quotes'],['POST','/commerce/orders'],['POST','/commerce/guest-cart-merge'],['POST','/commerce/quote-items/item-1/purchase-configuration'],['POST','/catalog/products/11/purchase-configuration/evaluate'],['POST','/catalog/products/11/purchase-configuration/quote'],['POST','/catalog/products/11/purchase-configuration/file-uploads'],['GET','/marketing/coupons'],['POST','/marketing/coupons/claims'],['GET','/billing/overview'],['GET','/billing/invoices/inv-1'],['POST','/billing/invoices/inv-1/balance-payments'],['GET','/commerce/orders/order-1'],['POST','/commerce/orders/order-1/payment-preparations'],['PUT','/commerce/payment-preparations/context-1']
 ])assert.equal((await request(account+path,{method,headers:bearer,...(['GET','DELETE'].includes(method)?{}:{body:'{}'})})).status,200,method+' '+path);
 const bytes=Buffer.from([0,1,255,42]);assert.equal((await request(account+'/catalog/purchase-configuration/file-uploads/upload-1/content',{method:'PUT',headers:{...bearer,'content-type':'application/octet-stream'},body:bytes})).status,200);
 assert.deepEqual(calls.at(-1).body,bytes);
 for(const [method,path] of [['GET','payment-channels?currencyCode=CNY'],['POST','payments'],['GET','payments/attempt-1']])assert.equal((await request('/x/idc-payment/api/v1/account/'+path,{method,headers:bearer,...(method==='POST'?{body:'{}'}:{})})).status,200);
});

test('private routes, origin forgery and missing authentication never reach the upstream',async t=>{
 const {calls,request,backendUrl}=await fixture(t);
 for(const path of ['/x/idc-core/api/v1/admin/catalog/products','/x/idc-payment/api/v1/admin/payments','/x/private/api/v1/account/commerce/cart'])assert.equal((await request(path,{headers:bearer})).status,404,path);
 for(const [method,path]of [['GET','/commerce/orders'],['GET','/catalog/products/11/purchase-configuration'],['POST','/catalog/products/11/private-action'],['DELETE','/catalog/products/11/purchase-configuration'],['PUT','/billing/overview'],['GET','/commerce/cart-items']])assert.equal((await request(account+path,{method,headers:bearer})).status,405,method+' '+path);
 assert.equal((await request(account+'/commerce/cart')).status,401);
 assert.equal((await request(account+'/commerce/orders',{method:'POST',headers:{authorization:'Basic invalid'}})).status,401);
 for(const headers of [{...bearer,origin:'https://evil.example'},{...bearer,'sec-fetch-site':'cross-site'},{...bearer,'sec-fetch-site':'same-site'}])assert.equal((await request(account+'/commerce/cart',{headers})).status,403);
 assert.equal((await request(account+'/catalog/products/11%2fprivate/purchase-configuration',{headers:bearer})).status,404);
 assert.equal(calls.length,0);
 // The ordinary Node adapter retains the real Host while using an internal URL base.
 const handle=createWebsiteHandler({publicRoot:tmpdir(),backendUrl,customerUrl:''});
 const mismatch=await handle(new Request('http://website.invalid'+account+'/commerce/cart',{headers:{...bearer,host:'website.local',origin:'https://evil.example'}}));assert.equal(mismatch.status,403);
 const matched=await handle(new Request('http://website.invalid'+account+'/commerce/cart',{headers:{...bearer,host:'website.local',origin:'https://website.local'}}));assert.equal(matched.status,200);
});

test('upstream failures return a bounded error without credentials or transport causes',async t=>{
 const {request,upstream}=await fixture(t);await new Promise(resolve=>{upstream.closeAllConnections();upstream.close(resolve);});
 const response=await request(account+'/catalog/products/11/purchase-configuration/server-login',{method:'POST',headers:bearer,body:JSON.stringify({password:'test-only-hidden-failure-password'})});
 assert.equal(response.status,502);const text=await response.text();assert.doesNotMatch(text,/test-only-hidden-failure-password|website-customer-token|ECONNREFUSED|fetch failed/);
});
