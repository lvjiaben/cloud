import {type Page} from '@playwright/test';
export class AboutRecoveryPage{
 constructor(readonly page:Page){}
 open(){return this.page.goto('/about.html');}
 body(){return this.page.locator('[data-about-company]');}
 async refresh(){await this.page.evaluate(()=>window.dispatchEvent(new Event('focus')));}
 async retry(){await this.page.locator('[data-brand-retry]').click();}
}
