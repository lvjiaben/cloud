import type {Page} from '@playwright/test';
export class AffiliatePage {
 constructor(readonly page:Page) {}
 async open(hash='') {await this.page.goto('/aff.html'+hash);await this.page.locator('.aff-nav').waitFor();await this.page.waitForLoadState('networkidle');}
 nav(id:string) {return this.page.locator('.aff-nav a[href="#'+id+'"]');}
 async section(id:string) {await this.nav(id).click();await this.page.waitForFunction(id=>{const e=document.getElementById(id)!;return e.getBoundingClientRect().top<200&&e.getBoundingClientRect().bottom>200;},id);}
 async language() {const button=this.page.locator('[data-public-locale-switch]');if(!await button.isVisible())await this.page.locator('.lina-menu-toggle').click();await button.click();}
 async hover() {await this.page.locator('.aff-card').first().hover();}
 async screenshot(path:string) {await this.page.waitForFunction(()=>[...document.querySelectorAll('.aff-reveal')].filter(e=>{const r=e.getBoundingClientRect();return r.top<innerHeight&&r.bottom>0;}).every(e=>getComputedStyle(e).opacity==='1'));await this.page.screenshot({path,fullPage:false});}
}
