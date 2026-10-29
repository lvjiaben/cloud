import type {Page} from '@playwright/test';
import {collectAssureCopy} from '../support/assure-copy.mjs';
export class AssurePage{
 constructor(readonly page:Page){}
 async open(){const response=await this.page.goto('/Assure.html');await this.ready();return response;}
 async ready(){await this.page.locator('[data-assure] main').waitFor();await this.page.waitForLoadState('networkidle');}
 copy(){return this.page.evaluate(collectAssureCopy);}
 async section(id:string){
  await this.page.locator('.et-hero-tab a[href="#assure-'+id+'"]').click();
  await this.page.waitForFunction(id=>{
   const top=document.getElementById('assure-'+id)!.getBoundingClientRect().top;
   const bar=document.querySelector('[data-assure] .et-container')!.getBoundingClientRect().height;
   return Math.abs(top-bar)<4||(scrollY+innerHeight>=document.documentElement.scrollHeight-2&&top<innerHeight);
  },id);
 }
 async menu(){
  const toggle=this.page.locator('.lina-menu-toggle');if(await toggle.isVisible())await toggle.click();
  const link=this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'支持与服务'});
  await link.focus();await link.press('Enter');
  await this.page.locator('.lina-open .lina-nav-panel').evaluate(async e=>{await Promise.all(e.getAnimations().map(a=>a.finished.catch(()=>{})));});
 }
 async enter(){await this.page.locator('.lina-open a').filter({has:this.page.getByText('服务保障',{exact:true})}).click();await this.ready();}
 async retry(){await this.page.locator('[data-brand-retry]').click();}
 screenshot(path:string){return this.page.screenshot({path,animations:'disabled'});}
}
