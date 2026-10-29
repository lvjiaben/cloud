// Independent website entry: public API/assets proxy plus active-provider SSR/static routing.
// SSR upstreams are operator configuration; no visitor can select an arbitrary proxy origin.
import http from 'node:http';
import https from 'node:https';
import {pipeline,Writable} from 'node:stream';
const backend = new URL(process.env.WEBSITE_API_ORIGIN || 'http://127.0.0.1:6030');
const customerUrl = process.env.WEBSITE_CUSTOMER_URL || 'http://localhost:6032';
const customer = new URL(customerUrl);
const ssr = JSON.parse(process.env.WEBSITE_SSR_ORIGINS || '{}');
for (const value of [backend.href, customer.href, ...Object.values(ssr)]) {const url=new URL(value);if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw new Error('Invalid configured upstream');}
function proxy(req,res,target,anonymous=false){
 const headers={...req.headers,host:target.host};delete headers.connection;
 if(anonymous){delete headers.authorization;delete headers['x-forwarded-host'];}
 const outgoing=(target.protocol==='https:'?https:http).request(target,{method:req.method,headers},incoming=>{
  res.writeHead(incoming.statusCode,incoming.headers);
  // Body failures and client disconnects must release both streams, never the gateway.
  pipeline(incoming,res,()=>{});
 });
 outgoing.setTimeout(20000,()=>outgoing.destroy(new Error('Upstream timeout')));
 outgoing.on('error',()=>{
  if(res.headersSent||res.destroyed){if(!res.destroyed)res.destroy();return;}
  res.writeHead(502,{'content-type':'text/plain; charset=utf-8'});res.end('官网服务暂时不可用，请稍后重试。');
 });
 const closeUpstream=()=>outgoing.destroy();
 res.once('close',closeUpstream);
 outgoing.once('close',()=>res.off('close',closeUpstream));
 req.once('aborted',closeUpstream);
 req.once('error',closeUpstream);
 req.pipe(outgoing);
}
const server=http.createServer(async(req,res)=>{
 const controller=new AbortController();
 const closeRequest=()=>{if(!res.writableFinished)controller.abort();};
 const requestSignal=milliseconds=>AbortSignal.any([controller.signal,AbortSignal.timeout(milliseconds)]);
 res.once('close',closeRequest);
 try {
  const url=new URL(req.url,'http://website.invalid');const path=url.pathname;
  if(path.includes('%')||path.includes('\\')||path.includes('..')){res.writeHead(404);return res.end();}
  if(path==='/__website/session'){
   const cookieName='lina_website_handoff';
   const cookieFlags='; Path=/__website/session; HttpOnly; SameSite=Lax'+(req.socket.encrypted?'; Secure':'');
   if(req.method==='PUT'){
    const source=req.headers.origin;let same=false;try{const u=new URL(source);same=['http:','https:'].includes(u.protocol)&&u.host===req.headers.host&&!u.username&&!u.password;}catch{}
    if(!same||req.headers['sec-fetch-site']&&!['same-origin','none'].includes(req.headers['sec-fetch-site'])){res.writeHead(403);return res.end();}
    const match=/^Bearer ([^\r\n]{1,8192})$/.exec(req.headers.authorization||'');if(!match){res.writeHead(401);return res.end();}
    const check=await fetch(new URL('/api/v1/user/info',backend),{headers:{authorization:'Bearer '+match[1]},signal:requestSignal(10000),redirect:'error'});
    if(!check.ok){await check.body?.cancel();res.writeHead(check.status===401?401:503);return res.end();}const payload=await check.json();if(String(payload.code)!=='0'){res.writeHead(String(payload.code)==='401'?401:403);return res.end();}
    res.writeHead(200,{'content-type':'application/json','cache-control':'no-store','set-cookie':cookieName+'='+encodeURIComponent(JSON.stringify({accessToken:match[1]}))+cookieFlags+'; Max-Age=86400'});return res.end(JSON.stringify({checked:true}));
   }
   if(req.method==='DELETE'){
    if(req.headers.origin){let valid=false;try{const source=new URL(req.headers.origin);valid=['http:','https:'].includes(source.protocol)&&source.host===req.headers.host&&!source.username&&!source.password;}catch{}if(!valid){res.writeHead(403);return res.end();}}
    if(req.headers['sec-fetch-site']&&!['same-origin','none'].includes(req.headers['sec-fetch-site'])){res.writeHead(403);return res.end();}
    res.writeHead(204,{'cache-control':'no-store','set-cookie':cookieName+'='+cookieFlags+'; Max-Age=0'});return res.end();
   }
   if(req.method==='POST'){
    if(req.headers.origin!==customer.origin){res.writeHead(403);return res.end();}
    let body='';for await(const chunk of req){body+=chunk;if(body.length>16384){res.writeHead(413);return res.end();}}
    const form=new URLSearchParams(body),returnTo=form.get('returnTo')||'/cart';let token=form.get('accessToken')||'';
    if(token.length>8192||/[\r\n]/.test(token)||!/^\/(?:en-US\/)?(?:$|cart(?:\.html)?|configure\.html|services\.html|checkout|products(?:\.html|\/[^/?#]+)?|pages\/[^/?#]+)\/?(?:[?#].*)?$/.test(returnTo)){res.writeHead(400);return res.end();}
    if(token){
     try {
      const check=await fetch(new URL('/api/v1/user/info',backend),{headers:{authorization:'Bearer '+token},signal:requestSignal(20000)});
      // Authentication middleware may return plain text. An expired session
      // must return to the public page as a guest, never become a template error.
      if(check.status===401){token='';await check.body?.cancel();}
      else if(!check.ok){await check.body?.cancel();res.writeHead(check.status===403?403:503,{'cache-control':'no-store'});return res.end();}
      else {const result=await check.json();if(String(result.code)==='401')token='';else if(String(result.code)!=='0'){res.writeHead(403,{'cache-control':'no-store'});return res.end();}}
     }catch {if(res.headersSent||res.destroyed){if(!res.destroyed)res.destroy();return;}res.writeHead(503,{'content-type':'text/plain; charset=utf-8','cache-control':'no-store'});return res.end('官网服务暂时不可用，请稍后重试。');}
    }
    res.writeHead(303,{'location':returnTo,'cache-control':'no-store','set-cookie':cookieName+'='+encodeURIComponent(JSON.stringify({accessToken:token}))+cookieFlags+'; Max-Age=86400'});return res.end();
   }
   if(req.method==='GET'){
    if(req.headers['sec-fetch-site']&&!['same-origin','none'].includes(req.headers['sec-fetch-site'])){res.writeHead(403);return res.end();}
    const raw=String(req.headers.cookie||'').split(';').map(part=>part.trim()).find(part=>part.startsWith(cookieName+'='));
    let session={checked:false,accessToken:''};
    if(raw){try{const parsed=JSON.parse(decodeURIComponent(raw.slice(cookieName.length+1)));if(typeof parsed.accessToken==='string')session={checked:true,accessToken:parsed.accessToken};}catch{/* Invalid handoff remains unsigned. */}}
    res.writeHead(200,{'content-type':'application/json','cache-control':'no-store'});return res.end(JSON.stringify(session));
   }
   res.writeHead(405);return res.end();
  }
  if(path==='/__website/configuration'&&req.method==='GET'){res.writeHead(200,{'content-type':'application/json','cache-control':'no-store'});return res.end(JSON.stringify({origin:'http'+(req.socket.encrypted?'s':'')+'://'+req.headers.host,customerOrigin:customer.origin}));}
  // Login and the final invoice use the complete customer template at the same
  // origin. Purchase configuration and checkout remain website-owned routes.
  if (/^\/(auth(?:\/|$)|account(?:\/|$)|payment\/return\/)/u.test(path)) return proxy(req,res,new URL(req.url,customer));
  if(path.startsWith('/api/v1/')) return proxy(req,res,new URL(req.url,backend));
  if(path.startsWith('/x/')){
   if(/^\/x\/idc-payment\/api\/v1\/callbacks\/(payments|refunds)\/[A-Za-z0-9_-]{1,128}$/u.test(path)){
    const allowGet=path.includes('/callbacks/payments/')&&req.method==='GET';
    if(req.method!=='POST'&&!allowGet){res.writeHead(405,{allow:path.includes('/callbacks/payments/')?'GET, POST':'POST'});return res.end();}
    return proxy(req,res,new URL(req.url,backend),true);
   }
   if (/^\/x\/[a-z0-9-]+\/api\/v1\/account(?:\/|$)/u.test(path) || path.startsWith('/x/idc-portal/api/v1/public/customer-center')) return proxy(req,res,new URL(req.url,backend));
   // Public announcements are owned by content-notice; management and read-state routes stay private.
   if(/^\/x\/content-notice\/api\/v1\/public\/announcements(?:\/[1-9][0-9]*)?$/u.test(path)){
    if(req.method!=='GET'){res.writeHead(405,{allow:'GET'});return res.end();}
    return proxy(req,res,new URL(req.url,backend),true);
   }
   const allowed=path==='/x/idc-portal/api/v1/public/brand'||/^\/x\/idc-core\/api\/v1\/public\/(catalog|content|commerce)(\/|$)/u.test(path);
   const action=/^\/x\/idc-core\/api\/v1\/public\/(commerce\/(carts|cart-items)|catalog\/products\/[^/]+\/(purchase-configuration\/(evaluate|quote)|campaign-exposures)|content\/items\/[^/]+\/download-tickets)$/u.test(path);
   if(!allowed){res.writeHead(404);return res.end();}if(!['GET','HEAD'].includes(req.method)&&!(req.method==='POST'&&action)&&!(req.method==='DELETE'&&/^\/x\/idc-core\/api\/v1\/public\/commerce\/cart-items\/[^/]+$/u.test(path))){res.writeHead(405);return res.end();}return proxy(req,res,new URL(req.url,backend),true);
  }
  if(path.startsWith('/x-assets/')){if(!['GET','HEAD'].includes(req.method)){res.writeHead(405);return res.end();}return proxy(req,res,new URL(req.url,backend),true);}
  if(!['GET','HEAD'].includes(req.method)){res.writeHead(405);return res.end();}
  const config=await fetch(new URL('/x/idc-portal/api/v1/public/brand',backend),{signal:requestSignal(20000)});if(!config.ok)throw new Error('brand');
  const entry=(await config.json()).data?.brand?.templateEntry;
  const match=/^\/x-assets\/([a-z0-9]+(?:-[a-z0-9]+)*)\/[^/]+\//u.exec(entry?.assetBase||'');
  if(!match||!entry.url.startsWith(entry.assetBase)||entry.url.includes('..'))throw new Error('template');
  if(ssr[match[1]])return proxy(req,res,new URL(req.url,ssr[match[1]]));
  const documentRoute=!/\.[^/]+$/u.test(path)||path.endsWith('.html');
  const asset=path==='/'||(entry.spa&&documentRoute)?entry.url:entry.assetBase+path.slice(1);
  const page=await fetch(new URL(asset,backend),{method:req.method,signal:requestSignal(20000)});
  const headers=Object.fromEntries(page.headers);
  // Fetch decodes upstream compression; forwarding the wire length/encoding would corrupt assets.
  for(const key of ['content-encoding','content-length','transfer-encoding','connection','keep-alive'])delete headers[key];
  const contentType=page.headers.get('content-type')||'application/octet-stream';headers['content-type']=contentType;
  if(page.ok&&/^text\/html(?:;|$)/iu.test(contentType)){
   let html=await page.text();const runtime='<script>window.__LINA_TEMPLATE__='+JSON.stringify({customerUrl}).replaceAll('<','\\u003c')+'</script>';
   const injection=runtime+(/<base[\s>]/iu.test(html)?'':'<base href="'+entry.assetBase.replaceAll('"','&quot;')+'">');
   html=html.replace(/(<head[^>]*>)/iu,'$1'+injection);headers['cache-control']='no-store';
   res.writeHead(page.status,headers);return res.end(req.method==='HEAD'?'':html);
  }
  res.writeHead(page.status,headers);
  if(req.method==='HEAD'||!page.body){await page.body?.cancel();return res.end();}
  // Keep fetch's body in Web streams: fromWeb can be orphaned when pipeline rejects an already closed response.
  await page.body.pipeTo(Writable.toWeb(res));
 }catch {if(res.headersSent||res.destroyed){if(!res.destroyed)res.destroy();return;}res.writeHead(503,{'content-type':'text/plain; charset=utf-8'});res.end('官网模板暂时不可用，请管理员检查已启用的模板及服务。');}
 finally {res.off('close',closeRequest);}
});
server.listen(Number(process.env.PORT||6680),process.env.HOST||'0.0.0.0');
