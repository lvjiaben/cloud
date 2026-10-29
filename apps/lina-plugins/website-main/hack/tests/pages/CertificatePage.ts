import type {Page} from '@playwright/test';
import {collectCertificateSource} from '../support/certificate-copy.mjs';
export class CertificatePage{
 constructor(readonly page:Page){}
 async ready(){await this.page.locator('[data-certificate] main').waitFor();await this.page.waitForLoadState('networkidle');}
 async open(){const response=await this.page.goto('/certificate.html');await this.ready();return response;}
 copy(){return this.page.evaluate(collectCertificateSource);}
 async screenshotSection(id:string,path:string){await this.page.locator('#certificate-'+id).scrollIntoViewIfNeeded();await this.page.screenshot({path,animations:'disabled'});}
 async menu(){
  await this.page.waitForFunction(()=>[...document.querySelectorAll('.horizontal-menu-navbar-menu>li>a')].some(e=>e.textContent?.includes('了解我们'))||document.querySelector('[data-brand-retry]'),{},{timeout:40000});
  const parent=this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'了解我们'});
  if(!await parent.count())await this.retry();
  const toggle=this.page.locator('.lina-menu-toggle');if(await toggle.isVisible())await toggle.click();await parent.focus();await parent.press('Enter');await this.page.locator('.lina-open .lina-nav-panel').evaluate(async e=>{await Promise.all(e.getAnimations().map(a=>a.finished.catch(()=>{})));});
 }
 async enter(){await this.page.locator('.lina-open a').filter({has:this.page.getByText('荣誉资质',{exact:true})}).click();await this.ready();}
 retry(){return this.page.locator('[data-brand-retry]').click();}
 preview(){return this.page.locator('.certificate-preview').click();}
 closePreview(){return this.page.getByRole('dialog').getByRole('button',{name:'关闭'}).click();}
}
