import type {Page} from '@playwright/test';
export class ContactPage{
 constructor(readonly page:Page){}
 open(){return this.page.goto('/contact.html');}
 card(key:string){return this.page.locator('#presales [data-contact-card="'+key+'"]');}
 async reveal(key:string){await this.card(key).evaluate(e=>e.scrollIntoView({behavior:'instant',block:'center'}));await this.card(key).locator('.currency-content-item-img').hover();}
 async address(){await this.page.locator('.et-hero-tabs a[href="#address"]').click();}
 async language(){await this.page.locator('[data-public-locale-switch]').click();}
 screenshot(path:string){return this.page.screenshot({path});}
}
