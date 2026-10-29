import test from 'node:test';
import assert from 'node:assert/strict';
const moduleURL=new URL('../../frontend/public/lina/commerce-session.js',import.meta.url);
function environment(t,fetcher){
 const originalWindow=globalThis.window,originalFetch=globalThis.fetch;
 globalThis.window={__LINA_TEMPLATE__:{customerUrl:'https://customer.example/'}};
 globalThis.fetch=fetcher;
 t.after(()=>{if(originalWindow===undefined)delete globalThis.window;else globalThis.window=originalWindow;globalThis.fetch=originalFetch;});
 return import(moduleURL.href+'?test='+crypto.randomUUID());
}
test('commerce account request requires a session and never persists access tokens',async t=>{
 const calls=[];const session=await environment(t,async(path,options)=>{calls.push({path,options});return Response.json(path==='/__website/session'?{checked:true,accessToken:'memory-only-token'}:{code:0,data:{cart:{version:1}}});});
 await assert.rejects(session.commerce('account/commerce/cart'));
 assert.equal(calls.length,0);assert.equal(await session.syncSession(),true);assert.equal(session.getAccessToken(),'memory-only-token');
 await session.commerce('account/commerce/cart');assert.equal(calls[1].path,'/x/idc-core/api/v1/account/commerce/cart');assert.equal(calls[1].options.headers.Authorization,'Bearer memory-only-token');
 await session.commerce('public/catalog/products/server/purchase-configuration');assert.equal(calls[2].options.headers.Authorization,undefined);
});
test('unauthorized response clears the in-memory identity; cross-owner paths are rejected',async t=>{
 const session=await environment(t,async path=>path==='/__website/session'?Response.json({checked:true,accessToken:'expired-private-token'}):Response.json({code:401,data:null},{status:401}));
 await session.syncSession();await assert.rejects(session.commerce('account/commerce/cart'),error=>!error.message.includes('expired-private-token'));
 assert.equal(session.getAccessToken(),'');await assert.rejects(session.commerce('account/commerce/orders','POST',{},undefined,'other-plugin'),/Invalid commerce path/);
});
test('customer links accept only local paths and a configured HTTP origin',async t=>{
 const session=await environment(t,()=>{throw new Error('No request expected');});
 assert.equal(session.customerURL('/account/billing/invoices/abc?pay=1'),'https://customer.example/account/billing/invoices/abc?pay=1');
 for(const path of ['//evil.example/','javascript:alert(1)','/\\evil.example','/%2fexample'])assert.equal(session.customerURL(path),'');
 window.__LINA_TEMPLATE__.customerUrl='https://private:secret@customer.example/';assert.equal(session.customerURL('/account/cart'),'');
});

test('sign-in with no configured customer origin reports failure without navigating',async t=>{
 const session=await environment(t,async()=>Response.json({checked:true,accessToken:''}));
 window.__LINA_TEMPLATE__={};
 await assert.rejects(session.signIn(),/客户中心地址尚未配置/);
 assert.equal(session.getAccessToken(),'');
});

test('a consumed one-shot handoff retains its memory session for subsequent actions',async t=>{
 let probes=0;const session=await environment(t,async path=>{if(path==='/__website/session'){probes++;return Response.json(probes===1?{checked:true,accessToken:'one-shot-session'}:{checked:false,accessToken:''});}return Response.json({code:0,data:{cart:{version:1}}});});
 assert.equal(await session.syncSession(),true);assert.equal(await session.syncSession(),true);assert.equal(probes,1);await session.commerce('account/commerce/cart');assert.equal(session.getAccessToken(),'one-shot-session');
});

test('one-use cart response matches the restored identity and never stores credentials',async t=>{
 const prior=globalThis.sessionStorage,values=new Map();globalThis.sessionStorage={setItem:(k,v)=>values.set(k,v),getItem:k=>values.get(k)||null,removeItem:k=>values.delete(k)};t.after(()=>{if(prior===undefined)delete globalThis.sessionStorage;else globalThis.sessionStorage=prior;});
 const session=await environment(t,async()=>Response.json({checked:true,accessToken:'private-cart-customer-session'}));await session.syncSession();const cart={version:8,items:[{cartItemNo:'item-own',quantity:1}]};await session.rememberCart(cart);assert.doesNotMatch(values.get('website-main-cart-response'),/private-cart-customer-session/);assert.deepEqual(await session.takeCart(),cart);assert.equal(await session.takeCart(),null);
 await session.rememberCart(cart);const raw=JSON.parse(values.get('website-main-cart-response'));raw.identity='different-customer';values.set('website-main-cart-response',JSON.stringify(raw));assert.equal(await session.takeCart(),null);
 await session.rememberCart(cart);const expired=JSON.parse(values.get('website-main-cart-response'));expired.savedAt=Date.now()-16000;values.set('website-main-cart-response',JSON.stringify(expired));assert.equal(await session.takeCart(),null);
});

test('business-envelope unauthorized clears the protected cookie before another page can restore it',async t=>{
 const calls=[];const session=await environment(t,async(path,options={})=>{calls.push({path,method:options.method||'GET'});if(path==='/__website/session')return options.method==='DELETE'?new Response(null,{status:204}):Response.json({checked:true,accessToken:'expired-session'});return Response.json({code:401,data:null});});
 await session.syncSession();await assert.rejects(session.commerce('account/commerce/cart'));assert.equal(session.getAccessToken(),'');assert.deepEqual(calls.at(-1),{path:'/__website/session',method:'DELETE'});
});
