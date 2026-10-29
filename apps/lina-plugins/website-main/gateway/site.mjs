import {createCommerceProxy} from './commerce.mjs';
// Ordinary file delivery and exact anonymous API proxy. No dynamic template lookup.
import {readFile,realpath,stat} from 'node:fs/promises';
import {resolve,extname,sep} from 'node:path';
const types={'.html':'text/html; charset=utf-8','.js':'application/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.png':'image/png','.ico':'image/x-icon','.woff':'font/woff','.woff2':'font/woff2','.webp':'image/webp','.jpg':'image/jpeg','.jpeg':'image/jpeg','.gif':'image/gif','.ttf':'font/ttf','.otf':'font/otf','.eot':'application/vnd.ms-fontobject'};
const plain=(status,text='')=>new Response(text,{status,headers:{'content-type':'text/plain; charset=utf-8'}});
export function createWebsiteHandler({publicRoot,backendUrl,customerUrl='',brandCache}){
 const root=resolve(publicRoot);const backend=new URL(backendUrl);
 const commerce=createCommerceProxy({backendUrl,customerUrl});
 const notFound=async method=>{try{return new Response(method==='HEAD'?null:await readFile(resolve(root,'404.html')),{status:404,headers:{'content-type':'text/html; charset=utf-8','cache-control':'no-store'}});}catch{return plain(404);}};
 for(const address of [backendUrl,...(customerUrl?[customerUrl]:[])]){
  const url=new URL(address);if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw new Error('Invalid configured origin');
 }
 return async request=>{
  const url=new URL(request.url);const path=url.pathname;
  let decoded;try{decoded=decodeURIComponent(path);}catch{return plain(404);}
  if(/%(?:2f|5c)/i.test(path)||decoded.includes('\\')||decoded.includes('\0')||decoded.split('/').includes('..'))return plain(404);
  const account=await commerce(request);if(account)return account;
  if(path==='/lina/runtime-config.json')return new Response(JSON.stringify({customerUrl}),{headers:{'content-type':'application/json','cache-control':'no-store'}});
  if(path.startsWith('/x/')||path.startsWith('/x-assets/')){
   const allowed=path.startsWith('/x-assets/')||path==='/x/idc-portal/api/v1/public/brand'||/^\/x\/content-notice\/api\/v1\/public\/announcements(?:\/\d+)?$/.test(path)||/^\/x\/idc-core\/api\/v1\/public\/(catalog|content|commerce)(\/|$)/u.test(path);
   const action=/^\/x\/idc-core\/api\/v1\/public\/(commerce\/(carts|cart-items)|catalog\/products\/[^/]+\/(purchase-configuration\/(evaluate|quote)|campaign-exposures)|content\/items\/[^/]+\/download-tickets)$/u.test(path);
   if(!allowed)return plain(404);
   if(!['GET','HEAD'].includes(request.method)&&!(request.method==='POST'&&action))return plain(405);
   const headers=new Headers(request.headers);
   for(const key of ['authorization','host','connection','x-forwarded-host','content-length','transfer-encoding'])headers.delete(key);
   const init={method:request.method,headers,redirect:'manual',signal:AbortSignal.timeout(20000)};
   if(!['GET','HEAD'].includes(request.method)){init.body=request.body;init.duplex='half';}
   try{
    const startedAt=Date.now();
    const upstream=await fetch(new URL(path+url.search,backend),init);
    if(brandCache&&path==='/x/idc-portal/api/v1/public/brand'&&request.method==='GET'&&upstream.ok){
     upstream.clone().json().then(envelope=>{if(String(envelope.code??0)==='0'&&envelope.data)return brandCache.publish(envelope.data.brand||null,startedAt);}).catch(error=>console.warn('Public identity sync:',error.message));
    }
    const outputHeaders=new Headers(upstream.headers);
    // Fetch decodes compressed bodies; do not forward their original wire length/encoding.
    for(const key of ['content-encoding','content-length','transfer-encoding'])outputHeaders.delete(key);
    return new Response(upstream.body,{status:upstream.status,headers:outputHeaders});
   }catch{return plain(502,'官网服务暂时不可用，请稍后重试。');}
  }
  if(!['GET','HEAD'].includes(request.method))return plain(405);
  try{
   const file=await realpath(resolve(root,decoded==='/'?'index.html':decoded.slice(1)));
   const canonicalRoot=await realpath(root);
   if(!file.startsWith(canonicalRoot+sep)||!(await stat(file)).isFile())return plain(404);
   let bytes=await readFile(file);const type=types[extname(file)]||'application/octet-stream';
   if(extname(file)==='.html'){
    // Existing browser setting name retained; not a plugin declaration.
    const runtime='<script>window.__LINA_TEMPLATE__='+JSON.stringify({customerUrl}).replaceAll('<','\\u003c')+'</script>';
    let identity=null;
    try{identity=await brandCache?.read();}catch{/* Public pages remain available when the optional cache cannot be read. */}
    const escapeAttribute=value=>value.replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
    const imageURLs=identity?[...new Set(['logoUrl','headerLogoUrl','footerLogoUrl'].map(key=>identity.brand?.[key]).filter(value=>typeof value==='string'&&value.length<=2048&&!/[\x00-\x20\\<>"'`]/.test(value)&&(value.startsWith('/')&&!value.startsWith('//')||/^https?:\/\//i.test(value))))]:[];
    const imagePreloads=imageURLs.map(value=>'<link rel="preload" as="image" href="'+escapeAttribute(value)+'" fetchpriority="high">').join('');
    const bootstrap=identity?'<script>window.__LINA_PUBLIC_IDENTITY__='+JSON.stringify(identity).replaceAll('<','\\u003c')+'</script>'+imagePreloads+'<script type="module" src="/lina/brand-bootstrap.js"></script>':'';
    bytes=Buffer.from(bytes.toString().replace(/(<head[^>]*>)/iu,'$1'+runtime+bootstrap));
   }
   return new Response(request.method==='HEAD'?null:bytes,{status:decoded==='/404.html'?404:200,headers:{'content-type':type,'cache-control':'no-store'}});
  }catch(error){if(['ENOENT','ENOTDIR'].includes(error.code))return notFound(request.method);return plain(503,'官网页面暂时不可用，请检查部署文件。');}
 };
}
