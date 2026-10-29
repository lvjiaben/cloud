import type {Page} from '@host-tests/support/playwright';
import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
export class WebsitePurchaseEntryPage {
 constructor(readonly page:Page,readonly origin:string){}
 openSession(mode:'login'|'probe'){return this.page.goto(this.origin+'/auth/website-session?'+mode+'=1&returnTo=%2Fcheckout&websiteOrigin='+encodeURIComponent(this.origin));}
 retry(){return this.page.getByRole('button',{name:/重\s*试/}).click();}
 async currentScreenshot(step:string){const now=new Date();const dir=resolve('../../temp',now.toISOString().slice(0,10).replaceAll('-',''));await mkdir(dir,{recursive:true});return this.page.screenshot({path:resolve(dir,now.toISOString().slice(11,19).replaceAll(':','')+'-website-session-'+step+'.png'),fullPage:true});}
 openProduct(){return this.page.goto(this.origin+'/account/products/test-product/configure');}
 screenshot(step:string){return this.page.screenshot({path:resolve('../../temp/20261016/190000-website-entry-'+step+'.png'),fullPage:true});}
}
