// Ordinary website process; pages are local files and business belongs to plugins.
import http from 'node:http';
import {Readable,Writable} from 'node:stream';
import {resolve} from 'node:path';
import {existsSync} from 'node:fs';
import {createWebsiteHandler} from './site.mjs';
import {createBrandIdentityCache} from './brand-identity.mjs';
const backendUrl=process.env.WEBSITE_API_ORIGIN||'http://127.0.0.1:6030';
const brandCache=createBrandIdentityCache({backendUrl,directory:process.env.WEBSITE_BRAND_CACHE_DIR||resolve(import.meta.dirname,'../.public-brand-cache')});
const sourcePublicRoot=resolve(import.meta.dirname,'../frontend/public');
const publicRoot=existsSync(sourcePublicRoot)?sourcePublicRoot:resolve(import.meta.dirname,'../public');
const handler=createWebsiteHandler({publicRoot,backendUrl,customerUrl:process.env.WEBSITE_CUSTOMER_URL||'',brandCache});
const server=http.createServer(async(req,res)=>{
 try{
  const init={method:req.method,headers:req.headers};
  if(!['GET','HEAD'].includes(req.method)){init.body=Readable.toWeb(req);init.duplex='half';}
  const result=await handler(new Request(new URL(req.url,'http://website.invalid'),init));
  for(const [key,value]of result.headers)if(key!=='set-cookie')res.setHeader(key,value);
  const cookies=result.headers.getSetCookie();if(cookies.length)res.setHeader('set-cookie',cookies);
  res.writeHead(result.status);
  // pipeTo also cancels the body when the client closed before the handler returned.
  // Constructing fromWeb first would orphan a Readable if pipeline rejects that closed response.
  if(result.body)await result.body.pipeTo(Writable.toWeb(res));else res.end();
 }catch{
  if(res.headersSent||res.destroyed){if(!res.destroyed)res.destroy();return;}
  res.writeHead(502,{'content-type':'text/plain; charset=utf-8'});res.end('官网服务暂时不可用，请稍后重试。');
 }
});
// Prepare public identity before accepting the first visitor, including its images.
await brandCache.refresh();
setInterval(()=>brandCache.refresh(),10000).unref();
server.listen(Number(process.env.PORT||6680),process.env.HOST||'127.0.0.1');
