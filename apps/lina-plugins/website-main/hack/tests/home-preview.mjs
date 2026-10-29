// Temporary loopback preview of exactly the production source. No backend proxy.
import http from 'node:http';
import {readFile,stat} from 'node:fs/promises';
import {resolve,extname,sep} from 'node:path';
const root=resolve(import.meta.dirname,'../../frontend/public');
const mime={'.html':'text/html; charset=utf-8','.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml','.woff':'font/woff','.woff2':'font/woff2','.ttf':'font/ttf','.eot':'application/vnd.ms-fontobject'};
const banner='<aside style="position:fixed;bottom:0;left:0;right:0;z-index:99999;background:#fff3cd;color:#664d03;padding:10px 24px;text-align:center;border:1px solid #ffecb5">模拟预览：示例内容未写入后台；不支持购买、登录或支付。真实官网为6680。本页主图为已审计中性图标，非原横幅素材。</aside>';
http.createServer(async(req,res)=>{
 const url=new URL(req.url,'http://preview.localhost');res.setHeader('Cache-Control','no-store');
 res.setHeader('Content-Security-Policy',"default-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; form-action 'self'; frame-src 'none'; base-uri 'none'");
 if(url.pathname.startsWith('/x/')){const fixture=await stat(new URL('./support/home-preview-data.mjs',import.meta.url));const {previewResponse}=await import('./support/home-preview-data.mjs?revision='+fixture.mtimeMs);const result=previewResponse(url,req.method);res.writeHead(result.status,{'Content-Type':'application/json'});res.end(JSON.stringify({code:result.status===200?0:result.status,data:result.data,message:result.message}));return;}
 if(!['GET','HEAD'].includes(req.method)){res.writeHead(403);res.end('模拟预览禁止写入');return;}
 const file=resolve(root,'.'+(url.pathname==='/'?'/index.html':url.pathname));
 if(!file.startsWith(root+sep)){res.writeHead(404);res.end();return;}
 try{let body=await readFile(file);if(extname(file)==='.html')body=Buffer.from(body.toString().replace('</body>',banner+'</body>'));res.writeHead(200,{'Content-Type':mime[extname(file)]||'application/octet-stream'});res.end(req.method==='HEAD'?undefined:body);}catch{res.writeHead(404);res.end('模拟预览没有此资源');}
}).listen(16688,'127.0.0.1',()=>console.log('模拟预览 http://127.0.0.1:16688/index.html — same production source; no backend connection'));
