import {Page} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve,extname,sep} from 'node:path';

// Offline original-only origin. No source scripts, form actions or API access.
export const originalOrigin='http://original.localhost';
export async function mountOriginalSite(page:Page){
 const root=resolve('D:/xft/Website/Zz_www.xiaobaiyun.cn');
 const types:Record<string,string>={'.html':'text/html; charset=utf-8','.css':'text/css','.svg':'image/svg+xml','.png':'image/png','.jpg':'image/jpeg','.jpeg':'image/jpeg','.gif':'image/gif','.woff':'font/woff','.woff2':'font/woff2','.ttf':'font/ttf','.eot':'application/vnd.ms-fontobject'};
 await page.route(originalOrigin+'/**',async route=>{
  const request=route.request(),url=new URL(request.url());
  if(request.method()!=='GET')return route.abort('blockedbyclient');
  let file:string;
  try{file=resolve(root,'.'+decodeURIComponent(url.pathname));}catch{return route.fulfill({status:404,body:''});}
  if(!file.startsWith(root+sep)||!types[extname(file).toLowerCase()])return route.fulfill({status:404,body:''});
  try{
   let body:Buffer|string=await readFile(file);
   if(extname(file).toLowerCase()==='.html')body=body.toString().replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi,'').replace(/\son\w+\s*=\s*(?:"[^"]*"|'[^']*')/gi,'');
   return route.fulfill({status:200,contentType:types[extname(file).toLowerCase()],headers:{'content-security-policy':"default-src 'self' data:; script-src 'none'; style-src 'self' 'unsafe-inline'; form-action 'none'; connect-src 'none'; frame-src 'none'; base-uri 'none'"},body});
  }catch{return route.fulfill({status:404,body:'Missing original local file'});}
 });
}
