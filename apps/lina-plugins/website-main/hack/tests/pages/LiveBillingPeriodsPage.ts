import {WebsitePage} from './SourceWebsitePage';
// Operates the installed website against its actual public pricing endpoints.
export class LiveBillingPeriodsPage extends WebsitePage {
 get terms(){return this.page.locator('[data-quote] button');}
 get total(){return this.page.locator('[data-total-amount]');}
 async choosePeriod(months:number){await this.terms.filter({hasText:months===12?'1年：':months+'月：'}).click();}
 async changeCpu(){const input=this.page.getByLabel('处理器核心数',{exact:true});const value=await input.locator('option').nth(1).getAttribute('value');await input.selectOption(value!);}
 async summary(){return this.page.locator('.allocation-footer').innerText();}
}
