// Independent website entry: public API/assets proxy plus active-provider SSR/static routing.
// SSR upstreams are operator configuration; no visitor can select an arbitrary proxy origin.
import http from 'node:http';
import https from 'node:https';
const backend = new URL(process.env.WEBSITE_API_ORIGIN || 'http://127.0.0.1:6030');
const customerUrl = process.env.WEBSITE_CUSTOMER_URL || 'http://localhost:6032';
const ssr = JSON.parse(process.env.WEBSITE_SSR_ORIGINS || '{}');
for (const value of [backend.href, ...Object.values(ssr)]) {const url=new URL(value);if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw new Error('Invalid configured upstream');}
function proxy(req,res,target,anonymous=false){
 const headers={...req.headers,host:target.host};delete headers.connection;
 if(anonymous){delete headers.authorization;delete headers['x-forwarded-host'];}
 const outgoing=(target.protocol==='https:'?https:http).request(target,{method:req.method,headers},incoming=>{res.writeHead(incoming.statusCode,incoming.headers);incoming.pipe(res);});
 outgoing.setTimeout(20000,()=>outgoing.destroy(new Error('Upstream timeout')));
 outgoing.on('error',()=>{if(!res.headersSent)res.writeHead(502,{'content-type':'text/plain; charset=utf-8'});res.end('官网服务暂时不可用，请稍后重试。');});req.pipe(outgoing);
}
const server=http.createServer(async(req,res)=>{
 try {
  const url=new URL(req.url,'http://website.invalid');const path=url.pathname;
  if(path.includes('%')||path.includes('\\')||path.includes('..')){res.writeHead(404);return res.end();}
  if(path.startsWith('/x/')){
   const allowed=path==='/x/idc-portal/api/v1/public/brand'||/^\/x\/idc-core\/api\/v1\/public\/(catalog|content|commerce)(\/|$)/u.test(path);
   const action=/^\/x\/idc-core\/api\/v1\/public\/(commerce\/(carts|cart-items)|catalog\/products\/[^/]+\/(purchase-configuration\/(evaluate|quote)|campaign-exposures)|content\/items\/[^/]+\/download-tickets)$/u.test(path);
   if(!allowed){res.writeHead(404);return res.end();}if(!['GET','HEAD'].includes(req.method)&&!(req.method==='POST'&&action)){res.writeHead(405);return res.end();}return proxy(req,res,new URL(req.url,backend),true);
  }
  if(path.startsWith('/x-assets/')){if(!['GET','HEAD'].includes(req.method)){res.writeHead(405);return res.end();}return proxy(req,res,new URL(req.url,backend),true);}
  if(!['GET','HEAD'].includes(req.method)){res.writeHead(405);return res.end();}
  const config=await fetch(new URL('/x/idc-portal/api/v1/public/brand',backend),{signal:AbortSignal.timeout(20000)});if(!config.ok)throw new Error('brand');
  const entry=(await config.json()).data?.brand?.templateEntry;
  const match=/^\/x-assets\/([a-z0-9]+(?:-[a-z0-9]+)*)\/[^/]+\//u.exec(entry?.assetBase||'');
  if(!match||!entry.url.startsWith(entry.assetBase)||entry.url.includes('..'))throw new Error('template');
  if(ssr[match[1]])return proxy(req,res,new URL(req.url,ssr[match[1]]));
  const documentRoute=!/\.[^/]+$/u.test(path)||path.endsWith('.html');
  const asset=path==='/'||(entry.spa&&documentRoute)?entry.url:entry.assetBase+path.slice(1);
  const page=await fetch(new URL(asset,backend),{signal:AbortSignal.timeout(20000)});if(!page.ok){res.writeHead(page.status);return res.end();}
  let html=await page.text();const runtime='<script>window.__LINA_TEMPLATE__='+JSON.stringify({customerUrl}).replaceAll('<','\\u003c')+'</script>';
  const injection=runtime+(/<base[\s>]/iu.test(html)?'':'<base href="'+entry.assetBase.replaceAll('"','&quot;')+'">');
  html=html.replace(/(<head[^>]*>)/iu,'$1'+injection);res.writeHead(200,{'content-type':'text/html; charset=utf-8','cache-control':'no-store'});return res.end(req.method==='HEAD'?'':html);
 }catch {res.writeHead(503,{'content-type':'text/plain; charset=utf-8'});res.end('官网模板暂时不可用，请管理员检查已启用的模板及服务。');}
});
server.listen(Number(process.env.PORT||6680),process.env.HOST||'0.0.0.0');
