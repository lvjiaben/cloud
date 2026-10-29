import {test as base,expect} from '@playwright/test';
import {readFile,readdir} from 'node:fs/promises';
import {resolve} from 'node:path';

// Exercise live catalogue and pricing without creating carts, orders or payments.
export const test=base.extend({page:async({page},use)=>{
 const errors:string[]=[];
 page.on('pageerror',error=>errors.push(error.message));
 if(process.env.WEBSITE_COMMERCE_CANDIDATE){
  const publicLina=resolve(import.meta.dirname,'../../../frontend/public/lina');
  for(const file of (await readdir(publicLina)).filter(name=>name.endsWith('.js')||name.endsWith('.css'))){
   const body=await readFile(resolve(publicLina,file),'utf8');
   await page.route('**/lina/'+file,route=>route.fulfill({contentType:file.endsWith('.css')?'text/css':'application/javascript',body}));
  }
 }
 await page.route('**/x/idc-core/**',async route=>{
  const request=route.request(),path=new URL(request.url()).pathname;
  if(['GET','HEAD'].includes(request.method())||request.method()==='POST'&&/\/purchase-configuration\/(evaluate|quote)$/.test(path))return route.fallback();
  errors.push('Unexpected business write: '+request.method()+' '+path);
  await route.abort('blockedbyclient');
 });
 await use(page);
 expect(errors).toEqual([]);
}});
export {expect};
