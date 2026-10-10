import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import type {Page} from '@host-tests/fixtures/auth';

export async function installDelivery(page:Page,surfaceKey:string,entry:string,responses:Record<string,unknown>){
 const pluginId='idc-zjmfmigrate';
 const assets=resolve(import.meta.dirname,'../../../frontend/dist');
 await page.route(`**/delivery-${pluginId}/**`,async(route)=>{
  const suffix=new URL(route.request().url()).pathname.split(`/delivery-${pluginId}/`)[1];
  if(!suffix||suffix.includes('..'))return route.abort();
  const type=suffix.endsWith('.js')?'application/javascript':suffix.endsWith('.css')?'text/css':'text/html';
  await route.fulfill({status:200,contentType:type,body:await readFile(resolve(assets,suffix))});
 });
 await page.addInitScript(({pluginId,surfaceKey,responses}:any)=>{
  (window as any).__deliveryRequests=[];
  window.postMessage=((message:any)=>{
   if(message?.direction!=='plugin-to-host')return;
   const send=(data:any)=>window.dispatchEvent(new MessageEvent('message',{source:window,data:{protocol:'linapro.plugin-page-bridge',version:'1.0',direction:'host-to-plugin',pluginId,surfaceKey,...data}}));
   if(message.method==='ready')queueMicrotask(()=>send({capability:'host.context.v1',payload:{locale:'zh-CN',theme:{mode:'light',cssVariables:{}}}}));
   if(message.method==='request'){(window as any).__deliveryRequests.push(message);queueMicrotask(()=>send({requestId:message.requestId,capability:message.capability,ok:message.payload.path in responses,result:responses[message.payload.path]}));}
  }) as any;
 },{pluginId,surfaceKey,responses});
 await page.goto(`/delivery-${pluginId}/${entry}`);
}
