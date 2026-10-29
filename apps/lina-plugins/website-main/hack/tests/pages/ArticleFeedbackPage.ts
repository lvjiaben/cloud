import type {Locator,Page} from '@playwright/test';
import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';

// Clipboard promises are browser-local controls; no system clipboard is written.
export class ArticleFeedbackPage {
 readonly status:Locator;
 readonly body:Locator;
 readonly directory:Locator;
 readonly retry:Locator;
 constructor(readonly page:Page){
  this.status=page.locator('[data-status]');
  this.body=page.locator('[data-content-body]');
  this.directory=page.locator('[data-content-directory]');
  this.retry=page.locator('[data-retry]');
 }
 async controlClipboard(){
  await this.page.addInitScript(()=>{
   const calls:{value:string;resolve:()=>void;reject:(error:Error)=>void}[]=[];
   Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:(value:string)=>new Promise<void>((resolve,reject)=>calls.push({value,resolve,reject}))}});
   (window as any).__articleClipboard=calls;
  });
 }
 async open(path:string){await this.page.goto(path);}
 async copy(){await this.page.locator('[data-content-tool=copy]').click();}
 async clipboardValues(){return this.page.evaluate(()=>(window as any).__articleClipboard.map((call:any)=>call.value) as string[]);}
 async settleCopy(index:number,success:boolean){
  await this.page.evaluate(async({index,success})=>{
   const call=(window as any).__articleClipboard[index];
   if(!call)throw new Error('Clipboard request missing');
   if(success)call.resolve();else call.reject(new Error('Controlled clipboard denial'));
   // Drain the real async handler, without replacing application state helpers.
   await Promise.resolve();await Promise.resolve();
  },{index,success});
 }
 async retryContent(){await this.retry.click();}
 async heading(title:string){await this.directory.getByRole('link',{name:title,exact:true}).click();}
 async capture(name:string,showStatus=false){
  if(showStatus)await this.status.scrollIntoViewIfNeeded();
  const folder=resolve(import.meta.dirname,'../../../../../../temp',new Date().toISOString().slice(0,10).replaceAll('-',''));
  await mkdir(folder,{recursive:true});
  const path=resolve(folder,Date.now()+'-tc038-'+name+'.png');
  await this.page.screenshot({path,fullPage:false});return path;
 }
}
