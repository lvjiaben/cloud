// Standalone admin static entry; the host and its embedded sources remain read-only.
import {createServer,request as httpRequest} from 'node:http';
import {request as httpsRequest} from 'node:https';
import {readFile,stat} from 'node:fs/promises';
import {dirname,resolve,extname,sep} from 'node:path';
import {fileURLToPath} from 'node:url';
const hop=new Set(['connection','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','transfer-encoding','upgrade']);
const mime={'.html':'text/html; charset=utf-8','.js':'application/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.png':'image/png','.ico':'image/x-icon','.woff':'font/woff','.woff2':'font/woff2'};
export function createAdminServer({assetRoot,backendOrigin}){
 const assets=resolve(assetRoot);const backend=new URL(backendOrigin);
 if(!['http:','https:'].includes(backend.protocol)||backend.username||backend.password)throw new Error('A valid backend HTTP origin is required');
 return createServer(async(req,res)=>{
  try{
   const parsed=new URL(req.url,'http://localhost');
   if(['/api/','/x/','/x-assets/','/portal/'].some(prefix=>parsed.pathname.startsWith(prefix))||parsed.pathname==='/api.json'){
    const target=new URL(backend);target.pathname=parsed.pathname;target.search=parsed.search;
    const headers=Object.fromEntries(Object.entries(req.headers).filter(([name])=>!hop.has(name)&&name!=='host'));headers.host=target.host;
    const proxy=(target.protocol==='https:'?httpsRequest:httpRequest)(target,{method:req.method,headers},upstream=>{
     const forwarded=Object.fromEntries(Object.entries(upstream.headers).filter(([name])=>!hop.has(name)));
     res.writeHead(upstream.statusCode||502,forwarded);upstream.on('error',()=>res.destroy());upstream.pipe(res);
    });
    proxy.setTimeout(30000,()=>proxy.destroy(new Error('Backend timeout')));
    proxy.on('error',()=>{if(!res.headersSent){res.writeHead(503,{'Content-Type':'text/plain; charset=utf-8'});res.end('Administration backend is unavailable. Please retry.');}else res.destroy();});
    req.on('aborted',()=>proxy.destroy());res.on('close',()=>proxy.destroy());req.pipe(proxy);return;
   }
   if(req.method!=='GET'&&req.method!=='HEAD'){res.writeHead(405,{'Allow':'GET, HEAD'});res.end();return;}
   const assetPath=parsed.pathname.startsWith('/admin/')?parsed.pathname.slice('/admin'.length):parsed.pathname;
   const path=resolve(assets,'.'+decodeURIComponent(assetPath));
   if(path!==assets&&!path.startsWith(assets+sep)){res.writeHead(404);res.end();return;}
   let file=path;
   try{if(!(await stat(file)).isFile())file=resolve(assets,'index.html');}catch{if(extname(path)){res.writeHead(404);res.end();return;}file=resolve(assets,'index.html');}
   const body=await readFile(file);res.writeHead(200,{'Content-Type':mime[extname(file)]||'application/octet-stream','Content-Length':body.length,'Cache-Control':extname(file)==='.html'?'no-store':'public, max-age=3600'});res.end(req.method==='HEAD'?undefined:body);
  }catch{if(!res.headersSent)res.writeHead(503,{'Content-Type':'text/plain; charset=utf-8'});res.end('Administration assets are unavailable. Rebuild the delivery package.');}
 });
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 const server=createAdminServer({assetRoot:resolve(dirname(fileURLToPath(import.meta.url)),'public'),backendOrigin:process.env.LINAPRO_BACKEND_PROXY_TARGET||'http://127.0.0.1:6030'});
 server.listen(Number(process.env.ADMIN_PORT||6031),process.env.ADMIN_HOST||'127.0.0.1',()=>console.log('Admin gateway is ready.'));
}
