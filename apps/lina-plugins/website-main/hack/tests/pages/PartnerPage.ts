import {type Page} from '@playwright/test';
export class PartnerPage {
 constructor(readonly page:Page){}
 async open(){return this.page.goto('/cooperation/partner.html');}
 async openHomePartnerMenu(){
  await this.page.goto('/');
  const toggle=this.page.locator('.lina-menu-toggle');
  if(await toggle.isVisible())await toggle.click();
  const menu=this.page.locator('.horizontal-menu-navbar-menu > li > a').filter({hasText:'生态合作'});
  await menu.focus();await menu.press('Enter');
  await this.page.locator('.lina-nav-panel[data-menu="生态合作"]').waitFor({state:'visible'});
 }
 partnerMenuLink(){return this.page.locator('.lina-nav-panel[data-menu="生态合作"] a.Submenu-item').filter({hasText:'合作伙伴'});}
 async ready(){
  await this.page.locator('[data-partner] .problem-content-item').last().waitFor();
  await this.page.locator('.horizontal-menu-navbar-logo [data-brand-name][aria-label]').first().waitFor({state:'attached'});
 }
 async advantage(index:number){await this.page.locator('.introduce-section-title').nth(index).click();}
 async section(index:number){
  await this.page.locator('.et-hero-tab a').nth(index).click();
  await this.page.evaluate(()=>new Promise<void>(resolve=>{
   let previous=scrollY,stableSince=performance.now();const started=stableSince;
   const settled=()=>{const now=performance.now();if(Math.abs(scrollY-previous)>0.5){previous=scrollY;stableSince=now;}
    if(now-stableSince>=150||now-started>=3000)resolve();else requestAnimationFrame(settled);
   };requestAnimationFrame(settled);
  }));
 }
 async contact(index:number){const opened=this.page.waitForEvent('popup');await this.page.locator('[data-partner] a[href="/contact.html"]').nth(index).click();return opened;}
 async scrollTable(selector:string,right:boolean){await this.page.locator(selector).evaluate((e,right)=>e.scrollLeft=right?e.scrollWidth:0,right);}
 async screenshot(path:string){await this.page.screenshot({path,fullPage:true});}
}
