// Isolated test preview, not an installed-plugin lifecycle substitute.
import http from 'node:http';
import {readFile} from 'node:fs/promises';
import {resolve,extname,sep} from 'node:path';
const root=resolve(import.meta.dirname,'../../../frontend/public');
const original=process.env.SOURCE_SITE_DIR;
const api=process.env.WEBSITE_API_ORIGIN;
const customerUrl=process.env.WEBSITE_CUSTOMER_URL||'';
const types={'.html':'text/html; charset=utf-8','.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.ttf':'font/ttf','.woff':'font/woff','.woff2':'font/woff2','.png':'image/png','.jpg':'image/jpeg'};
const server=http.createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://localhost');
  if(url.pathname.startsWith('/x/')||url.pathname.startsWith('/x-assets/')){
   if(!api){res.writeHead(503,{'content-type':'application/json'});res.end(JSON.stringify({code:503,message:'Real API origin not configured'}));return;}
   const chunks=[];for await(const chunk of req)chunks.push(chunk);
   const headers={accept:req.headers.accept||'*/*'};if(req.headers.cookie)headers.cookie=req.headers.cookie;if(req.headers['content-type'])headers['content-type']=req.headers['content-type'];
   const upstream=await fetch(new URL(req.url,api),{method:req.method,headers,body:['GET','HEAD'].includes(req.method)?undefined:Buffer.concat(chunks),signal:AbortSignal.timeout(15000)});
   const outputHeaders={'content-type':upstream.headers.get('content-type')||'application/octet-stream'};const cookies=upstream.headers.getSetCookie();if(cookies.length)outputHeaders['set-cookie']=cookies;
   res.writeHead(upstream.status,outputHeaders);res.end(Buffer.from(await upstream.arrayBuffer()));return;
  }
  const isOriginal=url.pathname.startsWith('/original/');const dir=isOriginal&&original?resolve(original):root;
  const name=decodeURIComponent(isOriginal?url.pathname.slice(10):url.pathname.slice(1))||'index.html';const file=resolve(dir,name);
  if(!file.startsWith(dir+sep)){res.writeHead(404);res.end();return;}
  let bytes=await readFile(file);if(extname(file)==='.html'){
   let html=bytes.toString();if(isOriginal)html=html.replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi,'').replace(/\son\w+\s*=\s*(?:"[^"]*"|'[^']*')/gi,'');
   else html=html.replace(/<head[^>]*>/i,m=>m+'<script>window.__LINA_TEMPLATE__='+JSON.stringify({customerUrl}).replaceAll('<','\\u003c')+'</script>');
   bytes=Buffer.from(html);
  }
  res.writeHead(200,{'content-type':types[extname(file)]||'application/octet-stream',...(isOriginal?{'content-security-policy':"default-src 'self' data:; script-src 'none'; style-src 'self' 'unsafe-inline'; form-action 'none'; connect-src 'none'; frame-src 'none'"}:{})});res.end(bytes);
 }catch{res.writeHead(404);res.end();}
});
server.listen(Number(process.env.PREVIEW_PORT||16680),'127.0.0.1',()=>console.log('Isolated preview ready '+server.address().port));
