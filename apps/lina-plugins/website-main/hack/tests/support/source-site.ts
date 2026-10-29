import {test as base,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve,extname,sep} from 'node:path';
const root=resolve(import.meta.dirname,'../../../frontend/public');
const mime:Record<string,string>={'.json':'application/json','.html':'text/html; charset=utf-8','.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png','.woff':'font/woff','.woff2':'font/woff2','.ttf':'font/ttf','.eot':'application/vnd.ms-fontobject'};
// Run current local source bytes without installing a plugin or starting a service.
// Only public read operations (including evaluation/quote) reach the existing backend.
export const test=base.extend<{customerOrigin:string;apiOrigin:string;websiteGateway:boolean}>({customerOrigin:['',{option:true}],apiOrigin:['http://127.0.0.1:6030',{option:true}],websiteGateway:[false,{option:true}],page:async({page,customerOrigin,apiOrigin,websiteGateway},use)=>{
 if(!['http://127.0.0.1:6030','http://127.0.0.1:16630','http://127.0.0.1:16634'].includes(apiOrigin))throw new Error('Only existing local backends are allowed');
 await page.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url());
  if(['http://localhost:6032','http://localhost:16632'].includes(customerOrigin)&&url.origin===customerOrigin){
   const read=['GET','HEAD'].includes(request.method());
   const pricing=request.method()==='POST'&&/\/purchase-configuration\/(evaluate|quote)$/.test(url.pathname);
   if(read||pricing||request.method()==='POST'&&url.pathname==='/api/v1/auth/login')return route.continue();
   return route.abort('blockedbyclient');
  }
  if(['http://localhost:16689','http://127.0.0.1:16689'].includes(url.origin)&&['GET','HEAD'].includes(request.method()))return route.continue();
  if(websiteGateway&&['http://localhost:6680','http://127.0.0.1:6680'].includes(url.origin)&&['GET','HEAD'].includes(request.method()))return route.continue();
  if(url.origin==='https://i.iheart.com'&&['GET','HEAD'].includes(request.method()))return route.continue();
  if(url.origin!=='http://localhost:6680')return route.abort('blockedbyclient');
  if(url.pathname.startsWith('/x/')){
   const read=request.method()==='GET'||request.method()==='HEAD';
   const pricing=request.method()==='POST'&&/^\/x\/idc-core\/api\/v1\/public\/catalog\/products\/[^/]+\/purchase-configuration\/(evaluate|quote)$/.test(url.pathname);
   if(!read&&!pricing)throw new Error('Business write blocked: '+request.method()+' '+url.pathname);
   return route.fulfill({response:await route.fetch({url:apiOrigin+url.pathname+url.search})});
  }
  const file=resolve(root,'.'+(url.pathname==='/'?'/index.html':url.pathname));
  if(!file.startsWith(root+sep))return route.fulfill({status:404,body:''});
  let body:Buffer|string;
  try{body=await readFile(file);}catch{await route.fulfill({status:404,body:'Missing local source asset'});return;}
  if(extname(file)==='.html')body=body.toString().replace(/(<head[^>]*>)/i,'$1<script>window.__LINA_TEMPLATE__='+JSON.stringify({customerUrl:customerOrigin||'http://localhost:6032'})+'</script>');
  await route.fulfill({status:url.pathname==='/404.html'?404:200,contentType:mime[extname(file)]||'application/octet-stream',body});
 });
 try{await use(page);}finally{await page.unrouteAll({behavior:'wait'});}
}});
export {expect};
