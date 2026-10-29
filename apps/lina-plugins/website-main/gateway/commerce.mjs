
const reply=(status,data,headers={})=>new Response(data===undefined?null:JSON.stringify(data),{status,headers:{'content-type':'application/json','cache-control':'no-store',...headers}});
// Only the current customer's published commerce contracts cross this proxy.
export function createCommerceProxy({backendUrl,customerUrl}){
 const backend=new URL(backendUrl),customer=customerUrl?new URL(customerUrl):null;
 const cookieName='website_main_handoff';
 const sameOrigin=request=>{
  const origin=request.headers.get('origin');
  if(!origin)return true;
  try{
   const source=new URL(origin),target=new URL(request.url),host=request.headers.get('host');
   // The ordinary Node adapter uses an internal URL base and retains the actual Host.
   return ['http:','https:'].includes(source.protocol)&&!source.username&&!source.password&&
    (target.hostname==='website.invalid'&&host?source.host===host:source.origin===target.origin);
  }catch{return false;}
 };
 const sameSite=request=>!request.headers.has('sec-fetch-site')||['same-origin','none'].includes(request.headers.get('sec-fetch-site'));
 return async request=>{
  const url=new URL(request.url),path=url.pathname;
  try{
  if(path==='/__website/configuration')return request.method==='GET'?reply(200,{customerOrigin:customer?.origin||''}):reply(405);
  if(path==='/__website/session'){
   const flags='; Path=/__website/session; HttpOnly; SameSite=Lax'+(url.protocol==='https:'?'; Secure':'');
   if(request.method==='PUT'){
    if(!sameSite(request)||!sameOrigin(request)||!request.headers.get('origin'))return reply(403);
    const match=/^Bearer ([^\r\n]{1,8192})$/.exec(request.headers.get('authorization')||'');if(!match)return reply(401);
    const check=await fetch(new URL('/api/v1/user/info',backend),{headers:{Authorization:'Bearer '+match[1]},redirect:'error',signal:AbortSignal.timeout(10000)});
    if(!check.ok){await check.body?.cancel();return reply(check.status===401?401:502);}const body=await check.json();if(String(body.code)!=='0')return reply(String(body.code)==='401'?401:403);
    return reply(200,{checked:true},{'set-cookie':cookieName+'='+encodeURIComponent(JSON.stringify({accessToken:match[1]}))+flags+'; Max-Age=86400'});
   }
   if(request.method==='DELETE'){if(!sameSite(request)||!sameOrigin(request))return reply(403);return reply(204,undefined,{'set-cookie':cookieName+'='+flags+'; Max-Age=0'});}
   if(request.method==='POST'){
    if(!customer||request.headers.get('origin')!==customer.origin)return reply(403);
    if(Number(request.headers.get('content-length'))>16384)return reply(413);
    const raw=await request.text();if(Buffer.byteLength(raw)>16384)return reply(413);
    const form=new URLSearchParams(raw),accessToken=form.get('accessToken')||'',returnTo=form.get('returnTo')||'/cart.html';
    if(!/^\/(?:cart|configure|products)\.html(?:\?[^#\r\n]*)?$/.test(returnTo)||/[\r\n]/.test(accessToken)||accessToken.length>8192)return reply(400);
    if(accessToken){
     const check=await fetch(new URL('/api/v1/user/info',backend),{headers:{Authorization:'Bearer '+accessToken},redirect:'error',signal:AbortSignal.timeout(10000)});
     if(!check.ok){await check.body?.cancel();return reply([401,403].includes(check.status)?403:502);}const body=await check.json();if(String(body.code)!=='0'||!body.data)return reply(403);
    }
    return reply(303,undefined,{location:returnTo,'set-cookie':cookieName+'='+encodeURIComponent(JSON.stringify({accessToken}))+flags+'; Max-Age=86400'});
   }
   if(request.method==='GET'){
    if(!sameSite(request)||!sameOrigin(request))return reply(403);
    const raw=(request.headers.get('cookie')||'').split(';').map(v=>v.trim()).find(v=>v.startsWith(cookieName+'=')),value=raw?.slice(cookieName.length+1);let session={checked:false,accessToken:''};try{const item=JSON.parse(decodeURIComponent(value||''));if(typeof item.accessToken==='string')session={checked:true,accessToken:item.accessToken};}catch{}
    return reply(200,session);
   }
   return reply(405);
  }
  const prefix='/x/idc-core/api/v1/account/';
  const core=path.startsWith(prefix),payment=path.startsWith('/x/idc-payment/api/v1/account/');
  if(!core&&!payment)return null;
  const id='[A-Za-z0-9_-]{1,40}',patterns={
   GET:[/^account\/commerce\/cart$/,new RegExp('^account/commerce/orders/'+id+'$'),/^account\/marketing\/coupons$/,/^account\/billing\/overview$/,new RegExp('^account/billing/invoices/'+id+'$')],
   POST:[/^account\/commerce\/(?:carts|cart-items|quotes|orders|guest-cart-merge)$/,new RegExp('^account/commerce/quote-items/'+id+'/purchase-configuration$'),new RegExp('^account/catalog/products/[^/]+/purchase-configuration/(?:evaluate|quote|file-uploads|server-login)$'),/^account\/marketing\/coupons\/claims$/,new RegExp('^account/commerce/orders/'+id+'/payment-preparations$'),new RegExp('^account/billing/invoices/'+id+'/balance-payments$')],
   PUT:[new RegExp('^account/commerce/cart-items/'+id+'$'),new RegExp('^account/catalog/purchase-configuration/file-uploads/'+id+'/content$'),new RegExp('^account/commerce/payment-preparations/'+id+'$')],
   DELETE:[new RegExp('^account/commerce/cart-items/'+id+'$')]
  };
  const relative=core?path.slice('/x/idc-core/api/v1/'.length):path.slice('/x/idc-payment/api/v1/'.length),isRead=request.method==='GET';
  const allowed=core?(patterns[request.method]||[]).some(p=>p.test(relative)):
   (isRead&&(relative==='account/payment-channels'||new RegExp('^account/payments/'+id+'$').test(relative)))||(request.method==='POST'&&relative==='account/payments');
  if(!allowed)return reply(405);
  const auth=request.headers.get('authorization')||'';if(!/^Bearer [^\r\n]+$/.test(auth))return reply(401);
  if(!sameSite(request)||!sameOrigin(request))return reply(403);
  const headers=new Headers({'authorization':auth,'accept':'application/json'});for(const key of ['content-type','accept-language'])if(request.headers.has(key))headers.set(key,request.headers.get(key));
  const result=await fetch(new URL(path+url.search,backend),{method:request.method,headers,redirect:'manual',signal:AbortSignal.timeout(20000),...(!isRead?{body:request.body,duplex:'half'}:{})});
  const output=new Headers(result.headers);for(const key of ['content-encoding','content-length','transfer-encoding','connection','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','upgrade','set-cookie'])output.delete(key);output.set('cache-control','no-store');return new Response(result.body,{status:result.status,headers:output});
  }catch{return reply(502,{message:'Website commerce is temporarily unavailable.'});}
 };
}
